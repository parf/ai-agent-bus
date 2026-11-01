package bus

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The shared state vectors (testdata/client-conformance/{kv,locks,bigfile}.json)
// through this client's own API, against the disposable real daemon. A step's
// `expect` is a value to compare, `{"refused":{"status":N}}` for a daemon
// refusal, or `{"refused":true}` for a refusal from the client or the daemon.

type vectorCase struct {
	ID    string           `json:"id"`
	Fresh bool             `json:"fresh"`
	Setup []string         `json:"setup"`
	Steps []map[string]any `json:"steps"`
	// identity.json and refusals.json: a fake daemon's raw answers and the client's reading of them.
	Identity string `json:"identity"` // the raw /identity body, or "current" / "old" in refusals.json
	Daemon   struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
		Allow  string `json:"allow"`
	} `json:"daemon"`
	Call   map[string]any `json:"call"` // refusals.json: an operation the client must refuse before sending anything
	Expect map[string]any `json:"expect"`
}

func stateVectors(t *testing.T, name string) []vectorCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "client-conformance", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []vectorCase
	if err = json.Unmarshal(raw, &rows); err != nil || len(rows) == 0 {
		t.Fatalf("shared vector file %s must hold concrete cases: %v", name, err)
	}
	return rows
}

func str(m map[string]any, key string) string { s, _ := m[key].(string); return s }

func expectation(step map[string]any) (map[string]any, bool) {
	e, _ := step["expect"].(map[string]any)
	return e, e != nil
}

// refusal checks err against a step's expected refusal, and reports whether the step expected one.
func refusal(t *testing.T, where string, step map[string]any, err error) bool {
	t.Helper()
	e, has := expectation(step)
	want, refused := e["refused"]
	if !has || !refused {
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", where, err)
		}
		return false
	}
	if err == nil {
		t.Fatalf("%s: expected a refusal %v, got success", where, want)
	}
	if spec, ok := want.(map[string]any); ok {
		var be *BusError
		if !errors.As(err, &be) {
			t.Fatalf("%s: expected a daemon refusal %v, got %v", where, spec, err)
		}
		if int(spec["status"].(float64)) != be.Status {
			t.Fatalf("%s: status %d, want %v (%s)", where, be.Status, spec["status"], be.Message)
		}
		if kind, ok := spec["message_has"].(string); ok && !strings.Contains(be.Message, kind) {
			t.Fatalf("%s: refusal text %q lacks %q", where, be.Message, kind)
		}
	}
	return true
}

func stepValue(step map[string]any) []byte {
	if r, ok := step["repeat"].(map[string]any); ok {
		return []byte(strings.Repeat(r["text"].(string), int(r["count"].(float64))))
	}
	if b, ok := step["value_b64"].(string); ok {
		raw, _ := base64.StdEncoding.DecodeString(b)
		return raw
	}
	return []byte(str(step, "value"))
}

func TestSharedKVVectors(t *testing.T) {
	addr, tokens := realFixture(t)
	shared := fixtureBus(t, addr, tokens["owner@fixture"], "owner@fixture")
	for _, c := range stateVectors(t, "kv") {
		t.Run(c.ID, func(t *testing.T) {
			owner := shared
			if c.Fresh { // a case that fills a store gets a daemon of its own
				a, tk := realFixture(t)
				owner = fixtureBus(t, a, tk["owner@fixture"], "owner@fixture")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			kv := owner.Record("jobs@fixture").KV
			for i, step := range c.Steps {
				where := c.ID + " step " + strconv.Itoa(i) + " " + str(step, "op") + " " + str(step, "name")
				kind := KVKind(str(step, "kind"))
				exp, _ := expectation(step)
				switch str(step, "op") {
				case "set":
					mode := KVSet
					if h := str(step, "how"); h != "" {
						mode = KVMode(h)
					}
					var value any = stepValue(step)
					switch kind {
					case KVInt:
						n, _ := strconv.ParseInt(str(step, "value"), 10, 64)
						value = n
					case KVJSON:
						var object map[string]any
						_ = json.Unmarshal(stepValue(step), &object)
						value = object
					}
					refusal(t, where, step, kv.Set(ctx, kind, str(step, "name"), value, mode))
				case "get":
					got, err := kv.Get(ctx, kind, str(step, "name"))
					if refusal(t, where, step, err) {
						continue
					}
					switch {
					case exp["value_b64"] != nil:
						if base64.StdEncoding.EncodeToString(got.Bytes) != exp["value_b64"] {
							t.Fatalf("%s: bytes %x", where, got.Bytes)
						}
					case exp["length"] != nil:
						if float64(len(got.Bytes)) != exp["length"] {
							t.Fatalf("%s: length %d", where, len(got.Bytes))
						}
					case exp["json"] != nil:
						var a, b any
						_ = json.Unmarshal(got.JSON, &a)
						raw, _ := json.Marshal(exp["json"])
						_ = json.Unmarshal(raw, &b)
						if string(mustJSON(a)) != string(mustJSON(b)) {
							t.Fatalf("%s: json %s", where, got.JSON)
						}
					case kind == KVInt:
						if strconv.FormatInt(got.Int, 10) != exp["value"] {
							t.Fatalf("%s: int %d, want %v", where, got.Int, exp["value"])
						}
					default:
						if string(got.Bytes) != exp["value"] {
							t.Fatalf("%s: %q, want %v", where, got.Bytes, exp["value"])
						}
					}
				case "fill":
					for n := 0; n < int(step["count"].(float64)); n++ {
						if err := kv.Set(ctx, kind, str(step, "prefix")+strconv.Itoa(n), "v", KVAdd); err != nil {
							t.Fatalf("%s: name %d: %v", where, n, err)
						}
					}
				case "delete":
					deleted, err := kv.Delete(ctx, kind, str(step, "name"))
					if refusal(t, where, step, err) {
						continue
					}
					if deleted != exp["deleted"] {
						t.Fatalf("%s: deleted %v, want %v", where, deleted, exp["deleted"])
					}
				case "inc":
					by, _ := strconv.ParseInt(str(step, "by"), 10, 64)
					got, err := kv.Increment(ctx, str(step, "name"), by)
					if refusal(t, where, step, err) {
						continue
					}
					if exp != nil && exp["value"] != nil && strconv.FormatInt(got, 10) != exp["value"] {
						t.Fatalf("%s: inc gave %d, want %v", where, got, exp["value"])
					}
				case "json":
					var ops []KVOp
					raw, _ := json.Marshal(step["ops"])
					_ = json.Unmarshal(raw, &ops)
					results, err := kv.JSON(ctx, str(step, "name"), ops)
					if refusal(t, where, step, err) {
						continue
					}
					if want, ok := exp["changed"].([]any); ok {
						for j, w := range want {
							if results[j].Changed != w {
								t.Fatalf("%s: op %d changed %v", where, j, results[j].Changed)
							}
						}
					}
				default:
					t.Fatalf("%s: unknown vector op", where)
				}
			}
		})
	}
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

func TestSharedLockVectors(t *testing.T) {
	for _, c := range stateVectors(t, "locks") {
		t.Run(c.ID, func(t *testing.T) {
			addr, tokens := realFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			buses := map[string]*Bus{
				"owner":  fixtureBus(t, addr, tokens["owner@fixture"], "owner@fixture"),
				"worker": fixtureBus(t, addr, tokens["#worker@fixture"], "#worker@fixture"),
			}
			for _, setup := range c.Setup {
				if setup != "worker-maintains-jobs" {
					t.Fatalf("unknown vector setup %q", setup)
				}
				if _, err := buses["owner"].Record("jobs@fixture").Manage(ctx, map[string]any{"maintainers": []string{"#worker@fixture"}}); err != nil {
					t.Fatal(err)
				}
			}
			for i, step := range c.Steps {
				where := c.ID + " step " + strconv.Itoa(i) + " " + str(step, "as") + " " + str(step, "op") + " " + str(step, "name")
				locks := buses[str(step, "as")].Record("jobs@fixture").Locks
				seconds, _ := step["ttl_s"].(float64)
				ttl := time.Duration(int64(seconds)) * time.Second
				exp, _ := expectation(step)
				var err error
				switch str(step, "op") {
				case "try":
					_, err = locks.TryAcquire(ctx, str(step, "name"), ttl)
				case "acquire":
					_, err = locks.Acquire(ctx, str(step, "name"), WithTTL(ttl), WithWait(time.Duration(int64(step["wait_s"].(float64)))*time.Second))
				case "extend":
					err = locks.Extend(ctx, str(step, "name"), ttl)
				case "release":
					err = locks.Release(ctx, str(step, "name"))
				case "force":
					var displaced string
					displaced, err = locks.ForceRelease(ctx, str(step, "name"))
					if err == nil && displaced != exp["displaced"] {
						t.Fatalf("%s: displaced %q, want %v", where, displaced, exp["displaced"])
					}
				case "holders":
					var raw json.RawMessage
					raw, err = locks.Holders(ctx)
					if err == nil {
						var got struct {
							Locks map[string]struct{ Holder string } `json:"locks"`
						}
						if e := json.Unmarshal(raw, &got); e != nil {
							t.Fatalf("%s: %v: %s", where, e, raw)
						}
						want := exp["holders"].(map[string]any)
						if len(got.Locks) != len(want) {
							t.Fatalf("%s: holders %s, want %v", where, raw, want)
						}
						for name, holder := range want {
							if got.Locks[name].Holder != holder {
								t.Fatalf("%s: holder of %s is %q, want %v", where, name, got.Locks[name].Holder, holder)
							}
						}
					}
				default:
					t.Fatalf("%s: unknown vector op", where)
				}
				refusal(t, where, step, err)
			}
		})
	}
}

func TestSharedBigfileVectors(t *testing.T) {
	for _, c := range stateVectors(t, "bigfile") {
		t.Run(c.ID, func(t *testing.T) {
			addr, tokens := realFixture(t)
			owner := fixtureBus(t, addr, tokens["owner@fixture"], "owner@fixture")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			big := owner.Record("jobs@fixture").Bigfile
			for i, step := range c.Steps {
				where := c.ID + " step " + strconv.Itoa(i) + " " + str(step, "op") + " " + str(step, "name")
				exp, _ := expectation(step)
				switch str(step, "op") {
				case "put":
					value := stepValue(step)
					seconds, _ := step["ttl_s"].(float64)
					opts := []BigfileOption{BigfileTTL(time.Duration(int64(seconds)) * time.Second)}
					if step["memory"] == true {
						opts = append(opts, BigfileMemory())
					}
					if sum := str(step, "sha256"); sum != "" {
						opts = append(opts, BigfileSHA256(sum))
					}
					info, err := big.Put(ctx, str(step, "name"), bytes.NewReader(value), int64(len(value)), opts...)
					if refusal(t, where, step, err) {
						continue
					}
					if exp["size"] != nil && float64(info.Size) != exp["size"] || exp["sha256"] != nil && info.SHA256 != exp["sha256"] ||
						exp["held"] != nil && info.Held != exp["held"] || exp["replaced"] != nil && info.Replaced != exp["replaced"] {
						t.Fatalf("%s: answered %+v, want %v", where, info, exp)
					}
				case "get":
					var out bytes.Buffer
					info, err := big.Get(ctx, str(step, "name"), &out)
					if refusal(t, where, step, err) {
						continue
					}
					if out.String() != exp["value"] || exp["sha256"] != nil && info.SHA256 != exp["sha256"] {
						t.Fatalf("%s: got %q %s, want %v", where, out.String(), info.SHA256, exp)
					}
				case "list":
					keys, err := big.List(ctx)
					refusal(t, where, step, err)
					names := []string{}
					for _, k := range keys {
						names = append(names, k.Name)
					}
					if string(mustJSON(names)) != string(mustJSON(exp["names"])) {
						t.Fatalf("%s: names %v, want %v", where, names, exp["names"])
					}
				case "delete":
					deleted, err := big.Delete(ctx, str(step, "name"))
					if refusal(t, where, step, err) {
						continue
					}
					if deleted != exp["deleted"] {
						t.Fatalf("%s: deleted %v, want %v", where, deleted, exp["deleted"])
					}
				default:
					t.Fatalf("%s: unknown vector op", where)
				}
			}
		})
	}
}

// protocolServer answers /identity with a vector's raw body, /ls with the refusal when given, and any other path with a status answer.
func protocolServer(t *testing.T, identity string, requests *[]string, refuse func(w http.ResponseWriter)) *Bus {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*requests = append(*requests, r.URL.Path)
		switch {
		case r.URL.Path == "/identity":
			fmt.Fprint(w, identity)
		case refuse != nil && r.URL.Path == "/ls":
			refuse(w)
		default:
			fmt.Fprint(w, `{"you":"owner@fixture"}`)
		}
	}))
	t.Cleanup(server.Close)
	b, err := Dial(Options{Addr: server.URL, Token: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	return b
}

func TestSharedIdentityVectors(t *testing.T) {
	for _, c := range stateVectors(t, "identity") {
		t.Run(c.ID, func(t *testing.T) {
			var requests []string
			var warnings bytes.Buffer
			previous := log.Writer()
			log.SetOutput(&warnings)
			defer log.SetOutput(previous)
			b := protocolServer(t, c.Identity, &requests, nil)
			_, err := b.Status(context.Background())
			if fatal, _ := c.Expect["fatal"].(string); fatal != "" {
				if err == nil || !strings.Contains(err.Error(), fatal) {
					t.Fatalf("want a fatal error naming %q, got %v", fatal, err)
				}
				if len(requests) != 1 {
					t.Fatalf("an application request followed a fatal handshake: %v", requests)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if requests[0] != "/identity" {
				t.Fatalf("identity was not checked first: %v", requests)
			}
			warns, _ := c.Expect["warns"].(string)
			if (warns != "" && !strings.Contains(warnings.String(), warns)) || (warns == "" && warnings.Len() != 0) {
				t.Fatalf("warning %q, want %q", warnings.String(), warns)
			}
		})
	}
}

func TestSharedRefusalVectors(t *testing.T) {
	for _, c := range stateVectors(t, "refusals") {
		t.Run(c.ID, func(t *testing.T) {
			var requests []string
			previous := log.Writer()
			log.SetOutput(io.Discard) // an old daemon's one-time warning is not under test here
			defer log.SetOutput(previous)
			identity := `{"protocol":1,"protocol_min":1}`
			if c.Identity == "old" {
				identity = `{}`
			}
			b := protocolServer(t, identity, &requests, func(w http.ResponseWriter) {
				if c.Daemon.Allow != "" {
					w.Header().Set("Allow", c.Daemon.Allow)
				}
				w.WriteHeader(c.Daemon.Status)
				fmt.Fprint(w, c.Daemon.Body)
			})
			if c.Call != nil {
				clientRefusal(t, b, c, &requests)
				return
			}
			_, err := b.Records(context.Background())
			var be *BusError
			if !errors.As(err, &be) {
				t.Fatalf("refusal lost: %v", err)
			}
			want := c.Expect
			if be.Status != int(want["status"].(float64)) || be.Kind != want["kind"] || be.Message != want["message"] {
				t.Fatalf("got %d %q %q, want %v", be.Status, be.Kind, be.Message, want)
			}
		})
	}
}

// clientRefusal runs a vector's call, which the client must refuse itself: the daemon is never asked for it.
func clientRefusal(t *testing.T, b *Bus, c vectorCase, requests *[]string) {
	t.Helper()
	locks := b.Record("jobs@fixture").Locks
	seconds, _ := c.Call["ttl_s"].(float64)
	ttl := time.Duration(int64(seconds)) * time.Second
	var err error
	switch str(c.Call, "op") {
	case "acquire":
		wait, _ := c.Call["wait_s"].(float64)
		_, err = locks.Acquire(context.Background(), str(c.Call, "name"), WithTTL(ttl), WithWait(time.Duration(int64(wait))*time.Second))
	case "try":
		_, err = locks.TryAcquire(context.Background(), str(c.Call, "name"), ttl)
	case "extend":
		err = locks.Extend(context.Background(), str(c.Call, "name"), ttl)
	default:
		t.Fatalf("unknown vector call %v", c.Call)
	}
	if err == nil {
		t.Fatalf("the client sent %v and was answered", c.Call)
	}
	var be *BusError
	if errors.As(err, &be) {
		t.Fatalf("the daemon refused %v, the client should have: %v", c.Call, err)
	}
	if slices.Contains(*requests, str(c.Expect, "not_requested")) {
		t.Fatalf("the daemon saw %s: %v", str(c.Expect, "not_requested"), *requests)
	}
}

package bus

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func hexOf(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// bigServer answers /status and runs h for the big paths.
func bigServer(t *testing.T, h http.HandlerFunc) *Bus {
	return fakeBus(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			statusReply(w)
			return
		}
		h(w, r)
	})
}

func TestBigPutStreamsTheBytesWithTheDocumentedQueryAndChecksTheAnswer(t *testing.T) {
	value := bytes.Repeat([]byte("abc"), 1000)
	var got []byte
	var query, ctype, token, sha string
	var length int64
	b := bigServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" || r.URL.Path != "/big" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		query, ctype, token, sha, length = r.URL.RawQuery, r.Header.Get("Content-Type"), r.Header.Get("X-Agent-Bus-Token"), r.Header.Get("X-Big-Sha256"), r.ContentLength
		got, _ = io.ReadAll(r.Body)
		fmt.Fprintf(w, `{"record":"#r","name":"k","size":%d,"sha256":%q,"held":"memory","expires":"2026-11-02T00:00:00Z","replaced":true}`, len(got), hexOf(got))
	})
	info, e := b.Record("#r").Big.Put(context.Background(), "k", bytes.NewReader(value), int64(len(value)), BigTTL(90*time.Minute), BigMemory(), BigSHA256(strings.ToUpper(hexOf(value))))
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(got, value) || length != int64(len(value)) || ctype != "application/octet-stream" || token != "fake-token" || sha != hexOf(value) {
		t.Fatal(len(got), length, ctype, token, sha)
	}
	for _, want := range []string{"record=%23r", "name=k", "ttl=5400", "memory=true"} {
		if !strings.Contains(query, want) {
			t.Fatalf("query %q lacks %q", query, want)
		}
	}
	if !info.Replaced || info.Held != "memory" || info.Size != int64(len(value)) || info.Expires.IsZero() {
		t.Fatalf("%+v", info)
	}
}

func TestBigPutRefusesOutOfRangeSizeAndTTLBeforeAnyRequest(t *testing.T) {
	var calls atomic.Int32
	b := bigServer(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) })
	g := b.Record("#r").Big
	for name, run := range map[string]func() error{
		"zero bytes": func() error { _, e := g.Put(context.Background(), "k", strings.NewReader(""), 0); return e },
		"over 32 MiB": func() error {
			_, e := g.Put(context.Background(), "k", strings.NewReader("x"), BigMaxBytes+1)
			return e
		},
		"ttl under 1s": func() error {
			_, e := g.Put(context.Background(), "k", strings.NewReader("x"), 1, BigTTL(time.Millisecond))
			return e
		},
		"ttl over 30d": func() error {
			_, e := g.Put(context.Background(), "k", strings.NewReader("x"), 1, BigTTL(31*24*time.Hour))
			return e
		},
		"ttl negative": func() error {
			_, e := g.Put(context.Background(), "k", strings.NewReader("x"), 1, BigTTL(-time.Second))
			return e
		},
	} {
		if e := run(); e == nil {
			t.Errorf("%s was not refused", name)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("%d requests were sent", calls.Load())
	}
}

func TestBigPutAnswerWithAnotherDigestIsAHashMismatch(t *testing.T) {
	b := bigServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		fmt.Fprintf(w, `{"record":"#r","name":"k","size":3,"sha256":%q,"held":"memory"}`, strings.Repeat("0", 64))
	})
	_, e := b.Record("#r").Big.Put(context.Background(), "k", strings.NewReader("abc"), 3)
	var m *BigHashMismatch
	if !errors.As(e, &m) || m.Want != hexOf([]byte("abc")) {
		t.Fatal(e)
	}
}

func TestBigGetStreamsAndVerifiesTheDigestAndLength(t *testing.T) {
	value := []byte("the big value")
	serve := func(sum string, cut bool) *Bus {
		return bigServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" || r.URL.Path != "/big" || r.URL.Query().Get("record") != "#r" || r.URL.Query().Get("name") != "k" {
				t.Errorf("%s %s", r.Method, r.URL.String())
			}
			w.Header().Set("X-Big-Sha256", sum)
			w.Header().Set("X-Big-Held", "file")
			w.Header().Set("X-Big-Expires", "2026-11-02T00:00:00Z")
			w.Header().Set("Content-Length", fmt.Sprint(len(value)))
			if cut {
				w.Write(value[:4])
				return
			}
			w.Write(value)
		})
	}
	var out bytes.Buffer
	info, e := serve(hexOf(value), false).Record("#r").Big.Get(context.Background(), "k", &out)
	if e != nil || out.String() != string(value) || info.Held != "file" || info.Size != int64(len(value)) || info.SHA256 != hexOf(value) || info.Expires.IsZero() {
		t.Fatal(e, out.String(), info)
	}
	out.Reset()
	_, e = serve(strings.Repeat("f", 64), false).Record("#r").Big.Get(context.Background(), "k", &out)
	var m *BigHashMismatch
	if !errors.As(e, &m) {
		t.Fatal("a wrong digest was accepted:", e)
	}
	_, e = serve(hexOf(value), true).Record("#r").Big.Get(context.Background(), "k", io.Discard)
	if e == nil || errors.As(e, &m) {
		t.Fatal("a body cut short was not reported as a transport failure:", e)
	}
	_, e = serve("", false).Record("#r").Big.Get(context.Background(), "k", io.Discard)
	if e == nil {
		t.Fatal("a get without a digest was accepted")
	}
}

func TestBigRefusalsKeepTheDaemonsKindAndStatus(t *testing.T) {
	for status, kind := range map[int]string{403: "acl", 404: "unknown", 413: "malformed", 400: "malformed"} {
		b := bigServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			fmt.Fprintf(w, `{"error":%q}`, kind+": refused")
		})
		_, e := b.Record("#r").Big.Get(context.Background(), "k", io.Discard)
		var be *BusError
		if !errors.As(e, &be) || be.Status != status || !strings.HasPrefix(be.Message, kind) {
			t.Fatalf("%d: %v", status, e)
		}
	}
}

func TestBigListAndDelete(t *testing.T) {
	b := bigServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/big/list":
			if r.Method != "GET" || r.URL.Query().Get("record") != "#r" {
				t.Errorf("%s %s", r.Method, r.URL.String())
			}
			io.WriteString(w, `{"record":"#r","keys":[{"name":"a","size":1,"sha256":"aa","held":"memory","expires":"2026-11-02T00:00:00Z"},{"name":"b","size":2,"sha256":"bb","held":"file","expires":"2026-11-03T00:00:00Z"}]}`)
		case "/big/delete":
			body, _ := io.ReadAll(r.Body)
			if r.Method != "POST" || !strings.Contains(string(body), `"name":"a"`) || !strings.Contains(string(body), `"record":"#r"`) {
				t.Errorf("%s %s", r.Method, body)
			}
			io.WriteString(w, `{"record":"#r","name":"a","deleted":true}`)
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
	})
	keys, e := b.Record("#r").Big.List(context.Background())
	if e != nil || len(keys) != 2 || keys[0].Name != "a" || keys[1].Held != "file" || keys[1].Size != 2 {
		t.Fatal(e, keys)
	}
	deleted, e := b.Record("#r").Big.Delete(context.Background(), "a")
	if e != nil || !deleted {
		t.Fatal(e, deleted)
	}
}

func TestBigUnavailableStoreIsABusErrorCarryingTheCause(t *testing.T) {
	const cause = "big-key store unavailable: the directory cannot be opened"
	b := bigServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		fmt.Fprintf(w, `{"error":%q}`, cause)
	})
	g := b.Record("#r").Big
	ctx := context.Background()
	_, put := g.Put(ctx, "k", strings.NewReader("x"), 1)
	_, get := g.Get(ctx, "k", io.Discard)
	_, list := g.List(ctx)
	_, del := g.Delete(ctx, "k")
	for name, e := range map[string]error{"put": put, "get": get, "list": list, "delete": del} {
		var be *BusError
		if !errors.As(e, &be) || be.Status != 503 || be.Message != cause {
			t.Errorf("%s: %v", name, e)
		}
	}
}

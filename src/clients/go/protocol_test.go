package bus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProtocolHandshake(t *testing.T) {
	cases := []struct {
		name, identity, want string
		old                  bool
	}{
		{"current", `{"protocol":1,"protocol_min":1}`, "", false},
		{"future compatible", `{"protocol":2,"protocol_min":1}`, "", false},
		{"absent", `{}`, "", true},
		{"old", `{"protocol":0,"protocol_min":0}`, "", true},
		{"too new", `{"protocol":2,"protocol_min":2}`, "daemon too new", false},
		{"fraction", `{"protocol":1.5}`, "protocol", false},
		{"negative", `{"protocol":-1}`, "protocol", false},
		{"null", `{"protocol":null}`, "protocol", false},
		{"minimum", `{"protocol":1,"protocol_min":2}`, "protocol_min", false},
		{"bool", `{"protocol":true}`, "protocol", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var requests []string
			var warnings bytes.Buffer
			oldOutput := log.Writer()
			log.SetOutput(&warnings)
			defer log.SetOutput(oldOutput)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.URL.Path)
				if r.URL.Path == "/identity" {
					fmt.Fprint(w, c.identity)
				} else {
					fmt.Fprint(w, `{"you":"owner@fixture"}`)
				}
			}))
			defer server.Close()
			b, err := Dial(Options{Addr: server.URL, Token: "fixture"})
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			_, err = b.Status(context.Background())
			if c.want != "" {
				if err == nil || !strings.Contains(err.Error(), c.want) {
					t.Fatalf("fatal protocol handshake: got %v, want %s", err, c.want)
				}
				if len(requests) != 1 {
					t.Fatalf("application request after fatal handshake: %v", requests)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(requests) < 2 || requests[0] != "/identity" {
				t.Fatalf("identity was not checked first: %v", requests)
			}
			if strings.Contains(warnings.String(), "daemon too old") != c.old {
				t.Fatalf("warning=%q old=%v", warnings.String(), c.old)
			}
		})
	}
}
func TestProtocolRefusalKinds(t *testing.T) {
	for _, old := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/identity" {
				if old {
					fmt.Fprint(w, `{}`)
				} else {
					fmt.Fprint(w, `{"protocol":1,"protocol_min":1}`)
				}
				return
			}
			if r.URL.Path == "/status" {
				fmt.Fprint(w, `{"you":"owner@fixture"}`)
				return
			}
			code, kind := 404, "no_route"
			if r.URL.Path == "/missing" {
				kind = "not_found"
			}
			if r.URL.Path == "/down" {
				code, kind = 503, "unavailable"
			}
			if r.URL.Path == "/panic" {
				code, kind = 500, ""
			}
			w.WriteHeader(code)
			fmt.Fprintf(w, `{"error":"cause","kind":%q}`, kind)
		}))
		b, _ := Dial(Options{Addr: server.URL, Token: "fixture"})
		for _, path := range []string{"/removed", "/missing", "/down", "/panic"} {
			var result any
			err := b.json(context.Background(), "GET", path, nil, nil, &result)
			var refusal *BusError
			if !errors.As(err, &refusal) {
				t.Fatalf("refusal lost: %v", err)
			}
			if path == "/removed" {
				if refusal.Kind != "no_route" || errors.Is(err, ErrKVNotFound) {
					t.Fatalf("no_route treated as missing data: %+v", refusal)
				}
				if old && !strings.Contains(err.Error(), "daemon too old for GET /removed") {
					t.Fatal(err)
				}
			}
			if path == "/missing" && (!errors.Is(err, ErrKVNotFound) || refusal.Kind != "not_found") {
				t.Fatal(err)
			}
			if path == "/down" && (refusal.Kind != "unavailable" || !strings.Contains(err.Error(), "cause")) {
				t.Fatal(err)
			}
			if path == "/panic" && (refusal.Kind != "" || refusal.Message != "internal daemon error") {
				t.Fatal(err)
			}
		}
		b.Close()
		server.Close()
	}
}
func TestRealDaemonProtocolAndRemovedRoute(t *testing.T) {
	addr, tokens := realFixture(t)
	b := fixtureBus(t, addr, tokens["owner@fixture"], "owner@fixture")
	var warnings bytes.Buffer
	out := log.Writer()
	log.SetOutput(&warnings)
	defer log.SetOutput(out)
	data, err := b.Identity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"protocol":1`)) || !bytes.Contains(data, []byte(`"protocol_min":1`)) {
		t.Fatalf("identity=%s", data)
	}
	var result any
	err = b.json(context.Background(), "POST", "/subscriber/remove", nil, map[string]string{"name": "x"}, &result)
	var refusal *BusError
	if !errors.As(err, &refusal) || refusal.Status != 404 || refusal.Kind != "no_route" {
		t.Fatalf("removed route lost kind: %v", err)
	}
	if warnings.Len() != 0 {
		t.Fatalf("current daemon warned: %s", &warnings)
	}
}

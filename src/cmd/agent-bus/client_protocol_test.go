package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// Older fake-daemon tests do not own the handshake; keep it out of their
// route/body assertions. Tests below and real-daemon tests use NewServer.
func protocolTestServer(h http.Handler) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/identity" {
			io.WriteString(w, `{"protocol":1,"protocol_min":1}`)
			return
		}
		h.ServeHTTP(w, r)
	}))
}
func cliProtocolTransport(t *testing.T, server *httptest.Server) {
	old := transport
	transport = func() (*http.Client, string) { return server.Client(), server.URL }
	t.Cleanup(func() { transport = old })
	t.Setenv("AGENT_BUS_TOKEN", "fixture")
}
func TestCLIProtocolHandshake(t *testing.T) {
	for _, c := range []struct{ identity, want string }{
		{`{"protocol":2,"protocol_min":2}`, "daemon too new"}, {`{"protocol":-1}`, "protocol"}, {`{"protocol":null}`, "protocol"}, {`{"protocol":true}`, "protocol"}, {`{"protocol":1.5}`, "protocol"}, {`{"protocol":1,"protocol_min":2}`, "protocol_min"}, {`{"protocol":1,"protocol_min":1}`, ""}, {`{"protocol":2,"protocol_min":1}`, "unsupported"}, {`{}`, "protocol"}, {`{"protocol":0,"protocol_min":0}`, "unsupported"}, {`{"protocol":1,"protocol_min":0}`, "unsupported"},
	} {
		t.Run(c.identity, func(t *testing.T) {
			var paths []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				if r.URL.Path == "/identity" {
					io.WriteString(w, c.identity)
				} else {
					io.WriteString(w, `{"you":"owner@test"}`)
				}
			}))
			defer server.Close()
			cliProtocolTransport(t, server)
			read, write, pipeErr := os.Pipe()
			if pipeErr != nil {
				t.Fatal(pipeErr)
			}
			defer read.Close()
			previous := os.Stderr
			os.Stderr = write
			_, _, err := callCtx(context.Background(), "GET", "/status", nil, nil)
			if c.want == "" {
				if _, _, again := callCtx(context.Background(), "GET", "/status", nil, nil); again != nil {
					t.Fatal(again)
				}
			}
			write.Close()
			os.Stderr = previous
			warning, _ := io.ReadAll(read)
			if len(warning) != 0 {
				t.Fatalf("warning=%q identity=%s", warning, c.identity)
			}
			if c.want != "" {
				if err == nil || !strings.Contains(err.Error(), c.want) {
					t.Fatalf("fatal handshake got %v want %s", err, c.want)
				}
				if len(paths) != 1 {
					t.Fatalf("application ran after fatal handshake: %v", paths)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			probes := 0
			for _, path := range paths {
				if path == "/identity" {
					probes++
				}
			}
			if probes != 1 {
				t.Fatalf("one process probed identity %d times for paths %v", probes, paths)
			}
			if paths[0] != "/identity" {
				t.Fatalf("handshake omitted: %v", paths)
			}
		})
	}
}
func TestCLIRealDaemonProtocolAndRemovedRoute(t *testing.T) {
	bus := core.New()
	if err := bus.EstablishDaemon("owner@fixture", nil); err != nil {
		t.Fatal(err)
	}
	bus.Persistence(memory.NewState())
	tokens, err := auth.Load(memory.NewTokens())
	if err != nil {
		t.Fatal(err)
	}
	daemon := api.New(bus, tokens, "owner@fixture")
	token, err := tokens.Issue("owner@fixture")
	if err != nil {
		t.Fatal(err)
	}
	defer daemon.Close()
	server := httptest.NewServer(daemon.Handler())
	defer server.Close()
	cliProtocolTransport(t, server)
	t.Setenv("AGENT_BUS_TOKEN", token)
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	old := os.Stderr
	os.Stderr = write
	defer func() { os.Stderr = old }()
	identity, code, err := callCtx(context.Background(), "GET", "/identity", nil, nil)
	if err != nil || code != 200 {
		t.Fatalf("identity %d %s %v", code, identity, err)
	}
	var got struct {
		Protocol int `json:"protocol"`
		Minimum  int `json:"protocol_min"`
	}
	if json.Unmarshal(identity, &got) != nil || got.Protocol != 1 || got.Minimum != 1 {
		t.Fatalf("identity=%s", identity)
	}
	data, code, err := callCtx(context.Background(), "POST", "/subscriber/remove", nil, map[string]string{"name": "x"})
	if err != nil || code != 404 || !strings.Contains(string(data), `"kind":"no_route"`) {
		t.Fatalf("CLI message lost no_route: %d %s %v", code, data, err)
	}
	if message := show(data, code, nil); message == nil || !strings.Contains(message.Error(), `"kind":"no_route"`) {
		t.Fatalf("CLI display lost no_route: %v", message)
	}
	write.Close()
	os.Stderr = old
	warnings, _ := io.ReadAll(read)
	if len(warnings) != 0 {
		t.Fatalf("current daemon warned: %s", warnings)
	}
}

func TestCLIProtocolRefusalMessages(t *testing.T) {
	for _, data := range []string{`{"error":"stack"}`, "proxy failure"} {
		got := protocolRefusal([]byte(data), 500)
		if string(got) != `{"error":"internal daemon error"}` {
			t.Fatalf("500 leaked cause: %s", got)
		}
	}
	for _, kind := range []string{"not_found", "no_route", "unavailable", "acl", "authority", "busy", "credential", "enrolment", "full", "malformed", "name-taken", "second-reader", "suspended"} {
		input := []byte(`{"error":"cause","kind":"` + kind + `"}`)
		got := protocolRefusal(input, 404)
		if !strings.Contains(string(got), `"kind":"`+kind+`"`) {
			t.Fatalf("kind lost: %s", got)
		}
		if string(got) != string(input) {
			t.Fatalf("refusal cause changed: %s", got)
		}
	}
}

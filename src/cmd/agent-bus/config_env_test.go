package main

import (
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/sandbox"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestRunScriptConfigurationAtEveryStart(t *testing.T) {
	var raw atomic.Value
	raw.Store("null")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/secret" { // an agent with no bus secret
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/config" {
			if r.URL.Query().Get("name") != "#config@h" {
				t.Error("wrong config name")
			}
			w.Write([]byte(raw.Load().(string)))
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	old := transport
	defer func() { transport = old }()
	transport = func() (*http.Client, string) { return srv.Client(), srv.URL }
	t.Setenv("AGENT_BUS_CONFIG", "inherited-wrong")
	dir := t.TempDir()
	out := filepath.Join(dir, "seen")
	ag := agent{Name: "#config@h", Script: `printf '%s' "${AGENT_BUS_CONFIG-unset}" > ` + out, Algo: algoArgs, box: sandbox.Off{}, work: dir, say: io.Discard}
	for _, tc := range []struct{ raw, want string }{{"null", "unset"}, {"{ }", "{}"}, {"{\n\"channel\":\"one\"}", `{"channel":"one"}`}, {`{"channel":"two"}`, `{"channel":"two"}`}} {
		raw.Store(tc.raw)
		handle(ag, protocol.Envelope{ID: "m", From: "caller@h", To: ag.Name})
		got, err := os.ReadFile(out)
		if err != nil || string(got) != tc.want {
			t.Fatalf("script config %q, %v; want %q", got, err, tc.want)
		}
	}
}

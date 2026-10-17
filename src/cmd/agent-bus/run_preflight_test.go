package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRunLocalRefusalsDoNotChangeTheRegistry(t *testing.T) {
	for _, name := range []string{"sandbox", "work directory", "log directory"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			t.Setenv("AGENT_BUS_TOKEN", "fixture-owner-token")
			t.Setenv("AGENT_BUS_NAME", "owner@h")
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.URL.Path == "/token" {
					w.Write([]byte(`{"token":"fixture-agent-token"}`))
					return
				}
				if r.URL.Path == "/register" {
					w.Write([]byte(`{}`))
					return
				}
				http.Error(w, "fixture refuses serving", http.StatusForbidden)
			}))
			defer server.Close()
			t.Setenv("AGENT_BUS_ADDR", server.URL)
			saved := transport
			transport = func() (*http.Client, string) { return server.Client(), server.URL }
			defer func() { transport = saved }()
			args := []string{"#preflight@h", "/bin/echo", "--sandbox", "off"}
			want := ""
			switch name {
			case "sandbox":
				args[len(args)-1] = "yes"
				want = "--sandbox is on or off"
			case "work directory":
				p := workPath("#preflight@h")
				if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte("not a directory"), 0600); err != nil {
					t.Fatal(err)
				}
				want = "no work directory"
			case "log directory":
				p := filepath.Dir(logPath("#preflight@h"))
				if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte("not a directory"), 0600); err != nil {
					t.Fatal(err)
				}
				want = p
			}
			err := serveScript(args)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("local refusal = %v, want %q", err, want)
			}
			if n := requests.Load(); n != 0 {
				t.Fatalf("local refusal made %d daemon requests", n)
			}
			if got := os.Getenv("AGENT_BUS_TOKEN"); got != "fixture-owner-token" {
				t.Fatal("local refusal switched the caller credential")
			}
			if got := os.Getenv("AGENT_BUS_NAME"); got != "owner@h" {
				t.Fatal("local refusal switched the caller identity")
			}
		})
	}
}

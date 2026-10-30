package main

import (
	"encoding/json"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/sandbox"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunConfigurationFailureAnswers(t *testing.T) {
	posts := make(chan protocol.Envelope, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/secret" { // an agent with no bus secret
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/config" {
			http.Error(w, "away", 503)
			return
		}
		var e protocol.Envelope
		_ = json.NewDecoder(r.Body).Decode(&e)
		posts <- e
		io.WriteString(w, `{}`)
	}))
	defer server.Close()
	old := transport
	defer func() { transport = old }()
	transport = func() (*http.Client, string) { return server.Client(), server.URL }
	dir := t.TempDir()
	handle(agent{Name: "#fixture@h", Script: "touch ran", Algo: algoArgs, work: dir, box: sandbox.Off{}, say: io.Discard}, protocol.Envelope{ID: "m", From: "caller@h"})
	if len(posts) != 1 {
		t.Fatalf("config failure lost the request: %d answers", len(posts))
	}
	e := <-posts
	if e.Receipt != "" || e.Body != "error: configuration unavailable" {
		t.Fatalf("wrong failure answer: %+v", e)
	}
	if _, err := os.Stat(filepath.Join(dir, "ran")); !os.IsNotExist(err) {
		t.Fatal("script ran without config")
	}
}
func TestVerbHelpAndDeadSubscribe(t *testing.T) {
	for _, verb := range []string{"ls", "help", "may-send", "run", "lock", "manage", "subscribe"} {
		t.Run(verb, func(t *testing.T) {
			args := []string{"-test.run=^TestFlagCLIHelper$", "--", verb, "--help"}
			if verb == "subscribe" {
				args = args[:len(args)-1]
			}
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "AB_CLI_FLAG_HELPER=1", "GORACE=atexit_sleep_ms=0")
			out, err := cmd.CombinedOutput()
			if verb == "subscribe" {
				if err == nil || !strings.Contains(string(out), "unknown verb") {
					t.Fatalf("dead subscribe still dispatched: %v %s", err, out)
				}
				return
			}
			if err != nil || !strings.Contains(string(out), "agent-bus "+verb) {
				t.Fatalf("verb help failed: %v %s", err, out)
			}
		})
	}
	if !strings.Contains(verbUsage("manage"), "--tags=<tags>") || !strings.Contains(verbUsage("manage"), "--add-tags=<names>") || !strings.Contains(verbUsage("manage"), "--add-to-set-tags=<names>") || !strings.Contains(verbUsage("manage"), "--remove-tags=<names>") {
		t.Fatal("manage usage omits tag flags")
	}
}

func TestHumanTablesKeepTabs(t *testing.T) {
	if got := humanText("name\tstate\nagent\trunning\x1b\x07"); got != "name\tstate\nagent\trunning" {
		t.Fatalf("table or terminal filter: %q", got)
	}
}

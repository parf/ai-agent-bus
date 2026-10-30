package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/sandbox"
)

// A script holds no credential: the runner's token is not in its
// environment. And a receipt sent to the agent is not work: it never runs the
// script, which would answer it.
func TestAScriptGetsNoTokenAndNoReceipts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/secret" { // an agent with no bus secret
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"id":"r"}`))
	}))
	defer server.Close()
	saved := transport
	defer func() { transport = saved }()
	transport = func() (*http.Client, string) { return server.Client(), server.URL }
	t.Setenv("AGENT_BUS_TOKEN", "the-agents-secret")

	dir := t.TempDir()
	runs, seen := filepath.Join(dir, "runs"), filepath.Join(dir, "env")
	script := filepath.Join(dir, "probe.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho run >>"+runs+"\nenv >"+seen+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ag := agent{Name: "#probe@h", Script: script, Algo: algoArgs, Instances: 1,
		box: sandbox.Off{}, work: dir, say: io.Discard}

	handle(ag, protocol.Envelope{ID: "m1", From: "alice@h", To: "#probe@h", Body: "hi"})
	env, err := os.ReadFile(seen)
	if err != nil {
		t.Fatalf("the script did not run: %v", err)
	}
	if strings.Contains(string(env), "the-agents-secret") {
		t.Fatal("the script was handed the agent's token")
	}
	if !strings.Contains(string(env), "AGENT_BUS_MESSAGE_ID=m1") {
		t.Fatalf("the script lost its envelope: %s", env)
	}

	handle(ag, protocol.Envelope{ID: "m2", From: "alice@h", To: "#probe@h", Receipt: protocol.ReceiptAck, Re: "x"})
	got, _ := os.ReadFile(runs)
	if n := strings.Count(string(got), "run"); n != 1 {
		t.Fatalf("the script ran %d times for one message and one receipt", n)
	}
}

// A daemon slow to take the ack does not hold the work past the caller's
// deadline: the ack gives up when the caller does, and the script is not run
// for a caller that stopped waiting meanwhile.
func TestASlowAckDoesNotRunWorkPastItsDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/secret" { // an agent with no bus secret
			http.NotFound(w, r)
			return
		}
		select {
		case <-time.After(800 * time.Millisecond):
		case <-r.Context().Done():
		}
		w.Write([]byte(`{"id":"r"}`))
	}))
	defer server.Close()
	saved := transport
	defer func() { transport = saved }()
	transport = func() (*http.Client, string) { return server.Client(), server.URL }
	t.Setenv("AGENT_BUS_TOKEN", "t")

	dir := t.TempDir()
	runs := filepath.Join(dir, "runs")
	script := filepath.Join(dir, "probe.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho run >>"+runs+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ag := agent{Name: "#probe@h", Script: script, Algo: algoArgs, Instances: 1,
		box: sandbox.Off{}, work: dir, say: io.Discard}
	began := time.Now()
	handle(ag, protocol.Envelope{ID: "m1", From: "alice@h", To: "#probe@h", Body: "hi",
		Deadline: began.Add(200 * time.Millisecond)})
	if took := time.Since(began); took > 600*time.Millisecond {
		t.Fatalf("the ack held the work for %v, past a 200ms deadline", took)
	}
	if _, err := os.Stat(runs); err == nil {
		t.Fatal("the script ran for a caller that had stopped waiting")
	}
}

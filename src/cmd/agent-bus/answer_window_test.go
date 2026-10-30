package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/sandbox"
)

func TestSlowScriptStartsItsDeliveryWindowWhenOutputIsReady(t *testing.T) {
	saved, savedFor := transport, answerFor
	defer func() { transport, answerFor = saved, savedFor }()
	answerFor = 40 * time.Millisecond
	t.Setenv("AGENT_BUS_TOKEN", "t")
	for _, tc := range []struct {
		name, output string
		deadline     bool
		want         int
	}{
		{"answer", "answer", false, 1},
		{"quiet done", "", false, 1},
		{"caller deadline", "answer", true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts := make(chan protocol.Envelope, 4)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/secret" { // an agent with no bus secret
					http.NotFound(w, r)
					return
				}
				if r.URL.Path == "/config" {
					w.Write([]byte("null"))
					return
				}
				var e protocol.Envelope
				if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
					t.Error(err)
				}
				posts <- e
				w.Write([]byte(`{"id":"r"}`))
			}))
			defer server.Close()
			transport = func() (*http.Client, string) { return server.Client(), server.URL }
			dir := t.TempDir()
			script := filepath.Join(dir, "probe.sh")
			if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 0.12\nprintf '"+tc.output+"'\n"), 0700); err != nil {
				t.Fatal(err)
			}
			ag := agent{Name: "#probe@h", Script: script, Algo: algoArgs, Instances: 1, box: sandbox.Off{}, work: dir, say: io.Discard}
			e := protocol.Envelope{ID: "m1", From: "alice@h", To: "#probe@h", Body: "hi"}
			if tc.deadline {
				e.Deadline = time.Now().Add(80 * time.Millisecond)
			}
			handle(ag, e)
			if len(posts) != tc.want {
				t.Fatalf("wanted %d deliveries, got %d", tc.want, len(posts))
			}
			if tc.want == 1 {
				final := <-posts
				if tc.output != "" && final.Body != tc.output || tc.output == "" && final.Receipt != protocol.ReceiptDone {
					t.Fatalf("wrong final delivery: %+v", final)
				}
			}
		})
	}
}

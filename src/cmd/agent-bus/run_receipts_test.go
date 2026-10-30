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

func TestRunReceiptsOnlyForSlowScripts(t *testing.T) {
	saved := transport
	defer func() { transport = saved }()
	t.Setenv("AGENT_BUS_TOKEN", "fixture")
	for _, tc := range []struct {
		name, delay string
		count       int
	}{{"fast", "0.1", 1}, {"slow", "3", 2}, {"stalled ack", "3", 1}} {
		t.Run(tc.name, func(t *testing.T) {
			posts := make(chan protocol.Envelope, 8)
			ackAt := make(chan time.Time, 1)
			cancelled := make(chan struct{})
			release := make(chan struct{})
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
					return
				}
				posts <- e
				if e.Receipt == protocol.ReceiptAck {
					ackAt <- time.Now()
					if tc.name == "stalled ack" {
						select {
						case <-r.Context().Done():
							close(cancelled)
						case <-release:
						}
					}
				}
				w.Write([]byte(`{}`))
			}))
			defer server.Close()
			defer close(release)
			transport = func() (*http.Client, string) { return server.Client(), server.URL }
			dir := t.TempDir()
			script := filepath.Join(dir, "reply.sh")
			if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep "+tc.delay+"\nprintf answer\n"), 0700); err != nil {
				t.Fatal(err)
			}
			start := time.Now()
			deadline := start.Add(10 * time.Second)
			if tc.name == "stalled ack" {
				deadline = start.Add(2500 * time.Millisecond)
			}
			handle(agent{Name: "#fixture@h", Script: script, Algo: algoArgs, box: sandbox.Off{}, work: dir, say: io.Discard}, protocol.Envelope{ID: "fixture", From: "caller@h", Deadline: deadline})
			if len(posts) != tc.count {
				t.Fatalf("want exactly %d messages, got %d", tc.count, len(posts))
			}
			if tc.name == "stalled ack" {
				select {
				case <-cancelled:
				case <-time.After(time.Second):
					t.Fatal("ack transport did not cancel at caller deadline")
				}
				if time.Since(start) > 4*time.Second {
					t.Fatal("stalled ack delayed completion beyond the script")
				}
				if e := <-posts; e.Receipt != protocol.ReceiptAck {
					t.Fatalf("expected only attempted ack, got %+v", e)
				}
				return
			}

			if tc.count == 2 {
				if e := <-posts; e.Receipt != protocol.ReceiptAck {
					t.Fatalf("first message is not ack: %+v", e)
				}
				if at := <-ackAt; at.Sub(start) < 1900*time.Millisecond {
					t.Fatalf("ack arrived before slow threshold: %s", at.Sub(start))
				}
			}
			if e := <-posts; e.Receipt != "" || e.Body != "answer" {
				t.Fatalf("last message is not answer: %+v", e)
			}
		})
	}
}

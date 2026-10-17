package main

import (
	"context"
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

func TestStopStillDeliversCompletedRepliesWithinTheirBound(t *testing.T) {
	savedTransport, savedFor := transport, answerFor
	defer func() { transport, answerFor = savedTransport, savedFor }()
	t.Setenv("AGENT_BUS_TOKEN", "fixture-token")
	fastAway(t)
	for _, tc := range []struct {
		name, output                   string
		expired, away, hung, beforeAck bool
	}{
		{name: "answer", output: "pong"},
		{name: "quiet done"},
		{name: "already stopping before ACK", output: "pong", beforeAck: true},
		{name: "caller deadline", output: "pong", expired: true},
		{name: "stopped away", output: "pong", away: true},
		{name: "hung response", output: "pong", hung: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			answerFor = 80 * time.Millisecond
			if tc.away {
				answerFor = 500 * time.Millisecond
			}
			stop, cancel := context.WithCancel(context.Background())
			defer cancel()
			posts := make(chan protocol.Envelope, 128)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var e protocol.Envelope
				if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
					t.Error(err)
					return
				}
				posts <- e
				if e.Receipt == protocol.ReceiptAck {
					cancel()
					w.Write([]byte(`{}`))
					return
				}
				if tc.away {
					http.Error(w, "fixture away", http.StatusServiceUnavailable)
					return
				}
				if tc.hung {
					select {
					case <-r.Context().Done():
					case <-time.After(600 * time.Millisecond):
					}
				}
				w.Write([]byte(`{}`))
			}))
			defer server.Close()
			transport = func() (*http.Client, string) { return server.Client(), server.URL }
			dir := t.TempDir()
			script := filepath.Join(dir, "reply.sh")
			sleep := "0.03"
			if tc.expired {
				sleep = "0.20"
			}
			if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep "+sleep+"\nprintf '"+tc.output+"'\n"), 0700); err != nil {
				t.Fatal(err)
			}
			ag := agent{Name: "#reply@h", Script: script, Algo: algoArgs, box: sandbox.Off{}, work: dir, say: io.Discard, stop: stop}
			e := protocol.Envelope{ID: "m", From: "caller@h", To: ag.Name, Body: "hello"}
			if tc.expired {
				e.Deadline = time.Now().Add(120 * time.Millisecond)
			}
			if tc.beforeAck {
				cancel()
			}
			at := time.Now()
			handle(ag, e)
			elapsed := time.Since(at)
			want := 2
			if tc.expired {
				want = 1
			}
			if len(posts) != want {
				t.Fatalf("completed reply after stop: wanted %d deliveries, got %d", want, len(posts))
			}
			if ack := <-posts; ack.Receipt != protocol.ReceiptAck {
				t.Fatalf("first delivery not ACK: %+v", ack)
			}
			if want == 2 {
				final := <-posts
				if tc.output != "" && final.Body != tc.output || tc.output == "" && final.Receipt != protocol.ReceiptDone {
					t.Fatalf("wrong completed reply: %+v", final)
				}
			}
			if (tc.away || tc.hung) && elapsed > 400*time.Millisecond {
				t.Fatalf("stop did not bound delivery/backoff: %s", elapsed)
			}
		})
	}
}

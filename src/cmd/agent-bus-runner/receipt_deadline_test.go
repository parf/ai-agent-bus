package main

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/sandbox"
)

func TestStalledAckReleasesMessageAtDeadline(t *testing.T) {
	for _, callerDeadline := range []bool{true, false} {
		name := "agent"
		if callerDeadline {
			name = "caller"
		}
		t.Run(name, func(t *testing.T) {
			started := make(chan time.Time, 1)
			release := make(chan struct{})
			done := make(chan struct{})
			defer func() { close(release); <-done }()
			tr := secretTransport(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/send" {
					var e protocol.Envelope
					if err := json.NewDecoder(req.Body).Decode(&e); err != nil {
						return nil, err
					}
					if e.Receipt == protocol.ReceiptAck {
						deadline, _ := req.Context().Deadline()
						started <- deadline
						select {
						case <-req.Context().Done():
							return nil, req.Context().Err()
						case <-release:
						}
					}
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
			})
			r := &runner{host: t.TempDir(), say: io.Discard}
			debug, err := filepath.Abs("../../agents/debug/debug.py")
			if err != nil {
				t.Fatal(err)
			}
			s := &served{name: "#fixture@h", work: t.TempDir(), dir: t.TempDir(), cmd: "python3 " + strconv.Quote(debug) + " sleep 10", box: sandbox.Off{}, jail: &jail{}, bus: &client{base: "http://fixture", http: &http.Client{Transport: tr}}, in: installed{Agent: agentFile{Deadline: "3s"}}}
			before := time.Now()
			deadline := before.Add(3 * time.Second)
			e := protocol.Envelope{ID: "fixture", From: "caller@h", To: s.name}
			if callerDeadline {
				e.Deadline = deadline
				s.in.Agent.Deadline = "10s"
			}
			go func() { r.runOne(s, e); close(done) }()
			select {
			case got := <-started:
				// Caller deadline is exact; the agent timeout starts inside runOne.
				if got.Before(deadline.Add(-time.Millisecond)) || got.After(deadline.Add(200*time.Millisecond)) {
					t.Errorf("ack deadline %v, want message deadline near %v", got, deadline)
				}
			case <-done:
				t.Fatal("script returned without exercising stalled ack")
			case <-time.After(5 * time.Second):
				t.Fatal("no delayed ack")
			}
			select {
			case <-done:
			case <-time.After(time.Until(deadline) + time.Second):
				t.Fatal("stalled ack retained runOne after message deadline")
			}
			if s.busy.Load() != 0 {
				t.Fatal("expired message retained busy slot")
			}
		})
	}
}

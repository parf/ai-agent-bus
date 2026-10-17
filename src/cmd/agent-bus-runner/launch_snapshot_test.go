package main

import (
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/sandbox"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

type snapshotTransport struct{}

func (snapshotTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
}

// Reload replaces these fields together while starts and stops run outside
// r.mu. Under -race the old partial snapshots race with this same writer.
func TestStartsAndStopsSnapshotReloadSettings(t *testing.T) {
	for _, path := range []string{"kept command", "script run", "kept stop"} {
		t.Run(path, func(t *testing.T) {
			r := &runner{say: io.Discard}
			s := &served{name: "#fixture@h", work: t.TempDir(), dir: t.TempDir(), cmd: "true", box: sandbox.Off{}, jail: &jail{}, bus: &client{http: &http.Client{Transport: snapshotTransport{}}, base: "http://fixture"}, in: installed{Agent: agentFile{Deadline: "1s"}}}
			k := &keeper{r: r, s: s, cancel: func() {}}
			finished := make(chan struct{})
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				i := 0
				for {
					select {
					case <-finished:
						return
					default:
					}
					r.mu.Lock()
					s.in.Agent = agentFile{Deadline: "1s", Form: []string{"args", "json"}[i%2]}
					s.cmd = []string{"true", "true "}[i%2]
					s.env = []string{"FIXTURE=value"}
					s.box = sandbox.Off{}
					s.net = i%2 == 0
					s.jail = &jail{}
					r.mu.Unlock()
					i++
				}
			}()
			n := 1000
			if path == "script run" {
				n = 25
			}
			for i := 0; i < n; i++ {
				switch path {
				case "kept command":
					if _, err := k.command(); err != nil {
						t.Error(err)
					}
				case "script run":
					r.runOne(s, protocol.Envelope{ID: "fixture", From: "caller@h"})
				case "kept stop":
					k.stop()
				}
			}
			close(finished)
			wg.Wait()
		})
	}
}

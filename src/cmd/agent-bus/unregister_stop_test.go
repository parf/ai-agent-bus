package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestStoppedRunnerWaitsForItsCancelledReader(t *testing.T) {
	t.Setenv("AGENT_BUS_TOKEN", "fixture-token")
	var posts, lookups atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/unregister" {
			if posts.Add(1) == 1 {
				http.Error(w, "reader still releasing", http.StatusConflict)
			}
			return
		}
		if r.URL.Path != "/lookup" {
			t.Errorf("unexpected route %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		readers := 1
		if lookups.Add(1) >= 2 {
			readers = 0
		}
		json.NewEncoder(w).Encode(protocol.Record{Name: "#svc@h", Readers: &readers})
	}))
	defer server.Close()
	saved := transport
	transport = func() (*http.Client, string) { return server.Client(), server.URL }
	defer func() { transport = saved }()
	if err := unregisterStopped("#svc@h"); err != nil {
		t.Fatal(err)
	}
	if posts.Load() != 2 || lookups.Load() != 2 {
		t.Fatalf("idle reader cleanup was not retried conditionally: posts=%d lookups=%d", posts.Load(), lookups.Load())
	}
}

func TestStoppedRunnerKeepsBacklogRefusalsAndUncertainRemoval(t *testing.T) {
	for _, mode := range []string{"backlog", "refused", "unknown-readers", "lost", "busy"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("AGENT_BUS_TOKEN", "fixture-token")
			var posts, lookups atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/unregister" {
					posts.Add(1)
					switch mode {
					case "refused":
						http.Error(w, "private", 403)
					case "lost":
						hangUp(t, w)
					default:
						http.Error(w, "still busy", 409)
					}
					return
				}
				lookups.Add(1)
				readers := 1
				record := protocol.Record{Name: "#svc@h", Readers: &readers}
				if mode == "backlog" {
					record.Queued = 1
					readers = 0
				}
				if mode == "unknown-readers" {
					record.Readers = nil
				}
				json.NewEncoder(w).Encode(record)
			}))
			defer server.Close()
			saved := transport
			transport = func() (*http.Client, string) { return server.Client(), server.URL }
			defer func() { transport = saved }()
			start := time.Now()
			err := unregisterStopped("#svc@h")
			if err == nil {
				t.Fatal("unsafe unregister reported success")
			}
			if posts.Load() != 1 {
				t.Fatalf("unsafe unregister replayed: %d", posts.Load())
			}
			if (mode == "refused" || mode == "lost") && lookups.Load() != 0 {
				t.Fatal("unconfirmed/refused removal began retry lookup")
			}
			if time.Since(start) > 2*time.Second {
				t.Fatal("reader cleanup wait was unbounded")
			}
		})
	}
}

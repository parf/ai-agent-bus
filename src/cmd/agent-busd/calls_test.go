package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/callstats"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func TestCallCounterCoversListenersRefusalsAndItsOwnRead(t *testing.T) {
	var calls atomic.Uint64
	history := callstats.New(&calls)
	history.Sample(time.Now())
	tokens, err := auth.Load(memory.NewTokens(), "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	face := api.New(core.New(), tokens, "owner@h")
	face.Calls(history.Snapshot)
	name, err := protocol.ParseName("owner@h")
	if err != nil {
		t.Fatal(err)
	}
	shared := httptest.NewServer(countCalls(&calls, face.Handler()))
	defer shared.Close()
	mapped := httptest.NewServer(countCalls(&calls, face.HandlerFor(name)))
	defer mapped.Close()
	for _, c := range []struct {
		server       *httptest.Server
		method, path string
		code         int
		token        string
	}{
		{shared, "GET", "/status", 401, ""}, {shared, "GET", "/status", 401, "bad-token"}, {shared, "GET", "/does-not-exist", 404, ""},
		{shared, "POST", "/identity", 405, ""}, {mapped, "GET", "/status", 200, ""},
	} {
		before := calls.Load()
		r, _ := http.NewRequest(c.method, c.server.URL+c.path, nil)
		r.Header.Set(api.HeaderToken, c.token)
		resp, err := c.server.Client().Do(r)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != c.code || calls.Load() != before+1 {
			t.Fatalf("%s %s: HTTP%d counter%d->%d", c.method, c.path, resp.StatusCode, before, calls.Load())
		}
	}
	resp, err := shared.Client().Get(shared.URL + "/identity")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var node protocol.NodeIdentity
	if err := json.NewDecoder(resp.Body).Decode(&node); err != nil {
		t.Fatal(err)
	}
	if node.Calls == nil || node.Calls.Total != 6 {
		t.Fatalf("identity did not count itself or lost other listener: %+v", node.Calls)
	}
	for _, w := range node.Calls.Windows {
		if !w.Available || w.Count != 6 {
			t.Fatalf("startup baseline is missing: %+v", w)
		}
	}
}

func TestCallIsCountedBeforeItsHandlerCompletes(t *testing.T) {
	var calls atomic.Uint64
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	h := countCalls(&calls, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-release }))
	go func() {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/consume", nil))
		close(finished)
	}()
	defer func() { close(release); <-finished }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("handler never entered")
	}
	if calls.Load() != 1 {
		t.Fatal("in-flight request not counted at admission")
	}
}

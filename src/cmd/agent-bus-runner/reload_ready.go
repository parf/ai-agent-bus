package main

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A control reload acknowledges replacement readers, not merely scheduled
// retirement. Serialize reload answers, but release the configuration lock
// before waiting so status and start/stop controls remain available.
func (r *runner) reloadStatus(parent context.Context, wait time.Duration) statusAnswer {
	r.reloadWaitMu.Lock()
	defer r.reloadWaitMu.Unlock()
	r.reloadMu.Lock()
	ctx, cancel := context.WithTimeout(parent, wait)
	defer cancel()
	r.mu.Lock()
	before := make(map[string]*keeper)
	for name, s := range r.served {
		before[name] = s.kept
	}
	r.mu.Unlock()
	r.stopped = nil
	r.apply()
	r.mu.Lock()
	pending := make(map[string]bool)
	for name, s := range r.served {
		if s.in.Agent.Serves == "itself" && (before[name] != s.kept || r.retiring[s.in.Key] != nil) {
			pending[name] = true
		}
	}
	r.mu.Unlock()
	r.reloadMu.Unlock()
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for len(pending) > 0 && ctx.Err() == nil {
		for name := range pending {
			r.mu.Lock()
			s := r.served[name]
			if s == nil || s.in.Agent.Serves != "itself" {
				delete(pending, name)
				r.mu.Unlock()
				continue
			}
			var as *client
			// Never count the old reader while its process is still retiring.
			if s != nil && r.retiring[s.in.Key] == nil && s.kept != nil {
				if count, _ := s.kept.snapshot(); count > 0 {
					as = s.bus
				}
			}
			r.mu.Unlock()
			if as != nil {
				raw, code, err := as.do(ctx, "GET", "/lookup", url.Values{"name": {name}}, nil)
				var record protocol.Record
				if err == nil && code == 200 && json.Unmarshal(raw, &record) == nil && (record.Reading || record.Readers != nil && *record.Readers > 0) {
					delete(pending, name)
				}
			}
		}
		if len(pending) == 0 {
			break
		}
		select {
		case <-tick.C:
		case <-ctx.Done():
		}
	}
	st := r.status()
	for i := range st.Agents {
		a := &st.Agents[i]
		if pending[a.Name] && (a.State == "running" || a.State == "idle" || a.State == "starting") {
			a.State, a.Reason = "starting", "replacement reader has not attached yet"
		}
	}
	return st
}

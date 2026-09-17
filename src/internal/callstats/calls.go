// Package callstats samples the daemon's existing process request counter.
package callstats

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// One hour of minute readings plus a baseline; no per-record data is retained.
const kept = 61

type sample struct {
	at    time.Time
	total uint64
}

type History struct {
	mu      sync.Mutex
	counter *atomic.Uint64
	samples []sample
}

func New(counter *atomic.Uint64) *History { return &History{counter: counter} }

// Sample is driven by the daemon's existing minute ticker. History is process
// local; it is not restored from the registry or filtered by a caller's ACL.
func (h *History) Sample(now time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if n := len(h.samples); n > 0 && !now.After(h.samples[n-1].at) {
		return
	}
	if len(h.samples) == kept {
		copy(h.samples, h.samples[1:])
		h.samples = h.samples[:kept-1]
	}
	h.samples = append(h.samples, sample{now, h.counter.Load()})
}

func (h *History) Snapshot(now time.Time) protocol.CallStats {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := protocol.CallStats{Total: h.counter.Load(), Windows: []protocol.CallWindow{}}
	for _, window := range []struct {
		span  time.Duration
		label string
	}{{time.Minute, "1m"}, {time.Hour, "1h"}} {
		span := window.span
		w := protocol.CallWindow{Window: window.label}
		cutoff := now.Add(-span)
		var baseline *sample
		for i := range h.samples {
			s := &h.samples[i]
			if s.at.After(now) {
				break
			}
			if baseline == nil || !s.at.After(cutoff) {
				baseline = s
			}
			if s.at.After(cutoff) {
				break
			}
		}
		if baseline != nil {
			w.Available = true
			w.Observed = now.Sub(baseline.at).Round(time.Second).String()
			w.Count = out.Total - baseline.total
		}
		out.Windows = append(out.Windows, w)
	}
	return out
}

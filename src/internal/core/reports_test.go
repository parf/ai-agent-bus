package core

import (
	"strings"
	"sync"

	"github.com/parf/ai-agent-bus/internal/ports"
)

// reports is a journal that keeps what the bus reported.
type reports struct {
	mu    sync.Mutex
	lines []string
}

func (r *reports) Audit(ports.AuditEntry) {}
func (r *reports) Report(s ports.Severity, m string) {
	r.mu.Lock()
	r.lines = append(r.lines, s.String()+": "+m)
	r.mu.Unlock()
}
func (r *reports) Request(ports.RequestLine) {}
func (r *reports) SetDebug(bool) error       { return nil }
func (r *reports) DebugOn() bool             { return false }

func (r *reports) has(sub string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, l := range r.lines {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}

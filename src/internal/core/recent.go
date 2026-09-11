package core

import "github.com/parf/ai-agent-bus/internal/protocol"

// How many envelopes the bus remembers having seen. Small on purpose: this
// is a window for a human watching, not a log.
const recentKept = 100

// note records that a message was accepted, **without its body**. The body is
// struck out here, at the only door into the ring, so that no later reader
// has to be trusted to leave it alone — the bus and its dashboard see
// envelopes and nothing else. See docs/05-discovery.md#dashboard.
func (b *Bus) note(e protocol.Envelope) {
	e.Body = ""
	if len(b.recent) == recentKept {
		b.recent = append(b.recent[:0], b.recent[1:]...)
	}
	b.recent = append(b.recent, e)
}

// Recent hands back what the bus has seen, newest first. It is this run's:
// a restart starts the window again, like uptime.
func (b *Bus) Recent() []protocol.Envelope {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]protocol.Envelope, 0, len(b.recent))
	for i := len(b.recent) - 1; i >= 0; i-- {
		out = append(out, b.recent[i])
	}
	return out
}

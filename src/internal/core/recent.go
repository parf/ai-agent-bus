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
//
// A caller sees the envelopes they were **party to** — sent or addressed to
// them — and the daemon Owner sees the node's. That is what makes the dashboard's
// exchanges view somebody's own rather than the operator's
// (docs/05-discovery.md#what-it-shows), and it is the same rule the registry
// answers by: what you may see, not everything there is.
func (b *Bus) Recent(caller string) []protocol.Envelope {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.acting(caller) != nil {
		return []protocol.Envelope{}
	}
	all := caller == b.admin
	out := make([]protocol.Envelope, 0, len(b.recent))
	for i := len(b.recent) - 1; i >= 0; i-- {
		if e := b.recent[i]; all || e.From == caller || e.To == caller {
			out = append(out, e)
		}
	}
	return out
}

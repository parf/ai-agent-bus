package core

import (
	"context"
	"fmt"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// An asks-only read (Q260, Plans/R1.0-Release/TODO.md#bundled-agents) takes only
// the messages whose `ask` is set and leaves every other message queued, in
// place and in order, for the ordinary reader. It is how an agent that has not
// attached its reader yet, or is still loading, can answer `help` and `status`
// without taking work. It is a filtered reader: it may wait beside the one
// unfiltered reader, it is never the "second" one, and it needs exactly the
// consume authority the caller already has. While an unfiltered reader waits,
// an arriving ask goes to it, since that agent answers about itself; the
// asks-only read gets an ask only when no unfiltered reader is waiting.

type asksOnlyKey struct{}

// WithAsksOnly makes a consume take only messages that carry an ask.
func WithAsksOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, asksOnlyKey{}, true)
}

// CheckAsks refuses a value of the `asks` read option that is not `only`,
// naming the option.
func CheckAsks(v string) error {
	if v != "only" {
		return fmt.Errorf("%w: asks must be only, not %q", ErrBadName, v)
	}
	return nil
}

// takes is whether a waiter would take e: its topic, tag and address filter,
// and, for an asks-only read, an ask.
func (w *waiter) takes(e protocol.Envelope) bool {
	return selects(w.topic, w.tag, w.addressed, e) && (!w.asks || e.Ask != "")
}

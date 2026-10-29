package core

import (
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A caller that waits for its own answer names its own inbox as the reply
// route. Between its send and its filtered read, and again after each receipt,
// no filtered read is outstanding, and the answer would go to whatever
// unfiltered reader the inbox has (a push adapter) and be lost to the caller.
// So the bus holds that reply — only that one — for filtered readers until the
// caller's deadline. It never drops or refuses: a held message is queued, and
// the unfiltered reader gets it when the hold ends.
// See docs/04-messaging.md#request-and-reply.

const (
	// maxHolds bounds the reservations one inbox keeps; a send past it
	// behaves as before, it is not refused.
	maxHolds = 64
	// maxHold bounds how long one reservation lasts, whatever the wait.
	maxHold = 10 * time.Minute
)

// hold is one reservation on a caller's own inbox: the replies to one request.
type hold struct {
	from       string // the name the request was sent to
	topic, tag string
	until      time.Time
	timer      *time.Timer
}

// reserve makes the reservation a send with a wait asks for, when it names
// its own reply route: reply_to is the sender itself, with a topic and a tag.
// A reply_to naming anyone else reserves nothing, since the sender is then not
// the one reading the answer. Caller holds b.mu.
func (b *Bus) reserve(e protocol.Envelope) {
	if e.ReplyTo == nil || e.ReplyTo.Name != e.From || e.To == e.From ||
		e.Topic == "" || e.Tag == "" || e.Deadline.IsZero() ||
		e.ReplyTo.Topic != e.Topic || e.ReplyTo.Tag != e.Tag {
		return
	}
	in := b.ensure(e.From)
	now := e.At
	in.holds = liveHolds(in.holds, now)
	if len(in.holds) >= maxHolds {
		return
	}
	until := e.Deadline
	if limit := now.Add(maxHold); until.After(limit) {
		until = limit
	}
	h := &hold{from: e.To, topic: e.Topic, tag: e.Tag, until: until}
	h.timer = time.AfterFunc(until.Sub(now), func() { b.release(e.From, h) })
	in.holds = append(in.holds, h)
}

// liveHolds drops the reservations whose moment has passed.
func liveHolds(hs []*hold, now time.Time) []*hold {
	kept := hs[:0]
	for _, h := range hs {
		if now.Before(h.until) {
			kept = append(kept, h)
			continue
		}
		h.timer.Stop()
	}
	for i := len(kept); i < len(hs); i++ {
		hs[i] = nil
	}
	return kept
}

// held says whether e is a reply some caller's reservation keeps for its own
// filtered reader: from the name the request went to, on its topic and tag.
func (h *hold) holds(e protocol.Envelope, now time.Time) bool {
	return now.Before(h.until) && e.From == h.from && e.Topic == h.topic && e.Tag == h.tag
}

func (in *inbox) held(e protocol.Envelope, now time.Time) bool {
	for _, h := range in.holds {
		if h.holds(e, now) {
			return true
		}
	}
	return false
}

// served ends the reservation e answers, when a filtered reader takes it: the
// first message that is not a receipt is the answer, and what follows on the
// same topic and tag is nobody's reply. Caller holds b.mu.
func (in *inbox) served(e protocol.Envelope, now time.Time) {
	if e.Receipt != "" {
		return
	}
	for i, h := range in.holds {
		if h.holds(e, now) {
			h.timer.Stop()
			in.holds = append(in.holds[:i], in.holds[i+1:]...)
			return
		}
	}
}

// release ends a reservation at its moment and offers what it kept back to an
// unfiltered reader that is blocked on the inbox. Without it a held reply on a
// quiet inbox would wait for the next send.
func (b *Bus) release(name string, h *hold) {
	b.mu.Lock()
	defer b.unlock()
	in := b.inboxes[name]
	if in == nil {
		return
	}
	for i, x := range in.holds {
		if x == h {
			in.holds = append(in.holds[:i], in.holds[i+1:]...)
			break
		}
	}
	now := time.Now()
	b.prune(in, now)
	for i := 0; i < len(in.queue); {
		e := in.queue[i]
		if in.held(e, now) {
			i++
			continue
		}
		w := in.unfilteredWaiter()
		if w == nil {
			return
		}
		in.queue = take(in.queue, i)
		in.out++
		in.waiters = dropWaiter(in.waiters, w)
		w.ch <- e
	}
}

func (in *inbox) unfilteredWaiter() *waiter {
	for _, w := range in.waiters {
		if !w.filtered {
			return w
		}
	}
	return nil
}

func dropWaiter(ws []*waiter, w *waiter) []*waiter {
	for i, x := range ws {
		if x == w {
			return drop(ws, i)
		}
	}
	return ws
}

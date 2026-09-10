// Package core is the domain: the registry and the inboxes. It touches
// nothing outside itself — no HTTP, no files, no clock beyond time.Now.
// See docs/10-modules.md.
package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// PoC bounds. A queue that grows without limit is a memory leak with a
// friendly name; overflow drops the oldest (ring) — see docs/04-messaging.md.
const maxQueue = 1000

var (
	ErrUnknown  = errors.New("no such name")
	ErrTwoReads = errors.New("inbox already has a reader")
	ErrBadName  = errors.New("bad name")
	ErrNotYet   = errors.New("pub/sub topics arrive at MVP")
)

// canon normalises a name so that "  x@y " and "x@y" are the same inbox.
// It lives here, not in a face, so every face gets the same answer.
func canon(s string) (string, error) {
	n, err := protocol.ParseName(s)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrBadName, err)
	}
	return n.String(), nil
}

// waiter is one blocked consume. A filtered waiter takes only the message it
// is waiting for; the unfiltered reader takes anything else.
type waiter struct {
	topic, tag string
	filtered   bool
	ch         chan protocol.Envelope
}

type inbox struct {
	queue   []protocol.Envelope
	waiters []*waiter
}

type Bus struct {
	mu      sync.Mutex
	records map[string]protocol.Record
	inboxes map[string]*inbox
	started time.Time
}

func New() *Bus {
	return &Bus{
		records: map[string]protocol.Record{},
		inboxes: map[string]*inbox{},
		started: time.Now(),
	}
}

func (b *Bus) Register(r protocol.Record) (protocol.Record, error) {
	name, err := canon(r.Name)
	if err != nil {
		return protocol.Record{}, err
	}
	if owner, err := canon(r.Owner); err == nil {
		r.Owner = owner
	}
	if r.Kind == "" {
		r.Kind = "generic"
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	r.Name = name
	r.At = time.Now()
	b.records[name] = r
	b.ensure(name)
	return r, nil
}

// Lookup answers what a name is, so a face can tell a topic from a filter
// without guessing. See docs/03-services-and-topics.md.
func (b *Bus) Lookup(name string) (protocol.Record, bool) {
	n, err := canon(name)
	if err != nil {
		return protocol.Record{}, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	r, ok := b.records[n]
	return r, ok
}

func (b *Bus) List(kind string) []protocol.Record {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]protocol.Record, 0, len(b.records))
	for _, r := range b.records {
		if kind == "" || r.Kind == kind {
			out = append(out, r)
		}
	}
	return out
}

// ensure creates the inbox for a name. Every registered principal has one;
// the address outlives the process, so a message can arrive while it is down.
func (b *Bus) ensure(name string) *inbox {
	in, ok := b.inboxes[name]
	if !ok {
		in = &inbox{}
		b.inboxes[name] = in
	}
	return in
}

// Send delivers one message to one inbox. A waiting consume takes it
// directly; otherwise it queues.
func (b *Bus) Send(e protocol.Envelope) (protocol.Envelope, error) {
	to, err := canon(e.To)
	if err != nil {
		return protocol.Envelope{}, err
	}
	from, err := canon(e.From)
	if err != nil {
		return protocol.Envelope{}, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	rec, known := b.records[to]
	if !known {
		return protocol.Envelope{}, ErrUnknown
	}
	// A queue topic is an inbox with a name, so publishing to one is an
	// ordinary send. Fan-out is not: a subscriber is undefined without an
	// ACL, so PoC stores the mode and says so. See docs/12-stages.md#poc.
	if rec.Kind == protocol.KindTopic && rec.Mode == protocol.ModePubSub {
		return protocol.Envelope{}, ErrNotYet
	}
	e.To, e.From = to, from
	e.ID = newID()
	e.At = time.Now()
	in := b.ensure(to)

	// A waiter that asked for this topic and tag is served ahead of the
	// unfiltered reader — otherwise a `call` loses its reply to whichever
	// reader happened to block first.
	// See docs/04-messaging.md#one-reader-per-inbox.
	for _, filteredPass := range []bool{true, false} {
		for i, w := range in.waiters {
			if w.filtered != filteredPass {
				continue
			}
			if w.filtered && (w.topic != e.Topic || w.tag != e.Tag) {
				continue
			}
			in.waiters = append(in.waiters[:i], in.waiters[i+1:]...)
			w.ch <- e
			return e, nil
		}
	}
	in.queue = append(in.queue, e)
	if len(in.queue) > maxQueue {
		in.queue = in.queue[1:] // ring: the oldest goes
	}
	return e, nil
}

// Consume takes one message, at-most-once: it is handed over and gone.
// A filter waits for one topic+tag; an unfiltered read takes the next of
// anything, and there may be only one of those at a time.
// See docs/04-messaging.md#one-reader-per-inbox.
func (b *Bus) Consume(ctx context.Context, name, topic, tag string, filtered bool) (protocol.Envelope, error) {
	name, err := canon(name)
	if err != nil {
		return protocol.Envelope{}, err
	}
	b.mu.Lock()
	in := b.ensure(name)

	for i, e := range in.queue {
		if !filtered || (e.Topic == topic && e.Tag == tag) {
			in.queue = append(in.queue[:i], in.queue[i+1:]...)
			b.mu.Unlock()
			return e, nil
		}
	}
	if !filtered {
		for _, w := range in.waiters {
			if !w.filtered {
				b.mu.Unlock()
				return protocol.Envelope{}, ErrTwoReads
			}
		}
	}
	w := &waiter{topic: topic, tag: tag, filtered: filtered, ch: make(chan protocol.Envelope, 1)}
	in.waiters = append(in.waiters, w)
	b.mu.Unlock()

	select {
	case e := <-w.ch:
		return e, nil
	case <-ctx.Done():
		// A send may have handed us a message just as the wait expired. Settle
		// that race under the same lock a send holds: either we take what was
		// delivered, or we remove the waiter so nothing can be delivered to it.
		if e, delivered := b.settle(name, w); delivered {
			return e, nil
		}
		return protocol.Envelope{}, ctx.Err()
	}
}

// settle removes a waiter, unless a message reached it first — in which case
// it returns that message. Nothing is dropped in between.
func (b *Bus) settle(name string, w *waiter) (protocol.Envelope, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	select {
	case e := <-w.ch:
		return e, true
	default:
	}
	in := b.ensure(name)
	for i, x := range in.waiters {
		if x == w {
			in.waiters = append(in.waiters[:i], in.waiters[i+1:]...)
			break
		}
	}
	return protocol.Envelope{}, false
}

type Status struct {
	Up       string `json:"up"`
	Services int    `json:"services"`
	Queued   int    `json:"queued"`
	Waiting  int    `json:"waiting"`
}

func (b *Bus) Status() Status {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := Status{Up: time.Since(b.started).Round(time.Second).String(), Services: len(b.records)}
	for _, in := range b.inboxes {
		s.Queued += len(in.queue)
		s.Waiting += len(in.waiters)
	}
	return s
}

func newID() string {
	var b [8]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

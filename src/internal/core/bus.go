// Package core is the domain: the registry and the inboxes. It touches
// nothing outside itself — no HTTP, no files, no clock beyond time.Now.
// See docs/10-modules.md.
package core

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// PoC bounds. A queue that grows without limit is a memory leak with a
// friendly name. What happens at the bound is the receiver's choice, and it
// refuses unless it asked for a ring — see docs/04-messaging.md#overflow.
const maxQueue = 1000

var (
	ErrUnknown  = errors.New("no such name")
	ErrTwoReads = errors.New("inbox already has a reader")
	ErrBadName  = errors.New("bad name")
	ErrNotYet   = errors.New("pub/sub topics arrive at MVP")
	ErrReceipt  = errors.New(`a receipt is "ack" or "done"`)
	ErrFull     = errors.New("the receiver's queue is full")
	ErrOverflow = errors.New("overflow is strict or ring")
	ErrConfig   = errors.New("a configuration is JSON")
	ErrNotOwner = errors.New("that record belongs to someone else")
	ErrPrivate  = errors.New("a configuration is private to the service it belongs to")
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
	dropped int // messages the ring threw away, since start
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
	if r.Full == "" {
		r.Full = protocol.OverflowStrict
	}
	if r.Full != protocol.OverflowStrict && r.Full != protocol.OverflowRing {
		return protocol.Record{}, fmt.Errorf("%w, not %q", ErrOverflow, r.Full)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	// Registering refreshes a description. It is not a way to take a record
	// over or to throw its configuration away: a service registers itself on
	// every start, and that must not destroy what it was configured with.
	// See docs/03-services-and-topics.md#configuring-a-template.
	//
	// A registration also never carries either half of a configuration:
	// the bytes have one write path and this is not it, and the digest is
	// derived from them (protocol.Record.Public), so accepting one from a
	// caller would let anyone claim any setup — which is exactly what the
	// digest exists to detect.
	r.Config, r.ConfigSHA = nil, ""
	if old, known := b.records[name]; known {
		r.Config = old.Config
		r.Owner = old.Owner
	}
	r.Name = name
	r.At = time.Now()
	b.records[name] = r
	b.ensure(name)
	return r, nil
}

// Configure attaches a configuration to a service, creating the service if it
// does not exist yet: configuring a service template is what produces a
// configured service, and a service has an inbox from the moment it exists.
//
// The configuration is opaque. The only thing checked is that it is JSON —
// the same "stored raw, shape-checked only" rule the MCP method info follows
// — so nothing here looks for a server, a user, a mailbox or a credential.
//
// A configuration is the one field that can hold a secret, so unlike a
// registration it is not something any caller may overwrite: an existing
// record belongs to its owner. See
// docs/03-services-and-topics.md#configuring-a-template.
func (b *Bus) Configure(name, caller string, cfg json.RawMessage) (protocol.Record, error) {
	n, err := canon(name)
	if err != nil {
		return protocol.Record{}, err
	}
	who, err := canon(caller)
	if err != nil {
		return protocol.Record{}, err
	}
	if !json.Valid(cfg) {
		return protocol.Record{}, fmt.Errorf("%w, and this is not", ErrConfig)
	}
	if string(bytes.TrimSpace(cfg)) == "null" {
		// null is JSON, but it reads back identically to never-configured,
		// which would make "is it configured?" unanswerable.
		return protocol.Record{}, fmt.Errorf("%w, and null is the absence of one", ErrConfig)
	}
	// Store one spelling. The digest a query gets is over what is stored
	// (protocol.Record.Public), so reformatting a configuration file must not
	// look like changing it.
	var canonical bytes.Buffer
	if err := json.Compact(&canonical, cfg); err != nil {
		return protocol.Record{}, fmt.Errorf("%w: %s", ErrConfig, err)
	}
	cfg = canonical.Bytes()
	b.mu.Lock()
	defer b.mu.Unlock()
	r, known := b.records[n]
	if !known {
		// Same defaults a bare registration gets: configuring is not a
		// second way to describe a service, only a way to give it config.
		r = protocol.Record{Name: n, Kind: "generic", Owner: who, Full: protocol.OverflowStrict}
	} else if r.Owner != who && n != who {
		// Writing is the owner's, and the service's own. Reading is neither:
		// see Config.
		return protocol.Record{}, fmt.Errorf("%w: %s is %s's", ErrNotOwner, n, r.Owner)
	}
	r.Config = cfg
	r.At = time.Now()
	b.records[n] = r
	b.ensure(n)
	return r, nil
}

// Config reads one back — for the service itself and nobody else, its owner
// included. Setup data goes in and is used; it does not come back out to be
// looked at. An owner configures a service and is told that it worked
// (Configure answers without the configuration), which is all an owner needs.
//
// It is not part of any listing either, so there is no other way to one.
func (b *Bus) Config(name, caller string) (json.RawMessage, error) {
	n, err := canon(name)
	if err != nil {
		return nil, err
	}
	who, err := canon(caller)
	if err != nil {
		return nil, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	r, known := b.records[n]
	if !known {
		return nil, fmt.Errorf("%w: %s", ErrUnknown, n)
	}
	if n != who {
		return nil, fmt.Errorf("%w: only %s may read it", ErrPrivate, n)
	}
	return r.Config, nil
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
	if ok {
		r = b.withLiveness(n, r)
	}
	return r, ok
}

// withLiveness answers the question a listing is really asked: not "does
// this name exist" but "is anything serving it". The daemon knows exactly —
// an inbox with a read outstanding has something on the other end.
//
// Held state, not stored state: it is attached on the way out and never
// written back, so nothing in the registry depends on who happened to be
// connected. Caller holds the lock.
func (b *Bus) withLiveness(name string, r protocol.Record) protocol.Record {
	in, ok := b.inboxes[name]
	if !ok {
		return r
	}
	r.Queued = len(in.queue)
	for _, w := range in.waiters {
		if !w.filtered {
			r.Reading = true
			break
		}
	}
	return r
}

func (b *Bus) List(kind string) []protocol.Record {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]protocol.Record, 0, len(b.records))
	for _, r := range b.records {
		if kind == "" || r.Kind == kind {
			// A listing is public to every caller; a configuration is not,
			// and a digest of it is what a query gets instead.
			out = append(out, b.withLiveness(r.Name, r.Public()))
		}
	}
	return out
}

// take removes one message and releases it. Shortening a slice leaves the
// vacated slot pointing at the envelope, so a drained inbox goes on holding
// every body it ever handed out; clearing the tail is what actually frees
// them. One place to get this right, because it is one line to forget.
//
// The backing array is kept — reusing a zeroed allocation is the point — but
// nothing dead stays reachable through it.
//
// Taking from the head by stepping past it instead of shifting was measured
// and rejected: it wins past a backlog of ~300 and on a full ring (2443 ns
// to 733), loses below that, and allocates 330-520 B on every message where
// this allocates none. New garbage per message is the worse trade inside a
// component that is 2.6% of the daemon's CPU.
func take(q []protocol.Envelope, i int) []protocol.Envelope {
	copy(q[i:], q[i+1:])
	q[len(q)-1] = protocol.Envelope{}
	return q[:len(q)-1]
}

// drop removes one waiter, and releases it for the same reason take does: a
// dead waiter holds a channel, and a slot that still points at it keeps that
// channel alive for as long as the inbox exists.
func drop(ws []*waiter, i int) []*waiter {
	copy(ws[i:], ws[i+1:])
	ws[len(ws)-1] = nil
	return ws[:len(ws)-1]
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
		return protocol.Envelope{}, fmt.Errorf("no such receiver: %s (%w)", to, ErrUnknown)
	}
	// A queue topic is an inbox with a name, so publishing to one is an
	// ordinary send. Fan-out is not: a subscriber is undefined without an
	// ACL, so PoC stores the mode and says so. See docs/12-stages.md#poc.
	if rec.Kind == protocol.KindTopic && rec.Mode == protocol.ModePubSub {
		return protocol.Envelope{}, fmt.Errorf("%s is a pub/sub topic: %w", to, ErrNotYet)
	}
	// Receipts are a closed set: a caller decides whether a message is an
	// answer by looking at this field, so a third value would read as one.
	if e.Receipt != "" && e.Receipt != protocol.ReceiptAck && e.Receipt != protocol.ReceiptDone {
		return protocol.Envelope{}, fmt.Errorf("%w, not %q", ErrReceipt, e.Receipt)
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
			in.waiters = drop(in.waiters, i)
			w.ch <- e
			return e, nil
		}
	}
	// A full queue either refuses the new message or forgets the oldest, and
	// the receiver's record says which. Refusing is the default because
	// losing a job silently is worse than failing visibly; a ring is for the
	// streams where the newest matters most (docs/04-messaging.md#overflow).
	if len(in.queue) >= maxQueue {
		if rec.Full != protocol.OverflowRing {
			return protocol.Envelope{}, fmt.Errorf("%w: %s holds %d", ErrFull, to, len(in.queue))
		}
		in.queue = take(in.queue, 0)
		b.dropped++ // a real loss, so `status` reports it
	}
	in.queue = append(in.queue, e)
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
	// An inbox belongs to a registered name. Creating one for whoever asks
	// would let any caller name leave a permanent entry behind — and the
	// wait could never end anyway, because Send refuses an unknown
	// receiver, so nothing can ever arrive in it.
	if _, known := b.records[name]; !known {
		b.mu.Unlock()
		return protocol.Envelope{}, fmt.Errorf("no inbox for %s: register it first (%w)", name, ErrUnknown)
	}
	in := b.ensure(name)

	for i, e := range in.queue {
		if !filtered || (e.Topic == topic && e.Tag == tag) {
			in.queue = take(in.queue, i)
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
			in.waiters = drop(in.waiters, i)
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
	Dropped  int    `json:"dropped"` // lost to overflow since start
}

func (b *Bus) Status() Status {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := Status{
		Up:       time.Since(b.started).Round(time.Second).String(),
		Services: len(b.records),
		Dropped:  b.dropped,
	}
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

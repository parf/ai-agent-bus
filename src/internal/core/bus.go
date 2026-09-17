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

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Bounds. A queue that grows without limit is a memory leak with a friendly
// name. What happens at the bound is the receiver's choice, and it
// refuses unless it asked for a ring — see docs/04-messaging.md#overflow.
const maxQueue = 1000

var (
	ErrProfile  = errors.New("invalid user profile")
	ErrInactive = errors.New("user access is suspended")
	// Held a credential, and the daemon holds nothing else for the name: no
	// profile and no record of its own. That is not a state to recover from,
	// it is nobody (docs/02-access.md#what-a-call-carries).
	ErrNoPrincipal = errors.New("that credential answers for nobody")
	ErrDisabled    = errors.New("service is disabled")
	ErrUnknown     = errors.New("no such name")
	ErrTwoReads    = errors.New("inbox already has a reader, and neither asked to share it")
	ErrBadName     = errors.New("bad name")
	ErrReceipt     = errors.New(`a receipt is "ack" or "done"`)
	ErrFull        = errors.New("the receiver's queue is full")
	ErrOverflow    = errors.New("overflow is strict or ring")
	ErrMode        = errors.New("a topic mode is queue or pubsub")
	ErrConfig      = errors.New("a configuration is JSON")
	ErrTTL         = errors.New("a ttl is a duration, like 30s")
	ErrWait        = errors.New("a wait is a duration, like 30s")
	ErrBound       = errors.New("a bound is a positive number of messages")
	ErrNotOwner    = errors.New("that record belongs to someone else")
	ErrExists      = errors.New("that name is already registered")
	ErrBusy        = errors.New("cannot unregister a busy inbox")
	ErrPrivate     = errors.New("a configuration is private to the service it belongs to")
	ErrNotAllow    = errors.New("not on that service's allow list")
	ErrPersonal    = errors.New("a personal service may name only direct service identities in its ACL and cannot have maintainers")
	ErrEnrol       = errors.New("enrolment")
	// A group is retired by emptying its membership
	// (docs/01-identity-and-roles.md#groups), so there is no removal to
	// ask for. A request that asks anyway is refused rather than read as a
	// membership change: emptying a group leaves every record that names it
	// alone, and unmapping the name would not have.
	ErrNoRemoval = errors.New("a group is retired by emptying its membership; there is no removal")
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
//
// share is how the daemon tells a worker pool from an accidental second
// reader: a pool says so. See docs/04-messaging.md#one-reader-per-inbox.
type waiter struct {
	topic, tag string
	filtered   bool
	share      bool
	ch         chan protocol.Envelope
	caller     string
	stopped    chan error
}

type inbox struct {
	// What has been through it, since the daemon started. Observed, like
	// the rest of the live state: counted here and attached on the way out,
	// never taken from a caller. A queue that is drained and one nobody
	// ever wrote to both read as empty, and these tell them apart.
	in, out int
	refused int
	// Its own loss, not the daemon's, for the reason protocol.Record.Dropped
	// gives.
	dropped, expired int

	queue   []protocol.Envelope
	waiters []*waiter
}

type Bus struct {
	dump     ports.Dump
	users    map[string]protocol.User
	mu       sync.Mutex
	activity []activitySample
	records  map[string]protocol.Record
	groups   map[string][]string
	admin    string
	// How many calls were refused, and for what. Counted because a bus that
	// is quiet and one that is refusing everything look identical from
	// outside — see docs/05-discovery.md#what-it-shows.
	refused map[string]int
	// Whether the snapshot this run read back was written by a graceful
	// stop. Logged once at start is not enough: whoever comes to look at a
	// gap in the work arrives long after the log line scrolled away.
	unclean bool
	inboxes map[string]*inbox
	started time.Time
	// What the bus has seen lately, bodies struck out — the dashboard's
	// only source. See docs/05-discovery.md#dashboard.
	recent []protocol.Envelope
	// Who holds the master ACL. See docs/02-access.md#acl.
	masters map[string]bool
	// Which realms are backed by a directory, what verifies a signature,
	// and the challenges outstanding. See docs/01-identity-and-roles.md#registration.
	dirs    map[string]ports.Directory
	sigs    ports.Signatures
	pending map[string]challenge
}

func New() *Bus {
	return &Bus{
		records: map[string]protocol.Record{},
		groups:  map[string][]string{},
		users:   map[string]protocol.User{},
		refused: map[string]int{},
		inboxes: map[string]*inbox{},
		started: time.Now(),
	}
}

// Register states a record. A name in a realm a directory backs cannot be
// created this way — it has to be enrolled, or the first caller to ask for a
// name would become it. See docs/01-identity-and-roles.md#registration.
func (b *Bus) Register(r protocol.Record) (protocol.Record, error) {
	return b.register(r, false, false)
}

// RegisterNew claims a name without replacing even the caller's own record.
func (b *Bus) RegisterNew(r protocol.Record) (protocol.Record, error) {
	return b.register(r, false, true)
}

func (b *Bus) register(r protocol.Record, enrolled, createOnly bool) (protocol.Record, error) {
	name, err := canon(r.Name)
	if err != nil {
		return protocol.Record{}, err
	}
	r.Name = name
	if owner, err := canon(r.Owner); err == nil {
		r.Owner = owner
	}
	if r.Kind == "" {
		r.Kind = "generic"
	}
	// A registration that names no owner is the name answering for itself,
	// which is what registering one has always meant where no realm vouches
	// for it (docs/01-identity-and-roles.md#registration). Defaulted here beside the
	// others, so that "owned by nobody" is not a state a record can be in.
	if r.Owner == "" {
		r.Owner = name
	}
	if r.Full == "" {
		r.Full = protocol.OverflowStrict
	}
	if r.Full != protocol.OverflowStrict && r.Full != protocol.OverflowRing {
		return protocol.Record{}, fmt.Errorf("%w, not %q", ErrOverflow, r.Full)
	}
	// A mode the daemon does not know reads as a queue, which is the mode a
	// caller asking for pub/sub least wants. Refused here, like overflow,
	// rather than only in the CLI.
	if r.Mode != "" && r.Mode != protocol.ModeQueue && r.Mode != protocol.ModePubSub {
		return protocol.Record{}, fmt.Errorf("%w, not %q", ErrMode, r.Mode)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	// Enrolment is the one caller that legitimately registers a name the
	// daemon does not yet know — it has just proved the realm's key for it
	// (docs/02-access.md#proving-possession) — and it says so here rather
	// than through a clause in mayOwn that every other caller could reach.
	if !enrolled {
		if err := b.mayOwn(r.Owner, name); err != nil {
			return protocol.Record{}, err
		}
	}
	if _, exists := b.records[name]; createOnly && exists {
		return protocol.Record{}, ErrExists
	}
	// Refusing a *new* name in a vouched-for realm is what turns "the owner
	// of a record may have its credential" from a hole into a rule: you
	// become the name by proving you hold its key, not by asking first.
	// See docs/01-identity-and-roles.md#registration.
	if _, known := b.records[name]; !known && !enrolled {
		if err := b.vouchedFor(name); err != nil {
			return protocol.Record{}, err
		}
	}
	// Registering refreshes a description. It is not a way to take a record
	// over or to throw its configuration away: a service registers itself on
	// every start, and that must not destroy what it was configured with.
	// See docs/03-services-and-topics.md#configuring-a-template.
	//
	// A registration also never carries either half of a configuration:
	// the bytes have one write path and this is not it, and the digest is
	// derived from them (protocol.Record.Public), so accepting one from a
	// caller would let anyone claim any setup — which is what the digest
	// exists to detect.
	// The same applies to the live fields: they are what the daemon
	// observes, attached to an answer on the way out, so a caller stating
	// them would be claiming a reader it does not have.
	if r.TTL != "" {
		d, err := time.ParseDuration(r.TTL)
		if err != nil || d <= 0 {
			return protocol.Record{}, fmt.Errorf("%w, not %q", ErrTTL, r.TTL)
		}
	}
	if r.Bound < 0 {
		return protocol.Record{}, fmt.Errorf("%w, not %d", ErrBound, r.Bound)
	}
	r.Config, r.ConfigSHA, r.Subs = nil, "", nil
	r.Maintainers, r.Disabled = "", false
	r.CanManage, r.CanTransfer = false, false
	r.Reading, r.Queued, r.In, r.Out = false, 0, 0, 0
	r.Dropped, r.Expired, r.Oldest, r.AtBound = 0, 0, "", false
	// Publishing a name is open to anyone; changing one that exists belongs
	// to its owner, and to the record itself — a service registering on
	// every start is not a stranger to its own name, and it is the only
	// other principal that could hold that name's credential.
	// See docs/01-identity-and-roles.md#ownership.
	if old, known := b.record(name); known {
		caller := r.Owner // the face puts the caller here, not a claim
		if old.Owner != "" && !b.manages(caller, old) {
			return protocol.Record{}, fmt.Errorf("%w: %s is %s's", ErrNotOwner, name, old.Owner)
		}
		r.Config = old.Config
		r.Owner = old.Owner
		r.Subs = old.Subs
		r.Maintainers, r.Disabled = old.Maintainers, old.Disabled
		// Personal is the owner's classification. A service refreshes its own
		// metadata on every start and cannot clear or set that owner choice.
		r.Personal = old.Personal
		// Metadata refreshes must not erase grants or lift a master refusal.
		// Explicit ACLs still replace the policy; Manage can clear either field.
		if r.Allow == nil {
			r.Allow = old.Allow
			r.NoMaster = r.NoMaster || old.NoMaster
		}
	}
	if err := b.validatePersonal(r); err != nil {
		return protocol.Record{}, err
	}
	r.At = time.Now()

	if enrolled {
		user := b.users[name]
		user.Name = name
		if user.State == "" {
			user.State = "active"
		}
		parsed, _ := protocol.ParseName(name)
		if parsed.Realm == "github" {
			for other, profile := range b.users {
				if other != name && profile.GithubUser == parsed.Local {
					return protocol.Record{}, fmt.Errorf("%w: GitHub login already belongs to another profile", ErrProfile)
				}
			}
			user.GithubUser = parsed.Local
		}
		b.users[name] = user
	}
	b.records[name] = r
	b.recheckInbox(name)
	b.ensure(name)
	if err := b.checkpoint(false); err != nil {
		return protocol.Record{}, err
	}
	// Public here, not in the face: the configuration is core's to guard,
	// and an answer that forgot to redact has already got out twice.
	return r.Public(), nil
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
	if err := b.acting(who); err != nil {
		return protocol.Record{}, err
	}
	r, known := b.record(n)
	if !known {
		// Creating here obeys what creating anywhere else obeys. This is a
		// creation path as much as registration is, and a rule only one of
		// them applied was not a rule: a name /register refused for being in
		// a vouched realm could be taken by configuring it instead, and then
		// issued a credential, with no key ever proved.
		// See docs/01-identity-and-roles.md#registration.
		if err := b.vouchedFor(n); err != nil {
			return protocol.Record{}, err
		}
		// Same defaults a bare registration gets: configuring is not a
		// second way to describe a service, only a way to give it config.
		r = protocol.Record{Name: n, Kind: "generic", Owner: who, Full: protocol.OverflowStrict}
	} else if !b.manages(who, r) {
		// Writing is the owner's, and the service's own. Reading is neither:
		// see Config.
		return protocol.Record{}, fmt.Errorf("%w: %s is %s's", ErrNotOwner, n, r.Owner)
	}
	r.Config = cfg
	r.At = time.Now()
	b.records[n] = r
	b.ensure(n)
	if err := b.checkpoint(false); err != nil {
		return protocol.Record{}, err
	}
	return r.Public(), nil
}

// Config reads one back — for the service itself and nobody else, its owner
// included. Setup data goes in and is used, not read back. An owner is told
// that configuring worked (Configure answers without the configuration),
// which is all an owner needs.
//
// It is in no listing either, so there is no other way to one.
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
	if err := b.acting(who); err != nil {
		return nil, err
	}
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
func (b *Bus) Lookup(caller, name string) (protocol.Record, bool) {
	n, err := canon(name)
	if err != nil {
		return protocol.Record{}, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	r, ok := b.records[n]
	// A name you may not see does not exist as far as you are concerned:
	// hiding it and refusing it are different answers, and discovery is the
	// half that hides. See docs/02-access.md#acl.
	if !ok || !b.may(caller, r) {
		return protocol.Record{}, false
	}
	return b.visible(caller, r), true
}

// OwnerOf reports who owns a name, and nothing more. It is a query, not the
// first half of a decision: the answer is stale the moment the lock is
// released, so anything that acts on ownership asks under the hold it acts in
// (IssueFor is what that looks like). Deciding out here is what let a transfer
// land between the answer and the act.
func (b *Bus) OwnerOf(name string) (string, bool) {
	n, err := canon(name)
	if err != nil {
		return "", false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	r, ok := b.record(n)
	return r.Owner, ok
}

// withLiveness answers the question a listing is really asked: not "does
// this name exist" but "is anything serving it". The daemon knows exactly —
// an inbox with a read outstanding has something on the other end.
//
// Held state, not stored state: it is attached on the way out and never
// written back, so nothing in the registry depends on who happened to be
// connected. Caller holds the lock.
func (b *Bus) withLiveness(name string, r protocol.Record) protocol.Record {
	r.Reading, r.Queued, r.In, r.Out = false, 0, 0, 0
	r.Dropped, r.Expired, r.Oldest, r.AtBound = 0, 0, "", false
	in, ok := b.inboxes[name]
	if !ok {
		return r
	}
	r.Queued, r.In, r.Out = len(in.queue), in.in, in.out
	// Answered here rather than left to be worked out from `queued`, because
	// a record that declares no bound takes the daemon's and a reader cannot
	// know what that is (docs/05-discovery.md#what-a-listing-answers).
	r.AtBound = len(in.queue) >= boundOf(r)
	r.Dropped, r.Expired = in.dropped, in.expired
	if len(in.queue) > 0 {
		// The head is the oldest: a queue is appended to and read from the
		// front (docs/04-messaging.md#inbox-queues).
		r.Oldest = time.Since(in.queue[0].At).Round(time.Second).String()
	}
	for _, w := range in.waiters {
		if !w.filtered {
			r.Reading = true
			break
		}
	}
	return r
}

// boundOf is how many messages a record's inbox may hold: what it declared,
// or the daemon's if it declared nothing.
func boundOf(r protocol.Record) int {
	if r.Bound == 0 {
		return maxQueue
	}
	return r.Bound
}

func (b *Bus) List(caller, kind string) []protocol.Record {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]protocol.Record, 0, len(b.records))
	for _, r := range b.records {
		if kind != "" && r.Kind != kind {
			continue
		}
		// Two things are held back: what the caller may not see at all,
		// and the configuration, which is nobody's but the service's — a
		// digest of it is what a query gets instead.
		if !b.may(caller, r) {
			continue
		}
		out = append(out, b.visible(caller, r))
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
	// Before the receiver is looked up, so that a caller who is nobody is not
	// told which names exist by which refusal it gets.
	if err := b.acting(from); err != nil {
		return protocol.Envelope{}, err
	}
	rec, known := b.records[to]
	if !known {
		return protocol.Envelope{}, fmt.Errorf("no such receiver: %s (%w)", to, ErrUnknown)
	}
	// Writing goes through the same two layers as reading: a principal that
	// may not see a service may not enqueue to it either, and is told so
	// rather than left to wonder. See docs/02-access.md#acl.
	if !b.may(from, rec) {
		return protocol.Envelope{}, fmt.Errorf("%s may not send to %s: %w", from, to, ErrNotAllow)
	}
	// The check is on the called name rather than on who is asking, so the
	// daemon owner is refused here as readily as a stranger.
	if err := b.suspension(to); err != nil {
		return protocol.Envelope{}, err
	}
	if rec.Disabled {
		return protocol.Envelope{}, ErrDisabled
	}
	// An answer that cannot be routed is the requester's problem to hear
	// about now. Registered, not live: the name owns a queue whether or not
	// anything is reading it. See docs/04-messaging.md#reply-routing.
	if e.ReplyTo != nil {
		back, err := canon(e.ReplyTo.Service)
		if err != nil {
			return protocol.Envelope{}, fmt.Errorf("reply-to: %w", err)
		}
		if _, known := b.records[back]; !known {
			return protocol.Envelope{}, fmt.Errorf("no such reply address: %s (%w)", back, ErrUnknown)
		}
		e.ReplyTo = &protocol.ReplyTo{Service: back, Topic: e.ReplyTo.Topic, Tag: e.ReplyTo.Tag}
	}
	// Receipts are a closed set: a caller decides whether a message is an
	// answer by looking at this field, so a third value would read as one.
	if e.Receipt != "" && e.Receipt != protocol.ReceiptAck && e.Receipt != protocol.ReceiptDone {
		return protocol.Envelope{}, fmt.Errorf("%w, not %q", ErrReceipt, e.Receipt)
	}
	e.To, e.From = to, from
	e.ID = newID()
	e.At = time.Now()
	d, err := life(rec.TTL, e.TTL)
	if err != nil {
		return protocol.Envelope{}, err
	}
	e.Expires = time.Time{}
	if d > 0 {
		e.Expires = e.At.Add(d)
	}
	// The caller's deadline travels with the request. The bus works out the
	// moment, like Expires, so both ends read one clock — and then does
	// nothing with it: giving up early is the service's call, not the
	// transport's. A caller may not state the moment itself, for the same
	// reason it may not state its own name.
	// See docs/04-messaging.md#request-and-reply.
	e.Deadline = time.Time{}
	if e.Wait != "" {
		w, err := time.ParseDuration(e.Wait)
		if err != nil || w <= 0 {
			return protocol.Envelope{}, fmt.Errorf("%w, not %q", ErrWait, e.Wait)
		}
		e.Deadline = e.At.Add(w)
	}
	// A queue topic is an inbox with a name, so publishing to one is an
	// ordinary send. A pub/sub topic keeps nothing of its own instead.
	if rec.Kind == protocol.KindTopic && rec.Mode == protocol.ModePubSub {
		return b.fanout(rec, e)
	}
	if err := b.deliver(rec, b.ensure(to), e); err != nil {
		return protocol.Envelope{}, err
	}
	b.note(e)
	return e, nil
}

// deliver puts one envelope into one inbox: into a waiter if one is there,
// into the queue otherwise, and into neither if the queue is full and its
// record said strict. It is the only door into an inbox, so a fan-out copy
// obeys the receiver's bound and overflow exactly as a send does.
// Caller holds the lock.
func (b *Bus) deliver(rec protocol.Record, in *inbox, e protocol.Envelope) error {
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
			in.in, in.out = in.in+1, in.out+1 // straight through: in and out at once
			w.ch <- e
			return nil
		}
	}
	// A full queue either refuses the new message or forgets the oldest, and
	// the receiver's record says which. Refusing is the default because
	// losing a job silently is worse than failing visibly; a ring is for the
	// streams where the newest matters most (docs/04-messaging.md#overflow).
	bound := boundOf(rec)
	if len(in.queue) >= bound {
		// What has already expired is not fullness: count it as expiry and
		// see whether there is room after all.
		b.prune(in, e.At)
	}
	if len(in.queue) >= bound {
		if rec.Full != protocol.OverflowRing {
			return fmt.Errorf("%w: %s holds %d", ErrFull, rec.Name, len(in.queue))
		}
		in.queue = take(in.queue, 0)
		in.dropped++ // a real loss, so `status` and `ls` report it
	}
	in.queue = append(in.queue, e)
	in.in++
	return nil
}

// fanout is what a pub/sub topic does instead of holding a queue: one copy
// into each subscriber's own inbox, where that subscriber's bound, overflow,
// TTL and reader apply. The topic itself keeps nothing.
//
// One subscriber cannot stop the topic: a copy that will not fit is counted
// as a drop and the others still go. A publisher a stopped reader can block
// is a queue, and a queue topic is what that caller wanted.
// Caller holds the lock.
func (b *Bus) fanout(topic protocol.Record, e protocol.Envelope) (protocol.Envelope, error) {
	b.ensure(topic.Name).in++ // publications accepted; none is kept
	b.note(e)
	for _, s := range topic.Subs {
		sub, known := b.records[s]
		// Asked again at every publish, not only at subscribe: access taken
		// away has to stop the copies, or subscribing would be a way to go
		// on reading a topic that stopped allowing you.
		if !known || !b.active(s) || sub.Disabled || !b.may(s, topic) {
			continue
		}
		c := e
		c.To = s
		if err := b.deliver(sub, b.ensure(s), c); err != nil {
			// The copy that would not fit is the SUBSCRIBER's loss: its
			// bound refused it, and its page is where that has to show.
			b.ensure(s).dropped++
		}
	}
	return e, nil
}

// Subscribe puts a name on a pub/sub topic, or takes it off. A subscription
// is a record and lives on the topic, so it travels in the snapshot and
// outlives a restart — and the copies land in the subscriber's own inbox,
// which is why the subscriber has to be registered first.
// See docs/04-messaging.md#push-and-pull.
func (b *Bus) Subscribe(caller, topic string, on bool) (protocol.Record, error) {
	who, err := canon(caller)
	if err != nil {
		return protocol.Record{}, err
	}
	n, err := canon(topic)
	if err != nil {
		return protocol.Record{}, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.acting(who); err != nil {
		return protocol.Record{}, err
	}
	r, known := b.records[n]
	// A topic you may not see does not exist as far as you are concerned,
	// exactly as a lookup answers. See docs/02-access.md#acl.
	if !known || !b.may(who, r) {
		return protocol.Record{}, fmt.Errorf("%w: %s", ErrUnknown, n)
	}
	if err := b.ownerSuspension(n); on && err != nil {
		return protocol.Record{}, err
	}
	if on && r.Disabled {
		return protocol.Record{}, ErrDisabled
	}
	if r.Kind != protocol.KindTopic || r.Mode != protocol.ModePubSub {
		return protocol.Record{}, fmt.Errorf("%w: only a pubsub topic has subscribers, and %s is not one", ErrMode, n)
	}
	if _, me := b.records[who]; !me {
		return protocol.Record{}, fmt.Errorf("%w: register %s first, so its copies have somewhere to land", ErrUnknown, who)
	}
	r.Subs = drop1(r.Subs, who)
	if on {
		r.Subs = append(r.Subs, who)
	}
	b.records[n] = r
	if err := b.checkpoint(false); err != nil {
		return protocol.Record{}, err
	}
	return b.withLiveness(n, r.Public()), nil
}

// drop1 returns the list without s, in a slice of its own: the stored record
// shares its backing array with whatever a query already handed out.
func drop1(list []string, s string) []string {
	out := list[:0:0]
	for _, x := range list {
		if x != s {
			out = append(out, x)
		}
	}
	return out
}

// life is how long this message is worth delivering: what the sender asked
// for, never more than what the receiver's queue allows. Either may be
// unset; unset on both means it waits until consumed.
// See docs/04-messaging.md#message-ttl.
func life(queue, msg string) (time.Duration, error) {
	var q, m time.Duration
	var err error
	if queue != "" {
		q, _ = time.ParseDuration(queue) // validated at Register
	}
	if msg != "" {
		if m, err = time.ParseDuration(msg); err != nil || m <= 0 {
			return 0, fmt.Errorf("%w, not %q", ErrTTL, msg)
		}
	}
	switch {
	case m == 0:
		return q, nil
	case q == 0 || m < q:
		return m, nil
	default:
		// Asking for longer than the queue keeps is not an error — the queue
		// simply does not keep it that long. The receiver owns its retention
		// the same way it owns its overflow.
		return q, nil
	}
}

// prune drops what has outlived its moment, counting it apart from overflow.
// It runs where the queue is already being walked, so the hot path pays for
// nothing it was not already doing.
func (b *Bus) prune(in *inbox, now time.Time) {
	if len(in.queue) == 0 {
		return
	}
	kept := in.queue[:0]
	for _, e := range in.queue {
		if !e.Expires.IsZero() && now.After(e.Expires) {
			in.expired++
			continue
		}
		kept = append(kept, e)
	}
	for i := len(kept); i < len(in.queue); i++ {
		in.queue[i] = protocol.Envelope{}
	}
	in.queue = kept
}

// Consume takes one message, at-most-once: it is handed over and gone.
// A filter waits for one topic+tag; an unfiltered read takes the next of
// anything, and there may be only one of those at a time unless the readers
// asked to share the inbox.
// See docs/04-messaging.md#one-reader-per-inbox.
func (b *Bus) Consume(ctx context.Context, name, topic, tag string, filtered, share bool) (protocol.Envelope, error) {
	return b.ConsumeAs(ctx, name, name, topic, tag, filtered, share)
}

func (b *Bus) ConsumeAs(ctx context.Context, caller, name, topic, tag string, filtered, share bool) (protocol.Envelope, error) {
	name, err := canon(name)
	if err != nil {
		return protocol.Envelope{}, err
	}
	b.mu.Lock()
	if err := b.acting(caller); err != nil {
		b.mu.Unlock()
		return protocol.Envelope{}, err
	}
	// An inbox belongs to a registered name. Creating one for whoever asks
	// would let any caller name leave a permanent entry behind — and the
	// wait could never end anyway, because Send refuses an unknown
	// receiver, so nothing can ever arrive in it.
	if _, known := b.records[name]; !known {
		b.mu.Unlock()
		return protocol.Envelope{}, fmt.Errorf("no inbox for %s: register it first (%w)", name, ErrUnknown)
	}
	rec := b.records[name]
	if !b.may(caller, rec) {
		b.mu.Unlock()
		return protocol.Envelope{}, ErrNotAllow
	}
	// Q63 permits draining an inactive name's own inbox by an active,
	// authorized caller. Only a separate owner's suspension blocks this path.
	// See docs/01-identity-and-roles.md#user-states.
	if err := b.ownerSuspension(name); err != nil {
		b.mu.Unlock()
		return protocol.Envelope{}, err
	}
	if rec.Disabled {
		b.mu.Unlock()
		return protocol.Envelope{}, ErrDisabled
	}
	in := b.ensure(name)
	// Never handed to a consumer: the check is here, where the message would
	// otherwise be handed over. See docs/04-messaging.md#message-ttl.
	b.prune(in, time.Now())

	for i, e := range in.queue {
		if !filtered || (e.Topic == topic && e.Tag == tag) {
			in.queue = take(in.queue, i)
			in.out++
			b.mu.Unlock()
			return e, nil
		}
	}
	// Two unfiltered readers are a pool when they both said so, and a bug
	// otherwise — a session's CLI reading the queue its push adapter is
	// reading. Asking is what tells the daemon which it has, and it costs a
	// worker one word. Either side declining keeps the old refusal: a reader
	// that wants the inbox to itself still gets it.
	// See docs/04-messaging.md#one-reader-per-inbox.
	if !filtered {
		for _, w := range in.waiters {
			if !w.filtered && !(share && w.share) {
				b.mu.Unlock()
				return protocol.Envelope{}, ErrTwoReads
			}
		}
	}
	w := &waiter{topic: topic, tag: tag, filtered: filtered, share: share, ch: make(chan protocol.Envelope, 1), caller: caller, stopped: make(chan error, 1)}
	in.waiters = append(in.waiters, w)
	b.mu.Unlock()

	select {
	case err := <-w.stopped:
		return protocol.Envelope{}, err
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
	in := b.inboxes[name]
	if in == nil {
		return protocol.Envelope{}, false
	}
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
	Expired  int    `json:"expired"` // outlived their TTL since start
	// Absent unless it has something to say: a first start has no previous
	// stop to have been clean or otherwise.
	Unclean bool `json:"unclean,omitempty"` // the last run did not stop cleanly
	// Refusals by kind, and only the kinds that have happened. A reason
	// with a zero beside it is noise on every other node.
	Refused map[string]int `json:"refused,omitempty"`
}

// Uptime reads only the process start time, immutable after New. Public node
// identity must not scan private inbox state to obtain this one value.
func (b *Bus) Uptime() string { return time.Since(b.started).Round(time.Second).String() }

func (b *Bus) Status() Status {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := Status{
		Up:       b.Uptime(),
		Services: len(b.records),
	}
	// Summed, not counted a second time: the node's total has one home, and
	// it is the inboxes that lost the work.
	for _, in := range b.inboxes {
		s.Queued += len(in.queue)
		s.Waiting += len(in.waiters)
		s.Dropped += in.dropped
		s.Expired += in.expired
	}
	s.Unclean = b.unclean
	if len(b.refused) > 0 {
		s.Refused = make(map[string]int, len(b.refused))
		for k, n := range b.refused {
			s.Refused[k] = n
		}
	}
	return s
}

// Owned lists the names this caller is answerable for: the records they own,
// and their own name, which holds a credential whether or not anything is
// registered under it. It is the registry half of the dashboard's my-names
// view — the credential half is the token store's
// (docs/05-discovery.md#what-it-shows).
func (b *Bus) Owned(caller string) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.acting(caller) != nil {
		return []string{}
	}
	out := []string{caller}
	for name, r := range b.records {
		if name != caller && r.Owner == caller {
			out = append(out, name)
		}
	}
	return out
}

// Refuse records that a call was turned away, and for what reason. The face
// calls it because one of the kinds — a credential that is not one — is
// refused before core is ever reached
// (docs/02-access.md#what-a-call-carries).
func (b *Bus) Refuse(kind string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refused[kind]++
}

func newID() string {
	var b [8]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

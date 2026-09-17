package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// Envelope is what the bus reads, counts and routes. The body is carried but
// not interpreted; from R1 it is ciphertext.
// See docs/02-access.md#trust-boundary.
type Envelope struct {
	ID    string    `json:"message_id"`
	From  string    `json:"from"`
	To    string    `json:"to"`
	Topic string    `json:"topic,omitempty"`
	Tag   string    `json:"tag,omitempty"`
	Body  string    `json:"body"`
	At    time.Time `json:"at"`

	// A receipt is an ordinary message that happens to say something about
	// another one: ack = got it, done = finished. Both come from the
	// receiver, carry the original topic and tag so the sender's wait matches
	// them, and name the message they are about.
	// See docs/04-messaging.md#receipts.
	Receipt string `json:"receipt,omitempty"` // "", "ack" or "done"
	Re      string `json:"re,omitempty"`      // the message_id this is about

	// ReplyTo sends the answer somewhere other than back to the sender. The
	// default route needs no field — the sender's own name with this topic
	// and tag — so this is set only when the answer belongs to a third
	// party. Its service must be REGISTERED when the request is accepted:
	// an answer that cannot be routed is a fact the requester should learn
	// now, not when it bounces.
	// See docs/04-messaging.md#reply-routing.
	ReplyTo *ReplyTo `json:"reply_to,omitempty"`

	// TTL is how long this message is worth delivering, stated by the
	// sender; the receiver's queue bounds it. A question nobody should
	// answer late sets one.
	// See docs/04-messaging.md#message-ttl.
	TTL string `json:"ttl,omitempty"`

	// Expires is when the daemon will stop delivering it, worked out once
	// at accept from the two TTLs. Absolute, so the queue compares rather
	// than parses — the sender states a duration, the bus keeps a moment.
	Expires time.Time `json:"expires,omitempty"`

	// Wait is how long the caller will wait for an answer, and Deadline is
	// the moment that lands on, worked out at accept the way Expires is.
	// It travels so that a service can see the answer is already too late
	// and not do the work at all.
	//
	// It is the CALLER's, which is what makes it a second pair rather than
	// a spelling of the TTL: the receiver's queue bounds a TTL and does not
	// bound this, and the bus never acts on it — a message whose caller has
	// gone is still delivered, because only the service knows whether the
	// work is worth doing for somebody else.
	// See docs/04-messaging.md#request-and-reply.
	Wait     string    `json:"wait,omitempty"`
	Deadline time.Time `json:"deadline,omitempty"`
}

// TooLate says the caller has stopped waiting. No deadline means no answer
// was promised by any moment, which is not the same as a deadline that has
// passed — so a message without one is never too late.
func (e Envelope) TooLate(now time.Time) bool {
	return !e.Deadline.IsZero() && now.After(e.Deadline)
}

// ReplyTo is a route, not a promise: registered says the name owns a queue,
// never that anything is reading it.
// See docs/04-messaging.md#reply-routing.
type ReplyTo struct {
	Service string `json:"service"`
	Topic   string `json:"topic,omitempty"`
	Tag     string `json:"tag,omitempty"`
}

// Record is a registered service, agent or topic. Registering is pushing a
// description; the thing itself need not know the bus exists.
// See docs/03-services-and-topics.md.
type Record struct {
	Name string `json:"name"`
	Kind string `json:"kind"`           // generic, agent, topic
	Addr string `json:"addr,omitempty"` // host:port, a path, a URL

	// Proto says HOW to call this, where Addr says where. Empty is the
	// answer for almost everything: no protocol means an ordinary agent-bus
	// service — send to its name and the daemon delivers to its inbox.
	// Anything else names a protocol the *caller* speaks directly: a hint
	// in the registry, not something the daemon implements or checks.
	// /etc/services is the suggested vocabulary and cannot be more than
	// that — it is outdated and incomplete, and much of what gets
	// registered here is not in it.
	// See docs/03-services-and-topics.md#how-to-call-it.
	Proto string `json:"protocol,omitempty"`

	Descr string `json:"descr,omitempty"`    // what ls and the MCP catalog show
	Mode  string `json:"mode,omitempty"`     // topics only: queue or pubsub
	Full  string `json:"overflow,omitempty"` // ring or strict; strict if unset

	// TTL and Bound are the queue's, declared on the record like overflow:
	// how long anything in it is worth keeping, and how much of it there
	// may be. Both unset take the daemon's defaults. A message may ask for
	// less than TTL and never for more.
	// See docs/03-services-and-topics.md#topics.
	TTL   string `json:"ttl,omitempty"`
	Bound int    `json:"bound,omitempty"`

	Owner       string `json:"owner"`
	Maintainers string `json:"maintainers,omitempty"`
	// Personal groups a service in the owner's web view. It changes neither
	// delivery nor access; core only enforces which authority assignments may
	// coexist with it. See docs/03-services-and-topics.md#personal-and-shared.
	Personal bool      `json:"personal,omitempty"`
	Disabled bool      `json:"disabled,omitempty"`
	At       time.Time `json:"at"`

	// Allow is the service ACL: who may see and use this. It is a field on
	// the record, not something inside Config, because the daemon enforces
	// it and will not read a private configuration. `*` means anyone who
	// can authenticate; empty means only management authority and the record's
	// own principal. NoMaster prevents the master layer from adding access to
	// a non-empty ACL.
	// See docs/02-access.md#acl.
	Allow    []string `json:"allow,omitempty"`
	NoMaster bool     `json:"no_master,omitempty"`

	// Subs is who receives a copy of what is published to a pub/sub topic.
	// A subscription is a record, so it lives here rather than in a
	// connection: it survives a restart with the rest of the registry, and
	// a subscriber that is down keeps its backlog in its own inbox.
	// Written by Subscribe alone — a registration never carries it.
	// See docs/04-messaging.md#push-and-pull.
	Subs []string `json:"subs,omitempty"`

	// Config is what a service template was configured with. It is opaque:
	// the bus checks that it is JSON and never reads inside — no server,
	// user, mailbox or credential is looked for. It leaves the daemon for
	// one caller only, the service it belongs to; every other answer
	// carries ConfigSHA in its place.
	// See docs/03-services-and-topics.md#configuring-a-template.
	Config json.RawMessage `json:"config,omitempty"`

	// ConfigSHA is what a query gets instead: enough to see that a service
	// is configured, that a write landed, and that two are the same, without
	// handing the configuration to anyone.
	ConfigSHA string `json:"config_sha,omitempty"`

	// Live state, filled in on the way out of a query and never stored:
	// a registry record says a name exists, while these fields report what
	// the daemon observes now. A registration is only a
	// description — "there is a MySQL on host:port" registers fine and
	// nothing on this bus answers for it — so "is it in the registry?" and
	// "can I call it through the daemon?" are different questions.
	// See docs/05-discovery.md#what-a-listing-answers.
	CanManage   bool `json:"can_manage,omitempty"`
	CanTransfer bool `json:"can_transfer,omitempty"`
	Reading     bool `json:"reading,omitempty"` // an unfiltered read is outstanding; retained for compatibility
	Readers     *int `json:"readers,omitempty"` // all outstanding reads, filtered and unfiltered together; nil means unobserved
	Queued      int  `json:"queued,omitempty"`  // messages waiting in it
	In          int  `json:"in,omitempty"`      // accepted for it since the daemon started
	Out         int  `json:"out,omitempty"`     // handed to a reader of it since then
	// Loss, per inbox rather than per daemon: a total tells an operator that
	// something is losing work, and not which name to go and look at.
	Dropped int `json:"dropped,omitempty"` // lost to its overflow since then
	Expired int `json:"expired,omitempty"` // outlived their TTL in it since then
	// How long the message at the head of its queue has been waiting. A
	// backlog nobody reads is what an incident looks like, and a count alone
	// cannot say whether that queue is busy or stalled.
	Oldest string `json:"oldest,omitempty"`
	// AtBound says the queue is at the limit it is allowed, so the next
	// message is refused or something is lost. The daemon answers it because
	// a record that declares no bound takes the daemon's, and a reader has
	// no way to know what that is.
	AtBound bool `json:"at_bound,omitempty"`
}

// AccountMapping says which bus principal one local OS account's private
// socket authenticates as. It is configuration, never a credential: the
// socket's ownership is the credential. See docs/09-setup.md#local-users.
type AccountMapping struct {
	Account   string `json:"account"`
	Principal string `json:"principal"`
}

// AccountMappings is the administrative view of the durable map. A change is
// persisted immediately but listeners belong to the supervisor, so the view
// says when a full daemon restart is still needed to apply it.
type AccountMappings struct {
	Mappings        []AccountMapping `json:"mappings"`
	RestartRequired bool             `json:"restart_required"`
}

// Public is what a record looks like to anyone but the service itself: the
// configuration replaced by a digest of it. Every answer that carries a
// record goes through here — a listing, a lookup, a registration and the
// answer to configuring one — because leaving it out has already been the
// same omission twice.
//
// The digest is what makes a query useful without exposing anything
// (ConfigSHA says what it answers).
//
// It is over the bytes AS STORED, which the bus has already compacted, so
// reformatting a configuration file does not look like changing it — and
// `sha256sum cfg.json` will not match one that has whitespace in it. Being a
// digest of the bytes, a SHORT, GUESSABLE configuration is recoverable by
// trying candidates; the answer to that is sealing, not a longer hash.
// See docs/03-services-and-topics.md#configuring-a-template.
func (r Record) Public() Record {
	if len(r.Config) > 0 {
		sum := sha256.Sum256(r.Config)
		r.ConfigSHA = hex.EncodeToString(sum[:])
		r.Config = nil
	}
	return r
}

// Receipt values. A closed set: anything else is not a receipt.
// See docs/04-messaging.md#receipts.
const (
	ReceiptAck  = "ack"
	ReceiptDone = "done"
)

// What a full queue does, declared per record, inboxes included.
// See docs/04-messaging.md#overflow.
const (
	OverflowStrict = "strict" // refuse the send, and say so
	OverflowRing   = "ring"   // drop the oldest to make room
)

// Topic kinds. A queue topic is an inbox with a name, and the mode is stored
// rather than a second meaning of subscription being invented.
// See docs/03-services-and-topics.md#topics.
const (
	KindTopic  = "topic"
	ModeQueue  = "queue"
	ModePubSub = "pubsub"
)

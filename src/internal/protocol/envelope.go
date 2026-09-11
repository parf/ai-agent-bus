package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// Envelope is what the bus reads, counts and routes. The body is carried but
// not interpreted; from MVP it is ciphertext. See docs/04-messaging.md.
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
	// Anything else names a protocol the *caller* speaks directly, and the
	// bus only passes the word along: it is a hint in the registry, not
	// something the daemon implements or checks. /etc/services is the
	// suggested vocabulary and cannot be more than that — it is outdated
	// and incomplete, and much of what gets registered here is not in it.
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

	Owner string    `json:"owner"`
	At    time.Time `json:"at"`

	// Config is what a service template was configured with. It is opaque:
	// the bus checks that it is JSON and never reads inside, so no field of
	// it means anything here — no server, user, mailbox or credential is
	// looked for. It leaves the daemon for one caller only, the service it
	// belongs to; every other answer carries ConfigSHA in its place.
	// See docs/03-services-and-topics.md#configuring-a-template.
	Config json.RawMessage `json:"config,omitempty"`

	// ConfigSHA is what a query gets instead: enough to see that a service
	// is configured, that a write landed, and that two are the same, without
	// handing the configuration to anyone.
	ConfigSHA string `json:"config_sha,omitempty"`

	// Live state, filled in on the way out of a query and never stored:
	// a registry record says a name exists, and these two say whether
	// anything is actually serving it. A registration is only a
	// description — "there is a MySQL on host:port" registers fine and
	// nothing on this bus answers for it — so "is it in the registry?" and
	// "can I call it through the daemon?" are different questions.
	// See docs/05-discovery.md#what-a-listing-answers.
	Reading bool `json:"reading,omitempty"` // a read on its inbox is outstanding now
	Queued  int  `json:"queued,omitempty"`  // messages waiting in it
}

// Public is what a record looks like to anyone but the service itself: the
// configuration replaced by a digest of it. Every answer that carries a
// record goes through here — a listing, a lookup, a registration and the
// answer to configuring one — because leaving it out has already been the
// same omission twice.
//
// The digest is what makes a query useful without exposing anything: an owner
// who cannot read a configuration back can still see that one is there, that
// a write landed, and that two services hold the same one.
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

// Topic kinds. A queue topic is an inbox with a name; pub/sub is MVP, and
// PoC stores the mode rather than inventing a second meaning of subscription.
// See docs/12-stages.md#poc.
const (
	KindTopic  = "topic"
	ModeQueue  = "queue"
	ModePubSub = "pubsub"
)

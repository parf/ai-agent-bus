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
}

// Record is a registered service, agent or topic. Registering is pushing a
// description; the thing itself need not know the bus exists.
// See docs/03-services-and-topics.md.
type Record struct {
	Name  string    `json:"name"`
	Kind  string    `json:"kind"`               // generic, agent, topic
	Addr  string    `json:"addr,omitempty"`     // host:port, a path, a URL
	Descr string    `json:"descr,omitempty"`    // what ls and the MCP catalog show
	Mode  string    `json:"mode,omitempty"`     // topics only: queue or pubsub
	Full  string    `json:"overflow,omitempty"` // ring or strict; strict if unset
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

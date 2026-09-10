package protocol

import "time"

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
	Kind  string    `json:"kind"`            // generic, agent, topic
	Addr  string    `json:"addr,omitempty"`  // host:port, a path, a URL
	Descr string    `json:"descr,omitempty"` // what ls and the MCP catalog show
	Mode  string    `json:"mode,omitempty"`  // topics only: queue or pubsub
	Owner string    `json:"owner"`
	At    time.Time `json:"at"`
}

// Topic kinds. A queue topic is an inbox with a name; pub/sub is MVP, and
// PoC stores the mode rather than inventing a second meaning of subscription.
// See docs/12-stages.md#poc.
const (
	KindTopic  = "topic"
	ModeQueue  = "queue"
	ModePubSub = "pubsub"
)

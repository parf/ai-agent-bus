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
}

// Record is a registered service, agent or topic. Registering is pushing a
// description; the thing itself need not know the bus exists.
// See docs/03-services-and-topics.md.
type Record struct {
	Name  string    `json:"name"`
	Kind  string    `json:"kind"`            // generic, agent, topic
	Addr  string    `json:"addr,omitempty"`  // host:port, a path, a URL
	Descr string    `json:"descr,omitempty"` // what ls and the MCP catalog show
	Owner string    `json:"owner"`
	At    time.Time `json:"at"`
}

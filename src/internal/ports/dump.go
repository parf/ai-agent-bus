package ports

// The dump port: what a restart must not lose, and nothing about how it is
// written down. See docs/04-messaging.md#durability.

import (
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Queue is one inbox as it stood: what is still in it, and what has been
// through it. The counters are part of the state — a queue that was drained
// and one nobody ever wrote to read the same without them
// (docs/05-discovery.md#what-a-listing-answers).
type Queue struct {
	Name    string
	In, Out int
	// Loss belongs to the inbox that suffered it, so a restart puts it back
	// where it happened rather than on a node-wide total.
	Dropped, Expired int
	Messages         []protocol.Envelope
}

// Snapshot is the daemon's memory at a moment. Registry records travel with
// the queues: a reloaded inbox that belongs to no record is a backlog nobody
// can read. The git snapshot is backup and peer sync, not this
// (docs/03-services-and-topics.md#registry-sync).
type Snapshot struct {
	At      time.Time
	Clean   bool // written by a graceful stop; false means the run was still going
	Records []protocol.Record
	Queues  []Queue
}

// Dump snapshots in-memory state and reads it back.
type Dump interface {
	// Save replaces the snapshot. The previous one is not kept: a reload
	// that could pick an older file would deliver consumed messages twice.
	Save(Snapshot) error
	// Load reports false when there is nothing to reload, which is how a
	// first start is told from one that follows a death.
	Load() (Snapshot, bool, error)
}

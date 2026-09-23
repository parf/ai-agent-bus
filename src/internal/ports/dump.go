package ports

// The durable-state port: what a restart must not lose, and nothing about how
// it is written down. See docs/constitution.md#persistence-and-loading and
// docs/04-messaging.md#durability.

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

// Snapshot is the daemon's durable state as a store loads it. Registry
// records travel with the queues: a reloaded inbox that belongs to no record
// is a backlog nobody can read.
type Snapshot struct {
	// OwnerEstablished distinguishes a store that has never had a daemon Owner
	// from one whose missing Owner is damage.
	OwnerEstablished bool   `json:"owner_established,omitempty"`
	Owner            string `json:"owner,omitempty"`
	// AccountsEstablished separates a first run, whose command-line mappings
	// seed the map, from an intentionally empty map.
	AccountsEstablished bool                      `json:"accounts_established,omitempty"`
	Accounts            []protocol.AccountMapping `json:"accounts,omitempty"`
	Users               []protocol.User           `json:",omitempty"`
	At                  time.Time
	Clean               bool // the last run stopped gracefully; false means it was still going
	Records             []protocol.Record
	Groups              map[string][]string `json:",omitempty"`
	Queues              []Queue
	// NextRecordID and NextUserID are the next internal IDs to hand out.
	// They only grow, so an ID is never reused after its entity is removed
	// (docs/constitution.md#common-record-fields).
	NextRecordID, NextUserID uint32
}

// Change is one management write: every entity it touches, as it will be
// once committed. A nil value removes that entity. Nothing outside the change
// is rewritten, so an edit of one record costs one record
// (docs/10-modules.md#07-implementation-requirements).
type Change struct {
	// Owner, when set, is the daemon Owner after the change.
	Owner *string
	// Accounts, when set, replaces the local-account map.
	Accounts map[string]string
	Users    map[string]*protocol.User
	Records  map[string]*protocol.Record
	Groups   map[string]*[]string
	// DropQueues names queues whose durable state goes with their record.
	DropQueues []string
	// Credentials rebinds the named credentials to a new pair, or removes
	// them when nil, in this same transaction: a transfer that commits
	// without its tokens, or a removal that leaves them, is not a change.
	Credentials map[string]*CredentialPair
	// NextRecordID and NextUserID, when set, are the new high-water marks.
	NextRecordID, NextUserID *uint32
}

// Empty reports whether the change writes nothing.
func (c Change) Empty() bool {
	return c.Owner == nil && c.Accounts == nil && len(c.Users) == 0 &&
		len(c.Records) == 0 && len(c.Groups) == 0 && len(c.DropQueues) == 0 && len(c.Credentials) == 0 &&
		c.NextRecordID == nil && c.NextUserID == nil
}

// Store keeps the durable state. Management writes commit one Change as one
// transaction before the daemon publishes it; queue contents and counters
// travel separately, on their own cadence, never once per message.
type Store interface {
	// Load reads the whole durable state. An unreadable, incompatible or
	// damaged store is an error, never an empty node.
	Load() (Snapshot, error)
	// Commit applies one change atomically: all of it, or none of it.
	Commit(Change) error
	// SaveQueues replaces the durable state of the queues given, and records
	// whether this save is the graceful stop's.
	SaveQueues(queues []Queue, clean bool) error
	Close() error
}

package ports

import "errors"

const EventMaxID = uint64(1<<53 - 1)

var ErrEventStateInvalid = errors.New("invalid event state")

// EventEntry is the shared eventlog entry model. Record IDs, never names, key
// storage so a new registration cannot inherit another registration's log.
type EventEntry struct {
	ID     uint64 `json:"id"`
	Level  string `json:"level"`
	Text   string `json:"text"`
	At     string `json:"at"`
	LastAt string `json:"last_at"`
	Count  uint64 `json:"count"`
}

// EventIDStore reserves blocks durably before any of their IDs are issued.
// Reservations are monotonic, including after clean or an uncertain write.
type EventIDStore interface {
	EventHighWater(recordID uint32) (uint64, error)
	ReserveEventIDs(recordID uint32, through uint64) error
}

// EventAlertStore is optional until the durable-alert adapter is assembled.
// Replace atomically stores a whole bounded alert snapshot, including removals.
// Load errors are fatal to that log, never interpreted as an empty log.
type EventAlertStore interface {
	LoadEventAlerts(recordID uint32) (alerts []EventEntry, active bool, err error)
	ReplaceEventAlerts(recordID uint32, alerts []EventEntry, active bool) error
}

// EventStatePruner removes private state of a never-reused, retired record ID.
// It is also called after an in-flight old-ID operation finishes.
type EventStatePruner interface{ DropEventState(recordID uint32) error }

// ErrSubscriptionExists is AddSubscription's answer for a taken id.
var ErrSubscriptionExists = errors.New("subscription id exists")

// SubscriptionStore keeps the subscriptions of agent logs: one row per
// subscription, its stored JSON opaque to the store, keyed by an id that is
// unique across sources and by the internal ID of the source record, which is
// never reused. The rows hold names and filters, never a credential. Nothing
// here replays: a subscription starts when it is added.
type SubscriptionStore interface {
	// ListSubscriptions returns one source's rows by id.
	ListSubscriptions(sourceID uint32) (map[string][]byte, error)
	// AddSubscription stores a new row; ErrSubscriptionExists when the id is taken.
	AddSubscription(sourceID uint32, id string, value []byte) error
	// DeleteSubscription removes one source's row and says whether there was one.
	DeleteSubscription(sourceID uint32, id string) (bool, error)
	// SetSubscriptionCounts writes the sent and dropped counters of one row.
	SetSubscriptionCounts(sourceID uint32, id string, sent, dropped int64) error
	// FindSubscription says which source holds an id.
	FindSubscription(id string) (sourceID uint32, found bool, err error)
	// SubscriptionSources lists the sources where creator holds a row.
	SubscriptionSources(creator string) ([]uint32, error)
}

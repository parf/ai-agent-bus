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

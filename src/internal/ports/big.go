package ports

import (
	"errors"
	"io"
	"time"
)

// The big-key port: named values too large for a message, held outside the
// bus's queues in memory or a file. See Plans/R1.0-Release/big-keys.md.

// BigInfo is one stored big value.
type BigInfo struct {
	Name    string
	Size    int64
	SHA256  string // lowercase hex of the whole value
	Held    string // "file" or "memory"
	Expires time.Time
}

// BigLimits are the caps a put is checked against, in bytes and names.
type BigLimits struct {
	Value, Record, Total int64
	Names                int
}

var (
	// ErrBigTooLarge is a value above the per-key cap.
	ErrBigTooLarge = errors.New("that is more than one big key holds")
	// ErrBigFull is a put that would pass a record's or the daemon's total, or
	// a record's key count.
	ErrBigFull = errors.New("that would pass the big-key limit")
	// ErrBigEmpty is a put of no bytes.
	ErrBigEmpty = errors.New("a big key holds at least one byte")
	// ErrBigHash is a put whose bytes do not match the SHA-256 the caller gave.
	ErrBigHash = errors.New("the bytes do not match the SHA-256 given")
)

// BigStore keeps big values by record ID and name. A record ID is never
// reused, so a removed record's keys are unreachable until they expire or
// BigDrop removes them. Expired keys are never returned.
type BigStore interface {
	// BigPut reads data to its end, refusing more than lim.Value bytes, none,
	// or a mismatch with sum when sum is not empty, and replaces any value
	// under name. replaced says whether one was there.
	BigPut(record uint32, name string, data io.Reader, sum string, memory bool, expires, now time.Time, lim BigLimits) (info BigInfo, replaced bool, err error)
	// BigGet opens one value; found is false when it is absent or expired.
	BigGet(record uint32, name string, now time.Time) (rc io.ReadCloser, info BigInfo, found bool, err error)
	// BigList is the record's live keys, sorted by name.
	BigList(record uint32, now time.Time) ([]BigInfo, error)
	// BigDelete removes one value and says whether it was there.
	BigDelete(record uint32, name string, now time.Time) (bool, error)
	// BigDrop removes every value of one record.
	BigDrop(record uint32) error
}

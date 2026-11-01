// Bigfiles: named values too large for a message, passed by key name. The
// registry decides who may reach one: the record's Owner, Maintainers and own
// Agent upload and delete, and download and list too; whoever its allow list
// admits downloads and lists only (Plans/R1.0-Release/bigfile.md#authority).
package core

import (
	"errors"
	"fmt"
	"io"
	"time"
	"unicode/utf8"

	"github.com/parf/ai-agent-bus/internal/ports"
)

var (
	ErrBigAbsent  = errors.New("no bigfile is stored under that name")
	ErrBigValue   = errors.New("that is not a bigfile this store takes")
	ErrBigNoStore = errors.New("this daemon keeps no bigfiles")
)

// Limits on bigfiles.
const (
	BigMaxName    = 256
	BigMaxValue   = 32 << 20
	BigMaxRecord  = 256 << 20
	BigMaxTotal   = 1 << 30
	BigMaxNames   = 100
	BigDefaultTTL = 24 * time.Hour
	BigMaxTTL     = 30 * 24 * time.Hour
)

// SetBigStore binds the holder of big values, at start.
func (b *Bus) SetBigStore(s ports.BigStore) {
	b.mu.Lock()
	b.big = s
	b.unlock()
}

// BigUnavailable says bigfiles cannot be held, and why: the daemon stays up,
// reports it as an error and refuses the four bigfile calls, naming the cause.
// reopen, when not nil, is tried again at most once a minute by a call that
// arrives, so a repaired directory recovers without a restart.
func (b *Bus) BigUnavailable(cause error, reopen func() (ports.BigStore, error)) {
	b.mu.Lock()
	b.big, b.bigCause, b.bigReopen, b.bigTried = nil, cause, reopen, b.clock()
	b.unlock()
	b.report(ports.Error, "bigfile directory could not be prepared; bigfile calls are refused: %s", cause)
}

// bigStore finds the holder and the record's ID. write asks for the authority
// to upload and delete; reading also takes the record's allow list.
func (b *Bus) bigStore(caller, record, name string, write bool) (ports.BigStore, uint32, error) {
	if name == "" || len(name) > BigMaxName || !utf8.ValidString(name) {
		return nil, 0, fmt.Errorf("%w: a name is 1 to %d bytes of UTF-8", ErrBigValue, BigMaxName)
	}
	record = LockRecord(record)
	b.mu.Lock()
	r, live := b.entity(record)
	may := live && b.resourceManages(caller, r)
	if live && !write {
		// The daemon's own record admits everyone to send to it; that is not a
		// grant to its bigfiles, as it is none to its key-value store.
		may = r.Name != DaemonAgent && b.may(caller, r)
	}
	if b.big == nil && b.bigReopen != nil && b.clock().Sub(b.bigTried) >= time.Minute {
		b.bigTried = b.clock()
		if s, err := b.bigReopen(); err == nil {
			b.big, b.bigCause, b.bigReopen = s, nil, nil
			b.report(ports.Warning, "bigfile directory is usable again; bigfile calls are served")
			// The start-up prune saw no holder; keys of records removed since
			// would keep their quota.
			b.pruneBigOrphans()
		}
	}
	store, cause := b.big, b.bigCause
	b.unlock()
	switch {
	case !live:
		return nil, 0, ErrUnknown
	case !may:
		return nil, 0, ErrNotOwner
	case store == nil && cause != nil:
		return nil, 0, fmt.Errorf("%w: the bigfile directory could not be prepared: %s", ErrBigNoStore, cause)
	case store == nil:
		return nil, 0, ErrBigNoStore
	}
	return store, r.ID, nil
}

// BigPut stores data under name, replacing a value there. A zero ttl is the
// default; sum, when not empty, is the lowercase hex SHA-256 data must have.
func (b *Bus) BigPut(caller, record, name string, data io.Reader, ttl time.Duration, memory bool, sum string) (ports.BigInfo, bool, error) {
	store, id, err := b.bigStore(caller, record, name, true)
	if err != nil {
		return ports.BigInfo{}, false, err
	}
	if ttl == 0 {
		ttl = BigDefaultTTL
	}
	if ttl < time.Second || ttl > BigMaxTTL {
		return ports.BigInfo{}, false, fmt.Errorf("%w: a lifetime is 1 second to %d days", ErrBigValue, BigMaxTTL/(24*time.Hour))
	}
	now := b.clock()
	info, replaced, err := store.BigPut(id, name, data, sum, memory, now.Add(ttl), now,
		ports.BigLimits{Value: BigMaxValue, Record: BigMaxRecord, Total: BigMaxTotal, Names: BigMaxNames})
	if err != nil {
		return info, replaced, bigErr(err)
	}
	// The record may have been removed while the bytes streamed in: its drop
	// ran before this key existed, so it would be left behind. A record's ID
	// is never reused, so an ID the registry no longer knows is a removed one.
	b.mu.Lock()
	_, known := b.recordByID[id]
	if !known {
		b.dropBig(id)
	}
	b.unlock()
	if !known {
		return ports.BigInfo{}, false, ErrUnknown
	}
	return info, replaced, nil
}

// BigGet opens one value. The caller closes it.
func (b *Bus) BigGet(caller, record, name string) (io.ReadCloser, ports.BigInfo, error) {
	store, id, err := b.bigStore(caller, record, name, false)
	if err != nil {
		return nil, ports.BigInfo{}, err
	}
	rc, info, found, err := store.BigGet(id, name, b.clock())
	if err != nil {
		return nil, ports.BigInfo{}, err
	}
	if !found {
		return nil, ports.BigInfo{}, ErrBigAbsent
	}
	return rc, info, nil
}

// BigList is record's live bigfiles.
func (b *Bus) BigList(caller, record string) (string, []ports.BigInfo, error) {
	store, id, err := b.bigStore(caller, record, "-", false)
	if err != nil {
		return "", nil, err
	}
	b.mu.Lock()
	name := b.recordByID[id]
	b.unlock()
	l, err := store.BigList(id, b.clock())
	return name, l, err
}

// BigDelete removes one value and says whether it was there.
func (b *Bus) BigDelete(caller, record, name string) (bool, error) {
	store, id, err := b.bigStore(caller, record, name, true)
	if err != nil {
		return false, err
	}
	return store.BigDelete(id, name, b.clock())
}

// bigErr says a store's refusals as the values they are.
func bigErr(err error) error {
	switch {
	case errors.Is(err, ports.ErrBigEmpty), errors.Is(err, ports.ErrBigHash):
		return fmt.Errorf("%w: %s", ErrBigValue, err)
	}
	return err
}

// dropBig removes every bigfile of one record. Caller holds b.mu.
func (b *Bus) dropBig(id uint32) {
	if b.big == nil {
		return
	}
	if err := b.big.BigDrop(id); err != nil {
		b.report(ports.Error, "the bigfiles of record ID %d could not be removed: %s", id, err)
	}
}

// PruneBigOrphans removes bigfiles whose record is not stored, found at start:
// a record removed while the daemon was down. They are nobody's, and unlike
// key-value values they hold bytes that expire anyway, so they are deleted.
func (b *Bus) PruneBigOrphans() {
	b.mu.Lock()
	defer b.unlock()
	b.pruneBigOrphans()
}

// pruneBigOrphans is PruneBigOrphans for a caller holding b.mu.
func (b *Bus) pruneBigOrphans() {
	lister, _ := b.big.(interface{ BigRecords() []uint32 })
	if lister == nil {
		return
	}
	for _, id := range lister.BigRecords() {
		if _, known := b.recordByID[id]; !known {
			b.dropBig(id)
			b.report(ports.Alert, "bigfiles of record ID %d named no stored record and were removed", id)
		}
	}
}

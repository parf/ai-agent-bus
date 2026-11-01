// Big keys: named values too large for a message, passed by key name. The
// registry decides who may reach one: the record's Owner, Maintainers and own
// Agent upload and delete, and download and list too; whoever its allow list
// admits downloads and lists only (Plans/R1.0-Release/big-keys.md#authority).
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
	ErrBigAbsent  = errors.New("no big key is stored under that name")
	ErrBigValue   = errors.New("that is not a big key this store takes")
	ErrBigNoStore = errors.New("this daemon keeps no big keys")
)

// Limits on big keys.
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
		may = b.may(caller, r)
	}
	store := b.big
	b.unlock()
	switch {
	case !live:
		return nil, 0, ErrUnknown
	case !may:
		return nil, 0, ErrNotOwner
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
	return info, replaced, bigErr(err)
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

// BigList is record's live big keys.
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

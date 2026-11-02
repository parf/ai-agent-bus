package core

import (
	"errors"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
)

const (
	// Five minutes avoids repeated provider/key/avatar work without keeping
	// changed keys or a formerly unknown login stale for long.
	directoryLookupTTL = 5 * time.Minute
	// One daemon-wide rolling allowance, shared by all realms and callers.
	// GitHub allows 60 anonymous REST requests/hour per originating IP:
	// https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api
	// The adapter has no token path. A lookup does about three fetches (keys,
	// user, avatar; not all REST), so a conservative 60/3 ceiling is ~20/h.
	// Twelve leaves headroom for other provider use. Cache hits are free.
	directoryLookupWindow = time.Hour
	directoryLookupLimit  = 12
)

var ErrDirectoryBusy = errors.New("directory lookup allowance is full")

type directoryBusy struct{ after time.Duration }

func (e *directoryBusy) Error() string             { return ErrDirectoryBusy.Error() }
func (e *directoryBusy) Unwrap() error             { return ErrDirectoryBusy }
func (e *directoryBusy) RetryAfter() time.Duration { return e.after }

type directoryLookupKey struct{ realm, login string }
type directoryLookupResult struct {
	entry   ports.DirectoryEntry
	err     error
	expires time.Time
}

func copyDirectoryEntry(entry ports.DirectoryEntry) ports.DirectoryEntry {
	entry.Keys = append([]string(nil), entry.Keys...)
	entry.Profile.PhotoPNG = append([]byte(nil), entry.Profile.PhotoPNG...)
	return entry
}

// lookupDirectory holds the bus lock only for cache/budget bookkeeping, never
// while the directory fetches. All results, including no keys and lookup errors,
// expire together. This deliberately delays real newcomers when the allowance
// is filled; that availability trade-off is accepted for 1.0 (B2).
func (b *Bus) lookupDirectory(realm, login string, directory ports.Directory) (ports.DirectoryEntry, error) {
	b.mu.Lock()
	now := b.clock()
	key := directoryLookupKey{realm, login}
	for name, result := range b.directoryCache {
		if !now.Before(result.expires) {
			delete(b.directoryCache, name)
		}
	}
	if result, ok := b.directoryCache[key]; ok {
		b.unlock()
		return copyDirectoryEntry(result.entry), result.err
	}
	first := 0
	for first < len(b.directoryFetches) && !now.Before(b.directoryFetches[first].Add(directoryLookupWindow)) {
		first++
	}
	b.directoryFetches = b.directoryFetches[first:]
	if len(b.directoryFetches) >= directoryLookupLimit {
		after := b.directoryFetches[0].Add(directoryLookupWindow).Sub(now)
		b.unlock()
		return ports.DirectoryEntry{}, &directoryBusy{after: after}
	}
	b.directoryFetches = append(b.directoryFetches, now)
	epoch := b.directoryEpoch
	b.unlock()

	entry, err := directory.Lookup(login)
	entry = copyDirectoryEntry(entry)
	b.mu.Lock()
	if epoch == b.directoryEpoch {
		if b.directoryCache == nil {
			b.directoryCache = make(map[directoryLookupKey]directoryLookupResult)
		}
		b.directoryCache[key] = directoryLookupResult{entry, err, b.clock().Add(directoryLookupTTL)}
	}
	b.unlock()
	return copyDirectoryEntry(entry), err
}

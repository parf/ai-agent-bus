package core

import (
	"errors"
	"fmt"
	"github.com/parf/ai-agent-bus/internal/ports"
	"sync"
	"testing"
	"time"
)

type countingDirectory struct {
	calls int
	entry ports.DirectoryEntry
	err   error
}

func (d *countingDirectory) Lookup(string) (ports.DirectoryEntry, error) {
	d.calls++
	return d.entry, d.err
}
func TestDirectoryLookupCachesPositiveAndNegativeUntilTTL(t *testing.T) {
	for _, kind := range []string{"positive", "unknown", "no keys"} {
		t.Run(kind, func(t *testing.T) {
			b := New()
			now := time.Unix(100, 0)
			b.clock = func() time.Time { return now }
			d := &countingDirectory{entry: ports.DirectoryEntry{Keys: []string{"key"}}}
			if kind != "positive" {
				d.entry = ports.DirectoryEntry{}
			}
			if kind == "unknown" {
				d.err = errors.New("unknown login")
			}
			b.Directories(map[string]ports.Directory{"h": d}, nil)
			for i := 0; i < 2; i++ {
				_, err := b.Challenge("alice@h")
				if kind == "positive" && err != nil {
					t.Fatal(err)
				}
				if kind != "positive" && !errors.Is(err, ErrEnrol) {
					t.Fatalf("negative lookup must refuse: %v", err)
				}
			}
			if d.calls != 1 {
				t.Fatalf("%s cached lookup fetched %d times, want 1", kind, d.calls)
			}
			now = now.Add(directoryLookupTTL)
			b.Challenge("alice@h")
			if d.calls != 2 {
				t.Fatalf("expired lookup fetched %d times, want 2", d.calls)
			}
		})
	}
}
func TestDirectoryLookupAllowanceIsDaemonWideAndCacheHitsAreFree(t *testing.T) {
	b := New()
	now := time.Unix(100, 0)
	b.clock = func() time.Time { return now }
	d := &countingDirectory{entry: ports.DirectoryEntry{Keys: []string{"key"}}}
	b.Directories(map[string]ports.Directory{"one": d, "two": d}, nil)
	for i := 0; i < directoryLookupLimit; i++ {
		if _, err := b.Challenge(fmt.Sprintf("person%d@one", i)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.Challenge("person0@one"); err != nil {
		t.Fatalf("cache hit spent allowance: %v", err)
	}
	now = now.Add(time.Minute + time.Second)
	_, blocked := b.Challenge("newcomer@two")
	var retry *directoryBusy
	if !errors.As(blocked, &retry) || retry.RetryAfter() != directoryLookupWindow-time.Minute-time.Second {
		t.Fatalf("retry must follow oldest lookup: %v", blocked)
	}
	if _, err := b.Challenge("newcomer@two"); !errors.Is(err, ErrDirectoryBusy) {
		t.Fatalf("uncached lookup beyond daemon allowance: %v, want busy", err)
	}
	if d.calls != directoryLookupLimit {
		t.Fatalf("refused lookup reached provider: %d fetches", d.calls)
	}
	now = now.Add(directoryLookupWindow)
	if _, err := b.Challenge("newcomer@two"); err != nil {
		t.Fatalf("allowance did not recover: %v", err)
	}
	if d.calls != directoryLookupLimit+1 {
		t.Fatalf("renewed allowance fetches=%d", d.calls)
	}
}
func TestDirectoryLookupCacheOwnsItsBytesAndReloadInvalidates(t *testing.T) {
	b := New()
	d := &countingDirectory{entry: ports.DirectoryEntry{Keys: []string{"key"}, Profile: ports.DirectoryProfile{PhotoPNG: []byte{1}}}}
	b.Directories(map[string]ports.Directory{"h": d}, nil)
	entry, err := b.lookupDirectory("h", "alice", d)
	if err != nil {
		t.Fatal(err)
	}
	entry.Keys[0] = "caller"
	entry.Profile.PhotoPNG[0] = 2
	d.entry.Keys[0] = "provider"
	d.entry.Profile.PhotoPNG[0] = 3
	entry, _ = b.lookupDirectory("h", "alice", d)
	if entry.Keys[0] != "key" || entry.Profile.PhotoPNG[0] != 1 {
		t.Fatalf("cache bytes aliased: %+v", entry)
	}
	b.Directories(map[string]ports.Directory{"h": d}, nil)
	entry, _ = b.lookupDirectory("h", "alice", d)
	if d.calls != 2 || entry.Keys[0] != "provider" {
		t.Fatalf("reload kept stale cache: %+v calls=%d", entry, d.calls)
	}
}

type blockingDirectory struct {
	started chan struct{}
	release chan struct{}
}

func (d blockingDirectory) Lookup(string) (ports.DirectoryEntry, error) {
	d.started <- struct{}{}
	<-d.release
	return ports.DirectoryEntry{Keys: []string{"key"}}, nil
}
func TestDirectoryLookupBoundsConcurrentMisses(t *testing.T) {
	b := New()
	d := blockingDirectory{make(chan struct{}, directoryLookupLimit+1), make(chan struct{})}
	var wg sync.WaitGroup
	defer wg.Wait()
	defer close(d.release)
	for i := 0; i < directoryLookupLimit; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); b.lookupDirectory("h", fmt.Sprint(i), d) }(i)
	}
	for i := 0; i < directoryLookupLimit; i++ {
		select {
		case <-d.started:
		case <-time.After(time.Second):
			t.Fatal("provider did not start")
		}
	}
	result := make(chan error, 1)
	wg.Add(1)
	go func() { defer wg.Done(); _, err := b.lookupDirectory("h", "extra", d); result <- err }()
	select {
	case err := <-result:
		if !errors.Is(err, ErrDirectoryBusy) {
			t.Fatalf("concurrent allowance: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("over-limit lookup blocked in provider")
	}
	select {
	case <-d.started:
		t.Fatal("over-limit lookup fetched")
	default:
	}
}

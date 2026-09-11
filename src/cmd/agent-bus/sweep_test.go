package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The scan is O(directory) and used to run per message. What keeps it off
// that path is the marker, so the schedule itself is what needs checking:
// with a fresh marker nothing is scanned, and an expired context survives.
func TestSweepIsScheduled(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, "old.json")
	write := func() {
		os.WriteFile(stale, []byte("{}"), 0o600)
		old := time.Now().Add(-2 * replyContextTTL)
		os.Chtimes(stale, old, old)
	}

	write()
	sweep(dir) // no marker yet: this one runs, and takes the stale file
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("the first sweep did not remove an expired context")
	}
	if _, err := os.Stat(filepath.Join(dir, sweptMarker)); err != nil {
		t.Fatalf("no marker was left behind: %v", err)
	}

	write()
	sweep(dir) // marker is fresh: this one must not scan at all
	if _, err := os.Stat(stale); err != nil {
		t.Fatal("swept again within the interval: the scan is back on the hot path")
	}

	old := time.Now().Add(-2 * sweepEvery)
	os.Chtimes(filepath.Join(dir, sweptMarker), old, old)
	sweep(dir) // marker is stale: due again
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("the sweep never runs again once the marker exists")
	}
}

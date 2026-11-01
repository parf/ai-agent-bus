package main

import (
	"path/filepath"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/store/bigfile"
)

// bigSweepEvery is how often expired big keys and their files are removed;
// every read expires lazily as well.
const bigSweepEvery = time.Minute

// openBigKeys is the holder of big keys: a private directory beside the
// database, which backups of the database and record exports never include
// (Plans/R1.0-Release/big-keys.md#trust-and-durability). A directory that
// cannot be prepared refuses the start: the bus never quietly serves a store
// that loses its keys at a restart.
func openBigKeys(home string) (ports.BigStore, error) {
	s, err := bigfile.Open(filepath.Join(home, "big-keys"), time.Now())
	if err != nil {
		return nil, err
	}
	go func() {
		for range time.Tick(bigSweepEvery) {
			s.Sweep(time.Now())
		}
	}()
	return s, nil
}

package main

import (
	"log"
	"path/filepath"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/store/bigfile"
	"github.com/parf/ai-agent-bus/internal/store/bigmem"
)

// bigSweepEvery is how often expired big keys and their files are removed;
// every read expires lazily as well.
const bigSweepEvery = time.Minute

// openBigKeys is the holder of big keys: a private directory beside the
// database, which backups of the database and record exports never include
// (Plans/R1.0-Release/big-keys.md#trust-and-durability). A directory that
// cannot be prepared leaves the bus serving, with the keys in memory only, and
// says so.
func openBigKeys(home string) ports.BigStore {
	s, err := bigfile.Open(filepath.Join(home, "big-keys"), time.Now())
	if err != nil {
		log.Printf("WARNING: big keys are held in memory only: %v", err)
		return bigmem.New()
	}
	go func() {
		for range time.Tick(bigSweepEvery) {
			s.Sweep(time.Now())
		}
	}()
	return s
}

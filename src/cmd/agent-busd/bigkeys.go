package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/store/bigfile"
)

// bigSweepEvery is how often expired bigfiles and their files are removed;
// every read expires lazily as well.
const bigSweepEvery = time.Minute

// openBigKeys is the holder of bigfiles: a private directory beside the
// database, which backups of the database and record exports never include
// (Plans/R1.0-Release/bigfile.md#trust-and-durability). A directory that
// cannot be prepared leaves the bus up with the four bigfile calls refused,
// the cause reported as an error and named in each refusal; a later call tries
// the directory again. Bigfiles are optional, so a planted file or a changed
// mode there never takes the node down, and the keys are never quietly held in
// memory instead.
func bindBigKeys(bus *core.Bus, home string, idx ports.BigIndex) {
	s, err := openBigKeys(home, idx)
	if err != nil {
		bus.BigUnavailable(err, func() (ports.BigStore, error) { return openBigKeys(home, idx) })
		return
	}
	bus.SetBigStore(s)
}

const bigDir = "bigfile"

func openBigKeys(home string, idx ports.BigIndex) (ports.BigStore, error) {
	if _, err := os.Lstat(filepath.Join(home, "big-keys")); err == nil {
		return nil, fmt.Errorf("unsupported big-keys layout; use a clean data directory")
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	s, err := bigfile.Open(filepath.Join(home, bigDir), idx, time.Now())
	if err != nil {
		return nil, err
	}
	// A start that removed anything says how much, by count, so a loss on the
	// way up after a crash is never silent.
	if r := s.Start; r != (bigfile.Reconciled{}) {
		log.Printf("bigfile start: removed %d rows without files, %d files without rows, %d temporary files", r.RowsWithoutFiles, r.FilesWithoutRows, r.Temporary)
	}
	go func() {
		for range time.Tick(bigSweepEvery) {
			s.Sweep(time.Now())
		}
	}()
	return s, nil
}

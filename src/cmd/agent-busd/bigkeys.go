package main

import (
	"path/filepath"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/store/bigfile"
)

// bigSweepEvery is how often expired big keys and their files are removed;
// every read expires lazily as well.
const bigSweepEvery = time.Minute

// openBigKeys is the holder of big keys: a private directory beside the
// database, which backups of the database and record exports never include
// (Plans/R1.0-Release/big-keys.md#trust-and-durability). A directory that
// cannot be prepared leaves the bus up with the four big-key calls refused,
// the cause reported as an error and named in each refusal; a later call tries
// the directory again. Big keys are optional, so a planted file or a changed
// mode there never takes the node down, and the keys are never quietly held in
// memory instead.
func bindBigKeys(bus *core.Bus, home string) {
	s, err := openBigKeys(home)
	if err != nil {
		bus.BigUnavailable(err, func() (ports.BigStore, error) { return openBigKeys(home) })
		return
	}
	bus.SetBigStore(s)
}

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

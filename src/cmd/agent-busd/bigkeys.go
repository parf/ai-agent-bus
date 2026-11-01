package main

import (
	"errors"
	"fmt"
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

// The directory was called big-keys before bigfile became the feature's name.
const (
	bigDir    = "bigfile"
	oldBigDir = "big-keys"
)

// migrateBigDir moves an old directory to its new name. One of them alone is
// the simple case: the old one is renamed, the new one is used. Both at once
// is not guessed at: the start refuses, and the cause names both, so an
// operator decides which holds the files.
func migrateBigDir(home string) error {
	oldPath, newPath := filepath.Join(home, oldBigDir), filepath.Join(home, bigDir)
	_, oldErr := os.Lstat(oldPath)
	_, newErr := os.Lstat(newPath)
	for _, e := range []error{oldErr, newErr} {
		if e != nil && !errors.Is(e, os.ErrNotExist) {
			return e
		}
	}
	switch {
	case oldErr == nil && newErr == nil:
		return fmt.Errorf("both %s and %s exist; keep the one that holds the bigfiles and remove the other", oldPath, newPath)
	case oldErr == nil:
		return os.Rename(oldPath, newPath)
	}
	return nil
}

func openBigKeys(home string, idx ports.BigIndex) (ports.BigStore, error) {
	if err := migrateBigDir(home); err != nil {
		return nil, err
	}
	s, err := bigfile.Open(filepath.Join(home, bigDir), idx, time.Now())
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

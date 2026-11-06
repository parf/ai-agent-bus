package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const leaseFile = ".lease"

// holdLease marks dir as in use by a running install or remove for as long as the returned file stays
// open: an advisory lock on a file inside it, which dies with the process. Another runner that starts
// sweeping leftovers sees the lock and leaves the directory alone.
func holdLease(dir string) (*os.File, error) {
	f, err := os.OpenFile(filepath.Join(dir, leaseFile), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// claimDir makes a private directory parent/prefix<random> that holds a locked lease from the first
// moment it is visible under that name: the directory is built under a claim name, locked, then
// renamed (the lock follows the file). A sweep therefore never sees a final-named directory whose
// owner has not locked it yet, and needs no age test. The claim name itself is swept when unlocked;
// a creator paused before its lock then fails at the rename and reports an error, losing nothing.
func claimDir(parent, prefix string) (string, *os.File, error) {
	var err error
	// A runner starting elsewhere can sweep the claim before its creator locks it; the creator then
	// finds it gone at the rename and claims again, once.
	for attempt := 0; attempt < 2; attempt++ {
		var dir string
		var f *os.File
		if dir, f, err = claimOnce(parent, prefix); err == nil || !errors.Is(err, os.ErrNotExist) {
			return dir, f, err
		}
	}
	return "", nil, err
}

// afterClaimLock is a test hook between locking the claim and renaming it.
var afterClaimLock func(claim string)

func claimOnce(parent, prefix string) (string, *os.File, error) {
	claim, err := os.MkdirTemp(parent, claimPrefix)
	if err != nil {
		return "", nil, err
	}
	f, err := holdLease(claim)
	if err != nil {
		os.RemoveAll(claim)
		return "", nil, err
	}
	if afterClaimLock != nil {
		afterClaimLock(claim)
	}
	final := filepath.Join(parent, prefix+strings.TrimPrefix(filepath.Base(claim), claimPrefix))
	if err := os.Rename(claim, final); err != nil {
		f.Close()
		os.RemoveAll(claim)
		return "", nil, err
	}
	return final, f, nil
}

const claimPrefix = ".claim-"

// leased says whether someone is still working in dir: its lease is held by a live process, or
// cannot be looked at (left alone). A directory with no lease, or with one nobody holds, is abandoned.
func leased(dir string) bool {
	f, err := os.OpenFile(filepath.Join(dir, leaseFile), os.O_RDWR, 0)
	if err != nil {
		// Only a lease that is not there is unheld; one that cannot be opened may well be held.
		return !errors.Is(err, os.ErrNotExist)
	}
	defer f.Close()
	// Only a clean acquire shows the lease is unheld; EWOULDBLOCK, and any other error (ENOLCK on a
	// network file system, EINTR), leave the directory alone.
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil
}

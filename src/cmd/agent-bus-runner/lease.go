package main

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"
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

// leased says whether someone is still working in dir: its lease is held, or it was made so
// recently that its owner has not yet taken the lease. A directory whose lease can be taken, or
// that never had one and is not new, is abandoned.
func leased(dir string, info os.FileInfo) bool {
	f, err := os.OpenFile(filepath.Join(dir, leaseFile), os.O_RDWR, 0)
	if err != nil {
		return time.Since(info.ModTime()) < 10*time.Second
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.Is(err, syscall.EWOULDBLOCK)
	}
	return false
}

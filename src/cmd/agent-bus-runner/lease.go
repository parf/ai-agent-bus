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

// leased says whether someone is still working in dir: its lease is held, or the lease is so new
// that its owner may not have taken the lock yet (it creates the file, then locks it). A directory
// whose lease can be taken and is not new, or that never had one and is not new, is abandoned.
func leased(dir string, info os.FileInfo) bool {
	const grace = 10 * time.Second
	f, err := os.OpenFile(filepath.Join(dir, leaseFile), os.O_RDWR, 0)
	if err != nil {
		return time.Since(info.ModTime()) < grace
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.Is(err, syscall.EWOULDBLOCK)
	}
	if st, err := f.Stat(); err != nil || time.Since(st.ModTime()) < grace {
		return true
	}
	return false
}

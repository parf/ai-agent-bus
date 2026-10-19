package main

import (
	"os"
	"path/filepath"
)

// Publish a private state snapshot without changing the inode that an
// already-open reader is using. These caches need atomic visibility, not a
// synchronous disk flush on every consumed message.
func writeState(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".state-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

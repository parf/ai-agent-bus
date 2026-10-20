package agentmeta

import (
	"fmt"
	"io"
	"os"
	"syscall"
)

// OpenFile refuses final symlinks and special files before reading settings.
// Nonblocking open prevents a replaced author file from blocking on a FIFO.
func OpenFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = fmt.Errorf("%s: not a regular file", path)
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// ReadFile reads package and runner settings without following a final link.
func ReadFile(path string) ([]byte, error) {
	f, err := OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

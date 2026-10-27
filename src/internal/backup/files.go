package backup

import (
	"errors"
	"os"
	"path/filepath"
)

// WriteFiles publishes ciphertext followed by its public sidecar. Exclusive
// creation refuses collisions and symlinks; a failed sidecar leaves no archive.
func WriteFiles(path string, ciphertext, info []byte) error {
	write := func(name string, body []byte) error {
		f, e := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return e
		}
		_, e = f.Write(body)
		if e == nil {
			e = f.Sync()
		}
		closeErr := f.Close()
		if e == nil {
			e = closeErr
		}
		if e != nil {
			os.Remove(name)
		}
		return e
	}
	if len(info) == 0 {
		return errors.New("backup sidecar is empty")
	}
	if e := write(path, ciphertext); e != nil {
		return e
	}
	if e := write(path+".info", info); e != nil {
		os.Remove(path)
		return e
	}
	return nil
}

const DefaultDirectory = "/var/lib/agent-bus/record-backups"

func Path(dir string, info Info) string { return filepath.Join(dir, Filename(info)) }

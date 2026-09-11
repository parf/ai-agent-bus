// Package jsonfile is the dump adapter that writes the snapshot as one JSON
// document, mode 0600 because it carries message bodies. The design names
// Parquet (docs/04-messaging.md#durability); that is one more adapter behind
// the same port and nothing inward changes for it
// (docs/10-modules.md#the-rule).
package jsonfile

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/parf/ai-agent-bus/internal/ports"
)

// Dump is the snapshot file at path.
type Dump struct{ path string }

func New(path string) *Dump { return &Dump{path: path} }

// Save replaces the file through a temporary one, so a start either reads
// the whole previous snapshot or the one before it, never half of each.
func (d *Dump) Save(s ports.Snapshot) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(d.path), 0o700); err != nil {
		return err
	}
	tmp := d.path + ".new"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, d.path)
}

// Load reports false when there is no file: a first start and one that
// follows a death have to be told apart, and the absence is the difference.
func (d *Dump) Load() (ports.Snapshot, bool, error) {
	b, err := os.ReadFile(d.path)
	if os.IsNotExist(err) {
		return ports.Snapshot{}, false, nil
	}
	if err != nil {
		return ports.Snapshot{}, false, err
	}
	var s ports.Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return ports.Snapshot{}, false, err
	}
	return s, true, nil
}

// Package file is the store adapter that keeps credentials in a text file:
// one line per principal, `name current [previous]`, mode 0600. It is what
// the MVP ships while what else lives in SQLite is open
// (docs/09-setup.md#storage) — a database is one more adapter and no change
// anywhere else, which is the point of the port
// (docs/10-modules.md#the-rule).
package file

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/parf/ai-agent-bus/internal/ports"
)

// Tokens is the credential file at path.
type Tokens struct{ path string }

// NewTokens does no I/O: a store that fails at construction is one the daemon
// cannot report on.
func NewTokens(path string) *Tokens { return &Tokens{path: path} }

// Load reads the file. A line with a single field is the bare token the PoC
// wrote; it comes back nameless, because whose it is is core's rule and not
// the file's.
func (t *Tokens) Load() ([]ports.Credential, error) {
	b, err := os.ReadFile(t.path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	var out []ports.Credential
	for _, line := range strings.Split(string(b), "\n") {
		switch f := strings.Fields(line); len(f) {
		case 0:
		case 1:
			out = append(out, ports.Credential{Current: f[0]})
		case 2:
			out = append(out, ports.Credential{Name: f[0], Current: f[1]})
		default:
			out = append(out, ports.Credential{Name: f[0], Current: f[1], Previous: f[2]})
		}
	}
	return out, nil
}

// Save rewrites the whole file through a temporary one, so a reader either
// sees the old set or the new one.
func (t *Tokens) Save(creds []ports.Credential) error {
	var b strings.Builder
	for _, c := range creds {
		fmt.Fprintf(&b, "%s %s", c.Name, c.Current)
		if c.Previous != "" {
			fmt.Fprintf(&b, " %s", c.Previous)
		}
		b.WriteByte('\n')
	}
	if err := os.MkdirAll(filepath.Dir(t.path), 0o700); err != nil {
		return err
	}
	tmp := t.path + ".new"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, t.path)
}

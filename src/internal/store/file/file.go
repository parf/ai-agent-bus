// Package file is the store adapter that keeps credentials in a text file:
// one line per principal, `name current [previous [issued]]`, mode 0600. A
// principal with no previous token but a date writes `-` in its place, so the
// fields stay positional. It is what the MVP ships while what else lives in
// SQLite is open
// (docs/09-setup.md#storage) — a database is one more adapter and no change
// anywhere else, which is the point of the port
// (docs/10-modules.md#the-rule).
package file

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
)

// Tokens is the credential file at path.
type Tokens struct{ path string }

// NewTokens does no I/O: a store that fails at construction is one the daemon
// cannot report on.
func NewTokens(path string) *Tokens { return &Tokens{path: path} }

// Load reads the file. Every line names its principal; a line that does not
// is corruption, not an older format, and is refused rather than guessed at.
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
			return nil, fmt.Errorf("%s: credential line names no principal: %q", t.path, line)
		case 2:
			out = append(out, ports.Credential{Name: f[0], Current: f[1]})
		case 3:
			out = append(out, ports.Credential{Name: f[0], Current: f[1], Previous: f[2]})
		default:
			c := ports.Credential{Name: f[0], Current: f[1], Previous: f[2]}
			if c.Previous == "-" {
				c.Previous = ""
			}
			// A date the daemon itself wrote and cannot read back means the
			// file is not the one it left; say so rather than carry on with
			// a credential whose other fields may be just as wrong.
			if c.Issued, err = time.Parse(time.RFC3339Nano, f[3]); err != nil {
				return nil, fmt.Errorf("credential file: %w", err)
			}
			out = append(out, c)
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
		switch prev := c.Previous; {
		case !c.Issued.IsZero():
			if prev == "" {
				prev = "-"
			}
			fmt.Fprintf(&b, " %s %s", prev, c.Issued.UTC().Format(time.RFC3339Nano))
		case prev != "":
			fmt.Fprintf(&b, " %s", prev)
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

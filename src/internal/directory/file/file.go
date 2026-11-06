// Package file is the directory adapter for manual enrolment: a text file of
// `login key-type key-blob [comment]` lines, one or more per login, in the
// same shape as an `authorized_keys` entry. The login is matched without regard to ASCII case. It is what a host uses before it
// trusts a provider, and what a test uses instead of the network.
// See docs/01-identity-and-authority.md#registration.
package file

import (
	"os"
	"strings"

	"github.com/parf/ai-agent-bus/internal/ports"
)

type Directory struct{ path string }

func New(path string) *Directory { return &Directory{path: path} }

// asciiLower folds A-Z only. A name's local part is lower-case a-z, 0-9 and a few marks, so a line
// written `Alice` names the same login as `alice`; strings.ToLower would also fold U+212A (the Kelvin
// sign) to k and let a non-ASCII login stand for an ASCII one.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// Lookup returns what login publishes, or nothing at all. An unknown login is
// not an error: it published no keys, which is the same refusal.
func (d *Directory) Lookup(login string) (ports.DirectoryEntry, error) {
	b, err := os.ReadFile(d.path)
	if err != nil {
		return ports.DirectoryEntry{}, err
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		who, key, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok || asciiLower(who) != login {
			continue
		}
		if key = strings.TrimSpace(key); key != "" {
			out = append(out, key)
		}
	}
	return ports.DirectoryEntry{Keys: out}, nil
}

// Package file is the directory adapter for manual enrolment: a text file of
// `login key-type key-blob [comment]` lines, one or more per login, in the
// same shape as an `authorized_keys` entry. It is what a host uses before it
// trusts a provider, and what a test uses instead of the network.
// See docs/01-identity.md#registration.
package file

import (
	"os"
	"strings"
)

type Directory struct{ path string }

func New(path string) *Directory { return &Directory{path: path} }

// Keys returns what login publishes, or nothing at all. An unknown login is
// not an error: it published no keys, which is the same refusal.
func (d *Directory) Keys(login string) ([]string, error) {
	b, err := os.ReadFile(d.path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		who, key, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok || who != login {
			continue
		}
		if key = strings.TrimSpace(key); key != "" {
			out = append(out, key)
		}
	}
	return out, nil
}

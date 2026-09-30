package authkeys

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Key is a fresh ssh-ed25519 public key in authorized_keys form.
func Key(t testing.TB) string {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	return "ssh-ed25519 " + base64.StdEncoding.EncodeToString(wire("ssh-ed25519", pub))
}

func wire(kind string, key []byte) []byte {
	var b []byte
	for _, s := range [][]byte{[]byte(kind), key} {
		b = binary.BigEndian.AppendUint32(b, uint32(len(s)))
		b = append(b, s...)
	}
	return b
}

func TestEd25519TakesOneKeyAndNothingElse(t *testing.T) {
	k := Key(t)
	if got, err := Ed25519("  " + k + " alice@laptop \n"); err != nil || got != k {
		t.Fatalf("a pasted key with its comment: %q %v", got, err)
	}
	pub := make([]byte, 32)
	for name, bad := range map[string]string{
		"options first":     `command="/bin/sh" ` + k,
		"a second line":     k + "\n" + k,
		"another type":      "ssh-rsa " + strings.Fields(k)[1],
		"not base64":        "ssh-ed25519 !!!!",
		"a lying blob":      "ssh-ed25519 " + base64.StdEncoding.EncodeToString(wire("ssh-rsa", pub)),
		"a short key":       "ssh-ed25519 " + base64.StdEncoding.EncodeToString(wire("ssh-ed25519", pub[:16])),
		"trailing bytes":    "ssh-ed25519 " + base64.StdEncoding.EncodeToString(append(wire("ssh-ed25519", pub), 0)),
		"the type alone":    "ssh-ed25519",
		"nothing":           "",
		"a private key":     "-----BEGIN OPENSSH PRIVATE KEY-----",
		"a quote in a line": k + ` "x`,
	} {
		got, err := Ed25519(bad)
		if name == "a quote in a line" {
			// A comment is dropped, whatever it holds.
			if err != nil || got != k {
				t.Errorf("%s: %q %v", name, got, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s was taken as %q", name, got)
		}
	}
}

func TestLineReadsBackAsWritten(t *testing.T) {
	l := Line("/usr/local/bin/agent-bus-token", "alice@h", Key(t))
	if who, program := Whose(l); who != "alice@h" || program != "agent-bus-token" {
		t.Fatalf("read back as %q %q", who, program)
	}
	if !strings.HasPrefix(l, `restrict,command="/usr/local/bin/agent-bus-token alice@h" ssh-ed25519 `) {
		t.Fatalf("line %q", l)
	}
}

// Writers holding the lock never lose each other's lines, and the file keeps
// the modes sshd insists on.
func TestUpdateIsLockedAndPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".ssh", "authorized_keys")
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := string(rune('a'+i)) + "@h"
			if err := Update(path, func(lines []string) ([]string, error) {
				return append(Without(lines, name), Line("/p/agent-bus-token", name, "ssh-ed25519 AAAA")), nil
			}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	lines, err := Read(path)
	if err != nil || len(lines) != 20 {
		t.Fatalf("%d lines after twenty writers: %v", len(lines), err)
	}
	for p, want := range map[string]os.FileMode{path: 0o600, filepath.Dir(path): 0o700} {
		if st, err := os.Stat(p); err != nil || st.Mode().Perm() != want {
			t.Errorf("%s is %v, want %o", p, st.Mode().Perm(), want)
		}
	}
}

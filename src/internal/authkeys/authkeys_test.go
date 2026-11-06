package authkeys

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"errors"
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
	l, err := Line("/usr/local/bin/agent-bus-token", "alice@h", Key(t))
	if err != nil {
		t.Fatal(err)
	}
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
				line, err := Line("/p/agent-bus-token", name, "ssh-ed25519 AAAA")
				if err != nil {
					return nil, err
				}
				return append(Without(lines, name), line), nil
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

func TestRestrictedLineRefusesASecondPublicKey(t *testing.T) {
	key := Key(t)
	for _, separator := range []string{"\n", "\r", "\r\n"} {
		if got, err := Line("/p/agent-bus-token", "alice@h", key+separator+key); err == nil || got != "" {
			t.Fatal("a second unrestricted key was accepted")
		}
	}
	for _, kind := range []string{"ssh-rsa", "ecdsa-sha2-nistp256", "sk-ssh-ed25519@openssh.com"} {
		line, err := Line("/p/agent-bus-token", "alice@h", kind+" AAAA comment\n")
		if err != nil || !strings.Contains(line, " "+kind+" AAAA comment") {
			t.Fatalf("host-side key type lost: %s %v", kind, err)
		}
	}
}

// A program path with whitespace cannot be read back to revoke, so Line refuses it by name.
func TestLineRefusesAProgramPathWithWhitespace(t *testing.T) {
	for _, program := range []string{"/opt/my tools/agent-bus-token", "/opt/tab\there/agent-bus-token", "/opt/line\nbreak/x"} {
		if l, err := Line(program, "alice@h", Key(t)); !errors.Is(err, ErrProgram) || l != "" {
			t.Errorf("%q: line %q err %v", program, l, err)
		}
	}
}

// A line of ours is exactly `<absolute program> <name>`; any other command has no owner and is not
// revoked as if it were ours.
func TestWhoseReadsOnlyTheTwoFieldFormWeWrite(t *testing.T) {
	key := Key(t)
	line := func(cmd string) string { return `restrict,command="` + cmd + `" ` + key }
	if who, program := Whose(line("/usr/local/bin/agent-bus-token alice@h")); who != "alice@h" || program != "agent-bus-token" {
		t.Errorf("our own line read back as %q %q", who, program)
	}
	if who, program := Whose(line("/p/agent-bus-admin alice@h")); who != "alice@h" || program != "agent-bus-admin" {
		t.Errorf("the admin line read back as %q %q", who, program)
	}
	for name, cmd := range map[string]string{
		"a program with arguments":   "/usr/bin/rsync --server -vlogDtpr . alice@h",
		"a shell command":            "/bin/sh -c alice@h",
		"a hand-written spaced path": "/opt/my tools/agent-bus-token alice@h",
		"a relative path":            "agent-bus-token alice@h",
		"a path as the name":         "/bin/cat /etc/passwd",
		"an option as the name":      "/bin/ls -l",
		"no name":                    "/bin/sh",
		"a python hook":              "/usr/bin/python3 /opt/hook.py alice@h",
		"printf with arguments":      "/usr/bin/printf hello alice@h",
		"echo naming our program":    "/bin/echo /opt/agent-bus-token alice@h",
		"a non-canonical name":       "/usr/local/bin/agent-bus-token Alice@H",
		"two-field foreign":          "/usr/bin/foo alice@h",
	} {
		if who, program := Whose(line(cmd)); who != "" || program != "" {
			t.Errorf("%s: attributed to %q %q", name, who, program)
		}
		if got := Without([]string{line(cmd)}, "alice@h"); len(got) != 1 {
			t.Errorf("%s: revoked as if it were ours", name)
		}
	}
	if who, _ := Whose(key); who != "" {
		t.Errorf("a plain key line has the owner %q", who)
	}
}

// Whatever Line writes, Whose reads back: only the allowed programs are written.
func TestLineWritesOnlyWhatWhoseReadsBack(t *testing.T) {
	if l, err := Line("agent-bus-token", "alice@h", Key(t)); !errors.Is(err, ErrProgramName) || l != "" {
		t.Fatalf("a relative program: line %q err %v", l, err)
	}
	if l, err := Line("/opt/x/mytoken", "alice@h", Key(t)); !errors.Is(err, ErrProgramName) || l != "" {
		t.Fatalf("a program with another name: line %q err %v", l, err)
	} else if !strings.Contains(err.Error(), "agent-bus-token") || !strings.Contains(err.Error(), "agent-bus-admin") {
		t.Fatalf("the error does not name the allowed programs: %v", err)
	}
	for _, program := range Programs {
		l, err := Line("/usr/local/bin/"+program, "alice@h", Key(t))
		if err != nil {
			t.Fatalf("%s refused: %v", program, err)
		}
		if who, got := Whose(l); who != "alice@h" || got != program {
			t.Errorf("%s read back as %q %q", program, who, got)
		}
	}
}

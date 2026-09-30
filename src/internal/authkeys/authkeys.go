// Package authkeys reads and writes the daemon account's authorized_keys:
// one line per principal, each restricted to a forced command naming it
// (docs/09-setup.md#ssh-admin). The admin program and the daemon both write
// the file, so the format and the locking live here once.
package authkeys

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// ErrKey is text that is not one ssh-ed25519 public key.
var ErrKey = errors.New("not an ssh-ed25519 public key")

// Ed25519 reads one public key as `ssh-ed25519 <base64> [comment]` and
// answers it rebuilt as `ssh-ed25519 <base64>`: nothing of the input but the
// key itself reaches the file, so no option, second line or quote can.
func Ed25519(text string) (string, error) {
	text = strings.TrimSpace(text)
	if strings.ContainsAny(text, "\r\n") {
		return "", fmt.Errorf("%w: one line, one key", ErrKey)
	}
	f := strings.Fields(text)
	if len(f) < 2 || f[0] != "ssh-ed25519" {
		return "", fmt.Errorf("%w: one begins ssh-ed25519", ErrKey)
	}
	blob, err := base64.StdEncoding.DecodeString(f[1])
	if err != nil {
		return "", fmt.Errorf("%w: the key is not base64", ErrKey)
	}
	// The wire form: a string naming the type, then the 32-byte key.
	kind, rest, ok := sshString(blob)
	if !ok || string(kind) != "ssh-ed25519" {
		return "", fmt.Errorf("%w: the key's own type is not ssh-ed25519", ErrKey)
	}
	key, rest, ok := sshString(rest)
	if !ok || len(key) != 32 || len(rest) != 0 {
		return "", fmt.Errorf("%w: the key is not 32 bytes", ErrKey)
	}
	return "ssh-ed25519 " + f[1], nil
}

func sshString(b []byte) (s, rest []byte, ok bool) {
	if len(b) < 4 {
		return nil, nil, false
	}
	n := binary.BigEndian.Uint32(b)
	if uint64(len(b)-4) < uint64(n) {
		return nil, nil, false
	}
	return b[4 : 4+n], b[4+n:], true
}

// Line is the entry for name: its key, reaching program with name as its
// only argument.
func Line(program, name, key string) string {
	return fmt.Sprintf("restrict,command=%q %s", program+" "+name, key)
}

// Whose reads a line back: the name it is for, and the program it reaches.
func Whose(line string) (name, program string) {
	i := strings.Index(line, `command="`)
	if i < 0 {
		return "", ""
	}
	rest := line[i+len(`command="`):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		return "", ""
	}
	fields := strings.Fields(rest[:j])
	if len(fields) != 2 {
		return "", ""
	}
	return fields[1], filepath.Base(fields[0])
}

// Without is lines minus name's.
func Without(lines []string, name string) []string {
	kept := lines[:0:0]
	for _, l := range lines {
		if who, _ := Whose(l); who == name {
			continue
		}
		kept = append(kept, l)
	}
	return kept
}

// Read is the file's lines; an absent file has none.
func Read(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var lines []string
	s := bufio.NewScanner(bytes.NewReader(b))
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	return lines, s.Err()
}

// Write replaces the file with lines, in the modes sshd insists on: it
// refuses a looser file or directory and says so only in its own log.
func Write(path string, lines []string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	var body strings.Builder
	for _, l := range lines {
		body.WriteString(l + "\n")
	}
	tmp := path + ".new"
	if err := os.WriteFile(tmp, []byte(body.String()), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Update reads, edits and writes the file under an exclusive lock, so two
// writers never lose each other's lines.
func Update(path string, edit func([]string) ([]string, error)) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	lines, err := Read(path)
	if err != nil {
		return err
	}
	next, err := edit(lines)
	if err != nil {
		return err
	}
	return Write(path, next)
}

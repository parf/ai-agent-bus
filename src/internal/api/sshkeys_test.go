package api

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/authkeys"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func edKey(t *testing.T) string {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	var b []byte
	for _, s := range [][]byte{[]byte("ssh-ed25519"), pub} {
		b = binary.BigEndian.AppendUint32(b, uint32(len(s)))
		b = append(b, s...)
	}
	return "ssh-ed25519 " + base64.StdEncoding.EncodeToString(b)
}

func keyBody(name, key string) string {
	b, _ := json.Marshal(map[string]string{"name": name, "key": key})
	return string(b)
}

// Whoever may edit a User may give them a key that reaches their credential
// over ssh; the line is the one agent-bus-admin user add writes.
func TestAUsersKeyIsAddedByWhoeverMayEditThem(t *testing.T) {
	b := core.New()
	s, tok := serverFor(t, b, "owner@h")
	for _, who := range []string{"admin@h", "peer@h", "bob@h", "eve@h"} {
		if _, err := b.SetUser("owner@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.SetGroup("owner@h", core.AdministratorsGroup, []string{"owner@h", "admin@h", "peer@h"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), ".ssh", "authorized_keys")

	// Without -ssh-keys the node takes keys on the host, and writes nothing.
	if code, body := send(s, tok("owner@h"), "POST", "/user/key", keyBody("bob@h", edKey(t)), ""); code != http.StatusNotImplemented {
		t.Fatalf("a node without -ssh-keys answered %d %s", code, body)
	}
	s.SSHKeys(path, "/usr/local/bin/agent-bus-token")

	key := edKey(t)
	if code, body := send(s, tok("admin@h"), "POST", "/user/key", keyBody("bob@h", key+" bob@laptop"), ""); code != http.StatusOK {
		t.Fatalf("an Administrator adding an ordinary User's key: %d %s", code, body)
	}
	lines, _ := authkeys.Read(path)
	if len(lines) != 1 || lines[0] != `restrict,command="/usr/local/bin/agent-bus-token bob@h" `+key {
		t.Fatalf("the file holds %q", lines)
	}
	// A second key replaces the first: one line per name.
	second := edKey(t)
	if code, _ := send(s, tok("owner@h"), "POST", "/user/key", keyBody("bob@h", second), ""); code != http.StatusOK {
		t.Fatal("the daemon Owner could not replace the key")
	}
	if lines, _ = authkeys.Read(path); len(lines) != 1 || !strings.HasSuffix(lines[0], second) {
		t.Fatalf("after a second key the file holds %q", lines)
	}
	for _, c := range []struct {
		who, name, key string
		code           int
	}{
		{"eve@h", "bob@h", edKey(t), http.StatusForbidden},       // an ordinary User, for another
		{"bob@h", "bob@h", edKey(t), http.StatusForbidden},       // nor for themselves
		{"admin@h", "peer@h", edKey(t), http.StatusForbidden},    // an Administrator, for a peer
		{"owner@h", "ghost@h", edKey(t), http.StatusNotFound},    // a User that does not exist
		{"owner@h", "bob@h", `command="/bin/sh" ` + key, http.StatusBadRequest},
		{"owner@h", "bob@h", "ssh-rsa AAAAB3NzaC1yc2E=", http.StatusBadRequest},
	} {
		if code, body := send(s, tok(c.who), "POST", "/user/key", keyBody(c.name, c.key), ""); code != c.code {
			t.Errorf("%s adding %s's key %.30q: %d %s, want %d", c.who, c.name, c.key, code, body, c.code)
		}
	}
	if lines, _ = authkeys.Read(path); len(lines) != 1 || !strings.HasSuffix(lines[0], second) {
		t.Fatalf("a refused request changed the file: %q", lines)
	}
	// An operator's key reaches the admin program and is the host's to change.
	if err := authkeys.Write(path, append(lines, authkeys.Line("/usr/local/bin/agent-bus-admin", "peer@h", edKey(t)))); err != nil {
		t.Fatal(err)
	}
	if code, body := send(s, tok("owner@h"), "POST", "/user/key", keyBody("peer@h", edKey(t)), ""); code != http.StatusConflict {
		t.Fatalf("an operator key was replaced: %d %s", code, body)
	}
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), "agent-bus-admin peer@h") {
		t.Fatal("the operator key is gone")
	}
}

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Only the runner's Owner and Maintainers manage it, and only as the daemon
// worked it out toward the runner itself.
func TestOnlyTheRunnersOwnerAndMaintainersManageIt(t *testing.T) {
	r := &runner{name: "#runner/me@h"}
	for _, c := range []struct {
		e    protocol.Envelope
		want bool
	}{
		{protocol.Envelope{Roles: []string{"owner"}}, true},
		{protocol.Envelope{Roles: []string{"guest", "maintainer"}}, true},
		{protocol.Envelope{OriginalTo: "#runner/me@h", Roles: []string{"owner"}}, true},
		{protocol.Envelope{}, false},
		{protocol.Envelope{Roles: []string{"admin"}}, false},
		{protocol.Envelope{OriginalTo: "#queue@h", Roles: []string{"owner"}}, false},
	} {
		if got := r.mayManage(c.e); got != c.want {
			t.Errorf("%+v: may manage %v, want %v", c.e, got, c.want)
		}
	}
}

// A write keeps the mode the host gave the file, and never replaces a link.
func TestAReplacedFileKeepsItsModeAndALinkIsRefused(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "agents.json")
	writeFile(t, p, "{}")
	if err := os.Chmod(p, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(p, []byte(`{"a":{}}`)); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o640 {
		t.Errorf("mode %v, want 0640", fi.Mode().Perm())
	}
	fresh := filepath.Join(dir, "new", "env")
	if err := replaceFile(fresh, []byte("A=1")); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(fresh); fi.Mode().Perm() != 0o600 {
		t.Errorf("a new file is %v, want 0600", fi.Mode().Perm())
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(p, link); err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(link, []byte("x")); err == nil {
		t.Error("a link was replaced")
	}
	if b, _ := os.ReadFile(p); string(b) != `{"a":{}}` {
		t.Errorf("the link's target changed: %s", b)
	}
}

// An agents.json entry naming nothing installed is not an agent to control.
func TestAControlRefusesAnEntryNamingNothingInstalled(t *testing.T) {
	r := &runner{realm: "h", installed: []installed{{Key: "ghost", Refused: "agents.json: no agent ghost is installed in agent.d"}, {Key: "real", Dir: "real"}}}
	if _, err := r.keyOf("ghost"); err == nil {
		t.Error("a dangling entry was taken as an agent")
	}
	if k, err := r.keyOf("#real@h"); err != nil || k != "real" {
		t.Errorf("real: %q %v", k, err)
	}
}

// An instance's agent is a directory name, never a path: "." and ".." are
// directories too, and would reach the runner's own files or out of runner/.
func TestAnInstanceKeyIsNeverAPath(t *testing.T) {
	home := t.TempDir()
	r := &runner{agents: filepath.Join(home, "agent.d"), host: filepath.Join(home, "runner")}
	writeFile(t, filepath.Join(r.agents, "mail", "agent.json"), `{"command":"x"}`)
	for _, key := range []string{"../outside", "./tokens", "./work", "../agent.d", "mail/..", "mail/.", "mail/env"} {
		if _, _, err := r.instanceKey(key); err == nil {
			t.Errorf("%s was taken as an instance", key)
		}
	}
	if d, i, err := r.instanceKey("mail/home"); err != nil || d != "mail" || i != "home" {
		t.Errorf("mail/home: %q %q %v", d, i, err)
	}
}

// agents.json saying null is no entries, not a nil map a later write panics on;
// a write of it is refused.
func TestAnAgentsFileOfNullIsNoEntries(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "agents.json"), "null")
	entries, err := readAgents(dir)
	if err != nil {
		t.Fatal(err)
	}
	entries["x"] = map[string]json.RawMessage{} // panics on a nil map
	for _, b := range []string{"null", " ", "[]"} {
		if checkAgents([]byte(b)) == nil {
			t.Errorf("%q was accepted as agents.json", b)
		}
	}
	if err := checkAgents([]byte(`{"a":{"enabled":false}}`)); err != nil {
		t.Error(err)
	}
}

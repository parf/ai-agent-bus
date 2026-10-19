package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	writeFile(t, filepath.Join(r.agents, "mail", "agent.json"), `{"description":"Test agent","command":"x"}`)
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

// A link under the runner's home is never read or written through, by name.
func TestTheHostsFilesAreNeverReachedThroughALink(t *testing.T) {
	home := t.TempDir()
	r := &runner{agents: filepath.Join(home, "agent.d"), host: filepath.Join(home, "runner")}
	elsewhere := filepath.Join(home, "elsewhere")
	writeFile(t, elsewhere, "SECRET=outside\n")
	writeFile(t, filepath.Join(r.agents, "mail", "env.dist"), "A=\n")
	if err := os.MkdirAll(filepath.Join(r.host, "mail"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, l := range []string{filepath.Join(r.host, "mail", "env"), filepath.Join(r.agents, "mail", "agent.json")} {
		if err := os.Symlink(elsewhere, l); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"runner/mail/env", "agent.d/mail/agent.json"} {
		if a, err := r.readFile(f); err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Errorf("read %s through a link: %+v %v", f, a, err)
		}
	}
	if err := r.writeHostFile("runner/mail/env", "SECRET=written\n"); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Errorf("write through a link: %v", err)
	}
	if b, _ := os.ReadFile(elsewhere); string(b) != "SECRET=outside\n" {
		t.Errorf("written through the link: %q", b)
	}
	if a, err := r.readFile("agent.d/mail/env.dist"); err != nil || (a.Content == nil || *a.Content != "A=\n") {
		t.Errorf("a plain file: %+v %v", a, err)
	}
}

// tokens and work are the runner's own under runner/, but an author may still
// name an agent so: its own files are read, its host files and instances are not.
func TestAnAgentNamedLikeTheRunnersDirectoriesHasNoHostFiles(t *testing.T) {
	home := t.TempDir()
	r := &runner{agents: filepath.Join(home, "agent.d"), host: filepath.Join(home, "runner")}
	writeFile(t, filepath.Join(r.agents, "tokens", "agent.json"), `{"description":"Test agent","command":"x"}`)
	if a, err := r.readFile("agent.d/tokens/agent.json"); err != nil || !a.Exists {
		t.Errorf("the author's file of an agent named tokens: %+v %v", a, err)
	}
	if _, err := r.readFile("runner/tokens/env"); err == nil {
		t.Error("runner/tokens/env was named")
	}
	if _, _, err := r.instanceKey("tokens/x"); err == nil {
		t.Error("an instance under runner/tokens was taken")
	}
}

// An instance's files go before its entry: a failure between leaves an entry
// a retry finds, never files no entry names.
func TestADeleteThatFailsKeepsTheEntry(t *testing.T) {
	home := t.TempDir()
	r := &runner{agents: filepath.Join(home, "agent.d"), host: filepath.Join(home, "runner")}
	writeFile(t, filepath.Join(r.agents, "mail", "agent.json"), `{"description":"Test agent","command":"x"}`)
	writeFile(t, filepath.Join(r.host, "agents.json"), `{"mail/home":{"enabled":true}}`)
	writeFile(t, filepath.Join(r.host, "mail", "home", "env"), "A=1\n")
	locked := filepath.Join(r.host, "mail", "home")
	if err := os.Chmod(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o700)
	if err := r.deleteInstance("mail/home"); err == nil {
		t.Fatal("the env could not be removed, yet the delete succeeded")
	}
	if b, _ := os.ReadFile(filepath.Join(r.host, "agents.json")); !strings.Contains(string(b), "mail/home") {
		t.Errorf("the entry went before its files: %s", b)
	}
}

// A directory on the way that is a link is refused too: the final file is
// then a plain one, and only the walk over each component sees the link.
func TestAHostDirectoryThatIsALinkIsNeverWalkedThrough(t *testing.T) {
	home := t.TempDir()
	r := &runner{agents: filepath.Join(home, "agent.d"), host: filepath.Join(home, "runner")}
	outside := filepath.Join(home, "outside")
	writeFile(t, filepath.Join(outside, "env"), "SECRET=outside\n")
	writeFile(t, filepath.Join(r.agents, "mail", "agent.json"), `{"description":"Test agent","command":"x"}`)
	if err := os.MkdirAll(r.host, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(r.host, "mail")); err != nil {
		t.Fatal(err)
	}
	if a, err := r.readFile("runner/mail/env"); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Errorf("read through a linked directory: %+v %v", a, err)
	}
	for _, f := range []string{"runner/mail/env", "runner/mail/agent.json"} {
		if err := r.writeHostFile(f, "SECRET=written\n"); err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Errorf("write %s through a linked directory: %v", f, err)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(outside, "env")); string(b) != "SECRET=outside\n" {
		t.Errorf("written through the linked directory: %q", b)
	}
	if _, err := os.Stat(filepath.Join(outside, "agent.json")); err == nil {
		t.Error("a file was created through the linked directory")
	}
}

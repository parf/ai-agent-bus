package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A package link may not reach the runner's own directory (env secrets, overrides) or the
// directory of packages: each is listed as refused by name, and its files are not readable.
func TestAPackageLinkIntoTheRunnersDirectoryOrAgentDIsRefusedByName(t *testing.T) {
	root, host := t.TempDir(), t.TempDir()
	directoryAgent(t, host, "private", "secret")
	directoryAgent(t, root, "real", "real")
	for name, target := range map[string]string{
		"hostpkg":  filepath.Join(host, "private"),
		"hostself": host,
		"sibling":  filepath.Join(root, "real"),
		"self":     root,
	} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	list, err := loadSources([]agentSource{{path: root}}, host)
	if err != nil {
		t.Fatal(err)
	}
	refused := map[string]string{}
	for _, in := range list {
		refused[in.Key] = in.Refused
	}
	if refused["real"] != "" {
		t.Fatalf("an ordinary package was refused: %q", refused["real"])
	}
	for _, name := range []string{"hostpkg", "hostself", "sibling", "self"} {
		why, listed := refused[name]
		if !listed || !strings.Contains(why, "symbolic link") || !strings.Contains(why, "inside") || !strings.Contains(why, filepath.Join(root, name)) {
			t.Errorf("link %s: listed=%v refused=%q, want a named refusal", name, listed, why)
		}
	}
}

// The same refusal reaches the file calls: a host-directory link is no package to read from.
func TestAFileCallThroughAHostDirectoryLinkIsRefused(t *testing.T) {
	r, _, _ := configurationFixture(t, "", "printf old")
	directoryAgent(t, r.host, "private", "secret")
	if err := os.Symlink(filepath.Join(r.host, "private"), filepath.Join(r.agents, "evil")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(r.host, "agents.json"), `{"fixture":{"sandbox":"off","enabled":true},"evil":{"enabled":false}}`)
	r.reload()
	if _, err := r.readFile("agent.d/evil/env.dist"); err == nil || !strings.Contains(err.Error(), "inside") {
		t.Fatalf("read through a link into the runner's directory: %v", err)
	}
}

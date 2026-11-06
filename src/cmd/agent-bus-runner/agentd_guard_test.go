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

// The guard compares absolute paths: a relative --host cannot let a link into it through.
func TestAPackageLinkIntoARelativeHostDirectoryIsStillRefused(t *testing.T) {
	root, host := t.TempDir(), t.TempDir()
	directoryAgent(t, host, "private", "secret")
	if err := os.Symlink(filepath.Join(host, "private"), filepath.Join(root, "hostpkg")); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relativeHost, err := filepath.Rel(cwd, host)
	if err != nil || filepath.IsAbs(relativeHost) {
		t.Skipf("no relative path from %s to %s: %v", cwd, host, err)
	}
	list, err := loadSources([]agentSource{{path: root}}, relativeHost)
	if err != nil || len(list) != 1 {
		t.Fatalf("%+v %v", list, err)
	}
	if !strings.Contains(list[0].Refused, "inside") {
		t.Fatalf("a link into a relative host directory loaded: %+v", list[0])
	}
	// A relative agents directory too.
	relativeRoot, err := filepath.Rel(cwd, root)
	if err != nil {
		t.Skip(err)
	}
	list, err = loadSources([]agentSource{{path: relativeRoot}}, host)
	if err != nil || len(list) != 1 || !strings.Contains(list[0].Refused, "inside") {
		t.Fatalf("a link into the host directory under a relative agents directory loaded: %+v %v", list, err)
	}
	// A relative link target under a relative agents directory resolves to a relative path.
	target, err := filepath.Rel(root, filepath.Join(host, "private"))
	if err != nil {
		t.Skip(err)
	}
	if err := os.Remove(filepath.Join(root, "hostpkg")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "hostpkg")); err != nil {
		t.Fatal(err)
	}
	list, err = loadSources([]agentSource{{path: relativeRoot}}, host)
	if err != nil || len(list) != 1 || !strings.Contains(list[0].Refused, "inside") {
		t.Fatalf("a relative link into the host directory loaded: %+v %v", list, err)
	}
}

// The guard fails closed: a protected root that cannot be resolved refuses the link, instead of
// skipping the comparison. A root that does not exist has no links to resolve and is compared by its
// absolute path.
func TestGuardRefusesWhenAProtectedRootCannotBeResolved(t *testing.T) {
	root := t.TempDir()
	checkout := directoryAgent(t, t.TempDir(), "private", "private")
	if err := os.Symlink(checkout, filepath.Join(root, "pkg")); err != nil {
		t.Fatal(err)
	}
	// A host path under a regular file cannot be resolved: ENOTDIR, not "does not exist".
	file := filepath.Join(t.TempDir(), "file")
	writeFile(t, file, "x")
	if _, _, err := packagePath(root, filepath.Join(file, "host"), "pkg"); err == nil {
		t.Fatal("an unresolvable protected host was silently skipped")
	}
	// A host that does not exist yet cannot hold the checkout: the link stands.
	if _, ok, err := packagePath(root, filepath.Join(t.TempDir(), "missing-host"), "pkg"); err != nil || !ok {
		t.Fatalf("a missing host refused a link elsewhere: ok=%v err=%v", ok, err)
	}
}

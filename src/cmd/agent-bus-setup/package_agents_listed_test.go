package main

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The release archive is what src/agents/package.sh writes, and setup refuses
// an archive that names a file it does not list ("not a release artifact").
// So an agent added to the packager without its files in bundleFiles breaks
// every fresh install, which is how the db agent's host-copied native
// libraries did from 0.9.129. This runs the real packager and holds each file
// it writes against the list, which it does not derive from.
func TestEveryFileTheAgentPackagerWritesIsInTheBundleContract(t *testing.T) {
	dest := t.TempDir()
	out, err := exec.Command("../../agents/package.sh", dest).CombinedOutput()
	if err != nil {
		t.Fatalf("agents/package.sh: %v\n%s", err, out)
	}
	listed := map[string]bool{}
	for _, name := range bundleFiles {
		listed[name] = true
	}
	var unlisted []string
	err = filepath.WalkDir(dest, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dest, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		// The debug agent's tree is tolerated by setup's acceptance rule.
		if !listed[name] && !strings.HasPrefix(name, "agents/debug/") {
			unlisted = append(unlisted, name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(unlisted)
	if len(unlisted) > 0 {
		t.Fatalf("agents/package.sh writes %d files setup's bundleFiles does not list, so setup would refuse the archive: %s", len(unlisted), strings.Join(unlisted[:min(len(unlisted), 8)], ", "))
	}
	if _, err := os.Stat(filepath.Join(dest, "agents/db")); err == nil {
		t.Fatal("agents/db is in the release: its native libraries are copied from the host, so it ships from a checkout until the owner decides (Q)")
	}
}

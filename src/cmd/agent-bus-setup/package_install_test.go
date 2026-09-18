package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/version"
)

var requiredBundleContract = []string{
	"agent-bus", "agent-busd", "agent-bus-admin", "agent-bus-setup", "agent-bus-token", "agent-bus-web",
	"mcp/server.js", "launchers/launcher.js",
	"launchers/ab-claude", "launchers/ab-codex", "launchers/ab-opencode",
	"internal/version/VERSION", "LICENSE.md", "INSTALL.md",
}

func fixtureBundle(t *testing.T, suffix string) string {
	t.Helper()
	root := t.TempDir()
	var lines []string
	for _, name := range bundleFiles {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		body := []byte("artifact " + name + " " + suffix + "\n")
		if name == "internal/version/VERSION" {
			body = []byte(version.String + "\n")
		}
		mode := os.FileMode(0o644)
		if executableFiles[name] {
			mode = 0o755
		}
		if err := os.WriteFile(filepath.Join(root, name), body, mode); err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256(body)
		lines = append(lines, fmt.Sprintf("%x  %s", h, name))
	}
	sort.Strings(lines)
	if err := os.WriteFile(filepath.Join(root, "MANIFEST.sha256"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func withInstallPaths(t *testing.T) (root, bin string) {
	t.Helper()
	oldRoot, oldBin := installRoot, installBin
	base := t.TempDir()
	installRoot, installBin = filepath.Join(base, "lib"), filepath.Join(base, "bin")
	t.Cleanup(func() { installRoot, installBin = oldRoot, oldBin })
	return installRoot, installBin
}

func TestBundleMustBeCompleteBeforeInstallation(t *testing.T) {
	if !slices.Equal(bundleFiles, requiredBundleContract) {
		t.Fatalf("bundle contract=%q, want %q", bundleFiles, requiredBundleContract)
	}
	root := fixtureBundle(t, "complete")
	if err := os.Remove(filepath.Join(root, "mcp/server.js")); err != nil {
		t.Fatal(err)
	}
	if _, err := checkBundle(root); err == nil || !strings.Contains(err.Error(), "mcp/server.js") {
		t.Fatalf("missing MCP face: %v", err)
	}
}

func TestBundleRejectsDamagedAndNonExecutableArtifacts(t *testing.T) {
	root := fixtureBundle(t, "damage")
	if err := os.WriteFile(filepath.Join(root, "mcp/server.js"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := checkBundle(root); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("damaged face: %v", err)
	}

	root = fixtureBundle(t, "mode")
	if err := os.Chmod(filepath.Join(root, "agent-busd"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := checkBundle(root); err == nil || !strings.Contains(err.Error(), "not executable") {
		t.Fatalf("non-executable daemon: %v", err)
	}
}

func TestBundleManifestIsAnExactAllowlist(t *testing.T) {
	root := fixtureBundle(t, "extra")
	extra := []byte("not installed\n")
	h := sha256.Sum256(extra)
	f, err := os.OpenFile(filepath.Join(root, "MANIFEST.sha256"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(f, "%x  extra-file\n", h); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "extra-file"), extra, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := checkBundle(root); err == nil || !strings.Contains(err.Error(), "artifacts") {
		t.Fatalf("extra manifest entry: %v", err)
	}
}

func TestInstallBundleSwitchesOneCompleteReleaseAndIsIdempotent(t *testing.T) {
	root, bin := withInstallPaths(t)
	b, err := checkBundle(fixtureBundle(t, "first"))
	if err != nil {
		t.Fatal(err)
	}
	current, err := installBundle(b)
	if err != nil {
		t.Fatal(err)
	}
	if current != filepath.Join(root, "current") {
		t.Fatalf("current=%s", current)
	}
	target, err := os.Readlink(current)
	if err != nil || target != filepath.Join("releases", b.releaseID) {
		t.Fatalf("current target=%q err=%v", target, err)
	}
	info, err := os.Stat(filepath.Join(root, "releases", b.releaseID))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("release mode=%v", info.Mode().Perm())
	}
	for name, want := range commandLinks() {
		got, err := os.Readlink(filepath.Join(bin, name))
		if err != nil || got != want {
			t.Errorf("%s target=%q want=%q err=%v", name, got, want, err)
		}
	}
	if _, err := installBundle(b); err != nil {
		t.Fatalf("identical reinstall: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "releases"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("release entries=%d err=%v", len(entries), err)
	}
}

func TestDifferentBundleOfSameVersionBecomesCurrent(t *testing.T) {
	root, _ := withInstallPaths(t)
	first, err := checkBundle(fixtureBundle(t, "first"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := checkBundle(fixtureBundle(t, "second"))
	if err != nil {
		t.Fatal(err)
	}
	if first.releaseID == second.releaseID {
		t.Fatal("different bundle got the same release id")
	}
	if _, err := installBundle(first); err != nil {
		t.Fatal(err)
	}
	if _, err := installBundle(second); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(filepath.Join(root, "current"))
	if err != nil || target != filepath.Join("releases", second.releaseID) {
		t.Fatalf("current target=%q err=%v", target, err)
	}
}

func TestDamagedExistingReleaseIsNeverSelectedAgain(t *testing.T) {
	root, _ := withInstallPaths(t)
	b, err := checkBundle(fixtureBundle(t, "installed"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := installBundle(b); err != nil {
		t.Fatal(err)
	}
	damaged := filepath.Join(root, "releases", b.releaseID, "mcp/server.js")
	if err := os.WriteFile(damaged, []byte("damaged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := installBundle(b); err == nil || !strings.Contains(err.Error(), "damaged") {
		t.Fatalf("damaged installed release: %v", err)
	}
}

func TestCommandCollisionLeavesReleaseUnselected(t *testing.T) {
	root, bin := withInstallPaths(t)
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "agent-bus"), []byte("unrelated"), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := checkBundle(fixtureBundle(t, "collision"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := installBundle(b); err == nil || !strings.Contains(err.Error(), "unrelated command") {
		t.Fatalf("collision err=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "current")); !os.IsNotExist(err) {
		t.Fatalf("current exists after collision: %v", err)
	}
}

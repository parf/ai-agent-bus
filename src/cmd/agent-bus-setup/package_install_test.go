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
	"agent-bus", "agent-busd", "agent-bus-admin", "agent-bus-setup", "agent-bus-token",
	"web/server.ts", "web/agent-bus-web.service",
	"mcp/server.js", "launchers/launcher.js",
	"launchers/ab-claude", "launchers/ab-codex", "launchers/ab-opencode",
	"internal/version/VERSION", "LICENSE.md", "INSTALL.md",
}

func fixtureBundle(t *testing.T, suffix string) string {
	return fixtureBundleVersion(t, version.String, suffix)
}

func fixtureBundleVersion(t *testing.T, bundleVersion, suffix string) string {
	t.Helper()
	root := t.TempDir()
	var lines []string
	for _, name := range bundleFiles {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		body := []byte("artifact " + name + " " + suffix + "\n")
		if name == "internal/version/VERSION" {
			body = []byte(bundleVersion + "\n")
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

func TestInstalledRollbackBundleMayHaveThePreviousVersion(t *testing.T) {
	root := fixtureBundleVersion(t, "0.5.68", "old")
	if _, err := checkBundle(root); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("incoming old-version bundle: %v", err)
	}
	b, err := checkBundleVersion(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(b.releaseID, "0.5.68-") {
		t.Fatalf("rollback release id=%s", b.releaseID)
	}
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
	if _, err := checkBundle(root); err == nil || !strings.Contains(err.Error(), "not a release artifact") {
		t.Fatalf("extra manifest entry: %v", err)
	}
}

// The web face's tree rides along: every file under web/ the manifest names
// is checked and installed, and a changed one is refused.
func TestWebSourcesAreInstalledAndChecked(t *testing.T) {
	withInstallPaths(t)
	root := fixtureBundle(t, "web")
	src := []byte("export const x = 1;\n")
	if err := os.WriteFile(filepath.Join(root, "web", "config.ts"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(src)
	f, err := os.OpenFile(filepath.Join(root, "MANIFEST.sha256"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(f, "%x  web/config.ts\n", h)
	f.Close()
	b, err := checkBundle(root)
	if err != nil {
		t.Fatal(err)
	}
	installed, err := installBundle(b)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(installed, "web", "config.ts")); err != nil || string(got) != string(src) {
		t.Fatalf("web/config.ts was not installed: %q %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(root, "web", "config.ts"), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := checkBundle(root); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("a changed web source: %v", err)
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

func TestDifferentBundleRequiresExplicitUpgradeSelection(t *testing.T) {
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
	if _, err := installBundle(second); err == nil || !strings.Contains(err.Error(), "--upgrade") {
		t.Fatalf("ordinary setup selected a different release: %v", err)
	}
	target, err := os.Readlink(filepath.Join(root, "current"))
	if err != nil || target != filepath.Join("releases", first.releaseID) {
		t.Fatalf("current target=%q err=%v", target, err)
	}
	if _, err := stageBundle(second); err != nil {
		t.Fatal(err)
	}
	if _, err := selectBundle(second.releaseID); err != nil {
		t.Fatal(err)
	}
	target, err = os.Readlink(filepath.Join(root, "current"))
	if err != nil || target != filepath.Join("releases", second.releaseID) {
		t.Fatalf("upgrade target=%q err=%v", target, err)
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

// An installed release from before 0.8.50 is a rollback target by its own
// manifest: the Go web binary and no web/ tree. It is still refused as an
// incoming package, and it still needs its daemon.
func TestOlderReleaseIsARollbackTargetByItsOwnManifest(t *testing.T) {
	old := func(t *testing.T, drop string) string {
		t.Helper()
		root := t.TempDir()
		var lines []string
		names := []string{"agent-bus", "agent-busd", "agent-bus-admin", "agent-bus-setup", "agent-bus-token", "agent-bus-web",
			"mcp/server.js", "launchers/launcher.js", "internal/version/VERSION", "LICENSE.md", "INSTALL.md"}
		for _, name := range names {
			if name == drop {
				continue
			}
			body := []byte("old " + name + "\n")
			if name == "internal/version/VERSION" {
				body = []byte("0.8.27\n")
			}
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, name), body, 0o755); err != nil {
				t.Fatal(err)
			}
			lines = append(lines, fmt.Sprintf("%x  %s", sha256.Sum256(body), name))
		}
		if err := os.WriteFile(filepath.Join(root, "MANIFEST.sha256"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return root
	}
	root := old(t, "")
	if b, err := checkBundleVersion(root, ""); err != nil || !strings.HasPrefix(b.releaseID, "0.8.27-") {
		t.Fatalf("0.8.27-shaped rollback target: %v", err)
	}
	if _, err := checkBundle(root); err == nil || !strings.Contains(err.Error(), "web/server.ts") {
		t.Fatalf("0.8.27-shaped incoming package: %v", err)
	}
	if _, err := checkBundleVersion(old(t, "agent-busd"), ""); err == nil || !strings.Contains(err.Error(), "agent-busd") {
		t.Fatalf("rollback target without its daemon: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "agent-bus-web"), []byte("changed"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := checkBundleVersion(root, ""); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("changed legacy web binary: %v", err)
	}
}

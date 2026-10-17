package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWebDirectoryFollowsExplicitExecutableSymlinks(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	checkout := filepath.Join(root, "checkout")
	bin := filepath.Join(root, "bin")
	for _, dir := range []string{checkout, bin} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	daemon := filepath.Join(checkout, "agent-busd")
	if err := os.WriteFile(daemon, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(bin, "agent-busd")
	if err := os.Symlink(daemon, link); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(checkout, "web")
	if got := webDirFor(link); got != want {
		t.Fatalf("linked executable's web directory = %q, want %q", got, want)
	}
	if got := webDirFor(daemon); got != want {
		t.Fatalf("direct executable's web directory = %q, want %q", got, want)
	}
	missing := filepath.Join(bin, "not-built-yet")
	if got := webDirFor(missing); got != filepath.Join(bin, "web") {
		t.Fatalf("unresolved explicit path's web directory = %q", got)
	}
}

func TestWebDirectoryKeepsManagedCurrentAfterResolvingSymlinks(t *testing.T) {
	withInstallPaths(t)
	if err := os.MkdirAll(installRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(installRoot)
	if err != nil {
		t.Fatal(err)
	}
	installRoot = resolvedRoot
	release := filepath.Join(installRoot, "releases", "fixture-build")
	if err := os.MkdirAll(release, 0o755); err != nil {
		t.Fatal(err)
	}
	daemon := filepath.Join(release, "agent-busd")
	if err := os.WriteFile(daemon, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(installRoot, "current")
	if err := os.Symlink(release, current); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	link := filepath.Join(bin, "agent-busd")
	if err := os.Symlink(filepath.Join(current, "agent-busd"), link); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(current, "web")
	for _, exe := range []string{daemon, filepath.Join(current, "agent-busd"), link} {
		if got := webDirFor(exe); got != want {
			t.Fatalf("managed executable %q's web directory = %q, want %q", exe, got, want)
		}
	}
}

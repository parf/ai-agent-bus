package main

import (
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestReleaseModesIgnoreRestrictiveUmask(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	dest := filepath.Join(root, "dest")
	_ = os.MkdirAll(filepath.Join(source, "web"), 0755)
	for _, name := range []string{"agent-busd", "web/server.ts", "agents/standalone/git/autopull.py"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(source, name)), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, name), []byte("fixture"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	old := syscall.Umask(027)
	defer syscall.Umask(old)
	for name, want := range map[string]os.FileMode{"agent-busd": 0755, "web/server.ts": 0644, "agents/standalone/git/autopull.py": 0755} {
		if err := copyBundleFile(source, dest, name); err != nil {
			t.Fatal(err)
		}
		st, err := os.Stat(filepath.Join(dest, name))
		if err != nil || st.Mode().Perm() != want {
			t.Fatalf("release file %s: %v %v", name, st, err)
		}
	}
	st, _ := os.Stat(filepath.Join(dest, "web"))
	if st.Mode().Perm() != 0755 {
		t.Fatalf("release directory: %v", st.Mode())
	}
}
func TestSamplesFailWhenUsersCannotBeRead(t *testing.T) {
	dir := shortSocketDir(t)
	socket := filepath.Join(dir, "bus.sock")
	listener, err := listenTestUnix(t, socket)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "unavailable", 503) })}
	go server.Serve(listener)
	defer server.Close()
	t.Setenv("AGENT_BUS_ADDR", socket)
	if _, err := newOwnerClient().people(); err == nil {
		t.Fatal("users failure hidden as empty people")
	}
}

func TestReleaseRootsIgnoreRestrictiveUmask(t *testing.T) {
	withInstallPaths(t)
	source := fixtureBundle(t, "root-modes")
	bundle, err := checkBundle(source)
	if err != nil {
		t.Fatal(err)
	}
	old := syscall.Umask(027)
	defer syscall.Umask(old)
	final, err := stageBundle(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := selectBundle(bundle.releaseID); err != nil {
		t.Fatal(err)
	}
	if err := installCommandLinks(); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{installRoot, filepath.Join(installRoot, "releases"), final, installBin} {
		st, err := os.Stat(dir)
		if err != nil || st.Mode().Perm() != 0755 {
			t.Fatalf("release root %s: %v %v", dir, st, err)
		}
	}
}

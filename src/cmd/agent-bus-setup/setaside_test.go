package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"syscall"
	"testing"
)

// fixtureNode is a daemon home with state and a drop-in directory with an
// override, as a reinstall finds them.
func fixtureNode(t *testing.T) (base, home, dropins string) {
	t.Helper()
	base = t.TempDir()
	home = filepath.Join(base, "daemon")
	dropins = filepath.Join(base, "agent-busd.service.d")
	for _, d := range []string{filepath.Join(home, ".ssh"), dropins} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, body := range map[string]string{
		filepath.Join(home, "agent-bus.db"):            "database",
		filepath.Join(home, ".ssh", "authorized_keys"): "keys",
		filepath.Join(dropins, "override.conf"):        "[Service]\nExecStart=\n",
	} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return base, home, dropins
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func TestSetAsideMovesHomeAndDropInsIntoARootOnlyDirectory(t *testing.T) {
	// A permissive umask must not widen the directory: its mode is the design.
	old := syscall.Umask(0)
	defer syscall.Umask(old)
	base, home, dropins := fixtureNode(t)
	aside := filepath.Join(base, "daemon.before-0.7-test")
	if err := moveAside(aside, home, dropins); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(aside)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("set-aside directory mode %v, want 0700", info.Mode().Perm())
	}
	if got := names(t, home); len(got) != 0 {
		t.Fatalf("home still holds %v", got)
	}
	if got := names(t, dropins); len(got) != 0 {
		t.Fatalf("drop-in directory still holds %v", got)
	}
	if b, err := os.ReadFile(filepath.Join(aside, "agent-busd.service.d", "override.conf")); err != nil || string(b) != "[Service]\nExecStart=\n" {
		t.Fatalf("drop-in was not set aside: %q %v", b, err)
	}
	if b, err := os.ReadFile(filepath.Join(aside, "home", "agent-bus.db")); err != nil || string(b) != "database" {
		t.Fatalf("database was not set aside: %q %v", b, err)
	}
	if err := moveAside(aside, home, dropins); err == nil {
		t.Fatal("an existing set-aside directory was reused")
	}
}

func TestRestoreAsidePutsTheNodeBack(t *testing.T) {
	base, home, dropins := fixtureNode(t)
	aside := filepath.Join(base, "daemon.before-0.7-test")
	if err := moveAside(aside, home, dropins); err != nil {
		t.Fatal(err)
	}
	if err := restoreAside(aside, home, dropins); err != nil {
		t.Fatal(err)
	}
	if got := names(t, home); !slices.Equal(got, []string{".ssh", "agent-bus.db"}) {
		t.Fatalf("home after restore: %v", got)
	}
	if b, err := os.ReadFile(filepath.Join(dropins, "override.conf")); err != nil || string(b) != "[Service]\nExecStart=\n" {
		t.Fatalf("drop-in was not restored: %q %v", b, err)
	}
	if _, err := os.Stat(aside); !os.IsNotExist(err) {
		t.Fatalf("empty set-aside directory remains: %v", err)
	}

	// What a failed install wrote is kept, and so is the directory holding it.
	if err := moveAside(aside, home, dropins); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "agent-bus.db"), []byte("fresh"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := restoreAside(aside, home, dropins); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(home, "agent-bus.db")); string(b) != "database" {
		t.Fatalf("restored database is %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(aside, "failed-reinstall", "agent-bus.db")); string(b) != "fresh" {
		t.Fatalf("the failed install's database was not kept: %q", b)
	}
}

func TestHoldersNamesAProcessWithTheHomeOpen(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "agent-bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cmd := exec.Command("sleep", "30")
	cmd.ExtraFiles = []*os.File{f}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	pid := strconv.Itoa(cmd.Process.Pid)
	got, err := holders(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(got, pid) {
		t.Fatalf("holders %v do not include the holding process %s", got, pid)
	}
	// A directory whose name is a prefix of the home's is not the home.
	other, err := holders(dir[:len(dir)-1])
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(other, pid) {
		t.Fatalf("a file in %s counted as holding %s", dir, dir[:len(dir)-1])
	}
}

func TestProgramVersionReadsTheBuildStamp(t *testing.T) {
	prog := filepath.Join(t.TempDir(), "agent-busd")
	stamped := "#!/bin/sh\nprintf '0.8.99\\nbuild_info: me@host 2026-09-23 10:00:00 abc1234\\n'\n"
	if err := os.WriteFile(prog, []byte(stamped), 0o755); err != nil {
		t.Fatal(err)
	}
	v, b, err := programVersion(prog)
	if err != nil || v != "0.8.99" || b != "me@host 2026-09-23 10:00:00 abc1234" {
		t.Fatalf("version=%q build=%q err=%v", v, b, err)
	}
	for _, unstamped := range []string{"echo 0.8.99", "printf '0.8.99\\nsomething else\\n'"} {
		if err := os.WriteFile(prog, []byte("#!/bin/sh\n"+unstamped+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, _, err := programVersion(prog); err == nil {
			t.Fatalf("%s was accepted as a stamped version", unstamped)
		}
	}
}

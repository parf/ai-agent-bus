package cgroup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A delegated subtree, as files: what the kernel's are written as.
func fakeTree(t *testing.T, controllers string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "cgroup.controllers"), []byte(controllers+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Taking the subtree moves the runner into a leaf of its own and enables the
// agents' controllers below the root; a subtree missing one is refused.
func TestADelegatedSubtreeIsTakenOrRefused(t *testing.T) {
	root := fakeTree(t, "cpuset cpu io memory pids")
	if _, err := delegate(root, 4242); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(root, "runner", "cgroup.procs")); got != "4242" {
		t.Errorf("the runner was moved as %q", got)
	}
	if got := read(t, filepath.Join(root, "cgroup.subtree_control")); got != "+cpu +io +memory +pids" {
		t.Errorf("subtree_control is %q", got)
	}
	if _, err := delegate(fakeTree(t, "cpu pids"), 1); err == nil || !strings.Contains(err.Error(), "the io controller is not delegated") {
		t.Errorf("a subtree without io, memory: %v", err)
	}
	if _, err := delegate(filepath.Join(t.TempDir(), "absent"), 1); err == nil {
		t.Error("a missing subtree was taken")
	}
}

// An agent's cgroup carries every limit, the unlimited ones as max, and never
// swap; setting them again replaces them.
func TestAnAgentsCgroupCarriesItsLimits(t *testing.T) {
	tr := &Tree{root: fakeTree(t, "cpu io memory pids")}
	g, err := tr.Agent("mail/work", Limits{Memory: 1 << 30, CPU: 200, Processes: 64, IOWeight: 50})
	if err != nil {
		t.Fatal(err)
	}
	defer g.Remove()
	if !strings.HasPrefix(filepath.Base(g.Path()), "agent-mail_work-") {
		t.Errorf("the cgroup is %s", g.Path())
	}
	// Two keys that read alike are two agents with two cgroups.
	other, err := tr.Agent("mail_work", Limits{})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Remove()
	if other.Path() == g.Path() {
		t.Errorf("mail/work and mail_work share %s", g.Path())
	}
	for file, want := range map[string]string{
		"memory.max": "1073741824", "memory.swap.max": "0", "pids.max": "64",
		"cpu.max": "200000 100000", "io.weight": "default 50",
	} {
		if got := read(t, filepath.Join(g.Path(), file)); got != want {
			t.Errorf("%s = %q, want %q", file, got, want)
		}
	}
	if err := g.Set(Limits{}); err != nil {
		t.Fatal(err)
	}
	for file, want := range map[string]string{"memory.max": "max", "pids.max": "max", "cpu.max": "max 100000"} {
		if got := read(t, filepath.Join(g.Path(), file)); got != want {
			t.Errorf("after Set, %s = %q, want %q", file, got, want)
		}
	}
}

func TestSizesAndSharesAreReadAsTheFormatsWriteThem(t *testing.T) {
	for in, want := range map[string]int64{"512M": 512 << 20, "1G": 1 << 30, "64k": 64 << 10, "100": 100} {
		if got, err := ParseBytes(in); err != nil || got != want {
			t.Errorf("ParseBytes(%q) = %d, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "1.5G", "-1M", "lots"} {
		if _, err := ParseBytes(bad); err == nil {
			t.Errorf("ParseBytes(%q) was accepted", bad)
		}
	}
	if n, err := ParsePercent("200%"); err != nil || n != 200 {
		t.Errorf("ParsePercent(200%%) = %d, %v", n, err)
	}
	for _, bad := range []string{"200", "0%", "x%"} {
		if _, err := ParsePercent(bad); err == nil {
			t.Errorf("ParsePercent(%q) was accepted", bad)
		}
	}
	if p, ok := unified("12:pids:/x\n0::/system.slice/r.service\n"); !ok || p != "/system.slice/r.service" {
		t.Errorf("unified = %q %v", p, ok)
	}
}

// Package cgroup is the runner's delegated cgroup subtree: one cgroup per
// agent, its limits written to that cgroup's own files, and every process of
// the agent started inside it (Plans/R1.0-Release/runner-implementation.md#sandboxing).
// The unit's Delegate=yes is what makes the subtree the runner's; no
// capability and no user manager is needed.
package cgroup

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Controllers are the ones the agents' limits are written to.
var Controllers = []string{"cpu", "io", "memory", "pids"}

// Limits are one agent's, as numbers: zero means the controller's own
// default, which for memory, CPU and processes is unlimited.
type Limits struct {
	Memory    int64 // bytes, memory.max; swap is never allowed
	CPU       int   // percent of one CPU, cpu.max
	Processes int   // pids.max
	IOWeight  int   // io.weight, 1..10000
}

// Tree is a delegated subtree the runner holds.
type Tree struct{ root string }

// Delegated takes the subtree the calling process was started in: it moves
// itself into a leaf of its own, since a cgroup with controllers enabled for
// its children may hold no process, and enables the agents' controllers below.
// Anything missing is an error, never a quiet run without limits.
func Delegated() (*Tree, error) {
	self, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return nil, err
	}
	path, ok := unified(string(self))
	if !ok {
		return nil, errors.New("not on the unified cgroup hierarchy (cgroup v2)")
	}
	return delegate(filepath.Join("/sys/fs/cgroup", path), os.Getpid())
}

// unified is the cgroup v2 path in a /proc/<pid>/cgroup listing.
func unified(listing string) (string, bool) {
	for _, line := range strings.Split(strings.TrimSpace(listing), "\n") {
		if p, ok := strings.CutPrefix(line, "0::"); ok && p != "" {
			return p, true
		}
	}
	return "", false
}

func delegate(root string, pid int) (*Tree, error) {
	have, err := os.ReadFile(filepath.Join(root, "cgroup.controllers"))
	if err != nil {
		return nil, fmt.Errorf("no cgroup at %s: %w", root, err)
	}
	for _, c := range Controllers {
		if !hasWord(string(have), c) {
			return nil, fmt.Errorf("the %s controller is not delegated to %s (Delegate=yes in its unit)", c, root)
		}
	}
	leaf := filepath.Join(root, "runner")
	if err := os.Mkdir(leaf, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("the subtree %s is not the runner's to write (Delegate=yes in its unit): %w", root, err)
	}
	if err := write(leaf, "cgroup.procs", strconv.Itoa(pid)); err != nil {
		return nil, fmt.Errorf("moving the runner into %s: %w", leaf, err)
	}
	enable := make([]string, len(Controllers))
	for i, c := range Controllers {
		enable[i] = "+" + c
	}
	if err := write(root, "cgroup.subtree_control", strings.Join(enable, " ")); err != nil {
		return nil, fmt.Errorf("enabling %s below %s: %w", strings.Join(Controllers, ", "), root, err)
	}
	return &Tree{root: root}, nil
}

func hasWord(s, w string) bool {
	for _, f := range strings.Fields(s) {
		if f == w {
			return true
		}
	}
	return false
}

func write(dir, file, value string) error {
	return os.WriteFile(filepath.Join(dir, file), []byte(value), 0o644)
}

// Group is one agent's cgroup.
type Group struct {
	mu     sync.Mutex
	limits Limits
	killed bool
	path   string
	dir    *os.File // held open: a child is cloned into it by descriptor
}

var unsafe = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

// Agent is key's cgroup with l written to it, made if it is not there.
func (t *Tree) Agent(key string, l Limits) (*Group, error) {
	// The key made readable, and a hash of it as written: a/b and a_b read
	// the same and are two agents, with two cgroups (review ri-M4 P3-4).
	sum := sha256.Sum256([]byte(key))
	path := filepath.Join(t.root, "agent-"+unsafe.ReplaceAllString(key, "_")+"-"+hex.EncodeToString(sum[:4]))
	if err := os.Mkdir(path, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	if err := apply(path, l); err != nil {
		return nil, err
	}
	dir, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &Group{path: path, dir: dir, limits: l}, nil
}

// apply writes every limit, the unlimited ones too, so a limit taken away at
// a reload is taken away from the running cgroup as well.
func apply(path string, l Limits) error {
	files := [][2]string{
		{"memory.max", orMax(l.Memory)},
		{"memory.swap.max", "0"},
		{"pids.max", orMax(int64(l.Processes))},
		{"cpu.max", cpuMax(l.CPU)},
	}
	if l.IOWeight > 0 {
		files = append(files, [2]string{"io.weight", "default " + strconv.Itoa(l.IOWeight)})
	}
	for _, f := range files {
		if err := write(path, f[0], f[1]); err != nil {
			return fmt.Errorf("%s: %w", f[0], err)
		}
	}
	return nil
}

func orMax(n int64) string {
	if n <= 0 {
		return "max"
	}
	return strconv.FormatInt(n, 10)
}

// cpuMax is a percentage of one CPU as cpu.max's quota over a 100 ms period.
func cpuMax(percent int) string {
	if percent <= 0 {
		return "max 100000"
	}
	return strconv.Itoa(percent*1000) + " 100000"
}

// Set writes l again, to the cgroup that is already the agent's.
func (g *Group) Set(l Limits) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if err := apply(g.path, l); err != nil {
		return err
	}
	g.limits = l
	return nil
}

// Kill marks this generation unusable before killing its processes. Kernels
// with the CLONE_INTO_CGROUP kill_seq bug kill every later child in a killed
// cgroup: https://kernel.googlesource.com/pub/scm/linux/kernel/git/tip/tip/+/8e359920216689b3b79e0fe8961a77fe312a511f
func (g *Group) Kill() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.killed = true
	return os.WriteFile(filepath.Join(g.path, "cgroup.kill"), []byte("1"), 0644)
}

// Prepare replaces a killed, now empty cgroup before another process starts.
// It refuses a busy group rather than starting in a poisoned generation.
func (g *Group) Prepare() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.killed {
		return nil
	}
	// cgroup.kill returns before every dying task has left the group.
	end := time.Now().Add(2 * time.Second)
	for {
		events, err := os.ReadFile(filepath.Join(g.path, "cgroup.events"))
		if err != nil {
			return err
		}
		if strings.Contains("\n"+string(events), "\npopulated 0\n") {
			break
		}
		if time.Now().After(end) {
			return fmt.Errorf("%s: still populated after cgroup.kill", g.path)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := os.Remove(g.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Mkdir(g.path, 0755); err != nil {
		return err
	}
	if err := apply(g.path, g.limits); err != nil {
		return err
	}
	dir, err := os.Open(g.path)
	if err != nil {
		return err
	}
	g.dir.Close()
	g.dir = dir
	g.killed = false
	return nil
}

// FD is the descriptor a child is cloned into (CLONE_INTO_CGROUP).
func (g *Group) FD() int { g.mu.Lock(); defer g.mu.Unlock(); return int(g.dir.Fd()) }

func (g *Group) Path() string { return g.path }

// Remove gives the cgroup up once nothing runs in it; one still busy stays,
// and is taken again by the next Agent of its key.
func (g *Group) Remove() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.dir.Close()
	os.Remove(g.path)
}

var size = regexp.MustCompile(`^([0-9]+)([KMGT]?)$`)

// ParseBytes reads a size as the formats write it: 512M, 1G, or bytes.
func ParseBytes(s string) (int64, error) {
	m := size.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(s)))
	if m == nil {
		return 0, fmt.Errorf("%q is not a size like 512M or 1G", s)
	}
	n, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0, err
	}
	shift := map[string]uint{"": 0, "K": 10, "M": 20, "G": 30, "T": 40}[m[2]]
	return n << shift, nil
}

// ParsePercent reads a CPU share as the formats write it: 200%.
func ParsePercent(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(s), "%"))
	if err != nil || n <= 0 || !strings.HasSuffix(strings.TrimSpace(s), "%") {
		return 0, fmt.Errorf("%q is not a CPU share like 200%%", s)
	}
	return n, nil
}

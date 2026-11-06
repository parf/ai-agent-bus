package main

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// hookGate parks a test hook until the test opens it. The test's cleanup opens it too, so a hook
// that arrives after a failure never parks for good holding reloadMu; a second arrival fails the test
// instead of panicking on a closed channel.
type hookGate struct {
	t                   *testing.T
	reached, release    chan struct{}
	reachOnce, openOnce sync.Once
}

func newHookGate(t *testing.T) *hookGate {
	g := &hookGate{t: t, reached: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(g.open)
	return g
}

func (g *hookGate) hook() {
	first := false
	g.reachOnce.Do(func() { close(g.reached); first = true })
	if !first {
		g.t.Error("the hook ran a second time")
	}
	<-g.release
}

func (g *hookGate) open() { g.openOnce.Do(func() { close(g.release) }) }

// J100: a shutdown that begins after an install's last cancellation check must not leave a process
// the install's apply starts once stopKept has gone by. The runner's stop waits for the install to
// publish, then stops what it serves, or the install's apply serves nothing; either way nothing runs.
func TestAShutdownDuringAnInstallLeavesNoProcessRunning(t *testing.T) {
	r, _, _ := configurationFixture(t, "itself", "exec sleep 30")
	awaitConfigurationState(t, r, "running")
	r.bundled = t.TempDir()
	pkg := filepath.Join(r.bundled, "fixture")
	writeFile(t, filepath.Join(pkg, "run.sh"), "exec sleep 31\n")
	writeFile(t, filepath.Join(pkg, "agent.json"), string(mustJSON(map[string]any{"description": "Test agent", "command": "/bin/sh " + filepath.Join(r.agents, "fixture", "run.sh"), "serves": "itself", "processes": 1, "deadline": "100ms"})))
	g := newHookGate(t)
	r.afterInstallCheck = g.hook
	installed := make(chan error, 1)
	go func() { installed <- r.installPackage(context.Background(), "install --replace fixture") }()
	select {
	case <-g.reached:
	case <-time.After(10 * time.Second):
		t.Fatal("the install never reached its last check")
	}
	// The runner is told to stop exactly here: after the install's last check, before its apply.
	stopped := make(chan struct{})
	go func() { r.stopKept(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(300 * time.Millisecond):
	}
	g.open()
	for name, ch := range map[string]<-chan struct{}{"the install": wait(installed), "the shutdown": stopped} {
		select {
		case <-ch:
		case <-time.After(10 * time.Second):
			t.Fatalf("%s did not finish within 10 s", name)
		}
	}
	time.Sleep(300 * time.Millisecond) // a keeper left running would have started its process by now
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, s := range r.served {
		if s.kept == nil {
			continue
		}
		s.kept.mu.Lock()
		n := len(s.kept.running)
		s.kept.mu.Unlock()
		if n != 0 || s.kept.ctx.Err() == nil {
			t.Errorf("%s has %d processes and a live keeper after shutdown", key, n)
		}
	}
	if _, err := os.Stat(filepath.Join(r.agents, "fixture", "run.sh")); err != nil {
		t.Fatalf("the install was not published: %v", err)
	}
}

func wait(errs <-chan error) <-chan struct{} {
	done := make(chan struct{})
	go func() { <-errs; close(done) }()
	return done
}

// J100: once stopKept has run, a reload or an install that takes reloadMu afterwards starts nothing.
func TestAReloadAfterShutdownStartsNothing(t *testing.T) {
	r, _, _ := configurationFixture(t, "itself", "exec sleep 30")
	awaitConfigurationState(t, r, "running")
	r.stopKept()
	r.mu.Lock()
	for _, s := range r.served {
		s.kept = nil // forget the stopped keeper, so a reload would adopt it again
	}
	r.served = make(map[string]*served)
	r.mu.Unlock()
	r.reload()
	time.Sleep(300 * time.Millisecond)
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.served) != 0 {
		t.Fatalf("a reload after shutdown served %d agents", len(r.served))
	}
}

// J100: a stop that begins while an apply is already past its closing check waits for it, then stops
// what it started.
func TestAShutdownDuringAnApplyStopsWhatItStarts(t *testing.T) {
	r, _, _ := configurationFixture(t, "itself", "exec sleep 30")
	awaitConfigurationState(t, r, "running")
	r.stopKept() // the first keeper is gone; the reload below adopts the agent again
	r.mu.Lock()
	r.served = make(map[string]*served)
	r.mu.Unlock()
	r.closing.Store(false)
	g := newHookGate(t)
	r.applyStarted = g.hook
	reloaded := make(chan struct{})
	go func() { r.reload(); close(reloaded) }()
	select {
	case <-g.reached:
	case <-time.After(10 * time.Second):
		t.Fatal("the reload never reached its apply")
	}
	stopped := make(chan struct{})
	go func() { r.stopKept(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(300 * time.Millisecond):
	}
	g.open()
	for name, ch := range map[string]<-chan struct{}{"the reload": reloaded, "the shutdown": stopped} {
		select {
		case <-ch:
		case <-time.After(10 * time.Second):
			t.Fatalf("%s did not finish within 10 s", name)
		}
	}
	time.Sleep(300 * time.Millisecond)
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, s := range r.served {
		if s.kept != nil && s.kept.ctx.Err() == nil {
			t.Errorf("%s has a live keeper after shutdown", key)
		}
	}
}

package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/cgroup"
	"github.com/parf/ai-agent-bus/internal/sandbox"
)

type retirementBus struct {
	name   string
	called chan struct{}
}

func (b retirementBus) RoundTrip(q *http.Request) (*http.Response, error) {
	select {
	case b.called <- struct{}{}:
	default:
	}
	body := `{}`
	switch q.URL.Path {
	case "/status":
		body = fmt.Sprintf(`{"you":%q}`, b.name)
	case "/lookup":
		body = `{"owner":"person"}`
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
}

// A keeper blocked in Wait stands for a child draining. Its cancellation and
// release are explicit, so a missing retirement wait fails rather than hangs.
func retiringFixture(t *testing.T, newName string) (*runner, *served, chan struct{}, func(), chan struct{}) {
	t.Helper()
	called := make(chan struct{}, 32)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	r := &runner{ctx: ctx, say: io.Discard, person: "person", host: t.TempDir(), agents: t.TempDir(), served: make(map[string]*served)}
	r.bus = &client{http: &http.Client{Transport: retirementBus{name: newName, called: called}}, base: "http://fixture"}
	in := installed{Key: "fixture", Dir: "fixture", Agent: agentFile{Command: "true", Serves: "itself", Deadline: "2s"}, Host: hostEntry{Sandbox: "off", Name: "#old@h"}, Span: span{Min: 1, Max: 1}}
	s := &served{in: in, name: "#old@h", cmd: "true", env: []string{}, work: t.TempDir(), dir: t.TempDir(), bus: r.bus, box: sandbox.Off{}, jail: &jail{}, cancel: func() {}}
	began := make(chan struct{})
	var once sync.Once
	k := &keeper{r: r, s: s, ctx: context.Background(), cancel: func() { once.Do(func() { close(began) }) }}
	k.wg.Add(1)
	s.kept = k
	r.served[s.name] = s
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(k.wg.Done) }
	t.Cleanup(release)
	if err := os.MkdirAll(filepath.Dir(r.tokenFile(newName)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.tokenFile(newName), []byte("fixture-token"), 0600); err != nil {
		t.Fatal(err)
	}
	return r, s, began, release, called
}
func serveReload(r *runner) chan struct{} {
	done := make(chan struct{})
	go func() {
		r.reloadMu.Lock()
		defer r.reloadMu.Unlock()
		r.mu.Lock()
		defer r.mu.Unlock()
		r.serve(r.ctx)
		close(done)
	}()
	return done
}
func awaitRetirement(t *testing.T, c <-chan struct{}) {
	t.Helper()
	select {
	case <-c:
	case <-time.After(time.Second):
		t.Fatal("retirement did not start")
	}
}
func assertStillRetiring(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
		t.Fatal("operation finished before the old keeper retired")
	case <-time.After(80 * time.Millisecond):
	}
}

func TestReadoptionWaitsForTheSameCgroupKeyAfterRename(t *testing.T) {
	r, s, began, release, called := retiringFixture(t, "#new@h")
	r.mu.Lock()
	r.installed = []installed{s.in}
	r.installed[0].Host.Name = "#new@h"
	r.mu.Unlock()
	done := serveReload(r)
	awaitRetirement(t, began)
	assertStillRetiring(t, done)
	select {
	case <-called:
		t.Error("replacement contacted the bus before its cgroup key retired")
	default:
	}
	release()
	awaitRetirement(t, done)
	r.mu.Lock()
	next := r.served["#new@h"]
	r.mu.Unlock()
	if next == nil || next.kept == nil {
		t.Fatal("replacement was not adopted after retirement")
	}
	r.stopKept()
}
func TestDisableWaitsForCommandRestartRetirement(t *testing.T) {
	r, s, began, release, _ := retiringFixture(t, safeRetiringName)
	s.cmd = "false" // the existing reload trigger: command changed
	r.installed = []installed{s.in}
	first := serveReload(r)
	awaitRetirement(t, first)
	awaitRetirement(t, began)
	no := false
	r.mu.Lock()
	r.installed[0].Host.Enabled = &no
	r.mu.Unlock()
	done := serveReload(r)
	assertStillRetiring(t, done)
	release()
	awaitRetirement(t, done)
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.served) != 0 {
		t.Fatal("disabled agent restarted after retirement")
	}
}

const safeRetiringName = "#old@h"

func TestRunnerShutdownWaitsForAlreadyRetiringKeepers(t *testing.T) {
	r, s, began, release, _ := retiringFixture(t, safeRetiringName)
	r.mu.Lock()
	r.stop(s.name, s)
	r.mu.Unlock()
	awaitRetirement(t, began)
	done := make(chan struct{})
	go func() { r.stopKept(); close(done) }()
	assertStillRetiring(t, done)
	release()
	awaitRetirement(t, done)
}
func TestKeeperSignalsOnlyJailsOfItsStartedProcesses(t *testing.T) {
	r := &runner{ctx: context.Background(), say: io.Discard}
	s := &served{name: "#fixture@h", work: t.TempDir(), dir: t.TempDir(), cmd: "/bin/sleep 3", box: sandbox.Off{}, jail: &jail{}, in: installed{Agent: agentFile{Deadline: "50ms"}, Span: span{Min: 1, Max: 1}}}
	k := r.keep(s, "")
	t.Cleanup(func() {
		k.cancel()
		k.mu.Lock()
		for cmd := range k.running {
			if cmd.Process != nil {
				syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		}
		k.mu.Unlock()
		k.wg.Wait()
	})
	until := time.Now().Add(time.Second)
	for k.count() == 0 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if k.count() == 0 {
		t.Fatal("fixture process did not start")
	}
	// A reload changed served.jail; no real cgroup is touched. SIGTERM's old
	// implementation only tries reading the empty Group's relative procs path.
	r.mu.Lock()
	s.jail = &jail{group: &cgroup.Group{}}
	r.mu.Unlock()
	k.signal(syscall.SIGTERM)
	until = time.Now().Add(200 * time.Millisecond)
	for k.count() != 0 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if k.count() != 0 {
		t.Fatal("keeper signaled the replacement jail instead of its own process")
	}

	// F6 remains unchanged: a later start reads the current launch settings,
	// and the loop records that particular jail beside that particular cmd.
	latest := &jail{files: 99}
	r.mu.Lock()
	s.jail = latest
	r.mu.Unlock()
	cmd, j, err := k.command()
	if err != nil {
		t.Fatal(err)
	}
	if j != latest || !strings.Contains(strings.Join(cmd.Args, " "), "--nofile=99:99") {
		t.Fatal("later start did not capture its current launch jail")
	}

	k.stop()
}

func TestReplacementKeyWaitsForOldKeyWithTheSameBusName(t *testing.T) {
	r, s, began, release, called := retiringFixture(t, safeRetiringName)
	r.installed = []installed{s.in}
	r.installed[0].Key = "another-key"
	done := serveReload(r)
	awaitRetirement(t, began)
	assertStillRetiring(t, done)
	select {
	case <-called:
		t.Error("new key contacted the bus before the old key retired")
	default:
	}
	release()
	awaitRetirement(t, done)
	r.mu.Lock()
	next := r.served[safeRetiringName]
	r.mu.Unlock()
	if next == nil || next.in.Key != "another-key" {
		t.Fatal("new key was not adopted after the old key retired")
	}
	r.stopKept()
}

package main

import (
	"context"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/cgroup"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/sandbox"
)

// passBox stands for a sandbox that mounts the socket: it is not "off", but
// runs the command as is, so the test needs no bubblewrap.
type passBox struct{}

func (passBox) Name() string              { return "pass" }
func (passBox) Wrap(j ports.Job) []string { return j.Argv }

// J30: a daemon restart makes a new socket at the same path. A kept sandboxed
// process still holds the old one through its mount, so the runner restarts it
// when the socket is replaced — and only then.
func TestKeptSandboxedProcessIsRestartedWhenTheBusSocketIsReplaced(t *testing.T) {
	check, first := socketCheck, backoffFirst
	socketCheck, backoffFirst = 10*time.Millisecond, 10*time.Millisecond
	defer func() { socketCheck, backoffFirst = check, first }()

	dir := t.TempDir()
	sock := filepath.Join(dir, "bus.sock")
	daemon, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	starts := filepath.Join(dir, "starts")
	script := filepath.Join(dir, "agent.sh")
	body := "echo start >> " + strconv.Quote(starts) + "\ntrap 'exit 0' TERM\nwhile :; do sleep .02; done\n"
	if err := os.WriteFile(script, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	r := &runner{ctx: context.Background(), say: io.Discard, addr: sock}
	s := &served{bus: unconfiguredClient(), name: "#fixture@h", cmd: "/bin/sh " + strconv.Quote(script), work: dir, dir: dir, box: passBox{}, jail: &jail{}, in: installed{Agent: agentFile{Deadline: "200ms"}, Span: span{Min: 1, Max: 1}}}
	k := r.keep(s, "")
	defer k.stop()

	count := func() int {
		b, _ := os.ReadFile(starts)
		return strings.Count(string(b), "start\n")
	}
	waitFor := func(n int, why string) {
		t.Helper()
		end := time.Now().Add(3 * time.Second)
		for count() < n {
			if time.Now().After(end) {
				t.Fatalf("%s: %d start(s), want %d", why, count(), n)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	waitFor(1, "the kept process did not start")

	// The same socket, looked at many times, restarts nothing.
	time.Sleep(20 * socketCheck)
	if n := count(); n != 1 {
		t.Fatalf("restarted %d time(s) while the socket was unchanged", n-1)
	}
	// The daemon stops: no socket at the path is waited out, not restarted for.
	daemon.Close()
	if _, err := os.Stat(sock); !os.IsNotExist(err) {
		t.Fatalf("the closed listener left its socket: %v", err)
	}
	time.Sleep(20 * socketCheck)
	if n := count(); n != 1 {
		t.Fatalf("restarted %d time(s) while no socket was at the path", n-1)
	}
	// The daemon comes back with a new socket at the same path.
	again, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	waitFor(2, "the kept process was not restarted onto the new socket")
	time.Sleep(20 * socketCheck)
	if n := count(); n != 2 {
		t.Fatalf("restarted %d more time(s) after it reached the new socket", n-2)
	}
}

// With the sandbox off the process opens the path afresh on each connect, so
// a replaced socket is no reason to restart it.
func TestUnsandboxedKeptProcessIsNotRestartedForAReplacedSocket(t *testing.T) {
	check := socketCheck
	socketCheck = 10 * time.Millisecond
	defer func() { socketCheck = check }()
	dir := t.TempDir()
	sock := filepath.Join(dir, "bus.sock")
	r := &runner{ctx: context.Background(), say: io.Discard, addr: sock}
	s := &served{bus: unconfiguredClient(), name: "#fixture@h", work: dir, dir: dir, jail: &jail{}}
	daemon, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer daemon.Close()
	k := &keeper{r: r, s: s}
	s.box = passBox{}
	if sock, err := k.socketToWatch(); sock == nil || err != nil {
		t.Fatalf("a sandboxed process with a socket path was not watched: %v", err)
	}
	s.box = sandbox.Off{}
	if sock, err := k.socketToWatch(); sock != nil || err != nil {
		t.Fatalf("a process with the sandbox off was watched: %v", err)
	}
	r.addr = "http://127.0.0.1:6767"
	s.box = passBox{}
	if sock, err := k.socketToWatch(); sock != nil || err != nil {
		t.Fatalf("a process talking over TCP was watched: %v", err)
	}
}

// The sandbox gets the bus socket alone. Its directory also holds the account
// sockets, which identify a caller without a token: with the directory mounted,
// an agent running as the runner's account could open the runner's and act as
// the runner. So J30 restarts the process instead of mounting the directory.
func TestSandboxSeesTheBusSocketButNotItsNeighbouringAccountSocket(t *testing.T) {
	if err := sandbox.ProbeBwrap(); err != nil {
		t.Skipf("the mount proof needs bubblewrap: %v", err)
	}
	dir := t.TempDir()
	run := filepath.Join(dir, "run")
	if err := os.Mkdir(run, 0711); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bus.sock", "user-runner.sock"} {
		l, err := net.Listen("unix", filepath.Join(run, name))
		if err != nil {
			t.Fatal(err)
		}
		defer l.Close()
	}
	work := filepath.Join(dir, "work")
	if err := os.Mkdir(work, 0700); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(dir, "agent")
	if err := os.Mkdir(pkg, 0700); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(pkg, "agent.sh")
	probe := "for f in bus.sock user-runner.sock; do if [ -S " + strconv.Quote(run) + "/$f ]; then echo \"$f seen\"; else echo \"$f hidden\"; fi; done > " + strconv.Quote(filepath.Join(work, "seen")) + "\n"
	if err := os.WriteFile(script, []byte(probe), 0600); err != nil {
		t.Fatal(err)
	}
	r := &runner{ctx: context.Background(), say: io.Discard, addr: filepath.Join(run, "bus.sock")}
	s := &served{bus: unconfiguredClient(), name: "#fixture@h", cmd: "/bin/sh " + strconv.Quote(script), work: work, dir: pkg, box: sandbox.Bwrap{}, jail: &jail{}, in: installed{Span: span{Min: 1, Max: 1}}}
	k := &keeper{r: r, s: s, ctx: context.Background()}
	cmd, _, err := k.command()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Run(); err != nil {
		t.Fatalf("the sandboxed probe failed: %v", err)
	}
	seen, err := os.ReadFile(filepath.Join(work, "seen"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(seen), "bus.sock seen\nuser-runner.sock hidden\n"; got != want {
		t.Fatalf("inside the sandbox: %q, want %q", got, want)
	}
}

// J30, review: a socket that appears between the look and the start must not
// leave the process unwatched for good. #ab-w1c's probe, synchronised on the
// loop's state rather than a line number: hold the keeper's lock so a loop that
// looked at an absent socket waits at it; create the socket, release, replace.
func TestASocketAppearingBeforeTheStartIsStillWatched(t *testing.T) {
	check, first := socketCheck, backoffFirst
	socketCheck, backoffFirst = 10*time.Millisecond, 10*time.Millisecond
	defer func() { socketCheck, backoffFirst = check, first }()
	dir := t.TempDir()
	sock := filepath.Join(dir, "bus.sock")
	starts := filepath.Join(dir, "starts")
	script := filepath.Join(dir, "agent.sh")
	body := "echo start >> " + strconv.Quote(starts) + "\ntrap 'exit 0' TERM\nwhile :; do sleep .02; done\n"
	if err := os.WriteFile(script, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	logs := restartLog{lines: make(chan string, 64)}
	r := &runner{ctx: context.Background(), say: logs, addr: sock}
	s := &served{bus: unconfiguredClient(), name: "#fixture@h", cmd: "/bin/sh " + strconv.Quote(script), work: dir, dir: dir, box: passBox{}, jail: &jail{}, in: installed{Agent: agentFile{Deadline: "200ms"}, Span: span{Min: 1, Max: 1}}}
	ctx, cancel := context.WithCancel(r.ctx)
	k := &keeper{r: r, s: s, ctx: ctx, cancel: cancel, running: map[*exec.Cmd]keptProcess{}, groups: map[*cgroup.Group]bool{}}
	k.mu.Lock()
	locked := true
	defer func() {
		if locked {
			k.mu.Unlock()
		}
		k.stop()
	}()
	k.wg.Add(1)
	go k.loop()
	waitingForLock := func() bool {
		stack := make([]byte, 1<<20)
		n := runtime.Stack(stack, true)
		for _, g := range strings.Split(string(stack[:n]), "\n\n") {
			if strings.Contains(g, "(*keeper).loop") && strings.Contains(g, "sync.(*Mutex).Lock") {
				return true
			}
		}
		return false
	}
	// The loop has looked at the absent socket: it either waits at the lock
	// with nothing to watch, or it refused the start and will try again.
	end := time.Now().Add(2 * time.Second)
	for !waitingForLock() {
		select {
		case line := <-logs.lines:
			if strings.Contains(line, "no bus socket") {
				goto looked
			}
		default:
		}
		if time.Now().After(end) {
			t.Fatal("the loop never looked at the socket")
		}
		time.Sleep(time.Millisecond)
	}
looked:
	daemon, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer daemon.Close()
	for end = time.Now().Add(2 * time.Second); !waitingForLock(); time.Sleep(time.Millisecond) {
		if time.Now().After(end) {
			t.Fatal("the loop never came back to start the process")
		}
	}
	k.mu.Unlock()
	locked = false
	count := func() int {
		b, _ := os.ReadFile(starts)
		return strings.Count(string(b), "start\n")
	}
	for end = time.Now().Add(2 * time.Second); count() < 1; time.Sleep(time.Millisecond) {
		if time.Now().After(end) {
			t.Fatal("the process never started")
		}
	}
	daemon.Close()
	again, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	for end = time.Now().Add(2 * time.Second); count() < 2 && time.Now().Before(end); time.Sleep(time.Millisecond) {
	}
	if n := count(); n != 2 {
		t.Fatalf("a socket that appeared before the start went unwatched: %d start(s), want 2", n)
	}
}

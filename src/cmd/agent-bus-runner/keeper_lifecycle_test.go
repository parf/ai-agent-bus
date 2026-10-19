package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/sandbox"
)

func TestKeptProcessIgnoringTermIsKilledAtItsDeadline(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	script := filepath.Join(dir, "agent.sh")
	if err := os.WriteFile(script, []byte("trap '' TERM\nprintf ready > "+strconv.Quote(ready)+"\nwhile :; do sleep .02; done\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r := &runner{ctx: context.Background(), say: io.Discard}
	s := &served{bus: unconfiguredClient(), name: "#fixture@h", cmd: "/bin/sh " + strconv.Quote(script), work: dir, dir: dir, box: sandbox.Off{}, jail: &jail{}, in: installed{Agent: agentFile{Deadline: "80ms"}, Span: span{Min: 1, Max: 1}}}
	k := r.keep(s, "")
	// Always release a mutant's TERM-ignoring process before the test exits.
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
	end := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(end) {
			t.Fatal("fixture did not install its TERM handler")
		}
		time.Sleep(2 * time.Millisecond)
	}
	done := make(chan struct{})
	began := time.Now()
	go func() { k.stop(); close(done) }()
	select {
	case <-done:
		if elapsed := time.Since(began); elapsed < 60*time.Millisecond {
			t.Fatalf("TERM-ignoring fixture stopped before its kill deadline: %s", elapsed)
		}
	case <-time.After(600 * time.Millisecond):
		t.Fatal("TERM-ignoring kept process was not killed at its deadline")
	}
	if n := k.count(); n != 0 {
		t.Fatalf("%d processes remained after stop", n)
	}
}

type restartLog struct{ lines chan string }

func (l restartLog) Write(p []byte) (int, error) {
	s := string(p)
	if strings.Contains(s, "restarting in ") {
		l.lines <- s
	}
	return len(p), nil
}

func TestKeptCrashLoopDoublesBackoffUpToItsCap(t *testing.T) {
	first, most := backoffFirst, backoffMost
	backoffFirst = 5 * time.Millisecond
	backoffMost = 20 * time.Millisecond
	defer func() { backoffFirst, backoffMost = first, most }()
	logs := restartLog{lines: make(chan string, 16)}
	r := &runner{ctx: context.Background(), say: logs}
	d := t.TempDir()
	s := &served{bus: unconfiguredClient(), name: "#fixture@h", cmd: "/bin/false", work: d, dir: d, box: sandbox.Off{}, jail: &jail{}, in: installed{Span: span{Min: 1, Max: 1}}}
	k := r.keep(s, "")
	defer k.stop()
	for _, want := range []string{"5ms", "10ms", "20ms", "20ms"} {
		select {
		case line := <-logs.lines:
			if !strings.Contains(line, "restarting in "+want+"\n") {
				t.Fatalf("want backoff %s, got %q", want, line)
			}
		case <-time.After(time.Second):
			t.Fatalf("no restart announced for expected backoff %s", want)
		}
	}
}

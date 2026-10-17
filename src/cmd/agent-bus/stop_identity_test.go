package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestStopLockOwnerFixture(t *testing.T) {
	if os.Getenv("AB_STOP_LOCK_FIXTURE") != "1" {
		return
	}
	name := os.Getenv("AB_STOP_LOCK_NAME")
	lock, err := hold(name)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := (running{Name: name, PID: os.Getpid()}).note(); err != nil {
		t.Fatal(err)
	}
	fmt.Println("ready")
	var b [1]byte
	os.Stdin.Read(b[:])
}

func startStopLockFixture(t *testing.T, name string) (*exec.Cmd, <-chan error) {
	t.Helper()
	holder := exec.Command(os.Args[0], "-test.run=^TestStopLockOwnerFixture$")
	holder.Env = append(os.Environ(), "AB_STOP_LOCK_FIXTURE=1", "AB_STOP_LOCK_NAME="+name)
	input, err := holder.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := holder.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	holderDone := make(chan error, 1)
	go func() { holderDone <- holder.Wait(); close(holderDone) }()
	t.Cleanup(func() {
		input.Close()
		holder.Process.Kill()
		select {
		case <-holderDone:
		case <-time.After(time.Second):
			t.Error("lock fixture did not exit")
		}
	})
	ready := make(chan string, 1)
	go func() { line, _ := bufio.NewReader(output).ReadString('\n'); ready <- line }()
	select {
	case line := <-ready:
		if line != "ready\n" {
			t.Fatalf("fixture: %q", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("lock owner not ready")
	}
	return holder, holderDone
}

func TestStopOnlySignalsTheNamedLockOwner(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	holder, holderDone := startStopLockFixture(t, "#locked@h")
	unrelated := exec.Command("sleep", "10")
	if err := unrelated.Start(); err != nil {
		t.Fatal(err)
	}
	unrelatedDone := make(chan error, 1)
	go func() { unrelatedDone <- unrelated.Wait() }()
	defer func() {
		unrelated.Process.Kill()
		select {
		case <-unrelatedDone:
		case <-time.After(time.Second):
		}
	}()
	for _, name := range []string{"#stale@h", "#locked@h"} {
		if err := (running{Name: name, PID: unrelated.Process.Pid}).note(); err != nil {
			t.Fatal(err)
		}
		if err := stopVerb([]string{name}); err == nil {
			t.Fatalf("stale note for %s signalled unrelated pid %d", name, unrelated.Process.Pid)
		}
		if err := unrelated.Process.Signal(syscall.Signal(0)); err != nil {
			t.Fatalf("unrelated process did not survive: %v", err)
		}
	}
	// A held name must not be enough: the note must name its actual holder.
	if err := (running{Name: "#locked@h", PID: holder.Process.Pid}).note(); err != nil {
		t.Fatal(err)
	}
	if err := stopVerb([]string{"#locked@h"}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-holderDone:
		if err == nil {
			t.Fatal("lock owner was not signalled")
		}
	case <-time.After(time.Second):
		t.Fatal("lock owner survived stop")
	}
}

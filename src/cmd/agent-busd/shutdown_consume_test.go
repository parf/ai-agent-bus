package main

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Hold the serving announcement until the parent has exercised SIGTERM.
// This makes a late signal.Notify registration an observable startup failure.
type servingBarrier struct{}

func (servingBarrier) Write(p []byte) (int, error) {
	n, err := os.Stdout.Write(p)
	if strings.Contains(string(p), "bus serving ") {
		var release [1]byte
		_, _ = os.Stdin.Read(release[:])
	}
	return n, err
}

func TestShutdownConsumeFixture(t *testing.T) {
	dir := os.Getenv("AB_SHUTDOWN_FIXTURE")
	if dir == "" {
		return
	}
	log.SetOutput(servingBarrier{})
	runBus(config{owner: "owner@h", db: filepath.Join(dir, "bus.db"), create: true, logDir: filepath.Join(dir, "logs")})
}

func TestShutdownCancelsPendingConsumesBeforeHTTPDrain(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + listener.Addr().String()
	file, err := listenerFile(listener)
	if err != nil {
		listener.Close()
		t.Fatal(err)
	}
	listener.Close()
	child := exec.Command(os.Args[0], "-test.run=^TestShutdownConsumeFixture$")
	child.Env = append(os.Environ(), "AB_SHUTDOWN_FIXTURE="+t.TempDir(), fdsEnv+"=owner:", accountsEnv+"={}")
	child.ExtraFiles = []*os.File{file}
	output, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	release, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer release.Close()
	child.Stderr = io.Discard
	if err := child.Start(); err != nil {
		file.Close()
		t.Fatal(err)
	}
	file.Close()
	ready := make(chan bool, 1)
	go func() {
		scanner := bufio.NewScanner(output)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "bus serving ") {
				ready <- true
				return
			}
		}
		ready <- false
	}()
	done := make(chan error, 1)
	go func() { done <- child.Wait(); close(done) }()
	defer func() {
		child.Process.Kill()
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	}()
	select {
	case ok := <-ready:
		if !ok {
			t.Fatal("child exited before serving announcement")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("child did not announce serving")
	}
	client := &http.Client{Timeout: time.Second}
	defer client.CloseIdleConnections()
	get := func(path string) ([]byte, error) {
		r, err := client.Get(base + path)
		if err != nil {
			return nil, err
		}
		defer r.Body.Close()
		return io.ReadAll(r.Body)
	}
	until := time.Now().Add(2 * time.Second)
	for {
		if _, err := get("/status"); err == nil {
			break
		}
		if time.Now().After(until) {
			t.Fatal("fixture did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	consume := make(chan int, 1)
	go func() {
		r, err := http.Get(base + "/consume?wait=30s")
		if err != nil {
			consume <- 0
			return
		}
		defer r.Body.Close()
		io.Copy(io.Discard, r.Body)
		consume <- r.StatusCode
	}()
	for {
		body, err := get("/lookup?name=owner%40h")
		var record struct {
			Readers *int `json:"readers"`
		}
		if err == nil && json.Unmarshal(body, &record) == nil && record.Readers != nil && *record.Readers == 1 {
			break
		}
		if time.Now().After(until) {
			t.Fatalf("consume not pending: %s (%v)", strings.TrimSpace(string(body)), err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	at := time.Now()
	if err := child.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	// The child remains at the serving barrier while the signal is delivered.
	// With the old ordering, SIGTERM terminates it before Notify can run.
	select {
	case err := <-done:
		t.Fatalf("SIGTERM at serving readiness killed child: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if _, err := release.Write([]byte{'\n'}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown waited on an idle long-poll instead of cancelling it")
	}
	t.Logf("shutdown with pending consume: %s", time.Since(at))
	select {
	case status := <-consume:
		if status != http.StatusNoContent {
			t.Fatalf("cancelled consume status=%d", status)
		}
	case <-time.After(time.Second):
		t.Fatal("consume was not answered")
	}
}

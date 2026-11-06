package main

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// Exercise the effective span independently of the package digest: a count
// change must replace adoption's slots and keeper even with unchanged code.
// Kept children announce readiness then block opening a FIFO, with no load
// loop or sleep used to infer the number running.
func TestProcessSpanChangeReadoptsSlotsAndKeepers(t *testing.T) {
	for _, serves := range []string{"", "itself"} {
		t.Run(fmt.Sprintf("serves=%s", serves), func(t *testing.T) {
			dir := t.TempDir()
			ready, hold := filepath.Join(dir, "ready"), filepath.Join(dir, "hold")
			for _, path := range []string{ready, hold} {
				if err := syscall.Mkfifo(path, 0600); err != nil {
					t.Fatal(err)
				}
			}
			reader, err := os.OpenFile(ready, os.O_RDWR, 0600)
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			signals := make(chan string, 8)
			go func() {
				scan := bufio.NewScanner(reader)
				for scan.Scan() {
					signals <- scan.Text()
				}
				close(signals)
			}()
			started := func(n int) []int {
				t.Helper()
				timer := time.NewTimer(3 * time.Second)
				defer timer.Stop()
				var pids []int
				for range n {
					select {
					case raw, ok := <-signals:
						if !ok || raw == "" {
							t.Fatal("readiness pipe closed before process started")
						}
						pid, err := strconv.Atoi(raw)
						if err != nil || pid <= 1 {
							t.Fatalf("invalid announced pid %q", raw)
						}
						pids = append(pids, pid)
					case <-timer.C:
						t.Fatal("kept processes did not announce the configured count")
					}
				}
				return pids
			}
			body := "printf '%s\\n' \"$$\" > " + strconv.Quote(ready) + "\nread -r stop < " + strconv.Quote(hold) + "\n"
			r, _, _ := configurationFixture(t, serves, body)
			// Retain keeper handles too: a deliberately broken retirement must not
			// leave its fixture children behind after the PID-death assertion fails.
			var keepers []*keeper
			t.Cleanup(func() {
				for _, k := range keepers {
					k.stop()
				}
			})
			current := func() *served {
				r.mu.Lock()
				defer r.mu.Unlock()
				return r.served["#fixture@h"]
			}
			initial := current()
			if initial == nil {
				t.Fatal("agent not served")
			}
			var previousPIDs []int
			if serves == "itself" {
				keepers = append(keepers, initial.kept)
				previousPIDs = started(1)
			}
			previous := initial
			for _, n := range []int{3, 1} {
				r.reloadMu.Lock()
				r.mu.Lock()
				r.installed[0].Span = span{Min: n, Max: n}
				r.serve(r.ctx)
				r.mu.Unlock()
				r.reloadMu.Unlock()
				next := current()
				if next == nil || next == previous {
					t.Fatalf("process span %d reused the old adoption", n)
				}
				if cap(next.slots) != n {
					t.Fatalf("slots capacity %d, want %d", cap(next.slots), n)
				}
				if serves == "itself" {
					keepers = append(keepers, next.kept)
					assertSpanPIDsGone(t, previousPIDs)
					previousPIDs = started(n)
					if live, _ := next.kept.snapshot(); live != n {
						t.Fatalf("running %d kept processes, want %d", live, n)
					}
					if previous.kept != nil {
						t.Fatal("old keeper retained after re-adoption")
					}
				}
				// An unchanged effective span does not restart the freshly adopted pool.
				r.reloadMu.Lock()
				r.mu.Lock()
				r.serve(r.ctx)
				r.mu.Unlock()
				r.reloadMu.Unlock()
				if current() != next {
					t.Fatal("unchanged process span re-adopted the agent")
				}
				previous = next
			}
		})
	}
}

// A bounded readiness check, not a timing assumption: keep every old PID
// before stopping and require the kernel to say it no longer exists.
func assertSpanPIDsGone(t *testing.T, pids []int) {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		var alive []int
		for _, pid := range pids {
			if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
				alive = append(alive, pid)
			}
		}
		if len(alive) == 0 {
			return
		}
		select {
		case <-timer.C:
			t.Fatalf("old kept processes remain after re-adoption: %v", alive)
		case <-tick.C:
		}
	}
}

func TestProcessSpanChangeLetsAnInFlightOneShotAnswer(t *testing.T) {
	dir := t.TempDir()
	hold, ready := filepath.Join(dir, "hold"), filepath.Join(dir, "ready")
	if err := syscall.Mkfifo(hold, 0600); err != nil {
		t.Fatal(err)
	}
	body := "printf ready > " + strconv.Quote(ready) + "\nread -r release < " + strconv.Quote(hold) + "\nprintf completed\n"
	r, b, _ := configurationFixture(t, "", body)
	r.mu.Lock()
	old := r.served["#fixture@h"]
	old.in.Agent.Deadline = "3s"
	r.installed[0].Agent.Deadline = "3s"
	r.mu.Unlock()
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: "held"}); err != nil {
		t.Fatal(err)
	}
	await(t, ready, "ready", time.Second)
	reloading := make(chan struct{})
	reloaded := make(chan *served, 1)
	go func() {
		r.reloadMu.Lock()
		r.mu.Lock()
		r.installed[0].Span = span{Min: 2, Max: 2}
		close(reloading)
		r.serve(r.ctx)
		next := r.served["#fixture@h"]
		r.mu.Unlock()
		r.reloadMu.Unlock()
		reloaded <- next
		close(reloaded)
	}()
	t.Cleanup(func() {
		select {
		case <-reloaded:
		case <-time.After(5 * time.Second):
			t.Error("reload did not finish")
		}
	})
	<-reloading
	// The FIFO open cannot hang if a broken stop killed its reader.
	release, err := os.OpenFile(hold, os.O_WRONLY|syscall.O_NONBLOCK, 0600)
	if err != nil {
		t.Fatalf("old one-shot no longer waiting for release: %v", err)
	}
	_, err = release.WriteString("go\n")
	release.Close()
	if err != nil {
		t.Fatal(err)
	}
	select {
	case next := <-reloaded:
		if next == nil || next == old {
			t.Fatal("in-flight span change did not re-adopt")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("span change did not complete after the old run answered")
	}
	if answer := configurationReply(t, b).Body; answer != "completed" {
		t.Fatalf("in-flight answer %q", answer)
	}
}

package main

import (
	"bufio"
	"fmt"
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
			started := func(n int) {
				t.Helper()
				timer := time.NewTimer(3 * time.Second)
				defer timer.Stop()
				for range n {
					select {
					case pid, ok := <-signals:
						if !ok || pid == "" {
							t.Fatal("readiness pipe closed before process started")
						}
					case <-timer.C:
						t.Fatal("kept processes did not announce the configured count")
					}
				}
			}
			body := "printf '%s\\n' \"$$\" > " + strconv.Quote(ready) + "\nread -r stop < " + strconv.Quote(hold) + "\n"
			r, _, _ := configurationFixture(t, serves, body)
			current := func() *served {
				r.mu.Lock()
				defer r.mu.Unlock()
				return r.served["#fixture@h"]
			}
			initial := current()
			if initial == nil {
				t.Fatal("agent not served")
			}
			if serves == "itself" {
				started(1)
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
					started(n)
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

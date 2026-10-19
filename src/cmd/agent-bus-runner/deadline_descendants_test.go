package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/sandbox"
)

type deadlineReplies struct {
	mu      sync.Mutex
	replies []protocol.Envelope
}

func (d *deadlineReplies) RoundTrip(q *http.Request) (*http.Response, error) {
	if q.URL.Path == "/config" {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("null")), Header: make(http.Header)}, nil
	}
	var e protocol.Envelope
	json.NewDecoder(q.Body).Decode(&e)
	d.mu.Lock()
	d.replies = append(d.replies, e)
	d.mu.Unlock()
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
}

func deadlineFixture(t *testing.T, body, deadline string) (*runner, *served, *deadlineReplies, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.sh")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	replies := &deadlineReplies{}
	r := &runner{say: io.Discard}
	s := &served{name: "#fixture@h", work: dir, dir: dir, cmd: "/bin/sh " + strconv.Quote(path), box: sandbox.Off{}, jail: &jail{}, bus: &client{http: &http.Client{Transport: replies}, base: "http://fixture"}, in: installed{Agent: agentFile{Deadline: deadline}}}
	return r, s, replies, dir
}

func TestScriptDeadlineKillsDescendantsHoldingOutput(t *testing.T) {
	for _, c := range []struct {
		name, start, tail string
		escaped           bool
	}{
		{"wait", "sleep 2", "wait", false},
		{"leader exits", "sleep 2", "exit 0", false},
		{"escaped pipe", "setsid sleep 2", "exit 0", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			if c.escaped {
				if _, err := exec.LookPath("setsid"); err != nil {
					t.Skip("setsid is not installed")
				}
			}
			r, s, _, dir := deadlineFixture(t, c.start+" &\nprintf '%s' \"$!\" > child.pid\n"+c.tail+"\n", "150ms")
			done := make(chan struct{})
			go func() { r.runOne(s, protocol.Envelope{ID: "fixture", From: "caller@h"}); close(done) }()
			// Save the child while it is alive, rather than asking an exited parent.
			var pid int
			end := time.Now().Add(600 * time.Millisecond)
			for pid == 0 && time.Now().Before(end) {
				if b, err := os.ReadFile(filepath.Join(dir, "child.pid")); err == nil {
					pid, _ = strconv.Atoi(string(b))
				}
				time.Sleep(2 * time.Millisecond)
			}
			t.Cleanup(func() {
				if pid > 0 {
					syscall.Kill(pid, syscall.SIGKILL)
				}
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Error("fixture run did not drain after cleanup")
				}
			})
			if pid == 0 {
				t.Fatal("fixture did not record its live child")
			}
			select {
			case <-done:
			case <-time.After(500 * time.Millisecond):
				t.Fatal("script deadline did not bound descendant output wait")
			}
			if c.escaped {
				return
			} // Off cannot contain a child that deliberately leaves its process group.
			// The signal was sent, but scheduling its death (or reaping the
			// orphan) need not finish before the output pipe closes.
			poll := time.NewTicker(2 * time.Millisecond)
			defer poll.Stop()
			limit := time.NewTimer(200 * time.Millisecond)
			defer limit.Stop()
			for {
				b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
				if os.IsNotExist(err) || errors.Is(err, syscall.ESRCH) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				_, state, _ := strings.Cut(string(b), ") ")
				if strings.HasPrefix(state, "Z ") {
					break
				}
				select {
				case <-poll.C:
				case <-limit.C:
					t.Fatalf("descendant %d is still alive after the deadline", pid)
				}
			}
		})
	}
}

func TestScriptKeepsBackgroundOutputUntilItsDeadline(t *testing.T) {
	for _, deadline := range []string{"", "2s"} {
		t.Run(deadline, func(t *testing.T) {
			r, s, replies, _ := deadlineFixture(t, "(sleep .12; printf complete) &\nexit 0\n", deadline)
			r.runOne(s, protocol.Envelope{ID: "fixture", From: "caller@h"})
			replies.mu.Lock()
			defer replies.mu.Unlock()
			var answer string
			for _, e := range replies.replies {
				answer += e.Body
			}
			if answer != "complete" {
				t.Fatalf("valid background output was lost: %q", answer)
			}
		})
	}
}

func TestScriptFailedStartDoesNotRegisterCancellation(t *testing.T) {
	r, s, _, _ := deadlineFixture(t, "printf unused\n", "1ms")
	s.box = unavailableScript{path: filepath.Join(t.TempDir(), "missing")}
	done := make(chan struct{})
	go func() { r.runOne(s, protocol.Envelope{ID: "fixture", From: "caller@h"}); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("failed script start did not return")
	}
}

type unavailableScript struct{ path string }

func (unavailableScript) Name() string              { return "off" }
func (b unavailableScript) Wrap(ports.Job) []string { return []string{b.path} }

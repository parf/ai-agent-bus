package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/sandbox"
)

// lockedBuffer is a log several goroutines write.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// hangUp drops the connection without an answer, as a daemon that died does.
func hangUp(t *testing.T, w http.ResponseWriter) {
	c, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		t.Error(err)
		return
	}
	c.Close()
}

func fastAway(t *testing.T) {
	first, most := awayFirst, awayMost
	awayFirst, awayMost = 10*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() { awayFirst, awayMost = first, most })
}

// fixtureRunner serves svc against handler and returns what serve returned,
// failing when it does not return within bound.
func fixtureRunner(t *testing.T, handler http.HandlerFunc, bound time.Duration) (error, string) {
	t.Helper()
	t.Setenv("AGENT_BUS_TOKEN", "fixture-token")
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	saved := transport
	transport = func() (*http.Client, string) { return server.Client(), server.URL }
	t.Cleanup(func() { transport = saved })
	box, err := sandbox.Pick("off")
	if err != nil {
		t.Fatal(err)
	}
	log := &lockedBuffer{}
	svc := service{Name: "#svc@h", Script: "echo pong", Algo: algoJSON, Instances: 1, box: box, work: t.TempDir(), say: log}
	done := make(chan error, 1)
	go func() { done <- serve(svc) }()
	select {
	case err := <-done:
		return err, log.String()
	case <-time.After(bound):
		t.Fatalf("serve did not return within %s; log:\n%s", bound, log.String())
		return nil, ""
	}
}

func fixtureMessage(w http.ResponseWriter) {
	json.NewEncoder(w).Encode(protocol.Envelope{ID: "m1", From: "caller@h", To: "#svc@h", Body: "ping"})
}

// answers collects the bodies the runner sent, receipts left out.
type answers struct {
	mu   sync.Mutex
	got  []string
	acks int
}

func (a *answers) take(t *testing.T, r *http.Request) {
	var e protocol.Envelope
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		t.Error(err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if e.Receipt == protocol.ReceiptAck {
		a.acks++
	} else if e.Receipt == "" {
		a.got = append(a.got, e.Body)
	}
}

// A daemon that goes away is waited for: the runner reads again when it is
// back and answers the message it then gets.
func TestRunnerReconnectsWhenTheDaemonComesBack(t *testing.T) {
	fastAway(t)
	var mu sync.Mutex
	reads := 0
	var sent answers
	err, log := fixtureRunner(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/consume":
			mu.Lock()
			reads++
			n := reads
			mu.Unlock()
			switch {
			case n <= 3:
				hangUp(t, w)
			case n == 4:
				fixtureMessage(w)
			default:
				// Once the answer is in, the fixture ends the run with a refusal.
				for i := 0; i < 200; i++ {
					sent.mu.Lock()
					k := len(sent.got)
					sent.mu.Unlock()
					if k > 0 {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				http.Error(w, "fixture done", http.StatusUnauthorized)
			}
		case "/send":
			sent.take(t, r)
		}
	}, 10*time.Second)
	if err == nil || !strings.Contains(err.Error(), "fixture done") {
		t.Fatalf("serve ended with %v; log:\n%s", err, log)
	}
	if len(sent.got) != 1 || sent.got[0] != "pong" {
		t.Fatalf("answers after the daemon came back: %q; log:\n%s", sent.got, log)
	}
	if strings.Count(log, "daemon away") != 1 || !strings.Contains(log, "reconnected after") {
		t.Fatalf("the absence was not said once and its end reported:\n%s", log)
	}
}

// A refusal is the daemon saying no, not being away: it ends the runner at
// once rather than being retried.
func TestRunnerStillEndsOnARefusal(t *testing.T) {
	fastAway(t)
	var mu sync.Mutex
	reads := 0
	err, log := fixtureRunner(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		reads++
		mu.Unlock()
		http.Error(w, "token refused", http.StatusUnauthorized)
	}, 5*time.Second)
	if err == nil || !strings.Contains(err.Error(), "token refused") || reads != 1 {
		t.Fatalf("a refusal gave %v after %d reads; log:\n%s", err, reads, log)
	}
}

// Straight after the link dropped, a second-reader refusal is the runner's own
// last long poll still waiting on the daemon; it is waited out, not fatal.
func TestRunnerWaitsOutItsOwnGhostRead(t *testing.T) {
	fastAway(t)
	var mu sync.Mutex
	reads := 0
	var sent answers
	err, log := fixtureRunner(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/consume":
			mu.Lock()
			reads++
			n := reads
			mu.Unlock()
			switch n {
			case 1:
				hangUp(t, w)
			case 2, 3:
				http.Error(w, "inbox already has a reader", http.StatusConflict)
			case 4:
				fixtureMessage(w)
			default:
				time.Sleep(200 * time.Millisecond)
				http.Error(w, "fixture done", http.StatusUnauthorized)
			}
		case "/send":
			sent.take(t, r)
		}
	}, 10*time.Second)
	if err == nil || !strings.Contains(err.Error(), "fixture done") || len(sent.got) != 1 {
		t.Fatalf("serve ended with %v, answers %q; log:\n%s", err, sent.got, log)
	}
}

// With no absence before it, a second reader is a real one, and still refused.
func TestRunnerRefusesARealSecondReader(t *testing.T) {
	fastAway(t)
	err, log := fixtureRunner(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "inbox already has a reader", http.StatusConflict)
	}, 5*time.Second)
	if err == nil || !strings.Contains(err.Error(), "already has a reader") {
		t.Fatalf("a second reader gave %v; log:\n%s", err, log)
	}
}

// An answer worked out while the daemon is away is delivered once it is back,
// exactly once.
func TestAnAnswerIsRetriedWhileTheDaemonIsAway(t *testing.T) {
	fastAway(t)
	var mu sync.Mutex
	reads, posts := 0, 0
	var sent answers
	err, log := fixtureRunner(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/consume":
			mu.Lock()
			reads++
			n := reads
			mu.Unlock()
			if n == 1 {
				fixtureMessage(w)
				return
			}
			time.Sleep(500 * time.Millisecond)
			http.Error(w, "fixture done", http.StatusUnauthorized)
		case "/send":
			mu.Lock()
			posts++
			n := posts
			mu.Unlock()
			// The ack (one try) and the next two answer tries find no daemon.
			if n <= 3 {
				hangUp(t, w)
				return
			}
			sent.take(t, r)
		}
	}, 10*time.Second)
	if err == nil || !strings.Contains(err.Error(), "fixture done") {
		t.Fatalf("serve ended with %v; log:\n%s", err, log)
	}
	if len(sent.got) != 1 || sent.got[0] != "pong" || posts != 4 {
		t.Fatalf("answers %q after %d posts; log:\n%s", sent.got, posts, log)
	}
}

// Ctrl-c while waiting for the daemon stops the runner at once, however long
// the wait it was in.
func TestStopDuringAnAbsenceIsPrompt(t *testing.T) {
	first := awayFirst
	awayFirst = time.Minute
	t.Cleanup(func() { awayFirst = first })
	var once sync.Once
	start := time.Now()
	err, log := fixtureRunner(t, func(w http.ResponseWriter, r *http.Request) {
		hangUp(t, w)
		// serve has its signal context by now, so the signal is caught.
		once.Do(func() {
			go func() {
				time.Sleep(100 * time.Millisecond)
				syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
			}()
		})
	}, 5*time.Second)
	if err != nil || time.Since(start) > 3*time.Second || !strings.Contains(log, "stopping") {
		t.Fatalf("stop during an absence: %v after %s; log:\n%s", err, time.Since(start), log)
	}
}

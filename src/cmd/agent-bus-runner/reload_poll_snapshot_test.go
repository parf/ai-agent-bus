package main

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Only consume is synthetic; reload's token/owner/config checks still use the
// real disposable API. Replace the client only after its old reader joined.
func pollingReloadFixture(t *testing.T, consume func(*http.Request) (*http.Response, error)) (*runner, *served) {
	t.Helper()
	r, _, _ := configurationFixture(t, "", "printf answer")
	r.mu.Lock()
	r.stop("#fixture@h", r.served["#fixture@h"], "a test stops it")
	r.mu.Unlock()
	c := *r.bus.http
	base := c.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	c.Transport = secretTransport(func(q *http.Request) (*http.Response, error) {
		if q.URL.Path == "/consume" {
			return consume(q)
		}
		return base.RoundTrip(q)
	})
	bus := *r.bus
	bus.http = &c
	r.bus = &bus
	r.reload()
	r.mu.Lock()
	s := r.served["#fixture@h"]
	r.mu.Unlock()
	if s == nil {
		t.Fatal("fixture was not adopted")
	}
	t.Cleanup(func() {
		s.cancel()
		select {
		case <-s.readDone:
		case <-time.After(time.Second):
			t.Error("reader did not stop")
		}
	})
	return r, s
}

func emptyPoll() *http.Response {
	return &http.Response{StatusCode: http.StatusNoContent, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}
}

func TestReloadWhileInboxPollsSnapshotsSharedSettings(t *testing.T) {
	var polls atomic.Int64
	var shared atomic.Bool
	entered := make(chan struct{})
	var once sync.Once
	r, s := pollingReloadFixture(t, func(q *http.Request) (*http.Response, error) {
		polls.Add(1)
		if q.URL.Query().Get("share") == "1" {
			shared.Store(true)
		}
		once.Do(func() { close(entered) })
		return emptyPoll(), nil
	})
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("poll reader did not start")
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 128; i++ {
			// The same served entry is updated, not replaced by a different fixture.
			writeFile(t, filepath.Join(r.host, "agents.json"), fmt.Sprintf(`{"fixture":{"sandbox":"off","share":%t}}`, i%2 == 1))
			r.reload()
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		s.cancel()
		t.Fatal("reload blocked while inbox polls were active")
	}
	until := time.Now().Add(time.Second)
	for !shared.Load() && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if polls.Load() < 2 || !shared.Load() {
		t.Fatalf("updated shared poll was not observed: polls=%d shared=%t", polls.Load(), shared.Load())
	}
	r.mu.Lock()
	same := r.served[s.name] == s
	r.mu.Unlock()
	if !same {
		t.Fatal("reload replaced the reader instead of exercising settings replacement")
	}
}

func TestReloadDoesNotHoldSettingsLockAcrossBlockedConsume(t *testing.T) {
	entered := make(chan struct{})
	var once sync.Once
	r, s := pollingReloadFixture(t, func(q *http.Request) (*http.Response, error) {
		once.Do(func() { close(entered) })
		<-q.Context().Done()
		return nil, q.Context().Err()
	})
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("blocking consume did not start")
	}
	done := make(chan struct{})
	go func() { r.reload(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		s.cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("reload did not release after consume cancellation")
		}
		t.Fatal("reload waited for the blocking consume's settings lock")
	}
}

// The production writer publishes under r.mu then settingsMu. Keep the
// replacement active until all asynchronous readers exercised below finish.
func replaceDeadlinesDuring(t *testing.T, r *runner, s *served) func() {
	t.Helper()
	finished := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		i := 0
		for {
			select {
			case <-finished:
				return
			default:
			}
			r.mu.Lock()
			s.settingsMu.Lock()
			s.in.Agent.Deadline = []string{"20ms", "21ms"}[i%2]
			s.jail = &jail{}
			s.settingsMu.Unlock()
			r.mu.Unlock()
			i++
		}
	}()
	var once sync.Once
	stop := func() { once.Do(func() { close(finished) }); <-done }
	t.Cleanup(stop)
	return stop
}

func TestJsonlIdleRetirementSnapshotsDeadlineDuringSettingsReplacement(t *testing.T) {
	testAnswers(t)
	st, _ := poolFixture(t, "trap '' TERM\nwhile read -r a; do printf '%s ok\\n' \"$(idof \"$a\")\"; done\n", 1, span{Min: 0, Max: 1}, "10ms")
	stop := replaceDeadlinesDuring(t, st.r, st.s)
	defer stop()
	for i := 0; i < 8; i++ {
		doAll(t, st, 1, time.Second)
		awaitLive(t, st, 0, time.Second)
	}
}

func TestStopKeptSnapshotsSettingsBeforeStartingShutdownGoroutines(t *testing.T) {
	r := &runner{say: io.Discard}
	st := &stream{cancel: func() {}, children: map[*streamChild]bool{}}
	s := &served{in: installed{Agent: agentFile{Deadline: "20ms"}}, jail: &jail{}, stream: st}
	r.served = map[string]*served{"#fixture@h": s}
	stop := replaceDeadlinesDuring(t, r, s)
	defer stop()
	for i := 0; i < 128; i++ {
		r.stopKept()
	}
}

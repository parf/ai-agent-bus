package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/sandbox"
)

// recorded is what a test sink was asked to write.
type recorded struct {
	mu  sync.Mutex
	got []agentEvent
}

func (c *recorded) add(e agentEvent) {
	c.mu.Lock()
	c.got = append(c.got, e)
	c.mu.Unlock()
}

func (c *recorded) list() []agentEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]agentEvent(nil), c.got...)
}

func (c *recorded) count(level string) int {
	n := 0
	for _, e := range c.list() {
		if e.level == level {
			n++
		}
	}
	return n
}

// recordingRunner is a runner whose events are recorded, not sent.
func recordingRunner(t *testing.T) (*runner, *recorded) {
	t.Helper()
	rec := &recorded{}
	r := &runner{ctx: context.Background(), say: io.Discard, served: map[string]*served{}, name: "#runner@h"}
	r.events = newEventSink(r)
	r.events.send = func(_ context.Context, e agentEvent) error { rec.add(e); return nil }
	r.events.start()
	t.Cleanup(r.events.close)
	return r, rec
}

func awaitCount(t *testing.T, rec *recorded, level string, want int) {
	t.Helper()
	end := time.Now().Add(2 * time.Second)
	for rec.count(level) < want {
		if time.Now().After(end) {
			t.Fatalf("%d %s events, want %d (of %d events in all)", rec.count(level), level, want, len(rec.list()))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// The write is one call to #bus as the agent: the agent's own token, never the
// runner's, the agent's own name as the target, and no wait and no reply route,
// so #bus has nothing to answer and nothing reaches the agent's inbox.
func TestEventWriteUsesTheAgentsOwnCredentialAndSendsNoReplyRoute(t *testing.T) {
	var mu sync.Mutex
	var sent []protocol.Envelope
	var tokens, other []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if req.Method == "POST" && req.URL.Path == "/send" {
			var e protocol.Envelope
			json.NewDecoder(req.Body).Decode(&e)
			sent = append(sent, e)
			tokens = append(tokens, req.Header.Get("X-Agent-Bus-Token"))
			io.WriteString(w, `{}`)
			return
		}
		other = append(other, req.Method+" "+req.URL.Path)
		w.WriteHeader(404)
	}))
	defer srv.Close()
	for _, who := range []string{"#a@h", "#b@h"} {
		sink := newEventSink(&runner{say: io.Discard, bus: &client{http: srv.Client(), base: srv.URL, token: "RUNNER"}})
		s := &served{name: who, bus: &client{http: srv.Client(), base: srv.URL, token: "TOKEN-" + who}}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := sink.callBus(ctx, agentEvent{s: s, level: "warning", text: "disk is full"}); err != nil {
			t.Fatalf("%s: %v", who, err)
		}
		cancel()
	}
	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 2 || len(other) != 0 {
		t.Fatalf("%d sends and other calls %v; want two sends and nothing else (no read of an answer)", len(sent), other)
	}
	for i, who := range []string{"#a@h", "#b@h"} {
		e := sent[i]
		if tokens[i] != "TOKEN-"+who {
			t.Errorf("%s: write used token %q, want its own", who, tokens[i])
		}
		if e.To != "#bus" || e.Body != "write "+who+" warning disk is full" {
			t.Errorf("%s: sent to %q body %q", who, e.To, e.Body)
		}
		if e.ReplyTo != nil || e.Wait != "" {
			t.Errorf("%s: the write asks for an answer: reply_to %+v wait %q", who, e.ReplyTo, e.Wait)
		}
	}
}

// A refusal or a silent #bus ends in a drop, within the deadline, never a hang.
func TestEventWriteToASilentBusEndsAtItsDeadline(t *testing.T) {
	old := eventDeadline
	eventDeadline = 150 * time.Millisecond
	defer func() { eventDeadline = old }()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		select {
		case <-release:
		case <-req.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)
	r := &runner{say: io.Discard}
	sink := newEventSink(r)
	s := &served{name: "#a@h", bus: &client{http: srv.Client(), base: srv.URL, token: "T"}}
	done := make(chan struct{})
	go func() { sink.deliver(context.Background(), agentEvent{s: s, level: "notice", text: "x"}); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("a silent #bus held the sender past its deadline")
	}
	if sink.dropped.Load() != 1 {
		t.Fatalf("dropped %d, want 1", sink.dropped.Load())
	}
}

// A refused send counts a drop and nothing else.
func TestEventWriteRefusedIsCountedNotRetried(t *testing.T) {
	var sends int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		sends++
		w.WriteHeader(403)
		io.WriteString(w, "refused")
	}))
	defer srv.Close()
	sink := newEventSink(&runner{say: io.Discard})
	s := &served{name: "#a@h", bus: &client{http: srv.Client(), base: srv.URL, token: "T"}}
	sink.deliver(context.Background(), agentEvent{s: s, level: "alert", text: "x"})
	mu.Lock()
	defer mu.Unlock()
	if sends != 1 || sink.dropped.Load() != 1 {
		t.Fatalf("sends %d, dropped %d; want one attempt and one drop", sends, sink.dropped.Load())
	}
}

// Configured secrets are masked before a line reaches the agent's log; other
// text is forwarded as it is, and a secret split across writes is still caught.
func TestStderrLinesReachTheLogAsWarningsMasked(t *testing.T) {
	r, rec := recordingRunner(t)
	s := &served{name: "#a@h", bus: &client{}}
	w := r.stderrOf(s, []string{"API_KEY=sekret-value-123"})
	io.WriteString(w, "connecting with sekret-value-123 now\n")
	io.WriteString(w, "second sekret-")
	io.WriteString(w, "value-123 line\n")
	io.WriteString(w, "no newline at the end")
	w.flush()
	awaitCount(t, rec, "warning", 3)
	got := rec.list()
	want := []string{"connecting with *** now", "second *** line", "no newline at the end"}
	for i, e := range got {
		if e.level != "warning" || e.text != want[i] || e.s != s {
			t.Errorf("event %d = %+v, want warning %q", i, e, want[i])
		}
		if strings.Contains(e.text, "sekret") {
			t.Errorf("a secret reached the log: %q", e.text)
		}
	}
}

// Bounds: a line with no newline is held to a fixed size and forwarded cut; a
// newline inside the text cannot start a second command.
func TestStderrForwardingIsBounded(t *testing.T) {
	r, rec := recordingRunner(t)
	s := &served{name: "#a@h", bus: &client{}}
	w := r.stderrOf(s, nil)
	for i := 0; i < 64; i++ {
		io.WriteString(w, strings.Repeat("é", 1024))
	}
	tee := w.out.(*teeLines)
	if len(tee.partial) > eventPartialMax {
		t.Fatalf("held %d bytes of an unfinished line, bound is %d", len(tee.partial), eventPartialMax)
	}
	io.WriteString(w, "\n")
	awaitCount(t, rec, "warning", 1)
	text := rec.list()[0].text
	if len(text) > eventLineMax || !strings.HasPrefix(text, "é") {
		t.Fatalf("forwarded %d bytes, want at most %d", len(text), eventLineMax)
	}
	if got := oneLine("a\nwrite #x alert b\r\x1b[31m", 200); strings.ContainsAny(got, "\n\r\x1b") {
		t.Fatalf("control characters survived: %q", got)
	}
}

// A sender that never returns must not block the child writing stderr, and the
// per-agent rate limit and shared queue drop and count instead.
func TestEventsNeverBlockTheChildAndDropWhenFull(t *testing.T) {
	r := &runner{ctx: context.Background(), say: io.Discard, name: "#runner@h"}
	r.events = newEventSink(r)
	block := make(chan struct{})
	r.events.send = func(ctx context.Context, e agentEvent) error { <-block; return nil }
	r.events.start()
	defer func() { close(block); r.events.close() }()
	s := &served{name: "#a@h", bus: &client{}}
	w := r.stderrOf(s, nil)
	wrote := make(chan struct{})
	go func() {
		for i := 0; i < 5000; i++ {
			io.WriteString(w, "line "+strconv.Itoa(i)+"\n")
		}
		close(wrote)
	}()
	select {
	case <-wrote:
	case <-time.After(2 * time.Second):
		t.Fatal("writing stderr blocked while the sender was stuck")
	}
	// One agent is held to its burst: a stuck sender holds one, the queue the rest.
	if n := int(r.events.dropped.Load()); n < 5000-eventBurst-1 {
		t.Fatalf("dropped %d of 5000, want at least %d", n, 5000-eventBurst-1)
	}
	// Many agents each under their own rate fill the shared queue: the rest drop.
	before := r.events.dropped.Load()
	filled := make(chan struct{})
	go func() {
		for i := 0; i < 600; i++ {
			r.events.emit(&served{name: "#many" + strconv.Itoa(i) + "@h", bus: &client{}}, levelNotice, "x")
		}
		close(filled)
	}()
	select {
	case <-filled:
	case <-time.After(2 * time.Second):
		t.Fatal("a full queue blocked the emitter")
	}
	if n := r.events.dropped.Load() - before; n < 600-eventQueue-1 {
		t.Fatalf("a full shared queue dropped %d of 600, want at least %d", n, 600-eventQueue-1)
	}
	// Another agent still gets its own allowance, and the drop was not an event.
	if !r.events.admit("#other@h") {
		t.Fatal("one agent's flood used another's allowance")
	}
}

// Start and a requested stop are notices; a requested stop is no failure.
func TestKeptStartIsANoticeAndARequestedStopIsNoAlert(t *testing.T) {
	r, rec := recordingRunner(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "agent.sh")
	os.WriteFile(script, []byte("while :; do sleep .02; done\n"), 0600)
	s := &served{bus: unconfiguredClient(), name: "#fixture@h", cmd: "/bin/sh " + strconv.Quote(script), work: dir, dir: dir, box: sandbox.Off{}, jail: &jail{}, in: installed{Key: "fixture", Agent: agentFile{Deadline: "80ms"}, Span: span{Min: 1, Max: 1}}}
	k := r.keep(s, "")
	awaitCount(t, rec, "notice", 1)
	k.stop()
	time.Sleep(100 * time.Millisecond)
	if n := rec.count("alert"); n != 0 {
		t.Fatalf("a requested stop raised %d alerts: %+v", n, rec.list())
	}
	if got := rec.list()[0]; got.text != "process started" || got.level != "notice" {
		t.Fatalf("first event %+v, want the start notice", got)
	}
}

// An exit nobody asked for is an alert, with its code and no words of the
// process's own.
func TestKeptUnrequestedExitIsAnAlertWithItsCode(t *testing.T) {
	first, most := backoffFirst, backoffMost
	backoffFirst, backoffMost = 200*time.Millisecond, 200*time.Millisecond
	defer func() { backoffFirst, backoffMost = first, most }()
	r, rec := recordingRunner(t)
	dir := t.TempDir()
	s := &served{bus: unconfiguredClient(), name: "#fixture@h", cmd: "/bin/sh -c 'echo hush-secret >&2; exit 3'", work: dir, dir: dir, box: sandbox.Off{}, jail: &jail{}, in: installed{Key: "fixture", Agent: agentFile{Deadline: "80ms"}, Span: span{Min: 1, Max: 1}}}
	k := r.keep(s, "")
	defer k.stop()
	awaitCount(t, rec, "alert", 1)
	for _, e := range rec.list() {
		if e.level == "alert" && e.text != "process exited unexpectedly, code 3" {
			t.Errorf("alert text %q", e.text)
		}
		if e.level == "alert" && strings.Contains(e.text, "hush") {
			t.Errorf("the process's own words reached an alert: %q", e.text)
		}
	}
	awaitCount(t, rec, "warning", 1)
}

// One exit is one event, however many hooks look at it, and a signal says so.
func TestExitEventIsOnePerProcessAndNamesASignal(t *testing.T) {
	r, rec := recordingRunner(t)
	s := &served{name: "#a@h", bus: &client{}}
	cmd := exec.Command("/bin/sh", "-c", "kill -9 $$")
	cmd.Run()
	r.exitEvent(s, cmd, false)
	r.exitEvent(s, cmd, true)
	awaitCount(t, rec, "alert", 1)
	time.Sleep(50 * time.Millisecond)
	got := rec.list()
	if len(got) != 1 || got[0].text != "process killed by signal "+syscall.SIGKILL.String() {
		t.Fatalf("events %+v, want one signal alert", got)
	}
}

// A one-message agent that exits badly raises an alert; one that fails to
// start does as well; a runner that is stopping raises none.
func TestOneMessageFailureIsAnAlertUnlessTheRunnerIsStopping(t *testing.T) {
	r, rec := recordingRunner(t)
	s := &served{name: "#a@h", bus: &client{}}
	cmd := exec.Command("/bin/sh", "-c", "exit 4")
	cmd.Run()
	r.exitEvent(s, cmd, false)
	awaitCount(t, rec, "alert", 1)
	stopping, cancel := context.WithCancel(context.Background())
	cancel()
	r.ctx = stopping
	r.exitEvent(s, cmd, r.ctx != nil && r.ctx.Err() != nil)
	time.Sleep(50 * time.Millisecond)
	if rec.count("alert") != 1 {
		t.Fatalf("a stopping runner raised an alert: %+v", rec.list())
	}
}

// With no sink (a stopped runner, any test) nothing happens and nothing panics.
func TestEventsWithoutASinkAreIgnored(t *testing.T) {
	r := &runner{say: io.Discard}
	s := &served{name: "#a@h", bus: &client{}}
	r.event(s, levelAlert, "x")
	io.WriteString(r.stderrOf(s, nil), "line\n")
	var k *eventSink
	k.emit(s, levelNotice, "x")
	k.close()
}

// Every write is handed a deadline no longer than eventDeadline.
func TestEventWriteIsGivenADeadline(t *testing.T) {
	sink := newEventSink(&runner{say: io.Discard})
	var until time.Time
	var ok bool
	sink.send = func(ctx context.Context, e agentEvent) error { until, ok = ctx.Deadline(); return nil }
	sink.deliver(context.Background(), agentEvent{s: &served{name: "#a@h", bus: &client{}}, level: "notice", text: "x"})
	if !ok || time.Until(until) > eventDeadline {
		t.Fatalf("write deadline %v (set %v), want one within %s", until, ok, eventDeadline)
	}
}

// A stream agent's last unterminated stderr line, often the crash message, is
// forwarded when its process is waited for.
func TestStreamAgentsLastUnterminatedStderrLineIsForwarded(t *testing.T) {
	testAnswers(t)
	r, rec := recordingRunner(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "agent.sh")
	os.WriteFile(script, []byte("printf 'last words' >&2\nexit 3\n"), 0600)
	s := &served{bus: unconfiguredClient(), name: "#fixture@h", cmd: "/bin/sh " + strconv.Quote(script), work: dir, dir: dir, box: sandbox.Off{}, jail: &jail{},
		in: installed{Key: "fixture", Agent: agentFile{Form: "jsonl", InFlight: 1, Deadline: "100ms"}, Span: span{Min: 1, Max: 1}}}
	r.installed = []installed{s.in}
	st := r.startStream(s)
	defer st.stop(100*time.Millisecond, nil)
	awaitCount(t, rec, "warning", 1)
	if got := rec.list(); got[0].level == "warning" && got[0].text != "last words" {
		t.Fatalf("forwarded %q, want the unterminated last line", got[0].text)
	}
}

// A flood of stderr warnings uses up the agent's warning allowance and fills the
// shared queue behind a slow sender; the exit alert that follows is still sent.
func TestWarningFloodThenAnExitAlertIsStillSent(t *testing.T) {
	r := &runner{ctx: context.Background(), say: io.Discard, name: "#runner@h"}
	r.events = newEventSink(r)
	rec := &recorded{}
	r.events.send = func(_ context.Context, e agentEvent) error {
		if e.level != levelAlert {
			time.Sleep(10 * time.Millisecond)
		}
		rec.add(e)
		return nil
	}
	r.events.start()
	defer r.events.close()
	s := &served{name: "#a@h", bus: &client{}}
	w := r.stderrOf(s, nil)
	for i := 0; i < 2000; i++ {
		io.WriteString(w, "noise "+strconv.Itoa(i)+"\n")
	}
	for i := 0; i < 400; i++ {
		r.events.emit(&served{name: "#many" + strconv.Itoa(i) + "@h", bus: &client{}}, levelWarning, "x")
	}
	if r.events.admit(s.name) {
		t.Fatal("the flood did not use up the agent's warning allowance; the test proves nothing")
	}
	cmd := exec.Command("/bin/sh", "-c", "exit 9")
	cmd.Run()
	r.exitEvent(s, cmd, false)
	awaitCount(t, rec, levelAlert, 1)
}

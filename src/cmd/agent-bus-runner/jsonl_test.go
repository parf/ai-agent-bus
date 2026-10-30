package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/sandbox"
)

// The answer line's shape is still the owner's: these checks read "<id> <body>".
func testAnswers(t *testing.T) {
	t.Helper()
	old := streamAnswer
	streamAnswer = func(line []byte) (string, string, bool) {
		id, body, ok := bytes.Cut(line, []byte(" "))
		return string(id), string(body), ok
	}
	t.Cleanup(func() { streamAnswer = old })
}

// idOf is a shell function printing a frame's message ID.
const idOf = `idof() { printf %s "$1" | sed 's/.*"message_id":"\([^"]*\)".*/\1/'; }
`

func streamFixture(t *testing.T, body string, inFlight int) (*stream, string) {
	t.Helper()
	return poolFixture(t, body, inFlight, span{Min: 1, Max: 1}, "")
}

func poolFixture(t *testing.T, body string, inFlight int, sp span, idle string) (*stream, string) {
	t.Helper()
	return poolFixtureOutput(t, body, inFlight, sp, idle, io.Discard)
}

func poolFixtureOutput(t *testing.T, body string, inFlight int, sp span, idle string, output io.Writer) (*stream, string) {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "agent.sh")
	if err := os.WriteFile(scriptPath, []byte(idOf+"printf s >> starts\n"+body), 0600); err != nil {
		t.Fatal(err)
	}
	r := &runner{ctx: context.Background(), say: output, served: map[string]*served{}}
	s := &served{bus: unconfiguredClient(), name: "#fixture@h", cmd: "/bin/sh " + strconv.Quote(scriptPath), work: dir, dir: dir, box: sandbox.Off{}, jail: &jail{},
		in: installed{Key: "fixture", Agent: agentFile{Form: "jsonl", InFlight: inFlight, Deadline: "100ms", Idle: idle}, Span: sp}}
	r.installed = []installed{s.in}
	st := r.startStream(s)
	t.Cleanup(func() { st.stop(100*time.Millisecond, nil) })
	return st, dir
}

func message(id string) protocol.Envelope {
	return protocol.Envelope{ID: id, From: "#caller@h", To: "#fixture@h", Body: "q-" + id}
}

// Two at once, answered out of order: possible only if both frames are held.
const twoThenReverse = `read -r a; read -r b
printf '%s got-%s\n' "$(idof "$b")" "$(idof "$b")"; printf '%s got-%s\n' "$(idof "$a")" "$(idof "$a")"
while read -r x; do :; done
`

func TestJsonlInFlightTwoHoldsTwoMessagesAnsweredInAnyOrder(t *testing.T) {
	testAnswers(t)
	st, _ := streamFixture(t, twoThenReverse, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got := make(chan string, 2)
	for _, id := range []string{"m1", "m2"} {
		go func() {
			body, err := st.do(ctx, message(id))
			if err != nil {
				body = "error " + err.Error()
			}
			got <- id + "=" + body
		}()
	}
	answers := []string{<-got, <-got}
	sort.Strings(answers)
	if strings.Join(answers, ",") != "m1=got-m1,m2=got-m2" {
		t.Fatalf("answers %v, want each to its own message", answers)
	}
}

func TestJsonlInFlightOneWritesTheNextFrameOnlyAfterAnAnswer(t *testing.T) {
	testAnswers(t)
	st, _ := streamFixture(t, twoThenReverse, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	errs := make(chan error, 2)
	for _, id := range []string{"m1", "m2"} {
		go func() { _, err := st.do(ctx, message(id)); errs <- err }()
	}
	if e1, e2 := <-errs, <-errs; e1 == nil || e2 == nil {
		t.Fatalf("with in_flight 1 a second frame was written before an answer: %v %v", e1, e2)
	}
}

func TestJsonlWedgedProcessIsKilledAtTheDeadlineAndStartedAgain(t *testing.T) {
	testAnswers(t)
	st, dir := streamFixture(t, "read -r a; sleep 30\n", 1)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if _, err := st.do(ctx, message("m1")); err == nil {
		t.Fatal("a wedged process answered")
	}
	await(t, filepath.Join(dir, "starts"), "ss", 3*backoffFirst)
}

// A child that never reads stdin fills the pipe: the write must neither hold
// the child's lock nor outlast the deadline, and stop must still get through.
func TestJsonlChildThatNeverReadsStdinCannotHoldAMessageBeyondItsDeadline(t *testing.T) {
	testAnswers(t)
	st, dir := streamFixture(t, "sleep 30\n", 1)
	await(t, filepath.Join(dir, "starts"), "s", time.Second)
	e := message("m1")
	e.Body = strings.Repeat("x", 256<<10)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	began := time.Now()
	returned := make(chan error, 1)
	go func() { _, err := st.do(ctx, e); returned <- err }()
	select {
	case err := <-returned:
		if err == nil {
			t.Fatal("a message to a child that never reads was answered")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("a message to a child that never reads was still blocked %v after a 50ms deadline", time.Since(began))
	}
	stopped := make(chan struct{})
	go func() { st.stop(100*time.Millisecond, nil); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("stop did not return: the pipe write holds the child's lock")
	}
}

func TestJsonlProcessExitingLosesWhatItHeld(t *testing.T) {
	testAnswers(t)
	st, _ := streamFixture(t, "read -r a; exit 3\n", 1)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := st.do(ctx, message("m1"))
	if err == nil || !strings.Contains(err.Error(), "exited") {
		t.Fatalf("got %v, want the process's exit", err)
	}
}

// Answers each frame after a pause, one at a time.
const slowEcho = `while read -r a; do sleep .3; printf '%s ok\n' "$(idof "$a")"; done
`

func (st *stream) live() int {
	st.mu.Lock()
	defer st.mu.Unlock()
	return len(st.children)
}

func awaitLive(t *testing.T, st *stream, want int, within time.Duration) {
	t.Helper()
	end := time.Now().Add(within)
	for st.live() != want {
		if time.Now().After(end) {
			t.Fatalf("%d processes live, want %d", st.live(), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func doAll(t *testing.T, st *stream, n int, within time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), within)
	defer cancel()
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() { _, err := st.do(ctx, message("m"+strconv.Itoa(i))); errs <- err }()
	}
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("message not answered: %v", err)
		}
	}
}

func TestJsonlPoolGrowsUnderLoadUpToItsMax(t *testing.T) {
	testAnswers(t)
	st, dir := poolFixture(t, slowEcho, 1, span{Min: 1, Max: 3}, "")
	await(t, filepath.Join(dir, "starts"), "s", time.Second)
	// Six messages of 0.3 s each, within 1.5 s: one process alone needs 1.8 s.
	doAll(t, st, 6, 1500*time.Millisecond)
	raw, _ := os.ReadFile(filepath.Join(dir, "starts"))
	if n := len(raw); n < 2 || n > 3 {
		t.Fatalf("%d processes started, want growth to at most 3", n)
	}
}

func TestJsonlPoolFromZeroStartsOnTheFirstMessage(t *testing.T) {
	testAnswers(t)
	st, dir := poolFixture(t, slowEcho, 1, span{Min: 0, Max: 1}, "")
	time.Sleep(100 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(dir, "starts")); err == nil {
		t.Fatal("a 0..1 pool started a process before any message")
	}
	doAll(t, st, 1, 2*time.Second)
	await(t, filepath.Join(dir, "starts"), "s", 0)
}

// The bounded check the plan names: without the idle stop the pool stays grown.
func TestJsonlPoolShrinksToItsMinAfterIdle(t *testing.T) {
	testAnswers(t)
	st, _ := poolFixture(t, slowEcho, 1, span{Min: 1, Max: 2}, "200ms")
	awaitLive(t, st, 1, time.Second)
	doAll(t, st, 4, 2*time.Second)
	if st.live() != 2 {
		t.Fatalf("%d processes live under load, want 2", st.live())
	}
	awaitLive(t, st, 1, 2*time.Second)
	time.Sleep(400 * time.Millisecond)
	if st.live() != 1 {
		t.Fatalf("%d processes live after idle, want min 1 kept", st.live())
	}
}

// A process that ignores TERM must not hold a place for ever once it is idle:
// it is killed at the deadline and the pool is back to its min.
func TestJsonlIdleProcessIgnoringSigtermIsKilledAtTheDeadline(t *testing.T) {
	testAnswers(t)
	st, _ := poolFixture(t, "trap '' TERM\n"+slowEcho, 1, span{Min: 1, Max: 2}, "200ms")
	awaitLive(t, st, 1, time.Second)
	doAll(t, st, 4, 2*time.Second)
	if st.live() != 2 {
		t.Fatalf("%d processes live under load, want 2", st.live())
	}
	awaitLive(t, st, 1, 1500*time.Millisecond)
}

func TestStreamAnswerReadsTheMessageIDAndBody(t *testing.T) {
	for _, tc := range []struct {
		line, id, body string
		ok             bool
	}{
		{`{"message_id":"m1","body":"hi"}`, "m1", "hi", true},
		{`{"message_id":"m1"}`, "m1", "", true},
		{`{"message_id":"m1","body":""}`, "m1", "", true},
		{`{"body":"hi"}`, "", "", false},
		{`m1 hi`, "", "", false},
	} {
		id, body, ok := streamAnswer([]byte(tc.line))
		if id != tc.id || body != tc.body || ok != tc.ok {
			t.Fatalf("%s: got %q %q %v", tc.line, id, body, ok)
		}
	}
}

// Through the bus: a jsonl agent holding two messages answers both callers,
// each its own, with the agent's answer line as written.
func TestJsonlAgentAnswersThroughTheBus(t *testing.T) {
	r, b, script := configurationFixture(t, "", "")
	agentJSON := filepath.Join(filepath.Dir(script), "agent.json")
	if err := os.WriteFile(script, []byte(idOf+`read -r a; read -r b
for x in "$b" "$a"; do printf '{"message_id":"%s","body":"got %s"}\n' "$(idof "$x")" "$(idof "$x")"; done
while read -r x; do :; done
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agentJSON, mustJSON(map[string]any{"description": "Test agent", "command": "/bin/sh " + script, "form": "jsonl", "in_flight": 2, "processes": 1, "deadline": "2s"}), 0600); err != nil {
		t.Fatal(err)
	}
	r.reload()
	ids := map[string]bool{}
	for _, body := range []string{"one", "two"} {
		e, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: body})
		if err != nil {
			t.Fatal(err)
		}
		ids[e.ID] = true
	}
	for range 2 {
		// The body names the message the agent answered; each sent one once.
		got := configurationReply(t, b)
		id, _ := strings.CutPrefix(got.Body, "got ")
		if !ids[id] {
			t.Fatalf("answer %q names no message still unanswered (sent %v)", got.Body, ids)
		}
		delete(ids, id)
	}
}

// A reload that replaces a jsonl pool waits for the old pool's work: the gate
// serve passes before adopting the replacement must not open while an old
// message is still held, or two pools overlap and the old one's cleanup can
// reach the new one.
func TestReloadWaitsForOldJsonlWorkBeforeReplacement(t *testing.T) {
	testAnswers(t)
	st, dir := streamFixture(t, "read -r x; printf b > busy; while [ ! -f release ]; do sleep .01; done; printf '%s ok\\n' \"$(idof \"$x\")\"\n", 1)
	s, r := st.s, st.r
	r.mu.Lock()
	s.stream = st
	s.cancel = func() {}
	r.served[s.name] = s
	r.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := st.do(ctx, message("old")); done <- err }()
	await(t, filepath.Join(dir, "busy"), "b", time.Second)
	r.mu.Lock()
	r.stop(s.name, s)
	r.waitRetired(s.in.Key) // serve does this immediately before adopting a replacement
	r.mu.Unlock()
	drained := make(chan struct{})
	go func() { st.wg.Wait(); close(drained) }()
	select {
	case <-drained:
	case <-time.After(20 * time.Millisecond):
		os.WriteFile(filepath.Join(dir, "release"), []byte("go"), 0600)
		t.Fatal("the replacement gate opened while the retired jsonl pool still ran its processes")
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("the old message was answered by a pool that was stopped")
		}
	case <-time.After(time.Second):
		t.Fatal("the old message was left waiting after its pool drained")
	}
}

// The real reload, not a hand-driven stop: a launch setting changes under a
// running jsonl pool, and the replacement is adopted only after the old pool
// has drained.
func TestReloadDoesNotAdoptBeforeAnOldJsonlPoolDrains(t *testing.T) {
	r, _, script := configurationFixture(t, "", "while :; do sleep .02; done\n")
	agent := filepath.Join(filepath.Dir(script), "agent.json")
	if err := os.WriteFile(agent, mustJSON(map[string]any{"description": "probe", "command": "/bin/sh " + script, "form": "jsonl", "processes": 1, "deadline": "1s"}), 0600); err != nil {
		t.Fatal(err)
	}
	r.reload()
	r.mu.Lock()
	old := r.served["#fixture@h"].stream
	r.mu.Unlock()
	awaitLive(t, old, 1, time.Second)
	if err := os.MkdirAll(filepath.Join(r.host, "fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	// A launch setting changes; the serving form does not.
	if err := os.WriteFile(filepath.Join(r.host, "fixture", "env"), []byte("CHANGE=two\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r.reload()
	r.mu.Lock()
	replacement := r.served["#fixture@h"].stream
	retiring := r.retiring["fixture"]
	r.mu.Unlock()
	if replacement == old {
		t.Fatal("a changed launch setting kept the old pool")
	}
	if retiring != nil {
		select {
		case <-retiring:
		default:
			t.Fatal("the replacement was adopted before the old jsonl pool drained")
		}
	}
	awaitLive(t, replacement, 1, 2*time.Second)
}

// A message taken before a reload retired the pool, whose goroutine reaches
// the runner after it, meets the stopped pool and is refused; it never runs as
// a one-message process of an agent that is kept as a pool.
func TestLateTakenJsonlMessageNeverFallsBackToOneShotAfterThePoolRetires(t *testing.T) {
	r, _, script := configurationFixture(t, "", "if read -r line; then printf late > late-frame; fi\n")
	if err := os.WriteFile(filepath.Join(filepath.Dir(script), "agent.json"), mustJSON(map[string]any{"description": "probe", "command": "/bin/sh " + script, "form": "jsonl", "processes": 1, "deadline": "100ms"}), 0600); err != nil {
		t.Fatal(err)
	}
	r.reload()
	r.mu.Lock()
	s := r.served["#fixture@h"]
	st := s.stream
	r.mu.Unlock()
	awaitLive(t, st, 1, time.Second)
	r.mu.Lock()
	r.stop(s.name, s)
	r.waitRetired(s.in.Key)
	r.mu.Unlock()
	r.runOne(s, message("taken-before-reload"))
	if _, err := os.Stat(filepath.Join(s.work, "late-frame")); err == nil {
		t.Fatal("a late taken jsonl message ran as a new one-shot process after its pool retired")
	}
}

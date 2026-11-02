package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Agent events (Plans/R1.0-Release/agent-events/README.md#r1-min-surface-and-authority):
// the runner writes what it observes of an agent into that agent's log on
// #bus, as the agent, with the agent's own credential (s.bus), never the
// runner's. The daemon accepts a write only when the caller is the target, so
// the runner cannot write another agent's log, and a line saying "runner" is
// only text the agent's managers read. Nothing here changes a process's
// status or the runner's own observations.
//
// The path is best effort and must never hold the agent: lines go through a
// bounded queue to one sender with a short deadline per write; a full queue,
// a rate limit, a slow or refusing #bus drops the event and counts it. Drops
// are counted in the runner's journal, never as another event.

const (
	busAgent        = "#bus"           // the daemon's agent: its method calls carry the log verbs
	eventUrgent     = 64               // alerts waiting for the sender; their own lane
	eventQueue      = 256              // events waiting for the sender, all agents together
	eventLineMax    = 512              // bytes of one stderr line forwarded, cut on a rune boundary
	eventPartialMax = 4 * eventLineMax // bytes of an unfinished stderr line held; the rest of it is dropped
	eventRate       = 10               // per agent per second, the daemon's admission rate
	eventBurst      = 20               // and its burst
	eventDropReport = 60 * time.Second // how often a drop count is said in the journal
	eventDrain      = 2 * time.Second  // how long the runner's shutdown waits for what is queued
	eventTopic      = "agent-event"
)

// eventDeadline bounds one write to #bus; a var only so a test can shorten it.
var eventDeadline = 3 * time.Second

const (
	levelNotice  = "notice"
	levelWarning = "warning"
	levelAlert   = "alert"
)

// agentEvent is one observation, to be written to s's own log.
type agentEvent struct {
	s     *served
	level string
	text  string
}

// eventSink queues events and sends them one at a time.
type eventSink struct {
	r *runner
	// Two lanes, the sender taking alerts first. An alert can therefore reach
	// the agent's log ahead of an earlier notice or warning still queued: a
	// quick "exited" alert may get a lower id than the "process started" notice
	// before it. The order of one agent's log is the order of arrival at #bus.
	queue   chan agentEvent                               // notices and warnings, all agents together
	urgent  chan agentEvent                               // alerts: a flood of lines never takes their place
	send    func(ctx context.Context, e agentEvent) error // replaced in tests
	dropped atomic.Int64

	mu      sync.Mutex
	buckets map[string]*bucket
	said    time.Time
	done    chan struct{}
	stop    context.CancelFunc
}

type bucket struct {
	tokens float64
	at     time.Time
}

func newEventSink(r *runner) *eventSink {
	k := &eventSink{r: r, queue: make(chan agentEvent, eventQueue), urgent: make(chan agentEvent, eventUrgent), buckets: map[string]*bucket{}, done: make(chan struct{})}
	k.send = k.callBus
	return k
}

// start runs the sender until close, which gives what is queued a short moment
// to go out: a stop at shutdown is the event worth keeping. Alerts have their
// own lane and are taken first. Each write has its own deadline and is never
// given the stop context, or an event taken just as close began would be lost.
func (k *eventSink) start() {
	ctx, cancel := context.WithCancel(context.Background())
	k.stop = cancel
	go func() {
		defer close(k.done)
		for {
			select {
			case e := <-k.urgent:
				k.deliver(context.Background(), e)
				continue
			default:
			}
			select {
			case e := <-k.urgent:
				k.deliver(context.Background(), e)
			case e := <-k.queue:
				k.deliver(context.Background(), e)
			case <-ctx.Done():
				end, cancel := context.WithTimeout(context.Background(), eventDrain)
				defer cancel()
				for end.Err() == nil {
					select {
					case e := <-k.urgent:
						k.deliver(end, e)
					case e := <-k.queue:
						k.deliver(end, e)
					default:
						return
					}
				}
				return
			}
		}
	}()
}

// close ends the sender after it has drained, within eventDrain. Safe on nil.
func (k *eventSink) close() {
	if k == nil || k.stop == nil {
		return
	}
	k.stop()
	<-k.done
}

func (k *eventSink) deliver(ctx context.Context, e agentEvent) {
	cx, cancel := context.WithTimeout(ctx, eventDeadline)
	defer cancel()
	if err := k.send(cx, e); err != nil {
		k.drop(e.s.name)
	}
}

// emit queues one event without ever waiting. The rate is per agent and per
// lane (alerts apart from the rest); the lanes are shared. Either limit drops
// and counts.
func (k *eventSink) emit(s *served, level, text string) {
	if k == nil || s == nil || s.bus == nil {
		return
	}
	text = oneLine(text, eventLineMax)
	if text == "" {
		return
	}
	// An alert has its own allowance and lane: a flood of stderr warnings
	// must never cost the runner the exit or failed-start alert.
	lane, key := k.queue, s.name
	if level == levelAlert {
		lane, key = k.urgent, s.name+"\x00alert"
	}
	if !k.admit(key) {
		k.drop(s.name)
		return
	}
	select {
	case lane <- agentEvent{s: s, level: level, text: text}:
	default:
		k.drop(s.name)
	}
}

// admit is the per-agent token bucket: eventRate a second, eventBurst at once.
func (k *eventSink) admit(name string) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	now := time.Now()
	b := k.buckets[name]
	if b == nil {
		b = &bucket{tokens: eventBurst, at: now}
		k.buckets[name] = b
	}
	b.tokens = min(float64(eventBurst), b.tokens+now.Sub(b.at).Seconds()*eventRate)
	b.at = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// drop counts a lost event and says the running count at most once a minute,
// in the journal: a drop is never itself an event.
func (k *eventSink) drop(name string) {
	n := k.dropped.Add(1)
	k.mu.Lock()
	say := time.Since(k.said) >= eventDropReport
	if say {
		k.said = time.Now()
	}
	k.mu.Unlock()
	if say && k.r != nil && k.r.say != nil {
		fmt.Fprintf(k.r.say, "%s: agent events dropped so far: %d (last for %s)\n", k.r.name, n, name)
	}
}

// callBus is one write to #bus as the agent: the write verb for the agent's
// own log, sent with no wait and no reply route, so #bus sends no answer at all
// and nothing can reach the agent's inbox, where an in-process reader of a kept
// or stream agent would take it for a message. The runner learns only what the
// transport says: a refused or failed send is a drop. One sender serves every
// agent, so a #bus that is slow to accept delays the events of all of them by
// up to eventDeadline each; the queue and the rate limit bound what waits.
func (k *eventSink) callBus(ctx context.Context, e agentEvent) error {
	env := protocol.Envelope{
		To: busAgent, Topic: eventTopic, Tag: newEventTag(),
		Body: "write " + e.s.name + " " + e.level + " " + e.text,
	}
	return e.s.bus.postContext(ctx, "/send", env, nil)
}

var eventTags atomic.Int64

func newEventTag() string {
	return fmt.Sprintf("ev-%d-%d", time.Now().UnixNano(), eventTags.Add(1))
}

// oneLine is text on one line, controls turned to spaces, cut to max bytes on
// a rune boundary. The daemon normalizes again; this keeps the call small and
// keeps a newline from making the write verb read a second command.
func oneLine(text string, limit int) string {
	var b strings.Builder
	for _, r := range text {
		if r < 0x20 || r == 0x7f {
			r = ' '
		}
		if b.Len()+utf8.RuneLen(r) > limit {
			break
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// event is the runner's one entry point; a runner built without a sink (tests,
// a stopped runner) ignores it.
func (r *runner) event(s *served, level, text string) {
	if r.events != nil {
		r.events.emit(s, level, text)
	}
}

// exitEvent says how a process ended, once, from the single place it was
// waited for. A requested end (the runner stopping it, an idle shrink) is
// already a notice from the stop; only an unrequested one is an alert. The text
// is fixed: code or signal, never the process's own words.
func (r *runner) exitEvent(s *served, cmd *exec.Cmd, requested bool) {
	if requested || cmd == nil || cmd.ProcessState == nil {
		return
	}
	text := fmt.Sprintf("process exited unexpectedly, code %d", cmd.ProcessState.ExitCode())
	if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		text = fmt.Sprintf("process killed by signal %s", ws.Signal())
	}
	r.event(s, levelAlert, text)
}

// teeLines forwards each finished stderr line, already masked, to the agent's
// log as a warning. It holds at most eventPartialMax bytes of an unfinished
// line and drops the rest of that line; it never blocks its writer.
type teeLines struct {
	out     interface{ Write([]byte) (int, error) }
	r       *runner
	s       *served
	partial []byte
	over    bool
}

func (t *teeLines) Write(p []byte) (int, error) {
	n, err := t.out.Write(p)
	for _, b := range p[:n] {
		if b == '\n' {
			t.line()
			continue
		}
		if len(t.partial) < eventPartialMax {
			t.partial = append(t.partial, b)
		} else {
			t.over = true
		}
	}
	return n, err
}

func (t *teeLines) line() {
	text := strings.TrimSuffix(string(t.partial), "\r")
	t.partial, t.over = t.partial[:0], false
	if strings.TrimSpace(text) != "" {
		t.r.event(t.s, levelWarning, text)
	}
}

// flush forwards an unfinished last line when the process is over.
func (t *teeLines) flush() {
	if len(t.partial) > 0 {
		t.line()
	}
}

// stderrOf is the writer for a child's stderr: the secret masker first, then
// the runner's journal, with each finished masked line also forwarded to the
// agent's own log when the runner writes events.
func (r *runner) stderrOf(s *served, secrets []string) *stderrLine {
	var out io.Writer = prefixed{r.say, s.name + ": "}
	if r.events != nil {
		out = &teeLines{out: out, r: r, s: s}
	}
	return secretStderr(out, secrets)
}

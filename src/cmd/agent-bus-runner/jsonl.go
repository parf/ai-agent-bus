package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/script"
)

// A jsonl agent's processes are kept: each is started once and fed one
// envelope line per message on its stdin, for as long as it lives
// (Plans/R1.0-Release/runner-implementation.md#long-lived-agents). Each holds
// up to in_flight messages at once; its answer lines echo the message ID, so
// they may come back in any order.

// streamAnswer reads one answer line: the message it answers and the body.
// It is the one place the line's shape lives: a JSON object naming the
// message_id it answers and its body; no body, or an empty one, is done.
// Nothing else about the line is read.
var streamAnswer = func(line []byte) (id, body string, ok bool) {
	var a struct {
		MessageID string `json:"message_id"`
		Body      string `json:"body"`
	}
	if json.Unmarshal(line, &a) != nil || a.MessageID == "" {
		return "", "", false
	}
	return a.MessageID, a.Body, true
}

// stream keeps one jsonl agent's processes and hands each message to one of
// them with a free place.
type stream struct {
	r      *runner
	s      *served
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	k      int // in_flight: how many messages one process may hold
	min    int // processes kept however quiet
	most   int // processes started as messages wait
	idle   time.Duration

	mu       sync.Mutex
	loops    int                   // processes kept, started or starting
	children map[*streamChild]bool // live processes
	freed    chan struct{}         // a place came free or a process started
}

// streamChild is one live process: where its frames go and what it holds.
type streamChild struct {
	mu      sync.Mutex
	wmu     sync.Mutex // one frame at a time on the pipe, never held with mu: a child that does not read blocks its writer
	in      io.WriteCloser
	out     io.Reader
	cmd     *exec.Cmd
	pending map[string]chan string // message ID → its answer
	dead    bool
	retired bool          // stopped for being idle: not started again
	used    time.Time     // when it last held a message
	gone    chan struct{} // closed once the process has exited and been reaped
}

// idleDefault is how long a process above min may be idle before it is
// stopped, unless agent.json says (Plans/R1.0-Release/runner.md#stop-and-reload).
const idleDefault = 5 * time.Minute

// isStream is whether the runner keeps the agent's processes as a jsonl pool.
func isStream(in installed) bool { return in.Agent.Form == "jsonl" && in.Agent.Serves != "itself" }

func inFlight(a agentFile) int {
	if a.InFlight < 1 {
		return 1
	}
	return a.InFlight
}

// startStream starts an agent's min processes and returns them; more start
// as messages wait, up to its max, and those above min stop when idle.
func (r *runner) startStream(s *served) *stream {
	ctx, cancel := context.WithCancel(r.ctx)
	k := inFlight(s.in.Agent)
	idle := idleDefault
	if d, err := time.ParseDuration(s.in.Agent.Idle); err == nil && d > 0 {
		idle = d
	}
	st := &stream{r: r, s: s, ctx: ctx, cancel: cancel, k: k, min: s.in.Span.Min, most: max(s.in.Span.Max, 1), idle: idle,
		children: map[*streamChild]bool{}, freed: make(chan struct{}, 1)}
	st.mu.Lock()
	for i := 0; i < st.min; i++ {
		st.loops++
		st.wg.Add(1)
		go st.loop()
	}
	st.mu.Unlock()
	if st.most > st.min {
		st.wg.Add(1)
		go st.shrink()
	}
	fmt.Fprintf(r.say, "%s: keeping %s, %d..%d process(es) of %d in flight each, sandbox %s\n", r.name, s.name, st.min, st.most, k, s.box.Name())
	return st
}

// grow starts one more process if the agent may have it and none is
// already starting, so one waiting message starts one process.
func (st *stream) grow() bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.loops >= st.most || st.loops > len(st.children) || st.ctx.Err() != nil {
		return false
	}
	st.loops++
	st.wg.Add(1)
	go st.loop()
	return true
}

// shrink stops a process above min that has held nothing for idle: SIGTERM,
// which a process that handles nothing simply dies of, losing nothing.
func (st *stream) shrink() {
	defer st.wg.Done()
	tick := time.NewTicker(max(st.idle/4, 10*time.Millisecond))
	defer tick.Stop()
	for {
		select {
		case <-st.ctx.Done():
			return
		case <-tick.C:
		}
		// The runner's lock is never taken under the pool's.
		j := st.r.launchSettings(st.s).jail
		st.mu.Lock()
		over := st.loops - st.min
		for c := range st.children {
			if over <= 0 {
				break
			}
			c.mu.Lock()
			if !c.dead && len(c.pending) == 0 && time.Since(c.used) >= st.idle {
				c.dead, c.retired = true, true
				over--
				fmt.Fprintf(st.r.say, "%s: %s: a process idle %s is stopped\n", st.r.name, st.s.name, st.idle)
				(*keeper)(nil).signalOne(c.cmd, j, syscall.SIGTERM)
				// A process that ignores TERM would hold its place for ever:
				// it has the deadline, then it is killed.
				go func(c *streamChild) {
					select {
					case <-c.gone:
					case <-time.After(deadlineOf(st.s.in.Agent)):
						(*keeper)(nil).signalOne(c.cmd, j, syscall.SIGKILL)
					case <-st.ctx.Done():
					}
				}(c)
			}
			c.mu.Unlock()
		}
		st.mu.Unlock()
	}
}

// loop keeps one process: started, its places offered, restarted with backoff
// when it exits. What it held when it exited is lost and said.
func (st *stream) loop() {
	defer st.wg.Done()
	defer func() {
		st.mu.Lock()
		st.loops--
		st.mu.Unlock()
	}()
	wait := backoffFirst
	for st.ctx.Err() == nil {
		began := time.Now()
		c, err := st.start()
		if err == nil {
			st.mu.Lock()
			st.children[c] = true
			st.mu.Unlock()
			st.wake()
			st.answers(c)
			err = c.cmd.Wait()
			close(c.gone)
			st.mu.Lock()
			delete(st.children, c)
			st.mu.Unlock()
			c.mu.Lock()
			c.dead = true
			for id, answer := range c.pending {
				fmt.Fprintf(st.r.say, "%s: %s was in flight when its process exited; lost\n", st.s.name, id)
				close(answer)
				delete(c.pending, id)
			}
			c.mu.Unlock()
			st.r.recordExit(st.s, nil, c.cmd)
			if c.retired {
				return
			}
		}
		if st.ctx.Err() != nil {
			return
		}
		if time.Since(began) >= stable {
			wait = backoffFirst
		}
		fmt.Fprintf(st.r.say, "%s: %s exited (%v); restarting in %s\n", st.r.name, st.s.name, err, wait)
		select {
		case <-time.After(wait):
		case <-st.ctx.Done():
			return
		}
		if wait *= 2; wait > backoffMost {
			wait = backoffMost
		}
	}
}

// start is one process of the agent, confined as a per-message script is,
// told the agent's layers but no message, with a pipe for its frames.
func (st *stream) start() (*streamChild, error) {
	s, r := st.s, st.r
	launch := r.launchSettings(s)
	if _, err := r.workPath(launch.key); err != nil {
		return nil, err
	}
	j := launch.jail
	if j.group != nil {
		if err := j.group.Prepare(); err != nil {
			return nil, fmt.Errorf("its cgroup: %w", err)
		}
	}
	env, err := configuration(st.ctx, s.bus, s.name, launch.env)
	if err != nil {
		return nil, err
	}
	argv := launch.box.Wrap(ports.Job{
		Argv: []string{"/bin/sh", "-c", "exec " + launch.cmd},
		Work: s.work, Read: launch.read, Over: launch.over, Net: launch.net, TmpSize: j.tmp, Seccomp: len(j.seccomp) > 0,
	})
	if j.files > 0 {
		argv = append([]string{"prlimit", fmt.Sprintf("--nofile=%d:%d", j.files, j.files), "--"}, argv...)
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = s.work
	attr := &syscall.SysProcAttr{Setpgid: true}
	if j.group != nil {
		useCgroup(attr, j.group)
	}
	cmd.SysProcAttr = attr
	base := script.ChildEnv(os.Environ())
	if launch.box.Name() != "off" {
		base = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=" + s.work, "LANG=C.UTF-8"}
	}
	cmd.Env = mergeEnv(base, env, []string{"AGENT_BUS_TO=" + s.name, "AGENT_BUS_WORK=" + s.work})
	cmd.Stderr = secretStderr(prefixed{r.say, s.name + ": "}, launch.secretEnv)
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if len(j.seccomp) > 0 {
		rd, wr, err := os.Pipe()
		if err != nil {
			return nil, err
		}
		_, werr := wr.Write(j.seccomp)
		wr.Close()
		if werr != nil {
			rd.Close()
			return nil, werr
		}
		cmd.ExtraFiles = []*os.File{rd}
		defer rd.Close()
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &streamChild{in: in, out: out, cmd: cmd, pending: map[string]chan string{}, used: time.Now(), gone: make(chan struct{})}, nil
}

// answers routes each answer line to the message it names, until the
// process closes its output.
func (st *stream) answers(c *streamChild) {
	lines := bufio.NewScanner(c.out)
	lines.Buffer(make([]byte, 64<<10), maxOutput+4096)
	for lines.Scan() {
		id, body, ok := streamAnswer(lines.Bytes())
		c.mu.Lock()
		answer := c.pending[id]
		if ok && answer != nil {
			delete(c.pending, id)
		}
		c.mu.Unlock()
		if !ok || answer == nil {
			fmt.Fprintf(st.r.say, "%s: an answer line names no message in flight; dropped\n", st.s.name)
			continue
		}
		c.mu.Lock()
		c.used = time.Now()
		c.mu.Unlock()
		answer <- body
		st.wake()
	}
}

// wake tells a waiting do that a place may be free.
func (st *stream) wake() {
	select {
	case st.freed <- struct{}{}:
	default:
	}
}

// place is a live process with room for one more message, its lock held.
func (st *stream) place() *streamChild {
	st.mu.Lock()
	defer st.mu.Unlock()
	var best *streamChild
	for c := range st.children {
		c.mu.Lock()
		if !c.dead && len(c.pending) < st.k && (best == nil || len(c.pending) < len(best.pending)) {
			if best != nil {
				best.mu.Unlock()
			}
			best = c
			continue
		}
		c.mu.Unlock()
	}
	return best
}

// do hands one message to a process with a free place and waits for its
// answer until ctx ends. A process past the deadline is wedged: it is killed,
// and what it held is lost; the loop starts it again.
func (st *stream) do(ctx context.Context, e protocol.Envelope) (string, error) {
	var c *streamChild
	if st.ctx.Err() != nil {
		return "", fmt.Errorf("its pool is stopped")
	}
	for c = st.place(); c == nil; c = st.place() {
		if st.ctx.Err() != nil {
			return "", fmt.Errorf("its pool is stopped")
		}
		// Every place is taken: a message waits, so the pool may grow.
		st.grow()
		select {
		case <-st.freed:
		case <-time.After(50 * time.Millisecond):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	answer := make(chan string, 1)
	c.pending[e.ID] = answer
	c.used = time.Now()
	c.mu.Unlock()
	// A child that never reads stdin fills the pipe and blocks its writer, so
	// the write is off the child's lock and bounded by ctx: the deadline kills
	// the child, which ends the write.
	frame := append(mustJSON(script.Input(e)), '\n')
	wrote := make(chan error, 1)
	go func() {
		c.wmu.Lock()
		defer c.wmu.Unlock()
		_, err := c.in.Write(frame)
		wrote <- err
	}()
	select {
	case err := <-wrote:
		if err != nil {
			c.mu.Lock()
			delete(c.pending, e.ID)
			c.mu.Unlock()
			st.wake()
			return "", err
		}
	case <-ctx.Done():
		fmt.Fprintf(st.r.say, "%s: %s passed its deadline while its process was not reading; its process is killed\n", st.s.name, e.ID)
		st.kill(c)
		return "", ctx.Err()
	}
	select {
	case body, ok := <-answer:
		if !ok {
			return "", fmt.Errorf("its process exited")
		}
		return body, nil
	case <-ctx.Done():
		fmt.Fprintf(st.r.say, "%s: %s passed its deadline; its process is killed\n", st.s.name, e.ID)
		st.kill(c)
		return "", ctx.Err()
	}
}

// kill ends one process and everything it started, and no other.
func (st *stream) kill(c *streamChild) {
	if c.cmd.Process == nil {
		return
	}
	// Only this process's tree: the agent's cgroup holds the pool's other
	// processes too, and what they hold is not this one's to lose.
	(*keeper)(nil).signalOne(c.cmd, st.r.launchSettings(st.s).jail, syscall.SIGKILL)
}

// stop writes no further frame and closes each process's input, waits for
// what is in flight up to the deadline, then SIGTERM, then kills.
func (st *stream) stop(deadline time.Duration, j *jail) {
	st.cancel()
	st.mu.Lock()
	for c := range st.children {
		c.mu.Lock()
		c.dead = true
		c.in.Close()
		c.mu.Unlock()
	}
	st.mu.Unlock()
	done := make(chan struct{})
	go func() { st.wg.Wait(); close(done) }()
	select {
	case <-done:
		return
	case <-time.After(deadline):
	}
	st.signal(syscall.SIGTERM, j)
	select {
	case <-done:
	case <-time.After(deadline):
		st.signal(syscall.SIGKILL, j)
		<-done
	}
}

// signal sends sig to every live process: to the agent's cgroup, sparing
// bubblewrap, or to each process group with the sandbox off.
func (st *stream) signal(sig syscall.Signal, j *jail) {
	if j != nil && j.group != nil {
		signalGroup(j.group, sig)
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	for c := range st.children {
		if c.cmd.Process != nil {
			syscall.Kill(-c.cmd.Process.Pid, sig)
		}
	}
}

// deadlineOf is how long a stopping process has, from agent.json, else stopWait.
func deadlineOf(a agentFile) time.Duration {
	if d, err := time.ParseDuration(a.Deadline); err == nil && d > 0 {
		return d
	}
	return stopWait
}

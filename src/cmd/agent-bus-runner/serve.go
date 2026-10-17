package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/script"
)

// Serving the installed agents, wave M.2
// (Plans/R1.0-Release/TODO.md#managed-runner-waves). Each agent is
// registered by the runner's User, forwards to #runner, and is read off the
// runner's one inbox by its own address, so a busy agent never holds an idle
// one's message (runner-implementation.md#many-names-into-one-inbox). It
// answers as itself, with its own token.

// maxOutput bounds what one run may answer with: the answer is held in the
// runner, outside the child's limits, so it is cut and said rather than
// buffered whole.
const maxOutput = 1 << 20

// served is one agent the runner is serving now.
type served struct {
	in     installed
	name   string
	cmd    string // the command line, ./ resolved to the agent's directory
	work   string
	token  string
	env    []string
	slots  chan struct{}
	busy   atomic.Int32
	cancel context.CancelFunc
}

// serveable says why an agent cannot be served by this wave, or "".
func serveable(in installed) string {
	switch in.Agent.Form {
	case "", "args", "json":
	default:
		return fmt.Sprintf("the %s form is not served yet (M.3)", in.Agent.Form)
	}
	if in.Agent.Serves == "itself" {
		return "an agent that serves itself is not run yet (M.9)"
	}
	return ""
}

// environment is the agent's layered env, or why it cannot be had: a
// template run as itself, or an instance still missing what env.dist requires.
func (r *runner) environment(in installed, name string) ([]string, string) {
	dir := filepath.Join(r.agents, in.Dir)
	dist, err := readEnv(filepath.Join(dir, "env.dist"))
	if err != nil {
		return nil, err.Error()
	}
	secret, err := r.busSecret(name)
	if err != nil {
		return nil, "its bus secret: " + err.Error()
	}
	agentWide, err := readEnv(filepath.Join(r.host, in.Dir, "env"))
	if err != nil {
		return nil, err.Error()
	}
	var own []envVar
	_, instance, isInstance := strings.Cut(in.Key, "/")
	if isInstance {
		if own, err = readEnv(filepath.Join(r.host, in.Dir, instance, "env")); err != nil {
			return nil, err.Error()
		}
	}
	env, missing := layered(dist, secret, agentWide, own)
	if len(missing) > 0 {
		if !isInstance {
			return nil, fmt.Sprintf("a template: configure an instance under runner/%s/<instance>/env (it needs %s)", in.Dir, strings.Join(missing, ", "))
		}
		return nil, "needs " + strings.Join(missing, ", ")
	}
	return env, ""
}

// busSecret is the record's secret, in env-file form; none before the
// record exists or has one.
func (r *runner) busSecret(name string) ([]envVar, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, code, err := r.owner.do(ctx, "GET", "/secret", url.Values{"name": {name}}, nil)
	switch {
	case err != nil:
		return nil, err
	case code == http.StatusNotFound:
		return nil, nil
	case code >= 400:
		return nil, errors.New(strings.TrimSpace(string(out)))
	}
	return parseEnv("bus secret", out)
}

// command resolves a command line written relative to the agent's checkout.
func command(dir, line string) string {
	if strings.HasPrefix(line, "./") {
		return "'" + strings.ReplaceAll(filepath.Join(dir, line[2:]), "'", `'\''`) + "'"
	}
	return line
}

// serve brings what is served in line with what is installed: new agents are
// registered, forwarded to the runner and read; ones gone or refused are
// unforwarded and stopped. It holds r.mu.
func (r *runner) serve(ctx context.Context) {
	want := map[string]bool{}
	// Two installed agents that come to one name would be one served agent
	// with the other's command: both are refused, by name.
	named := map[string][]string{}
	for _, in := range r.installed {
		if in.Refused == "" && in.enabled() {
			n := r.agentName(in)
			named[n] = append(named[n], in.Key)
		}
	}
	for i := range r.installed {
		in := &r.installed[i]
		if in.Refused != "" || !in.enabled() {
			continue
		}
		name := r.agentName(*in)
		if keys := named[name]; len(keys) > 1 {
			in.Refused = fmt.Sprintf("%s is also the name of %s", name, strings.Join(others(keys, in.Key), ", "))
			continue
		}
		if why := serveable(*in); why != "" {
			in.Refused = why
			continue
		}
		env, why := r.environment(*in, name)
		if why != "" {
			in.Refused = why
			continue
		}
		if s, ok := r.served[name]; ok {
			// A one-message form reads its env at each start: the next run
			// gets the new one (Plans/R1.0-Release/runner.md#stop-and-reload).
			s.in, s.env = *in, env
			s.cmd = command(filepath.Join(r.agents, in.Dir), in.Agent.Command)
			want[name] = true
			continue
		}
		s, err := r.adopt(ctx, *in, name, env)
		if err != nil {
			in.Refused = err.Error()
			fmt.Fprintf(r.say, "%s: %s is refused: %v\n", r.name, in.Key, err)
			continue
		}
		r.served[name] = s
		want[name] = true
	}
	for name, s := range r.served {
		if !want[name] {
			s.cancel()
			delete(r.served, name)
			r.dropping = append(r.dropping, dropped{name, s})
		}
	}
}

// others is keys without key.
func others(keys []string, key string) []string {
	var out []string
	for _, k := range keys {
		if k != key {
			out = append(out, k)
		}
	}
	return out
}

type dropped struct {
	name string
	s    *served
}

// adopt registers an agent as the runner's, forwards it to the runner, takes
// its token and starts reading for it.
func (r *runner) adopt(ctx context.Context, in installed, name string, env []string) (*served, error) {
	if err := r.owner.post("/register", protocol.Record{
		Name: name, Kind: protocol.KindAgent, Descr: in.Agent.Description, Roles: in.Agent.Roles,
	}, nil); err != nil {
		return nil, fmt.Errorf("register: %w", err)
	}
	subs := []string{r.name}
	if err := r.owner.post("/manage", map[string]any{"name": name, "subs": subs}, nil); err != nil {
		return nil, fmt.Errorf("forward to %s: %w", r.name, err)
	}
	var tok struct {
		Token string `json:"token"`
	}
	if err := r.owner.post("/token", map[string]any{"name": name, "rotate": false}, &tok); err != nil || tok.Token == "" {
		return nil, fmt.Errorf("its token: %v", err)
	}
	work := filepath.Join(r.host, "work", in.Key)
	if err := os.MkdirAll(work, 0o700); err != nil {
		return nil, err
	}
	max := in.Span.Max
	if max < 1 {
		max = 1
	}
	rctx, cancel := context.WithCancel(ctx)
	s := &served{in: in, name: name, work: work, token: tok.Token, env: env,
		cmd:   command(filepath.Join(r.agents, in.Dir), in.Agent.Command),
		slots: make(chan struct{}, max), cancel: cancel}
	go r.read(rctx, s)
	fmt.Fprintf(r.say, "%s: serving %s, %d at a time\n", r.name, name, max)
	return s, nil
}

// drop stops serving an agent: it no longer forwards here, its reader stops,
// and what was already forwarded for it is answered as refused, never left
// in front of every other agent's work.
// It runs without r.mu: it talks to the daemon, which may be slow, and
// status must not wait on it.
func (r *runner) drop(name string, s *served) {
	if err := r.owner.post("/manage", map[string]any{"name": name, "subs": []string{}}, nil); err != nil {
		fmt.Fprintf(r.say, "%s: could not stop forwarding %s: %v\n", r.name, name, err)
	}
	for {
		e, got, err := r.bus.nextFor(context.Background(), name, 0)
		if err != nil || !got {
			break
		}
		if e.Receipt == "" {
			r.answerAs(s.token, e, "error: "+name+" is not served here any more")
		}
	}
	fmt.Fprintf(r.say, "%s: no longer serving %s\n", r.name, name)
}

// read takes this agent's messages off the runner's inbox, one per free
// process: the slot is taken before the read, so nothing is taken that no
// process is free to run.
func (r *runner) read(ctx context.Context, s *served) {
	const poll = 55 * time.Second
	for {
		select {
		case s.slots <- struct{}{}:
		case <-ctx.Done():
			return
		}
		e, got, err := r.bus.nextFor(ctx, s.name, poll)
		if ctx.Err() != nil {
			<-s.slots
			return
		}
		if err != nil || !got {
			<-s.slots
			if err != nil {
				fmt.Fprintf(r.say, "%s: reading for %s: %v; reading again\n", r.name, s.name, err)
				select {
				case <-time.After(2 * time.Second):
				case <-ctx.Done():
					return
				}
			}
			continue
		}
		go func() {
			defer func() { <-s.slots }()
			r.runOne(s, e)
		}()
	}
}

// runOne serves one message: ack, the script once, then its answer or done —
// all as the agent that was addressed.
func (r *runner) runOne(s *served, e protocol.Envelope) {
	if e.Receipt != "" {
		return
	}
	if e.TooLate(time.Now()) {
		fmt.Fprintf(r.say, "%s: %s arrived after its caller gave up; not run\n", s.name, e.ID)
		return
	}
	s.busy.Add(1)
	defer s.busy.Add(-1)
	r.receipt(s.token, e, protocol.ReceiptAck)

	ctx := context.Background()
	if d, err := time.ParseDuration(s.in.Agent.Deadline); err == nil && d > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d)
		defer cancel()
	}
	if !e.Deadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, e.Deadline)
		defer cancel()
	}
	argv := script.Argv(s.cmd, s.in.Agent.Form == "args", e.Body, e.Roles)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = s.work
	if s.in.Agent.Form != "args" {
		cmd.Stdin = bytes.NewReader(append(mustJSON(e), '\n'))
	}
	cmd.Env = mergeEnv(script.ChildEnv(os.Environ()), s.env, []string{
		"AGENT_BUS_MESSAGE_ID=" + e.ID, "AGENT_BUS_FROM=" + e.From, "AGENT_BUS_TO=" + s.name,
		"AGENT_BUS_TOPIC=" + e.Topic, "AGENT_BUS_TAG=" + e.Tag, "AGENT_BUS_WORK=" + s.work})
	out := &bounded{max: maxOutput}
	cmd.Stdout = out
	cmd.Stderr = prefixed{r.say, s.name + ": "}
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(r.say, "%s: %s exited badly for %s: %v\n", s.name, s.in.Agent.Command, e.ID, err)
		return
	}
	answer := strings.TrimRight(out.buf.String(), "\n")
	if out.cut {
		answer += fmt.Sprintf("\n[agent-bus-runner: the answer was cut at %d bytes]", maxOutput)
		fmt.Fprintf(r.say, "%s: the answer to %s was cut at %d bytes\n", s.name, e.ID, maxOutput)
	}
	if answer == "" {
		r.receipt(s.token, e, protocol.ReceiptDone)
		return
	}
	r.answerAs(s.token, e, answer)
}

// mergeEnv is the child's environment, each later list winning a name an
// earlier one set: the runner's own, then the agent's layers, then what the
// runner tells it about the message. A child's getenv takes the first of a
// duplicated name, so a plain append would let the inherited PATH beat the
// agent's, and an env file's AGENT_BUS_FROM beat the real sender.
func mergeEnv(lists ...[]string) []string {
	at, out := map[string]int{}, []string{}
	for _, list := range lists {
		for _, kv := range list {
			name, _, _ := strings.Cut(kv, "=")
			if i, seen := at[name]; seen {
				out[i] = kv
				continue
			}
			at[name] = len(out)
			out = append(out, kv)
		}
	}
	return out
}

// route is where e's receipts and answer go.
func route(e protocol.Envelope) (string, string, string) {
	if e.ReplyTo != nil {
		return e.ReplyTo.Name, e.ReplyTo.Topic, e.ReplyTo.Tag
	}
	return e.From, e.Topic, e.Tag
}

func (r *runner) receipt(token string, e protocol.Envelope, kind string) {
	back, topic, tag := route(e)
	if err := r.bus.as(token).post("/send", protocol.Envelope{To: back, Topic: topic, Tag: tag, Receipt: kind, Re: e.ID}, nil); err != nil {
		fmt.Fprintf(r.say, "%s: could not %s %s: %v\n", r.name, kind, e.ID, err)
	}
}

func (r *runner) answerAs(token string, e protocol.Envelope, body string) {
	back, topic, tag := route(e)
	if err := r.bus.as(token).post("/send", protocol.Envelope{To: back, Topic: topic, Tag: tag, Body: body}, nil); err != nil {
		fmt.Fprintf(r.say, "%s: could not answer %s: %v\n", r.name, e.ID, err)
	}
}

// bounded keeps the first max bytes written and says whether more came.
type bounded struct {
	buf bytes.Buffer
	max int
	cut bool
}

func (b *bounded) Write(p []byte) (int, error) {
	if room := b.max - b.buf.Len(); room < len(p) {
		b.cut = true
		if room > 0 {
			b.buf.Write(p[:room])
		}
		return len(p), nil
	}
	return b.buf.Write(p)
}

// prefixed writes each line a child says on stderr into the runner's log
// under its agent's name.
type prefixed struct {
	w      interface{ Write([]byte) (int, error) }
	prefix string
}

var logMu sync.Mutex

func (p prefixed) Write(b []byte) (int, error) {
	logMu.Lock()
	defer logMu.Unlock()
	for _, line := range strings.SplitAfter(string(b), "\n") {
		if line != "" {
			fmt.Fprint(p.w, p.prefix+line)
		}
	}
	return len(b), nil
}

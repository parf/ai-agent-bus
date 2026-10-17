package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/sandbox"
	"github.com/parf/ai-agent-bus/internal/script"
)

// Serving the installed agents, waves M.2 and M.10
// (Plans/R1.0-Release/TODO.md#managed-runner-waves). Each agent is its
// person's: the runner creates it, keeps the token the creating register
// answered, and reads the agent's own inbox and secret as the agent, so a busy
// agent never holds an idle one's message and nothing is forwarded. It answers
// as itself.

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
	bus    *client // as the agent, with its own token
	env    []string
	slots  chan struct{}
	busy   atomic.Int32
	box    ports.Sandbox // bwrap unless the host said sandbox off
	dir    string        // the agent's checkout, the one directory it reads
	net    bool          // its agent.json asked for the network
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
func (r *runner) environment(in installed, as *client) ([]string, string) {
	dir := filepath.Join(r.agents, in.Dir)
	dist, err := readEnv(filepath.Join(dir, "env.dist"))
	if err != nil {
		return nil, err.Error()
	}
	secret, err := busSecret(as)
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

// busSecret is the agent's own secret, asked as the agent, in env-file form;
// none while it has none.
func busSecret(as *client) ([]envVar, error) {
	name, err := as.whoami()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, code, err := as.do(ctx, "GET", "/secret", url.Values{"name": {name}}, nil)
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
// claimed and read; ones gone or refused are stopped, and what is sent to them
// waits in their own inboxes. It holds r.mu.
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
		box, why := r.sandboxFor(*in)
		if why != "" {
			in.Refused = why
			continue
		}
		if keys := named[name]; len(keys) > 1 {
			in.Refused = fmt.Sprintf("%s is also the name of %s", name, strings.Join(others(keys, in.Key), ", "))
			continue
		}
		if why := serveable(*in); why != "" {
			in.Refused = why
			continue
		}
		as := r.bus
		if s, ok := r.served[name]; ok {
			as = s.bus
		} else {
			tok, err := r.claim(*in, name)
			if err != nil {
				in.Refused = err.Error()
				fmt.Fprintf(r.say, "%s: %s is refused: %v\n", r.name, in.Key, err)
				continue
			}
			as = r.bus.as(tok)
		}
		env, why := r.environment(*in, as)
		if why != "" {
			in.Refused = why
			continue
		}
		if s, ok := r.served[name]; ok {
			// A one-message form reads its env at each start: the next run
			// gets the new one (Plans/R1.0-Release/runner.md#stop-and-reload).
			s.in, s.env, s.box, s.net = *in, env, box, wantsNet(*in)
			s.cmd = command(filepath.Join(r.agents, in.Dir), in.Agent.Command)
			want[name] = true
			continue
		}
		s, err := r.adopt(ctx, *in, name, as, env, box)
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
			fmt.Fprintf(r.say, "%s: no longer serving %s\n", r.name, name)
		}
	}
}

// tokenFile is where the runner keeps an agent's token, in its private home.
func (r *runner) tokenFile(name string) string {
	return filepath.Join(r.host, "tokens", url.PathEscape(name))
}

// claim is the agent's token: the one kept since the runner created it, or the
// one the creating register answers now. A name that exists with no token kept
// here is an agent the runner was never handed, and is refused by name, never
// taken: its Owner hands it over once (docs/02-access.md#getting-a-token).
func (r *runner) claim(in installed, name string) (string, error) {
	file := r.tokenFile(name)
	handOver := fmt.Sprintf("its Owner hands it over once: agent-bus-token '%s' into %s", name, file)
	if b, err := os.ReadFile(file); err == nil {
		tok := strings.TrimSpace(string(b))
		if who, err := r.bus.as(tok).whoami(); err != nil || who != name {
			return "", fmt.Errorf("the token kept for %s is not its any more (%v); %s", name, err, handOver)
		}
		if err := refreshManagedTags(r.bus.as(tok), in, name); err != nil {
			return "", err
		}
		return tok, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, code, err := r.bus.doWith(ctx, "POST", "/register", nil, protocol.Record{
		Name: name, Kind: protocol.KindAgent, Descr: in.Agent.Description, Roles: in.Agent.Roles,
		Tags: protocol.NormalizeTags(append(append(append([]string(nil), in.Agent.Tags...), in.Host.Tags...), "runner")),
	}, map[string]string{"If-None-Match": "*"})
	switch {
	case err != nil:
		return "", fmt.Errorf("register: %w", err)
	case code == http.StatusPreconditionFailed:
		return "", fmt.Errorf("%s exists and its token is not kept here; %s", name, handOver)
	case code >= 400:
		return "", fmt.Errorf("register: %s", strings.TrimSpace(string(out)))
	}
	var created struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(out, &created); err != nil || created.Token == "" {
		return "", fmt.Errorf("%s was created, but no token came back; %s", name, handOver)
	}
	if err := keep(file, created.Token); err != nil {
		return "", fmt.Errorf("%s was created, but its token could not be kept (%v); %s", name, err, handOver)
	}
	return created.Token, nil
}

// keep writes a token readable by the runner alone, whole or not at all.
func keep(file, token string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return err
	}
	tmp := file + ".new"
	if err := os.WriteFile(tmp, []byte(token+"\n"), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, file)
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

// adopt starts reading for an agent the runner holds the token of.
func (r *runner) adopt(ctx context.Context, in installed, name string, as *client, env []string, box ports.Sandbox) (*served, error) {
	work := filepath.Join(r.host, "work", in.Key)
	if err := os.MkdirAll(work, 0o700); err != nil {
		return nil, err
	}
	max := in.Span.Max
	if max < 1 {
		max = 1
	}
	rctx, cancel := context.WithCancel(ctx)
	s := &served{in: in, name: name, work: work, bus: as, env: env,
		box: box, dir: filepath.Join(r.agents, in.Dir), net: wantsNet(in),
		cmd:   command(filepath.Join(r.agents, in.Dir), in.Agent.Command),
		slots: make(chan struct{}, max), cancel: cancel}
	go r.read(rctx, s)
	fmt.Fprintf(r.say, "%s: serving %s, %d at a time, sandbox %s\n", r.name, name, max, box.Name())
	return s, nil
}

// read takes this agent's messages off its own inbox, one per free process:
// the slot is taken before the read, so nothing is taken that no process is
// free to run.
func (r *runner) read(ctx context.Context, s *served) {
	const poll = 55 * time.Second
	for {
		select {
		case s.slots <- struct{}{}:
		case <-ctx.Done():
			return
		}
		e, got, err := s.bus.next(ctx, poll)
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
	r.receipt(s.bus, e, protocol.ReceiptAck)

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
	argv := s.box.Wrap(ports.Job{
		Argv: script.Argv(s.cmd, s.in.Agent.Form == "args", e.Body, e.Roles),
		Work: s.work, Read: []string{s.dir}, Net: s.net,
	})
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = s.work
	if s.in.Agent.Form != "args" {
		cmd.Stdin = bytes.NewReader(append(mustJSON(e), '\n'))
	}
	// A sandboxed child starts from nothing of the runner's: what it is told
	// is the agent's layers and the message, over a bare PATH and HOME.
	base := script.ChildEnv(os.Environ())
	if s.box.Name() != "off" {
		base = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=" + s.work, "LANG=C.UTF-8"}
	}
	cmd.Env = mergeEnv(base, s.env, []string{
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
		r.receipt(s.bus, e, protocol.ReceiptDone)
		return
	}
	r.answerAs(s.bus, e, answer)
}

// sandboxFor is the agent's sandbox: bubblewrap unless agents.json turned it
// off for that agent, and a refusal, never a quiet run unconfined, on a host
// that cannot give one (Plans/R1.0-Release/runner.md#sandboxing).
func (r *runner) sandboxFor(in installed) (ports.Sandbox, string) {
	switch in.Host.Sandbox {
	case "off":
		return sandbox.Off{}, ""
	case "", "on":
	default:
		return nil, fmt.Sprintf("agents.json: sandbox %q is on or off", in.Host.Sandbox)
	}
	r.probe.Do(func() { r.noSandbox = sandbox.ProbeBwrap() })
	if r.noSandbox != nil {
		return nil, "the sandbox is on and this host cannot give one: " + r.noSandbox.Error()
	}
	return sandbox.Bwrap{}, ""
}

// wantsNet is whether agent.json asked for the network; which hosts is M.7's.
func wantsNet(in installed) bool { return in.Agent.Needs != nil && len(in.Agent.Needs.Network) > 0 }

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

func (r *runner) receipt(as *client, e protocol.Envelope, kind string) {
	back, topic, tag := route(e)
	if err := as.post("/send", protocol.Envelope{To: back, Topic: topic, Tag: tag, Receipt: kind, Re: e.ID}, nil); err != nil {
		fmt.Fprintf(r.say, "%s: could not %s %s: %v\n", r.name, kind, e.ID, err)
	}
}

func (r *runner) answerAs(as *client, e protocol.Envelope, body string) {
	back, topic, tag := route(e)
	if err := as.post("/send", protocol.Envelope{To: back, Topic: topic, Tag: tag, Body: body}, nil); err != nil {
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

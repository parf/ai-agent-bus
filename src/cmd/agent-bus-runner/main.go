// Command agent-bus-runner is the runner: its person's agent on the
// bus, #runner/<person>@<realm>, that serves the agents installed under
// agent.d as the host configured them under runner/
// (Plans/R1.0-Release/runner.md#runner). It has no bus identity of its
// own and never holds its person's token: it is started with its own agent's
// token, creates each agent it serves for its person, and keeps the token the
// creating register answered (runner M.10,
// Plans/R1.0-Release/runner-implementation.md#who-it-runs-as).
//
// It registers, reads what is installed, answers the `status` and `reload`
// calls (M.1), and serves each installed agent of a one-message form (M.2,
// serve.go); the rest come in later waves
// (Plans/R1.0-Release/TODO.md#runner-waves).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/parf/ai-agent-bus/internal/agentmeta"
	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/cgroup"
	"github.com/parf/ai-agent-bus/internal/display"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/version"
)

var usage = display.ManText(runnerUsage)

const runnerUsage = ` — serve installed agents · ` + display.Homepage + `

USAGE
  agent-bus-runner --host=<directory> [--agents=<directory>] [--bundled=<directory>]
  agent-bus-runner -h, --help | --version

DESCRIPTION
  Run a person's installed agents on the bus. --host is the runner
  account's private directory; --agents holds shared packages. The runner reads
  agent.d/*/agent.json and runner/agents.json. Its token belongs to that
  person's runner agent, #runner/<person>@<realm>, never to the person.
  Control it over the bus: status reports what is served; reload rereads
  packages and settings. Start, stop and restart last until the next reload;
  enable and disable persist in runner/agents.json. Read and write manage host
  files by relative path; create and delete take an <agent>/<instance> name.

OPTIONS
  --host=directory — runner account's private directory (required)
  --agents=directory — shared agent packages (default /var/lib/agent-bus/agent.d)
  --bundled=directory — bundled packages; opt-in through agents.json

GLOBAL OPTIONS
  -h, --help — show program help; use alone
  --version — print version and build; use alone

BUS COMMANDS
  status — show installed agents
  reload — reread packages and host settings
  start|stop|restart <agent> — control until the next reload
  enable|disable <agent> — persist whether an agent runs
  read <path> — read a host settings file
  write <path> — write a host file from the following body lines
  create|delete <agent>/<instance> — manage instances

NAMES
  The token belongs to #runner/<person>@<realm>, never to the person.

ENVIRONMENT
  AGENT_BUS_TOKEN — runner agent credential
  AGENT_BUS_ADDR — daemon address (or local socket discovery)
  AGENT_BUS_TLS_FINGERPRINT — HTTPS certificate pin

EXIT STATUS
  0 — success or graceful stop
  1 — missing host, refusal or runtime failure
  2 — invalid command-line option

EXAMPLES
  agent-bus-runner --host=/var/lib/agent-bus/parf-ab-runner
  agent-bus call '#runner/parf@demo' status

SEE ALSO
  ` + display.SourceURL + `/docs/runner/configuration.md`

func main() {
	if version.Print() {
		return
	}
	bundled := flag.String("bundled", defaultBundledDirectory(), "the bundled agent directory (opt-in)")
	agents := flag.String("agents", "/var/lib/agent-bus/agent.d", "the shared directory of agent code")
	host := flag.String("host", "", "the runner account's private directory (required)")
	flag.Usage = func() { fmt.Fprintln(os.Stderr, "agent-bus-runner "+version.String+usage) }
	flag.Parse()
	if *host == "" {
		fmt.Fprintln(os.Stderr, "agent-bus-runner: --host wants the runner account's private directory")
		os.Exit(1)
	}
	r := &runner{
		startedAt: time.Now().UTC(),
		agents:    *agents,
		bundled:   *bundled,
		host:      *host,
		say:       os.Stderr,
	}
	r.addr = os.Getenv("AGENT_BUS_ADDR")
	if r.addr == "" {
		r.addr = api.ClientSocket()
	}
	r.bus = newClient(r.addr)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := r.run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "agent-bus-runner:", err)
		os.Exit(1)
	}
}

type runner struct {
	startedAt time.Time // immutable process start observation, not a deployment timestamp
	name      string    // the runner's own agent, whoever its token is
	person    string    // the User that agent is — and so every agent it serves — owned by
	realm     string
	agents    string // shared agent.d
	bundled   string // bundled agents; opt-in
	host      string // runner/
	say       io.Writer
	bus       *client // as the runner's agent
	addr      string  // where the bus is: a socket path, or an address

	reloadWaitMu sync.Mutex               // serialize reload answers without holding other controls
	reloadMu     sync.Mutex               // a reload may drop mu while the same key retires
	retiring     map[string]chan struct{} // guarded by mu; cgroup identity is the installed key
	mu           sync.Mutex
	ctx          context.Context
	installed    []installed
	loadErr      string // a load that failed as a whole: agents.json unreadable
	served       map[string]*served
	stopped      map[string]bool // installed keys a stop call took down; guarded by reloadMu

	probe     sync.Once // whether this host can sandbox, asked once
	noSandbox error
	cgOnce    sync.Once // the delegated cgroup subtree, taken once
	cg        *cgroup.Tree
	cgErr     error
	filterOne sync.Once // the system-call filter, compiled once
	filter    []byte
	filterErr error
}

// run learns who the runner is, loads what is installed, and answers calls
// until ctx ends.
func (r *runner) run(ctx context.Context) error {
	r.ctx, r.served = ctx, map[string]*served{}
	if err := r.become(); err != nil {
		return err
	}
	r.reload()
	fmt.Fprintf(r.say, "%s is the runner (%s), %s\n", r.name, version.String, r.summary())
	const poll = 55 * time.Second
	for {
		// The runner's own inbox: each agent's is read as that agent (serve.go).
		e, got, err := r.bus.next(ctx, poll)
		if ctx.Err() != nil {
			fmt.Fprintf(r.say, "%s: stopping\n", r.name)
			r.stopKept()
			return nil
		}
		if err != nil {
			// The daemon being away is not the runner failing: wait, read again.
			fmt.Fprintf(r.say, "%s: %v; reading again\n", r.name, err)
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				r.stopKept()
				return nil
			}
			continue
		}
		if got {
			r.handle(e)
		}
	}
}

// become learns who the runner is from its token, and refuses to be anything
// but an agent: a User's token is its person's own, which the runner never
// holds, and a runner holding it could act as them everywhere (runner M.10).
// The record and its ACL are its person's, set when it was created.
func (r *runner) become() error {
	who, err := r.bus.whoami()
	if err != nil {
		return fmt.Errorf("who am I: %w", err)
	}
	if !protocol.IsAgentName(who) {
		return fmt.Errorf("%s is a person, not a runner: start it with the token of its person's agent, #runner/<person>@<realm>, never the person's own", who)
	}
	r.name = who
	if n, err := protocol.ParseName(r.name); err == nil {
		r.realm = n.Realm
	}
	owner, err := r.bus.ownerOf(r.name)
	if err != nil {
		return fmt.Errorf("whose runner %s is: %w", r.name, err)
	}
	r.person = owner
	return nil
}

// reload rereads what is installed, and forgets what stop calls took down. It
// never fails as a call: a bad file is a refused agent, or a load error, said
// in status and in the log by name.
func (r *runner) reload() {
	r.reloadMu.Lock()
	defer r.reloadMu.Unlock()
	r.stopped = nil
	r.apply()
}

// apply rereads what is installed and serves it, keeping what stop calls took
// down stopped. It holds reloadMu.
func (r *runner) apply() {
	list, err := loadSources(r.sources(), r.host)
	for i := range list {
		list[i].Stopped = r.stopped[list[i].Key]
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.installed, r.loadErr = list, ""
	if err != nil {
		r.installed, r.loadErr = nil, err.Error()
		fmt.Fprintf(r.say, "%s: nothing loaded: %v\n", r.name, err)
	}
	r.serve(r.ctx)
	for _, in := range r.installed {
		if in.Refused != "" {
			fmt.Fprintf(r.say, "%s: %s is refused: %s\n", r.name, in.Key, in.Refused)
		}
	}
}

func (r *runner) summary() string {
	st := r.status()
	refused := 0
	for _, a := range st.Agents {
		if a.State == "refused" {
			refused++
		}
	}
	return fmt.Sprintf("%d installed, %d refused", len(st.Agents), refused)
}

// The answer to `status`, as approved
// (Plans/R1.0-Release/runner-config.md#file-formats).
type statusAnswer struct {
	StartedAt *time.Time    `json:"started_at,omitempty"`
	AgentDir  string        `json:"agent_dir,omitempty"`
	RunnerDir string        `json:"runner_dir,omitempty"`
	Runner    string        `json:"runner"`
	Version   string        `json:"version"`
	Error     string        `json:"error,omitempty"`
	Agents    []agentStatus `json:"agents"`
}

type agentStatus struct {
	AgentDir  string        `json:"agent_dir,omitempty"`
	Serves    string        `json:"serves,omitempty"`
	Sandbox   string        `json:"sandbox,omitempty"`
	Limits    *statusLimits `json:"limits,omitempty"`
	StartedAt *time.Time    `json:"started_at,omitempty"`
	LastExit  *exitStatus   `json:"last_exit,omitempty"`
	Name      string        `json:"name"`
	Enabled   bool          `json:"enabled"`
	State     string        `json:"state"`
	Processes *processes    `json:"processes,omitempty"`
	Reason    string        `json:"reason,omitempty"`
	Key       string        `json:"key,omitempty"` // its agents.json key, which names its files
}

type processes struct {
	Running int `json:"running"`
	Min     int `json:"min"`
	Max     int `json:"max"`
}

func (r *runner) status() statusAnswer {
	r.mu.Lock()
	defer r.mu.Unlock()
	st := statusAnswer{AgentDir: r.agents, RunnerDir: r.host, Runner: r.name, Version: version.String, Error: r.loadErr, Agents: []agentStatus{}}
	if !r.startedAt.IsZero() {
		st.StartedAt = &r.startedAt
	}
	for _, in := range r.installed {
		a := agentStatus{AgentDir: r.codeDir(in), Name: r.agentName(in), Key: in.Key, Enabled: in.enabled(), LastExit: in.LastExit}
		switch in.Agent.Serves {
		case "", "runner":
			if in.Agent.Command != "" {
				a.Serves = "runner"
			}
		case "itself":
			a.Serves = "itself"
		}
		if s := r.served[a.Name]; s != nil {
			a.Sandbox = "off"
			if s.box.Name() != "off" {
				a.Sandbox = "on"
				a.Limits = &statusLimits{MemoryBytes: s.jail.tmp, Processes: s.jail.processes, DiskBytes: s.jail.disk}
			}
		}
		switch {
		case in.Unconfigured:
			a.State, a.Reason = "unconfigured", in.Refused
		case in.Refused != "":
			a.State, a.Reason = "refused", in.Refused
		case !a.Enabled:
			a.State = "disabled"
			if in.Bundled && in.Host.Enabled == nil {
				a.State = "available"
			}
		case in.Stopped:
			a.State, a.Reason = "stopped", "stopped by a call; start or reload serves it again"
		default:
			a.State = "stopped"
			running := 0
			if r.retiring[in.Key] != nil {
				a.State = "starting"
			} else if s, ok := r.served[a.Name]; ok && s.kept != nil {
				running, a.StartedAt = s.kept.snapshot()
				a.State = "running"
				if running == 0 {
					a.State = "starting"
				}
			} else if ok {
				running = int(s.busy.Load())
				a.State = "idle"
				if running > 0 {
					a.State = "running"
				}
			}
			a.Processes = &processes{Running: running, Min: in.Span.Min, Max: in.Span.Max}
		}
		st.Agents = append(st.Agents, a)
	}
	return st
}

// agentName is the host's complete name, or the directory and instance in
// the runner's own realm.
func (r *runner) agentName(in installed) string {
	if in.Host.Name != "" {
		return in.Host.Name
	}
	n := "#" + in.Key
	if r.realm != "" {
		n += "@" + r.realm
	}
	return n
}

// help is the runner's self-description, its answer to an ask for help
// (docs/constitution.md#agent-self-description).
func (r *runner) help(manage bool) string {
	help := "# " + r.name + `: a person's agent runner

Serves the agents installed in the shared agent.d, each under its own name
and sandbox, and answers calls about them. Its ACL admits status; other
operations require its Owner or Maintainers.

- status
  Answers JSON: the runner's name and version, and each installed agent with
  whether it is enabled, its state (idle, running, refused with a reason) and
  its processes running against their minimum and maximum.
  Example: agent-bus call '` + r.name + `' status
- reload
  Waits up to 15 seconds for replacement kept readers; unfinished replacements
  are reported as starting.
  Rereads what is installed, starting what was added or enabled, stopping
  what was removed, and restarting an agent that serves itself when its
  command, package content or environment changed; then answers as status does. It also serves
  again what a stop call took down.
The calls below are for the runner's Owner and Maintainers only.
- start <agent>, stop <agent>, restart <agent>
  Serves the agent, stops serving it, or both in turn, until the next reload
  or the runner's restart. The agent is its name or its agents.json key. A
  message already being run finishes; what waits stays in its inbox. Answers
  as status does.
  Example: agent-bus call '` + r.name + `' 'restart #hello@host'
- enable <agent>, disable <agent>
  Writes enabled true or false on its runner/agents.json entry, which lasts,
  then applies it. Answers as status does.
- read <file>
  Answers JSON: the file and whether it exists. Host env files give only
  sha256, size and variable names, never values; other files give content. A file is
  agent.d/<agent>/agent.json or env.dist (the author's, read only),
  runner/agents.json, or runner/<agent>[/<instance>]/env or agent.json (an
  override of processes, idle, deadline and depends).
  Example: agent-bus call '` + r.name + `' 'read runner/mail/work/env'
- write <file>
  The rest of the message, after its first line, replaces one of the host's
  files whole, checked first as the runner reads it; then applied. Answers as
  status does.
- create <agent>/<instance>, delete <agent>/<instance>
  Adds an instance, its agents.json entry and an empty env, or removes them
  with its overrides; its work, kept token and bus record stay. Answers as
  status does.`
	if !manage {
		before, _, _ := strings.Cut(help, "\n- reload\n")
		return before
	}
	return help
}

// handle answers one call. A receipt is news, not a call, and is not
// answered.
func (r *runner) handle(e protocol.Envelope) {
	if e.Receipt != "" {
		return
	}
	if e.Ask != "" {
		body, _, err := agentmeta.DispatchAsk(e.Ask, map[string]agentmeta.AskHandler{
			"help":        func() (string, error) { return r.help(r.mayManage(e)), nil },
			"help-config": func() (string, error) { return "none", nil },
			"help-secret": func() (string, error) { return "none", nil },
			"mcp":         func() (string, error) { return `{"tools":[]}`, nil },
			"status":      func() (string, error) { return statusLine(r.status()), nil },
		})
		if err != nil {
			body = "error: " + err.Error()
		}
		r.answer(e, body)
		return
	}
	var answer string
	switch body := strings.TrimSpace(e.Body); {
	case body == "status":
		answer = string(mustJSON(r.status()))
	case managing[verbOf(body)] && !r.mayManage(e):
		answer = fmt.Sprintf("error: %s is for the runner's Owner and Maintainers: it reaches the host's files and their secrets", verbOf(body))
	case body == "reload":
		go func() {
			parent := r.ctx
			if parent == nil {
				parent = context.Background()
			}
			if !e.Deadline.IsZero() {
				var cancel context.CancelFunc
				parent, cancel = context.WithDeadline(parent, e.Deadline)
				defer cancel()
			}
			r.answer(e, string(mustJSON(r.reloadStatus(parent, 15*time.Second))))
		}()
		return
	case verbOf(body) == "read":
		if f, err := r.readFile(strings.TrimSpace(strings.TrimPrefix(body, "read"))); err != nil {
			answer = "error: " + err.Error()
		} else {
			answer = string(mustJSON(f))
		}
	case verbOf(body) == "write":
		// The content is everything after the first line, exactly as sent.
		first, content, _ := strings.Cut(strings.TrimLeft(e.Body, " \t\r\n"), "\n")
		if err := r.writeHostFile(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(first), "write")), content); err != nil {
			answer = "error: " + err.Error()
		} else {
			answer = string(mustJSON(r.status()))
		}
	case verbOf(body) == "create" || verbOf(body) == "delete":
		args := strings.Fields(body)
		var err error
		switch {
		case len(args) != 2:
			err = fmt.Errorf("%s takes one instance: %s <agent>/<instance>", args[0], args[0])
		case args[0] == "create":
			err = r.createInstance(args[1])
		default:
			err = r.deleteInstance(args[1])
		}
		if err != nil {
			answer = "error: " + err.Error()
		} else {
			answer = string(mustJSON(r.status()))
		}
	case controls[verbOf(body)]:
		args := strings.Fields(body)
		if len(args) != 2 {
			answer = fmt.Sprintf("error: %s takes one agent: %s <agent>", args[0], args[0])
		} else if err := r.control(args[0], args[1]); err != nil {
			answer = "error: " + err.Error()
		} else {
			answer = string(mustJSON(r.status()))
		}
	default:
		answer = fmt.Sprintf("error: %s answers status, reload, start, stop, restart, enable, disable, read, write, create and delete, not %q", r.name, strings.TrimSpace(e.Body))
	}
	r.answer(e, answer)
}

func (r *runner) answer(e protocol.Envelope, answer string) {
	back, topic, tag := e.From, e.Topic, e.Tag
	if e.ReplyTo != nil {
		back, topic, tag = e.ReplyTo.Name, e.ReplyTo.Topic, e.ReplyTo.Tag
	}
	if err := r.bus.post("/send", protocol.Envelope{To: back, Topic: topic, Tag: tag, Body: answer}, nil); err != nil {
		fmt.Fprintf(r.say, "%s: could not answer %s: %v\n", r.name, e.ID, err)
	}
}

// controls are the calls that take one agent (control.go).
var controls = map[string]bool{"start": true, "stop": true, "restart": true, "enable": true, "disable": true}

// managing are the calls only the runner's Owner and Maintainers make: they
// change what runs, or reach the host's files and the secrets in them. status
// stays with whoever the runner's ACL admits.
var managing = map[string]bool{"reload": true, "start": true, "stop": true, "restart": true, "enable": true, "disable": true,
	"read": true, "write": true, "create": true, "delete": true}

// mayManage is whether the sender is the runner's Owner or one of its
// Maintainers, as the daemon worked out at the send (docs/03-records-agent.md):
// only toward the runner itself, never a role carried in from a record that
// forwarded here.
func (r *runner) mayManage(e protocol.Envelope) bool {
	if e.OriginalTo != "" && e.OriginalTo != r.name {
		return false
	}
	for _, role := range e.Roles {
		if role == "owner" || role == "maintainer" {
			return true
		}
	}
	return false
}

func verbOf(body string) string {
	if f := strings.Fields(body); len(f) > 0 {
		return f[0]
	}
	return ""
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// client is the runner's half of the HTTP API, over its account socket.
type client struct {
	http  *http.Client
	base  string
	token string
}

func newClient(addr string) *client {
	if addr == "" {
		addr = api.ClientSocket()
	}
	c, base := api.Dial(addr)
	// The runner is its person's agent wherever it connects, so its token is
	// always stated: an account socket alone would make it the account's User.
	return &client{http: c, base: base, token: os.Getenv("AGENT_BUS_TOKEN")}
}

func (c *client) do(ctx context.Context, method, path string, q url.Values, body any) ([]byte, int, error) {
	return c.doWith(ctx, method, path, q, body, nil)
}

func (c *client) doWith(ctx context.Context, method, path string, q url.Values, body any, header map[string]string) ([]byte, int, error) {
	var buf io.Reader
	if body != nil {
		buf = bytes.NewReader(mustJSON(body))
	}
	u := c.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, buf)
	if err != nil {
		return nil, 0, err
	}
	if c.token != "" {
		req.Header.Set(api.HeaderToken, c.token)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	return out, resp.StatusCode, err
}

// as is the same client speaking with another token: an agent's, to answer
// as that agent on the same socket.
func (c *client) as(token string) *client {
	cc := *c
	cc.token = token
	return &cc
}

func (c *client) post(path string, body, into any) error {
	return c.postContext(context.Background(), path, body, into)
}

func (c *client) postContext(parent context.Context, path string, body, into any) error {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	out, code, err := c.do(ctx, "POST", path, nil, body)
	if err != nil {
		return err
	}
	if code >= 400 {
		return errors.New(strings.TrimSpace(string(out)))
	}
	if into != nil {
		return json.Unmarshal(out, into)
	}
	return nil
}

// whoami is the name the client's token is.
func (c *client) whoami() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, code, err := c.do(ctx, "GET", "/status", nil, nil)
	if err != nil {
		return "", err
	}
	if code >= 400 {
		return "", errors.New(strings.TrimSpace(string(out)))
	}
	var st struct {
		You string `json:"you"`
	}
	if err := json.Unmarshal(out, &st); err != nil || st.You == "" {
		return "", fmt.Errorf("the daemon did not say: %s", strings.TrimSpace(string(out)))
	}
	return st.You, nil
}

// ownerOf is who owns a record the client may see.
func (c *client) ownerOf(name string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, code, err := c.do(ctx, "GET", "/lookup", url.Values{"name": {name}}, nil)
	if err != nil {
		return "", err
	}
	if code >= 400 {
		return "", errors.New(strings.TrimSpace(string(out)))
	}
	var rec struct {
		Owner string `json:"owner"`
	}
	if err := json.Unmarshal(out, &rec); err != nil || rec.Owner == "" {
		return "", fmt.Errorf("the daemon did not say whose %s is", name)
	}
	return rec.Owner, nil
}

// next waits up to poll for one message on the client's own inbox.
func (c *client) next(ctx context.Context, poll time.Duration) (protocol.Envelope, bool, error) {
	var e protocol.Envelope
	read, cancel := context.WithTimeout(ctx, poll+15*time.Second)
	defer cancel()
	out, code, err := c.do(read, "GET", "/consume", url.Values{"wait": {poll.String()}}, nil)
	switch {
	case err != nil:
		return e, false, err
	case code == http.StatusNoContent:
		return e, false, nil
	case code >= 400:
		return e, false, errors.New(strings.TrimSpace(string(out)))
	}
	if err := json.Unmarshal(out, &e); err != nil {
		return e, false, fmt.Errorf("not an envelope: %w", err)
	}
	return e, true, nil
}

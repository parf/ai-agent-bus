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
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/version"
)

const usage = ` — the runner

  AGENT_BUS_TOKEN=<token> agent-bus-runner [--home /var/lib/agent-bus]

The token is its person's agent's, #runner/<person>@<realm>, never the
person's own; the runner is whoever that token is. It reads
agent.d/*/agent.json and runner/agents.json, serves each agent installed
there as its person's, and answers two calls: status, and reload, which
rereads them.
  agent-bus call '#runner/<person>@<realm>' status
See Plans/R1.0-Release/runner.md#runner.`

func main() {
	if version.Print() {
		return
	}
	home := flag.String("home", "/var/lib/agent-bus", "the tree holding agent.d and runner")
	flag.Usage = func() { fmt.Fprintln(os.Stderr, "agent-bus-runner "+version.String+usage) }
	flag.Parse()
	r := &runner{
		agents: filepath.Join(*home, "agent.d"),
		host:   filepath.Join(*home, "runner"),
		say:    os.Stderr,
		bus:    newClient(os.Getenv("AGENT_BUS_ADDR")),
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := r.run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "agent-bus-runner:", err)
		os.Exit(1)
	}
}

type runner struct {
	name   string // the runner's own agent, whoever its token is
	realm  string
	agents string // agent.d
	host   string // runner/
	say    io.Writer
	bus    *client // as the runner's agent

	mu        sync.Mutex
	ctx       context.Context
	installed []installed
	loadErr   string // a load that failed as a whole: agents.json unreadable
	served    map[string]*served

	probe     sync.Once // whether this host can sandbox, asked once
	noSandbox error
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
			return nil
		}
		if err != nil {
			// The daemon being away is not the runner failing: wait, read again.
			fmt.Fprintf(r.say, "%s: %v; reading again\n", r.name, err)
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
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
	return nil
}

// reload rereads what is installed. It never fails as a call: a bad file is a
// refused agent, or a load error, said in status and in the log by name.
func (r *runner) reload() {
	list, err := load(r.agents, r.host)
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
	Runner  string        `json:"runner"`
	Version string        `json:"version"`
	Error   string        `json:"error,omitempty"`
	Agents  []agentStatus `json:"agents"`
}

type agentStatus struct {
	Name      string     `json:"name"`
	Enabled   bool       `json:"enabled"`
	State     string     `json:"state"`
	Processes *processes `json:"processes,omitempty"`
	Reason    string     `json:"reason,omitempty"`
}

type processes struct {
	Running int `json:"running"`
	Min     int `json:"min"`
	Max     int `json:"max"`
}

func (r *runner) status() statusAnswer {
	r.mu.Lock()
	defer r.mu.Unlock()
	st := statusAnswer{Runner: r.name, Version: version.String, Error: r.loadErr, Agents: []agentStatus{}}
	for _, in := range r.installed {
		a := agentStatus{Name: r.agentName(in), Enabled: in.enabled()}
		switch {
		case in.Refused != "":
			a.State, a.Reason = "refused", in.Refused
		case !a.Enabled:
			a.State = "disabled"
		default:
			a.State = "stopped"
			running := 0
			if s, ok := r.served[a.Name]; ok {
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

// handle answers one call. A receipt is news, not a call, and is not
// answered.
func (r *runner) handle(e protocol.Envelope) {
	if e.Receipt != "" {
		return
	}
	var answer string
	switch strings.TrimSpace(e.Body) {
	case "status":
		answer = string(mustJSON(r.status()))
	case "reload":
		r.reload()
		answer = string(mustJSON(r.status()))
	default:
		answer = fmt.Sprintf("error: %s answers status and reload, not %q", r.name, strings.TrimSpace(e.Body))
	}
	back, topic, tag := e.From, e.Topic, e.Tag
	if e.ReplyTo != nil {
		back, topic, tag = e.ReplyTo.Name, e.ReplyTo.Topic, e.ReplyTo.Tag
	}
	if err := r.bus.post("/send", protocol.Envelope{To: back, Topic: topic, Tag: tag, Body: answer}, nil); err != nil {
		fmt.Fprintf(r.say, "%s: could not answer %s: %v\n", r.name, e.ID, err)
	}
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

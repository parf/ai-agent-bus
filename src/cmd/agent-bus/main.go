// agent-bus: the CLI face. Every verb is one call to the daemon; no routing,
// no retry and no domain logic lives here. See docs/10-modules.md.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

const usage = `agent-bus — talk to agent-busd

  agent-bus status
  agent-bus token <name> [--rotate]   print that principal's token
  agent-bus register <name> [--kind k] [--addr a] [--descr d] [--overflow ring|strict]
                            [--allow a@b,c@d | --allow '*'] [--no-master]  who may see and use it
                            [--ttl 1h] [--bound 1000]  how long its queue keeps, and how much
                            [--protocol p]  how to call it; unset = this bus
  agent-bus ls [<name>] [--kind k]
  agent-bus send <to> [--topic t] [--tag g] [--reply-to name] [--ttl 30s] <text>
  agent-bus call <to> [--topic t] [--tag g] [--wait 30s] <text>
  agent-bus consume [--topic t] [--tag g] [--wait 30s] [--follow]
  agent-bus ack <message-id>
  agent-bus done <message-id>
  agent-bus reply <message-id> <text>
  agent-bus reply --to <name> [--topic t] [--tag g] <text>
  agent-bus topic create <name> [--kind queue|pubsub] [--descr d] [--overflow ring|strict]
                                [--ttl 1h] [--bound 1000]
  agent-bus publish --topic <name> <text>
  agent-bus start <name> --algo=std|args <script> [-N] [--descr d]
  agent-bus start                     (the same, as JSON on stdin)
  agent-bus service-template <name> -         configure it, JSON on stdin
  agent-bus service-template <name> '{"k":1}' the same, inline
  agent-bus service-template <name>           print that configuration
  agent-bus setup [--owner u@r] [--user account=u@r] [--addr a]
                  [--dry-run] [--print-unit]   install the service account and the unit

Environment: AGENT_BUS_NAME (user@realm), AGENT_BUS_TOKEN, AGENT_BUS_ADDR.
On your own socket the first two are supplied for you and can be left unset.`

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		die(usage)
	}
	verb, rest := args[0], args[1:]
	var err error
	switch verb {
	case "status":
		err = get("/status", nil)
	case "register":
		err = register(rest)
	case "token":
		err = tokenVerb(rest)
	case "ls":
		err = ls(rest)
	case "send":
		err = send(rest)
	case "call":
		err = callVerb(rest)
	case "consume":
		err = consume(rest)
	case "ack":
		err = receipt(protocol.ReceiptAck, rest)
	case "done":
		err = receipt(protocol.ReceiptDone, rest)
	case "topic":
		err = topic(rest)
	case "service-template":
		err = serviceTemplate(rest)
	case "publish":
		err = publish(rest)
	case "start":
		err = start(rest)
	case "setup":
		err = setup(rest)
	case "reply":
		err = reply(rest)
	case "help", "-h", "--help":
		fmt.Println(usage)
	default:
		die("unknown verb %q\n\n%s", verb, usage)
	}
	if err != nil {
		die("%v", err)
	}
}

// tokenFor asks the daemon for a principal's credential. The caller must own
// the name or be the daemon's owner; the refusal says which.
func tokenFor(name string) (string, error) { return getToken(name, false) }

func getToken(name string, rotate bool) (string, error) {
	out, code, err := call("POST", "/token", nil, map[string]any{"name": name, "rotate": rotate})
	if err != nil {
		return "", err
	}
	if code >= 400 {
		return "", fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	var got struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(out, &got); err != nil || got.Token == "" {
		return "", fmt.Errorf("no token in the answer: %s", strings.TrimSpace(string(out)))
	}
	return got.Token, nil
}

// tokenVerb gets a principal's credential and prints it alone, so
// `AGENT_BUS_TOKEN=$(agent-bus token me@host)` is the whole setup. Asking
// twice is a read; `--rotate` is how you ask for a new one.
// See docs/02-access.md#getting-a-token.
func tokenVerb(args []string) error {
	pos, flags := split(args)
	if len(pos) != 1 {
		return fmt.Errorf("token wants one name")
	}
	_, rotate := flags["rotate"]
	tok, err := getToken(pos[0], rotate)
	if err != nil {
		return err
	}
	fmt.Println(tok)
	return nil
}

func register(args []string) error {
	pos, flags := split(args)
	if len(pos) != 1 {
		return fmt.Errorf("register wants one name")
	}
	n, err := bound(flags)
	if err != nil {
		return err
	}
	return post("/register", protocol.Record{
		Name: pos[0], Kind: flags["kind"], Addr: flags["addr"], Descr: flags["descr"],
		Full: flags["overflow"], Proto: flags["protocol"],
		TTL: flags["ttl"], Bound: n,
		Allow: allow(flags), NoMaster: has(flags, "no-master"),
	})
}

// ls lists the registry, or answers about one name when given one. Asking
// about a single service does not pull the whole registry down for it, and
// what comes back carries the digest of that service's configuration rather
// than the configuration itself.
// See docs/03-services-and-topics.md#configuring-a-template.
func ls(args []string) error {
	pos, flags := split(args)
	if len(pos) > 1 {
		return fmt.Errorf("ls takes one name, or none")
	}
	q := url.Values{}
	if len(pos) == 1 {
		q.Set("name", pos[0])
		return get("/lookup", q)
	}
	if k := flags["kind"]; k != "" {
		q.Set("kind", k)
	}
	return get("/ls", q)
}

func send(args []string) error {
	pos, flags := split(args)
	if len(pos) < 2 {
		return fmt.Errorf("send wants <to> and text")
	}
	e := protocol.Envelope{
		To: pos[0], Topic: flags["topic"], Tag: flags["tag"],
		Body: strings.Join(pos[1:], " "), TTL: flags["ttl"],
	}
	// --reply-to keeps the exchange's topic and tag unless told otherwise:
	// the third party matches the answer the same way the sender would.
	if back := flags["reply-to"]; back != "" {
		e.ReplyTo = &protocol.ReplyTo{Service: back, Topic: e.Topic, Tag: e.Tag}
	}
	return post("/send", e)
}

// call is send plus the wait for its answer. There is no third verb on the
// wire and no dispatcher here: the daemon serves a filtered consume ahead of
// the unfiltered reader (docs/04-messaging.md#request-and-reply).
//
// That priority holds only while a wait is outstanding, and a receipt is an
// ordinary message that ends one — the wait reports it and asks again. So on
// an inbox a push adapter also reads, the answer can go to that reader
// instead: call from a name of its own when the answer matters
// (docs/04-messaging.md#one-reader-per-inbox).
func callVerb(args []string) error {
	pos, flags := split(args)
	if len(pos) < 2 {
		return fmt.Errorf("call wants <to> and text")
	}
	topic, tag := flags["topic"], flags["tag"]
	if topic == "" {
		topic = "call"
	}
	if tag == "" {
		tag = newTag() // unique, so nothing else answers this wait
	}
	// Everything that can be refused is refused before anything is sent: once
	// the message is accepted the work may already be happening, and a
	// validation error after that reads as "nothing happened".
	deadline := 30 * time.Second
	if v := flags["wait"]; v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("bad --wait: %v", err)
		}
		if d <= 0 {
			return fmt.Errorf("--wait must be positive, not %s", d)
		}
		deadline = d
	}
	// A caller that wants an answer needs an address for it to arrive at.
	// Registration is a record you state (docs/01-identity.md#registration),
	// and this is the caller stating it — but only if it has none, since
	// re-stating it here would overwrite a description its owner meant.
	me := whoami()
	if known, err := registered(me); err != nil {
		return err
	} else if !known {
		if err := postQuiet("/register", protocol.Record{Name: me, Kind: "agent"}); err != nil {
			return fmt.Errorf("could not register as %s: %w", me, err)
		}
	}
	sent, code, err := call("POST", "/send", nil, protocol.Envelope{
		To: pos[0], Topic: topic, Tag: tag, Body: strings.Join(pos[1:], " "),
	})
	if err != nil {
		return err
	}
	if code >= 400 {
		return fmt.Errorf("%s", strings.TrimSpace(string(sent)))
	}

	// --wait is the caller's deadline, so it bounds the HTTP exchange too, not
	// only the wait the daemon is asked for: a slow or hung transfer would
	// otherwise run on to the client's generic timeout.
	until := time.Now().Add(deadline)
	ctx, cancel := context.WithDeadline(context.Background(), until)
	defer cancel()
	tooLate := fmt.Errorf("no answer within %s (the message was accepted; do not resend it)", deadline)
	errFinished := errors.New("the service finished and sent no answer (not a timeout; do not resend it)")

	q := url.Values{"topic": {topic}, "tag": {tag}}
	for {
		left := time.Until(until)
		if left <= 0 {
			return tooLate
		}
		q.Set("wait", left.String()) // not rounded: rounding the last half
		//                              second down to 0s spins on the daemon
		e, body, got, err := next(ctx, q)
		if err != nil {
			if ctx.Err() != nil {
				return tooLate
			}
			return err
		}
		if !got {
			continue
		}
		if e.Receipt != "" {
			fmt.Fprintf(os.Stderr, "%s from %s\n", e.Receipt, e.From)
			// A receipt is not the answer — but `done` says no answer is
			// coming, so waiting on past it only spends the deadline.
			// See docs/04-messaging.md#receipts.
			if e.Receipt == protocol.ReceiptDone {
				return errFinished
			}
			continue
		}
		os.Stdout.Write(body)
		return nil
	}
}

// receipt says something back about a message this client consumed: `ack`
// got it, `done` finished it. Both are ordinary messages on the same topic
// and tag, naming the one they are about, so one function serves both verbs
// and the closed set stays closed by construction.
// See docs/04-messaging.md#receipts.
func receipt(kind string, args []string) error {
	pos, _ := split(args)
	if len(pos) != 1 {
		return fmt.Errorf("%s wants one message-id", kind)
	}
	c, found := recall(pos[0])
	if !found {
		return fmt.Errorf("message %s is not one this client consumed", pos[0])
	}
	return post("/send", protocol.Envelope{
		To: c.From, Topic: c.Topic, Tag: c.Tag, Receipt: kind, Re: c.ID,
	})
}

// bound reads --bound, which is a count of messages and not a duration: the
// two flags sit next to each other and a typo in either should say which.
func bound(flags map[string]string) (int, error) {
	v := flags["bound"]
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("a bound is a positive number of messages, not %q", v)
	}
	return n, nil
}

// A topic is a record like any other; `topic create` is the sugar that says
// so. See docs/03-services-and-topics.md.
// serviceTemplate configures a service template into a configured service,
// and reads that configuration back. One verb, because the direction is
// obvious from whether a configuration was handed to it — and a hyphenated
// single word, because a two-word verb has no spelling in the MCP face,
// where a tool name is `[a-zA-Z0-9_-]{1,64}`.
//
// The configuration is arbitrary JSON and stays opaque: nothing here or in
// the daemon looks inside it.
// See docs/03-services-and-topics.md#configuring-a-template.
func serviceTemplate(args []string) error {
	pos, _ := split(args)
	if len(pos) == 0 || len(pos) > 2 {
		return fmt.Errorf("service-template wants a name, and a configuration to set one:\n" +
			"  cat cfg.json | agent-bus service-template <template/instance@host> -\n" +
			"  agent-bus service-template <template/instance@host> '{\"k\":\"v\"}'\n" +
			"  agent-bus service-template <template/instance@host>")
	}
	name := pos[0]
	if len(pos) == 1 {
		// No configuration named: print the one that is there. There is no
		// guard here against forgetting the "-" — stdin is not a terminal in
		// a script, in CI, or in the service reading its own config at
		// start, which is most of the times this is called.
		q := url.Values{}
		q.Set("name", name)
		return get("/config", q)
	}
	raw := []byte(pos[1])
	if pos[1] == "-" {
		var err error
		if raw, err = io.ReadAll(os.Stdin); err != nil {
			return fmt.Errorf("reading the configuration from stdin: %w", err)
		}
	}
	if !json.Valid(raw) {
		return fmt.Errorf("a configuration is JSON, and this is not")
	}
	return post("/configure", struct {
		Name   string          `json:"name"`
		Config json.RawMessage `json:"config"`
	}{name, json.RawMessage(raw)})
}

func topic(args []string) error {
	if len(args) == 0 || args[0] != "create" {
		return fmt.Errorf("the only topic verb is: topic create <name> [--kind queue|pubsub]")
	}
	pos, flags := split(args[1:])
	if len(pos) != 1 {
		return fmt.Errorf("topic create wants one name")
	}
	mode := flags["kind"]
	if mode == "" {
		mode = protocol.ModeQueue
	}
	if mode != protocol.ModeQueue && mode != protocol.ModePubSub {
		return fmt.Errorf("a topic is %s or %s", protocol.ModeQueue, protocol.ModePubSub)
	}
	n, err := bound(flags)
	if err != nil {
		return err
	}
	return post("/register", protocol.Record{
		Name: pos[0], Kind: protocol.KindTopic, Mode: mode, Descr: flags["descr"],
		Full: flags["overflow"], TTL: flags["ttl"], Bound: n,
		Allow: allow(flags), NoMaster: has(flags, "no-master"),
	})
}

// publish is a send to a topic. A publisher need not be a registered service
// — it only needs a name and a token. See docs/12-stages.md#poc.
func publish(args []string) error {
	pos, flags := split(args)
	if flags["topic"] == "" || len(pos) == 0 {
		return fmt.Errorf("publish wants --topic <name> and text")
	}
	return post("/send", protocol.Envelope{
		To: flags["topic"], Topic: flags["topic"], Body: strings.Join(pos, " "),
	})
}

func consume(args []string) error {
	_, flags := split(args)
	q := url.Values{}
	for _, k := range []string{"topic", "tag", "wait"} {
		if v := flags[k]; v != "" {
			q.Set(k, v)
		}
	}
	_, follow := flags["follow"]
	for {
		e, body, got, err := next(context.Background(), q)
		if err != nil {
			return err
		}
		if !got {
			if follow {
				continue // long poll again
			}
			return nil
		}
		remember(e) // the client keeps the reply context, not the daemon
		os.Stdout.Write(body)
		if !follow {
			return nil
		}
	}
}

// reply is sugar: a send back to whoever sent the message, with topic and tag
// copied. The daemon keeps no reply state — this client resolves the id from
// what it consumed. See docs/04-messaging.md#reply-routing.
func reply(args []string) error {
	pos, flags := split(args)
	to, topic, tag := flags["to"], flags["topic"], flags["tag"]
	text := pos

	if to == "" {
		if len(pos) < 2 {
			return fmt.Errorf("reply wants <message-id> and text, or --to")
		}
		c, ok := recall(pos[0])
		if !ok {
			return fmt.Errorf("message %s is not one this client consumed; use --to --topic --tag", pos[0])
		}
		to, topic, tag, text = c.From, c.Topic, c.Tag, pos[1:]
	}
	if len(text) == 0 {
		return fmt.Errorf("reply wants text")
	}
	return post("/send", protocol.Envelope{To: to, Topic: topic, Tag: tag, Body: strings.Join(text, " ")})
}

// registered says whether a name already has a record. `ls` is the discovery
// verb and PoC lists are small, so this is one GET rather than an endpoint of
// its own.
func registered(name string) (bool, error) {
	if _, err := protocol.ParseName(name); err != nil {
		return false, err
	}
	q := url.Values{}
	q.Set("name", name)
	body, code, err := call("GET", "/lookup", q, nil)
	switch {
	case err != nil:
		return false, err
	case code == http.StatusNotFound:
		return false, nil
	case code >= 400:
		return false, fmt.Errorf("%s", strings.TrimSpace(string(body)))
	}
	return true, nil
}

func newTag() string {
	var b [6]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// ---- talking to the daemon -------------------------------------------------

func call(method, path string, q url.Values, body any) ([]byte, int, error) {
	return callCtx(context.Background(), method, path, q, body)
}

// callCtx is the same with a deadline the caller owns: `start` gives it the
// signal context so a long consume ends when the service is asked to stop.
func callCtx(ctx context.Context, method, path string, q url.Values, body any) ([]byte, int, error) {
	// On your own socket there is nothing to set: the daemon knows the
	// account at the other end and supplies both parameters. Everywhere
	// else they have to be sent. See docs/02-access.md#local-socket.
	name := os.Getenv("AGENT_BUS_NAME")
	token := os.Getenv("AGENT_BUS_TOKEN")
	if (name == "" || token == "") && !onOwnSocket() {
		return nil, 0, fmt.Errorf("set AGENT_BUS_NAME (user@realm) and AGENT_BUS_TOKEN")
	}
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		buf = bytes.NewReader(b)
	}
	client, base := transport()
	u := base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, buf)
	if err != nil {
		return nil, 0, err
	}
	if name != "" {
		req.Header.Set(api.HeaderUser, name)
	}
	if token != "" {
		req.Header.Set(api.HeaderToken, token)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	return out, resp.StatusCode, err
}

// whoami is this client's name: what was stated, or what the daemon says
// when the socket supplied it instead. Asked once — it cannot change under
// a running command. See docs/02-access.md#local-socket.
var whoami = sync.OnceValue(func() string {
	if n := os.Getenv("AGENT_BUS_NAME"); n != "" {
		return n
	}
	out, code, err := call("GET", "/status", nil, nil)
	if err != nil || code >= 400 {
		return ""
	}
	var got struct {
		You string `json:"you"`
	}
	json.Unmarshal(out, &got)
	return got.You
})

// onOwnSocket says whether we are talking over a socket the daemon opened
// for this account, which is the one place the two parameters come for free.
func onOwnSocket() bool {
	// Judged from the address, not from transport's base URL: that is
	// "http://localhost" over a unix socket too.
	addr := socketPath()
	return !strings.HasPrefix(addr, "http://") && api.IsUserSocket(addr)
}

func socketPath() string {
	if a := os.Getenv("AGENT_BUS_ADDR"); a != "" {
		return a
	}
	return api.DefaultSocket()
}

// transport speaks HTTP over either a unix socket or loopback TCP: same
// protocol on both listeners. Built once — `consume --follow` polls in a loop,
// and a fresh Transport each time would leak idle connections.
// See docs/decisions.md.
var transport = sync.OnceValues(func() (*http.Client, string) {
	return api.Dial(os.Getenv("AGENT_BUS_ADDR"))
})

func get(path string, q url.Values) error { return show(call("GET", path, q, nil)) }
func post(path string, body any) error    { return show(call("POST", path, nil, body)) }

// postQuiet is the same call without printing the answer: a long-running
// service writes messages, not JSON, to its stdout.
func postQuiet(path string, body any) error {
	out, code, err := call("POST", path, nil, body)
	if err != nil {
		return err
	}
	if code >= 400 {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func show(body []byte, code int, err error) error {
	if err != nil {
		return err
	}
	if code >= 400 {
		return fmt.Errorf("%s", strings.TrimSpace(string(body)))
	}
	os.Stdout.Write(body)
	return nil
}

// ---- the client's own memory of what it consumed ---------------------------
//
// The daemon keeps no reply state (docs/04-messaging.md#reply-routing), so the
// client that consumed a message keeps what it needs to answer: the routing
// fields, never the body. One file per message, so two filtered consumes
// running at once cannot overwrite each other.

// replyContext is where an answer goes, resolved once when the message is
// consumed. A request may point its answer at a third party, and the client
// that replies is the only thing that still holds the envelope — so the
// route is worked out here, not guessed later.
// See docs/04-messaging.md#reply-routing.
type replyContext struct {
	ID    string `json:"message_id"`
	From  string `json:"from"`
	Topic string `json:"topic,omitempty"`
	Tag   string `json:"tag,omitempty"`
}

// route is where a reply to this message belongs: the sender, unless the
// request named somewhere else.
func route(e protocol.Envelope) replyContext {
	c := replyContext{ID: e.ID, From: e.From, Topic: e.Topic, Tag: e.Tag}
	if e.ReplyTo != nil {
		c.From = e.ReplyTo.Service
		c.Topic, c.Tag = e.ReplyTo.Topic, e.ReplyTo.Tag
	}
	return c
}

const (
	replyContextTTL = 24 * time.Hour
	// sweepEvery bounds how often the directory scan runs. A context lives a
	// day, so an hour of slack costs nothing and keeps the scan off the
	// per-message path.
	sweepEvery  = time.Hour
	sweptMarker = ".swept"
)

func stateDir() string {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".cache")
	}
	return filepath.Join(dir, "agent-bus", safe(whoami()))
}

func remember(e protocol.Envelope) {
	dir := stateDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		warn("cannot keep reply context: %v", err)
		return
	}
	b, err := json.Marshal(route(e))
	if err != nil {
		warn("cannot keep reply context: %v", err)
		return
	}
	if err := os.WriteFile(filepath.Join(dir, e.ID+".json"), b, 0o600); err != nil {
		warn("cannot keep reply context for %s: %v — reply will need --to", e.ID, err)
		return
	}
	sweep(dir)
}

func recall(id string) (replyContext, bool) {
	var c replyContext
	b, err := os.ReadFile(filepath.Join(stateDir(), safe(id)+".json"))
	if err != nil || json.Unmarshal(b, &c) != nil {
		return replyContext{}, false
	}
	return c, true
}

// sweep drops contexts older than a day, so the directory does not grow
// forever. It reads the whole directory, which is not cheap — measured at
// 17ms and 5MB over 10,000 contexts — so it must not run per message. A
// marker file's mtime is the schedule, and every short-lived invocation
// shares it: the per-message cost becomes one stat.
func sweep(dir string) {
	marker := filepath.Join(dir, sweptMarker)
	if info, err := os.Stat(marker); err == nil && time.Since(info.ModTime()) < sweepEvery {
		return
	}
	if err := os.WriteFile(marker, nil, 0o600); err != nil {
		return // unwritable: skip rather than sweep on every message
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Name() == sweptMarker {
			continue
		}
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > replyContextTTL {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// safe keeps a name or id from escaping the cache directory.
var unsafe = strings.NewReplacer("/", "_", "\\", "_", "..", "_", "@", "-at-", " ", "_")

func safe(s string) string { return unsafe.Replace(s) }

func warn(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "agent-bus: "+format+"\n", a...)
}

// ---- tiny arg splitting ----------------------------------------------------

// split separates positional args from --flags. A flag with no value (--follow)
// is recorded as present and empty.
// Flags that are on or off. Without this the word after one is taken as its
// value, and `--follow consume` reads as follow="consume".
var onOff = map[string]bool{"follow": true, "no-master": true}

// allow is the service ACL as stated on the command line: a comma-separated
// list, `*` for anyone who can authenticate, absent for no answer of its own.
// See docs/01-identity.md#acl.
func allow(flags map[string]string) []string {
	v, ok := flags["allow"]
	if !ok || strings.TrimSpace(v) == "" {
		return nil
	}
	var out []string
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func has(flags map[string]string, k string) bool { _, ok := flags[k]; return ok }

func split(args []string) ([]string, map[string]string) {
	pos := []string{}
	flags := map[string]string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "--") {
			pos = append(pos, a)
			continue
		}
		k, v, hasEq := strings.Cut(strings.TrimPrefix(a, "--"), "=")
		switch {
		case hasEq:
			flags[k] = v
		case i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") && !onOff[k]:
			flags[k] = args[i+1]
			i++
		default:
			flags[k] = ""
		}
	}
	return pos, flags
}

func die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

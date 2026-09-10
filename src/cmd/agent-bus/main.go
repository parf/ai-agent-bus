// agent-bus: the CLI face. Every verb is one call to the daemon; no routing,
// no retry and no domain logic lives here. See docs/10-modules.md.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

const usage = `agent-bus — talk to agent-busd

  agent-bus status
  agent-bus register <name> [--kind k] [--addr a] [--descr d]
  agent-bus ls [--kind k]
  agent-bus send <to> [--topic t] [--tag g] <text>
  agent-bus call <to> [--topic t] [--tag g] [--wait 30s] <text>
  agent-bus consume [--topic t] [--tag g] [--wait 30s] [--follow]
  agent-bus ack <message-id>
  agent-bus reply <message-id> <text>
  agent-bus reply --to <name> [--topic t] [--tag g] <text>
  agent-bus topic create <name> [--kind queue|pubsub] [--descr d]
  agent-bus publish --topic <name> <text>
  agent-bus start <name> --algo=std|args <script> [-N] [--descr d]
  agent-bus start                     (the same, as JSON on stdin)

Environment: AGENT_BUS_NAME (user@realm), AGENT_BUS_TOKEN, AGENT_BUS_ADDR.`

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
	case "ls":
		err = ls(rest)
	case "send":
		err = send(rest)
	case "call":
		err = callVerb(rest)
	case "consume":
		err = consume(rest)
	case "ack":
		err = ack(rest)
	case "topic":
		err = topic(rest)
	case "publish":
		err = publish(rest)
	case "start":
		err = start(rest)
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

func register(args []string) error {
	pos, flags := split(args)
	if len(pos) != 1 {
		return fmt.Errorf("register wants one name")
	}
	return post("/register", protocol.Record{
		Name: pos[0], Kind: flags["kind"], Addr: flags["addr"], Descr: flags["descr"],
	})
}

func ls(args []string) error {
	_, flags := split(args)
	q := url.Values{}
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
	return post("/send", protocol.Envelope{
		To: pos[0], Topic: flags["topic"], Tag: flags["tag"],
		Body: strings.Join(pos[1:], " "),
	})
}

// call is send plus the wait for its answer. There is no third verb on the
// wire and no dispatcher here: the daemon serves a filtered consume ahead of
// the unfiltered reader, so the reply finds this caller
// (docs/04-messaging.md#request-and-reply).
//
// A receipt is an ordinary message on the same topic and tag, so the wait
// reports it and keeps waiting for the answer
// (docs/04-messaging.md#receipts).
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
	// A caller that wants an answer needs an address for it to arrive at.
	// Registration is a record you state (docs/01-identity.md#registration),
	// and this is the caller stating it.
	if err := postQuiet("/register", protocol.Record{
		Name: os.Getenv("AGENT_BUS_NAME"), Kind: "agent",
	}); err != nil {
		return fmt.Errorf("could not register as %s: %w", os.Getenv("AGENT_BUS_NAME"), err)
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

	deadline := 30 * time.Second
	if v := flags["wait"]; v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("bad --wait: %v", err)
		}
		deadline = d
	}
	until := time.Now().Add(deadline)
	q := url.Values{"topic": {topic}, "tag": {tag}}
	for {
		left := time.Until(until)
		if left <= 0 {
			return fmt.Errorf("no answer within %s (the message was accepted; do not resend it)", deadline)
		}
		q.Set("wait", left.Round(time.Second).String())
		body, code, err := call("GET", "/consume", q, nil)
		if err != nil {
			return err
		}
		if code == http.StatusNoContent {
			continue
		}
		if code >= 400 {
			return fmt.Errorf("%s", strings.TrimSpace(string(body)))
		}
		var e protocol.Envelope
		if err := json.Unmarshal(body, &e); err == nil && e.Receipt != "" {
			fmt.Fprintf(os.Stderr, "%s from %s\n", e.Receipt, e.From)
			continue // a receipt is not the answer
		}
		os.Stdout.Write(body)
		return nil
	}
}

// ack says "got it" back to whoever sent the message: an ordinary message on
// the same topic and tag, naming the one it is about.
// See docs/04-messaging.md#receipts.
func ack(args []string) error {
	pos, _ := split(args)
	if len(pos) != 1 {
		return fmt.Errorf("ack wants one message-id")
	}
	c, found := recall(pos[0])
	if !found {
		return fmt.Errorf("message %s is not one this client consumed", pos[0])
	}
	return post("/send", protocol.Envelope{
		To: c.From, Topic: c.Topic, Tag: c.Tag, Receipt: "ack", Re: c.ID,
	})
}

// A topic is a record like any other; `topic create` is the sugar that says
// so. See docs/03-services-and-topics.md.
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
	return post("/register", protocol.Record{
		Name: pos[0], Kind: protocol.KindTopic, Mode: mode, Descr: flags["descr"],
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
	for {
		body, code, err := call("GET", "/consume", q, nil)
		if err != nil {
			return err
		}
		if code == http.StatusNoContent {
			if _, follow := flags["follow"]; follow {
				continue // long poll again
			}
			return nil
		}
		if code >= 400 {
			return fmt.Errorf("%s", strings.TrimSpace(string(body)))
		}
		var e protocol.Envelope
		if err := json.Unmarshal(body, &e); err == nil {
			remember(e) // the client keeps the reply context, not the daemon
		}
		os.Stdout.Write(body)
		if _, follow := flags["follow"]; !follow {
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

func newTag() string {
	var b [6]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// ---- talking to the daemon -------------------------------------------------

func call(method, path string, q url.Values, body any) ([]byte, int, error) {
	name := os.Getenv("AGENT_BUS_NAME")
	token := os.Getenv("AGENT_BUS_TOKEN")
	if name == "" || token == "" {
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
	req, err := http.NewRequest(method, u, buf)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set(api.HeaderUser, name)
	req.Header.Set(api.HeaderToken, token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	return out, resp.StatusCode, err
}

// transport speaks HTTP over either a unix socket or loopback TCP: same
// protocol on both listeners. Built once — `consume --follow` polls in a loop,
// and a fresh Transport each time would leak idle connections.
// See docs/decisions.md.
var transport = sync.OnceValues(func() (*http.Client, string) {
	addr := os.Getenv("AGENT_BUS_ADDR")
	if addr == "" {
		addr = defaultSocket()
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	if strings.HasPrefix(addr, "http://") {
		return client, strings.TrimSuffix(addr, "/")
	}
	client.Transport = &http.Transport{
		IdleConnTimeout: 90 * time.Second,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", addr)
		},
	}
	return client, "http://unix"
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

type replyContext struct {
	ID    string `json:"message_id"`
	From  string `json:"from"`
	Topic string `json:"topic,omitempty"`
	Tag   string `json:"tag,omitempty"`
}

const replyContextTTL = 24 * time.Hour

func stateDir() string {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".cache")
	}
	return filepath.Join(dir, "agent-bus", safe(os.Getenv("AGENT_BUS_NAME")))
}

func remember(e protocol.Envelope) {
	dir := stateDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		warn("cannot keep reply context: %v", err)
		return
	}
	b, err := json.Marshal(replyContext{ID: e.ID, From: e.From, Topic: e.Topic, Tag: e.Tag})
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
// forever. Cheap enough to do on write.
func sweep(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > replyContextTTL {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// safe keeps a name or id from escaping the cache directory.
func safe(s string) string {
	return strings.NewReplacer("/", "_", "\\", "_", "..", "_", "@", "-at-", " ", "_").Replace(s)
}

func warn(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "agent-bus: "+format+"\n", a...)
}

// ---- tiny arg splitting ----------------------------------------------------

// split separates positional args from --flags. A flag with no value (--follow)
// is recorded as present and empty.
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
		case i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") && k != "follow":
			flags[k] = args[i+1]
			i++
		default:
			flags[k] = ""
		}
	}
	return pos, flags
}

func defaultSocket() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "agent-bus", "bus.sock")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("agent-bus-%d.sock", os.Getuid()))
}

func die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

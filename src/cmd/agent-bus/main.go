// agent-bus: the CLI face. Every verb is one call to the daemon; no routing,
// no retry and no domain logic lives here. See src/MODULES.md.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
	"unicode"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/display"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/tokencmd"
	"github.com/parf/ai-agent-bus/internal/version"
)

var cliAddress string

func main() {
	if version.Print() {
		return
	}
	args := os.Args[1:]
	// Global options precede the verb: register --service-addr is the service's
	// advertised address, not the address of the bus being contacted.
	if len(args) > 0 && (args[0] == "--addr" || strings.HasPrefix(args[0], "--addr=")) {
		if args[0] == "--addr" {
			if len(args) < 2 || strings.HasPrefix(args[1], "-") {
				die("--addr requires a socket path or HTTP URL")
			}
			cliAddress, args = args[1], args[2:]
		} else {
			cliAddress, args = strings.TrimPrefix(args[0], "--addr="), args[1:]
		}
		if cliAddress == "" {
			die("--addr requires a socket path or HTTP URL")
		}
	}
	if len(args) == 0 {
		die(usage)
	}
	verb, rest := args[0], args[1:]
	if len(rest) == 1 && (rest[0] == "--help" || verb == "token" && rest[0] == "-h") {
		if help := verbUsage(verb); help != "" {
			fmt.Println(help)
			return
		}
	}
	// A delimiter also ends the name shorthand; literal text stays untouched.
	end := len(rest)
	for i, arg := range rest {
		if arg == "--" {
			end = i
			break
		}
	}
	for i := 0; i+1 < end; i++ {
		if rest[i] == "--agent" && strings.HasPrefix(rest[i+1], "--") {
			die("--agent wants a name, not option %s", rest[i+1])
		}
	}
	rest = append(protocol.ExpandAgentFlags(rest[:end]), rest[end:]...)
	var err error
	switch verb {
	case "token":
		if cliAddress != "" {
			if err = os.Setenv("AGENT_BUS_ADDR", cliAddress); err != nil {
				break
			}
		}
		err = tokencmd.Run(rest)
	case "status":
		_, flags, parseErr := split(rest)
		if err = only(flags, parseErr); err == nil {
			err = get("/status", nil)
		}
	case "register":
		err = register(rest)
	case "unregister":
		pos, flags, parseErr := split(rest)
		if err = only(flags, parseErr); err != nil {
			break
		}
		if len(pos) != 1 {
			err = fmt.Errorf("unregister wants one record name")
		} else if err = postQuiet("/unregister", map[string]string{"name": pos[0]}); err == nil {
			fmt.Printf("%s unregistered\n", pos[0])
		}
	case "lock":
		err = lockTake(false)(rest)
	case "try-lock":
		err = lockTake(true)(rest)
	case "release":
		err = lockRelease(rest)
	case "extend":
		err = lockExtend(rest)
	case "holders":
		err = lockHolders(rest)
	case "kv":
		err = kvVerb(rest)
	case "ls":
		err = ls(rest)
	case "may-send":
		err = maySend(rest)
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
	case "channel":
		err = channel(rest)
	case "config", "agent-template":
		err = agentTemplate(rest)
	case "secret":
		err = secret(rest)
	case "publish":
		err = publish(rest)
	case "unsubscribe":
		err = unsubscribe(rest)
	case "run":
		err = serveScript(rest)
	case "stop":
		err = stopVerb(rest)
	case "logs":
		err = logsVerb(rest)
	case "manage":
		err = manage(rest)
	case "group":
		err = group(rest)
	case "enrol", "enroll":
		err = enrol(rest)
	case "reply":
		err = reply(rest)
	case "help":
		err = agentHelp(rest, os.Stdout)
	case "-h", "--help":
		_, flags, parseErr := split(rest)
		if err = only(flags, parseErr); err == nil {
			fmt.Println(usage)
		}
	case "--version", "-version":
		_, flags, parseErr := split(rest)
		if err = only(flags, parseErr); err == nil {
			err = fmt.Errorf("%s takes no arguments", verb)
		}
	default:
		die("unknown verb %q\n\n%s", verb, usage)
	}
	if err != nil {
		die("%v", err)
	}
}

// tokenFor asks the daemon for a principal's credential — `run` needs one
// for the agent it is about to run. Anyone asking for their own runs
// agent-bus token (docs/07-setup.md#the-programs). The caller must own
// the name or be the daemon's owner; the refusal says which.
func tokenFor(name string) (string, error) {
	out, code, err := call("POST", "/token", nil, map[string]any{"name": name, "rotate": false})
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

func register(args []string) error {
	pos, flags, err := split(args)
	if err := only(flags, err, "kind", "service-addr", "addr", "descr", "overflow", "personal", "allow", "ttl", "bound", "protocol",
		"uri", "template", "source", "mime", "title", "size", "mcp-name", "icons", "annotations", "tags"); err != nil {
		return err
	}
	if len(pos) != 1 {
		return fmt.Errorf("register wants one name")
	}
	if v, ok := flags["service-addr"]; ok {
		if old, exists := flags["addr"]; exists && old != v {
			return fmt.Errorf("use one service address")
		}
		flags["addr"] = v
	}
	res, err := resourceFlags(flags)
	if err != nil {
		return err
	}
	n, err := bound(flags)
	if err != nil {
		return err
	}
	kind := flags["kind"]
	// --personal names an agent: it is the owner's own agent, grouped in their
	// view, and no other kind may carry the flag. Stating both is allowed and
	// stating a different one is the caller's error to see.
	if kind == "" && has(flags, "personal") {
		kind = protocol.KindAgent
	}
	if kind == "" && res != nil {
		kind = protocol.KindResource
	}
	tags, err := protocol.NormalizeTags([]string{flags["tags"]})
	if err != nil {
		return err
	}
	return post("/register", protocol.Record{
		Name: pos[0], Kind: kind, Addr: flags["addr"], Descr: flags["descr"],
		Full: flags["overflow"], Proto: flags["protocol"],
		TTL: flags["ttl"], Bound: n,
		Allow: allow(flags), Personal: has(flags, "personal"), Resource: res, Tags: tags,
	})
}

// resourceFlags is a 📚 card's MCP descriptor, from --uri and its companions;
// nil when none is given (docs/03-records-resource.md#what-a-resource-is).
func resourceFlags(flags map[string]string) (*protocol.Resource, error) {
	if flags["uri"] == "" {
		for _, f := range []string{"template", "source", "mime", "title", "size", "mcp-name", "icons", "annotations"} {
			if has(flags, f) {
				return nil, fmt.Errorf("--%s describes a resource, which needs --uri", f)
			}
		}
		return nil, nil
	}
	r := &protocol.Resource{
		URI: flags["uri"], Template: has(flags, "template"), Source: flags["source"],
		MimeType: flags["mime"], Title: flags["title"], Name: flags["mcp-name"],
		Icons: json.RawMessage(flags["icons"]), Annotations: json.RawMessage(flags["annotations"]),
	}
	if v := flags["size"]; v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("--size is bytes, like 1024")
		}
		r.Size = n
	}
	return r, nil
}

// ls lists the registry, or answers about one name when given one. Asking
// about a single record does not pull the whole registry down for it, and
// what comes back carries the digest of that record's configuration rather
// than the configuration itself.
// See docs/03-records-agent.md#configuring-a-template.
func ls(args []string) error {
	human, all := false, false
	filtered := make([]string, 0, len(args))
	options := true
	for _, arg := range args {
		if options && (arg == "-h" || arg == "--table") {
			human = true
			continue
		}
		filtered = append(filtered, arg)
		if arg == "--" {
			options = false
		}
	}
	pos, flags, err := splitJoined(filtered, false, "tag")
	if err := only(flags, err, "kind", "all", "tag"); err != nil {
		return err
	}
	all = has(flags, "all")
	if len(pos) > 1 {
		return fmt.Errorf("ls takes one name, or none")
	}
	q := url.Values{}
	path := "/ls"
	if len(pos) == 1 {
		q.Set("name", pos[0])
		path = "/lookup"
	}
	if k := flags["kind"]; k != "" {
		q.Set("kind", k)
	}
	if tag := flags["tag"]; tag != "" {
		q.Set("tag", tag)
	}
	// With no name, kind or --all, ls answers the everyday question: which
	// agents can take a message right now (docs/05-discovery.md#cli-listing).
	live := len(pos) == 0 && flags["kind"] == "" && flags["tag"] == "" && !all
	if live {
		q.Set("kind", protocol.KindAgent)
	}
	body, code, err := call("GET", path, q, nil)
	if err != nil || code >= 400 {
		return show(body, code, err)
	}
	if live {
		if body, err = withReaders(body); err != nil {
			return err
		}
	}
	if !human {
		return show(body, code, nil)
	}
	var records []protocol.Record
	if len(pos) == 1 {
		var record protocol.Record
		err = json.Unmarshal(body, &record)
		records = append(records, record)
	} else {
		err = json.Unmarshal(body, &records)
	}
	if err != nil {
		return err
	}
	if len(records) == 0 {
		if live {
			fmt.Println("No agent is being read right now. --kind agent lists every agent; --all lists everything.")
		} else {
			fmt.Println("No matching records.")
		}
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tKIND\tOWNER\tREADERS\tQUEUED\tLAST USED\tDESCRIPTION")
	now := time.Now()
	// Keep metadata in one cell, including tabs, newlines and terminal controls.
	cell := func(s string) string {
		return strings.Map(func(c rune) rune {
			if unicode.IsControl(c) {
				return ' '
			}
			return c
		}, s)
	}
	for _, r := range records {
		// A service is external and has no queue here, so it has no reader
		// count and no backlog to report: a zero would be an observation of
		// something that does not exist.
		// See docs/03-records.md#record-kinds.
		readers, queued := readerCount(r.Readers), strconv.Itoa(r.Queued)
		// Groups list members and resources describe content; neither has a queue.
		if r.Kind == protocol.KindService || r.Kind == protocol.KindGroup || r.Kind == protocol.KindResource {
			readers, queued = "-", "-"
		}
		used := "-"
		if r.LastUsed != nil {
			used = display.Ago(*r.LastUsed, now)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", cell(r.Name), display.Entity(r.Kind), cell(r.Owner), readers, queued, used, cell(r.Descr))
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Println("\nREADERS  outstanding consume requests, filtered and unfiltered together; an observation, not health or completed work")
	// The legend spells the cell it explains. The cells above print "-", and a
	// legend naming a character the table never shows explains nothing.
	fmt.Println("         -  for \U0001F4E1, which is external and has no queue here, for \U0001F465, a list of members, and for \U0001F4DA, resource content; unavailable means not measured, which is a different fact")
	fmt.Println("LAST USED  when the name's credential last made a call; -  it never has, or holds none")
	return nil
}

func readerCount(readers *int) string {
	if readers == nil {
		return "unavailable"
	}
	return strconv.Itoa(*readers)
}

func send(args []string) error {
	pos, flags, err := splitAsk(args)
	if err := only(flags, err, "topic", "tag", "reply-to", "ttl", "help", "mcp", "help-config", "help-secret", "help-json", "status"); err != nil {
		return err
	}
	ask, err := askOf(pos, flags)
	if err != nil {
		return err
	}
	if ask == "" && len(pos) < 2 {
		return fmt.Errorf("send wants <to> and text")
	}
	e := protocol.Envelope{
		To: pos[0], Topic: flags["topic"], Tag: flags["tag"],
		Body: strings.Join(pos[1:], " "), TTL: flags["ttl"], Ask: ask,
	}
	// --reply-to keeps the exchange's topic and tag unless told otherwise:
	// the third party matches the answer the same way the sender would.
	if back := flags["reply-to"]; back != "" {
		e.ReplyTo = &protocol.ReplyTo{Name: back, Topic: e.Topic, Tag: e.Tag}
	}
	return post("/send", e)
}

// call is send plus the wait for its answer. There is no third verb on the
// wire and no dispatcher here: the daemon serves a filtered consume ahead of
// the unfiltered reader (docs/04-messaging.md#request-and-reply).
//
// That priority holds only while a wait is outstanding, and a receipt is an
// ordinary message that ends one — the wait reports it and asks again. So the
// call names its own inbox as the reply route, and the daemon holds the answer
// for this filtered read instead of the push adapter's
// (docs/04-messaging.md#one-reader-per-inbox).
func callVerb(args []string) error {
	pos, flags, err := splitJoined(args, true, "tagged")
	if err := only(flags, err, "topic", "tag", "tagged", "wait", "help", "mcp", "help-config", "help-secret", "help-json", "status"); err != nil {
		return err
	}
	wait, err := callWait(flags["wait"])
	if err != nil {
		return err
	}
	if _, tagged := flags["tagged"]; tagged {
		return callTagged(pos, flags, wait)
	}
	ask, err := askOf(pos, flags)
	if err != nil {
		return err
	}
	if ask == "" && len(pos) < 2 {
		return fmt.Errorf("call wants <to> and text")
	}
	topic, tag := flags["topic"], flags["tag"]
	if topic == "" {
		topic = "call"
	}
	if tag == "" {
		tag = newTag()
	}
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	if err := callAddress(ctx); err != nil {
		return err
	}
	body, err := callTarget(ctx, protocol.Envelope{To: pos[0], Topic: topic, Tag: tag, Body: strings.Join(pos[1:], " "), Ask: ask}, wait, os.Stderr)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(body)
	return err
}

// receipt says something back about a message this client consumed: `ack`
// got it, `done` finished it. Both are ordinary messages on the same topic
// and tag, naming the one they are about, so one function serves both verbs
// and the closed set stays closed by construction.
// See docs/04-messaging.md#receipts.
func receipt(kind string, args []string) error {
	pos, flags, err := split(args)
	if err := only(flags, err); err != nil {
		return err
	}
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

// agentTemplate configures an agent template into a configured record, and
// reads that configuration back. One verb, because the direction is
// obvious from whether a configuration was handed to it — and a hyphenated
// single word, because a two-word verb has no spelling in the MCP face,
// where a tool name is `[a-zA-Z0-9_-]{1,64}`.
//
// The configuration is arbitrary JSON and stays opaque: nothing here or in
// the daemon looks inside it.
// See docs/03-records-agent.md#configuring-a-template.
func agentTemplate(args []string) error {
	pos, flags, err := split(args)
	if err := only(flags, err); err != nil {
		return err
	}
	if len(pos) == 0 || len(pos) > 2 {
		return fmt.Errorf("config wants a name, and a configuration to set one:\n" +
			"  cat cfg.json | agent-bus config <template/instance@host> -\n" +
			"  agent-bus config <template/instance@host> '{\"k\":\"v\"}'\n" +
			"  agent-bus config <template/instance@host>\n" +
			"Keep secrets in Secret and config in Configuration; see " + display.SourceURL + "/docs/03-records-agent.md#configuring-a-template")
	}
	name := pos[0]
	if len(pos) == 1 {
		// No configuration named: print the one that is there. There is no
		// guard here against forgetting the "-" — stdin is not a terminal in
		// a script, in CI, or in the agent reading its own config at
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

// secret sets a record's credential (an agent's, a service's or a group's), and reads it
// back. One verb for the same reason agent-template is one: the direction is
// whether a secret was handed to it.
//
// What is written is bytes and nothing checks inside them; what is read is
// written to stdout exactly as stored, with no trailing newline, because a
// credential goes into a shell or an environment and every byte added on the
// way is one whatever uses it has to strip off again.
// See docs/03-records-service.md#secrets.
func secret(args []string) error {
	pos, flags, err := split(args)
	if err := only(flags, err); err != nil {
		return err
	}
	if len(pos) == 0 || len(pos) > 2 {
		return fmt.Errorf("secret wants a record name, and a secret to set one:\n" +
			"  cat .env | agent-bus secret <name@host> -\n" +
			"  agent-bus secret <name@host> 'TOKEN=abc'\n" +
			"  agent-bus secret <name@host>")
	}
	name := pos[0]
	if len(pos) == 1 {
		q := url.Values{}
		q.Set("name", name)
		return get("/secret", q)
	}
	raw := []byte(pos[1])
	if pos[1] == "-" {
		var err error
		if raw, err = io.ReadAll(os.Stdin); err != nil {
			return fmt.Errorf("reading the secret from stdin: %w", err)
		}
	}
	// The answer is the redacted record, so what comes back is the digest.
	return post("/secret", struct {
		Name   string `json:"name"`
		Secret string `json:"secret"`
	}{name, string(raw)})
}

// A channel is a record like any other; `channel create` is the sugar that
// says so. The verb is `channel` and not `topic` because an envelope's
// `topic` is a label on one message, which is a different thing.
// See docs/03-records-channel.md.
func channel(args []string) error {
	pos, flags, err := split(args)
	if err := only(flags, err, "kind", "descr", "overflow", "ttl", "bound", "allow"); err != nil {
		return err
	}
	if len(pos) == 0 || pos[0] != "create" {
		return fmt.Errorf("the only channel verb is: channel create <name> [--kind queue|pubsub]")
	}
	pos = pos[1:]
	if len(pos) != 1 {
		return fmt.Errorf("channel create wants one name")
	}
	// A channel is a record kind rather than a mode on one, so --kind names
	// the kind directly and there is nothing else to store.
	// See docs/03-records.md#record-kinds.
	kind := flags["kind"]
	if kind == "" {
		kind = protocol.KindQueue
	}
	if kind != protocol.KindQueue && kind != protocol.KindPubSub {
		return fmt.Errorf("a channel is %s or %s", protocol.KindQueue, protocol.KindPubSub)
	}
	n, err := bound(flags)
	if err != nil {
		return err
	}
	return post("/register", protocol.Record{
		Name: pos[0], Kind: kind, Descr: flags["descr"],
		Full: flags["overflow"], TTL: flags["ttl"], Bound: n,
		Allow: allow(flags),
	})
}

// publish is a send to a pubsub channel. A publisher need not be a registered agent
// — a token is the whole of what it needs.
// See docs/03-records-channel.md.
func publish(args []string) error {
	pos, flags, err := split(args)
	if err := only(flags, err, "channel"); err != nil {
		return err
	}
	if flags["channel"] == "" || len(pos) == 0 {
		return fmt.Errorf("publish wants --channel <name> and text")
	}
	// The channel's own name is also stamped as the message's topic, so a
	// recipient can pick out copies that came from it with `consume --topic`.
	// See docs/03-records-channel.md#what-publish-puts-on-the-message.
	return post("/send", protocol.Envelope{
		To: flags["channel"], Topic: flags["channel"], Body: strings.Join(pos, " "),
	})
}

// subscribe takes the caller off a 📣 channel's Deliver-To list. There is no
// name to pass, because the only name you may take off is your own; putting
// one on is the channel manager's, and this CLI has no management verb at
// all. The join direction is still sent, so an existing script is told by the
// daemon who to ask rather than by this program that the verb is unknown.
// See docs/04-messaging.md#deliver-to.
func unsubscribe(args []string) error {
	pos, flags, err := split(args)
	if err := only(flags, err); err != nil {
		return err
	}
	if len(pos) != 1 {
		return fmt.Errorf("unsubscribe wants one channel")
	}
	return post("/subscribe", map[string]any{"channel": pos[0], "off": true})
}

func consume(args []string) error {
	_, flags, err := split(args)
	if err := only(flags, err, "inbox", "topic", "tag", "wait", "follow", "share"); err != nil {
		return err
	}
	q := url.Values{}
	for _, k := range []string{"inbox", "topic", "tag"} {
		if has(flags, k) && flags[k] == "" {
			return fmt.Errorf("--%s wants a value", k)
		}
	}
	for _, k := range []string{"inbox", "topic", "tag", "wait"} {
		if v := flags[k]; v != "" {
			q.Set(k, v)
		}
	}
	// One of a pool, so a second reader waiting on the same empty inbox is
	// meant rather than a mistake (docs/04-messaging.md#one-reader-per-inbox).
	if has(flags, "share") {
		q.Set("share", "1")
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
	pos, flags, err := split(args)
	if err := only(flags, err, "to", "topic", "tag"); err != nil {
		return err
	}
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
// verb and lists are small, so this is one GET rather than an endpoint of its
// own.
func registered(name string) (bool, error) {
	return registeredContext(context.Background(), name)
}

func registeredContext(ctx context.Context, name string) (bool, error) {
	if _, err := protocol.ParseName(name); err != nil {
		return false, err
	}
	q := url.Values{}
	q.Set("name", name)
	body, code, err := callCtx(ctx, "GET", "/lookup", q, nil)
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

// callCtx is the same with a deadline the caller owns: `run` gives it the
// signal context so a long consume ends when the agent is asked to stop.
func callCtx(ctx context.Context, method, path string, q url.Values, body any) ([]byte, int, error) {
	// On your own socket there is nothing to set: the socket says who is
	// calling. Everywhere else the token does, and it is the only thing sent.
	// See docs/02-access.md#what-a-call-carries.
	token := os.Getenv("AGENT_BUS_TOKEN")
	if token == "" && !onOwnSocket() {
		return nil, 0, fmt.Errorf("set AGENT_BUS_TOKEN")
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
	if token != "" {
		req.Header.Set(api.HeaderToken, token)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, away{err}
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return out, resp.StatusCode, away{err}
	}
	return out, resp.StatusCode, nil
}

// whoami is this client's name, and the daemon is the only one who knows it:
// the token decides who we are, so an environment variable saying otherwise
// would only be a second answer to disagree with. Asked once — it cannot
// change under a running command. See docs/02-access.md#what-a-call-carries.
var whoami = sync.OnceValue(func() string {
	name, _ := whoamiContext(context.Background())
	return name
})

func whoamiContext(ctx context.Context) (string, error) {
	out, code, err := callCtx(ctx, "GET", "/status", nil, nil)
	if err != nil {
		return "", err
	}
	if code >= 400 {
		return "", fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	var got struct {
		You string `json:"you"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		return "", err
	}
	return got.You, nil
}

// onOwnSocket says whether we are talking over a socket the daemon opened
// for this account, which is the one place a token is not needed.
func onOwnSocket() bool {
	// Judged from the address, not from transport's base URL: that is
	// "http://localhost" over a unix socket too.
	addr := socketPath()
	return !api.IsTCP(addr) && api.IsUserSocket(addr)
}

func socketPath() string {
	if cliAddress != "" {
		return cliAddress
	}
	if a := os.Getenv("AGENT_BUS_ADDR"); a != "" {
		return a
	}
	return api.ClientSocket()
}

// transport speaks HTTP over either a unix socket or loopback TCP: same
// protocol on both listeners. Built once — `consume --follow` polls in a loop,
// and a fresh Transport each time would leak idle connections.
// See docs/decisions.md.
var transport = sync.OnceValues(connect)

func connect() (*http.Client, string) {
	return api.Dial(socketPath())
}

// A runner starts as the account and then becomes its agent. On the
// account's own socket the daemon takes the token of an agent that account
// owns (docs/02-access.md#local-socket), so the runner stays there — the one
// socket forwarded to another host is enough. A daemon from before that
// answers as the account still, and then the runner moves to the shared
// socket beside it, as it always did.
func useToken(token string) {
	os.Setenv("AGENT_BUS_TOKEN", token)
	addr := socketPath()
	if !api.IsUserSocket(addr) || api.IsTCP(addr) {
		return
	}
	if want := os.Getenv("AGENT_BUS_NAME"); want != "" {
		if body, code, err := call("GET", "/status", nil, nil); err == nil && code < 400 {
			var st struct {
				You string `json:"you"`
			}
			if json.Unmarshal(body, &st) == nil && st.You == want {
				return
			}
		}
	}
	client, _ := transport()
	client.CloseIdleConnections()
	cliAddress = filepath.Join(filepath.Dir(addr), "bus.sock")
	os.Setenv("AGENT_BUS_ADDR", cliAddress)
	transport = sync.OnceValues(connect)
}

func get(path string, q url.Values) error { return show(call("GET", path, q, nil)) }
func post(path string, body any) error    { return show(call("POST", path, nil, body)) }

// postQuiet is the same call without printing the answer: a long-running
// agent writes messages, not JSON, to its stdout.
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

// withReaders keeps the records someone is reading right now, each exactly as
// the daemon sent it.
func withReaders(body []byte) ([]byte, error) {
	var all []json.RawMessage
	if err := json.Unmarshal(body, &all); err != nil {
		return nil, err
	}
	kept := make([]json.RawMessage, 0, len(all))
	for _, raw := range all {
		var r struct {
			Readers *int `json:"readers"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, err
		}
		if r.Readers != nil && *r.Readers > 0 {
			kept = append(kept, raw)
		}
	}
	out, err := json.Marshal(kept)
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
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
		c.From = e.ReplyTo.Name
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
	if err := writeState(filepath.Join(dir, e.ID+".json"), b); err != nil {
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
var onOff = map[string]bool{"follow": true, "personal": true, "share": true, "network": true,
	"int": true, "json": true, "add": true, "replace": true, "help": true, "help-config": true, "help-secret": true, "help-json": true, "mcp": true, "force": true, "all": true, "template": true, "stay-registered": true, "no-color": true, "no-asks": true}

// askOf is what --help or --mcp asks: it travels as the envelope's ask with
// no body, never as text, so text beside it is refused
// (docs/constitution.md#agent-self-description).
func askOf(pos []string, flags map[string]string) (string, error) {
	ask := ""
	for _, key := range []string{"help", "help-config", "help-secret", "help-json", "status", "mcp"} {
		if _, ok := flags[key]; !ok {
			continue
		}
		if ask != "" {
			return "", fmt.Errorf("--%s and --%s ask one thing each; pass one", ask, key)
		}
		ask = key
	}
	if ask != "" && len(pos) != 1 {
		return "", fmt.Errorf("--%s takes <to> and no text", ask)
	}
	return ask, nil
}

// allow is the record ACL as stated on the command line: a comma-separated
// list, `*` for anyone who can authenticate, absent for no answer of its own.
// See docs/02-access.md#acl.
func allow(flags map[string]string) []string {
	v, ok := flags["allow"]
	if !ok || strings.TrimSpace(v) == "" {
		return nil
	}
	return protocol.SplitACLText(v)
}

func has(flags map[string]string, k string) bool { _, ok := flags[k]; return ok }

// Name an unknown option first; a known valued option must have an argument.
func only(flags map[string]string, parseErr error, allowed ...string) error {
	known := map[string]bool{}
	for _, name := range allowed {
		known[name] = true
	}
	for name := range flags {
		if !known[name] {
			return fmt.Errorf("unknown option --%s", name)
		}
	}
	return parseErr
}

func split(args []string) ([]string, map[string]string, error) { return splitOptions(args, false) }

// Only send/call have a boolean --status; manage --status keeps its value.
func splitAsk(args []string) ([]string, map[string]string, error) { return splitOptions(args, true) }

// splitJoined is split for a listing whose repeated options add up: each
// repeat of a named option joins the earlier values with a comma, so
// `ls --tag=a --tag=b` asks for the records carrying both, as `--tag=a,b` does.
func splitJoined(args []string, asks bool, joined ...string) ([]string, map[string]string, error) {
	return splitOptionsJoined(args, asks, joined)
}

func splitOptions(args []string, asks bool) ([]string, map[string]string, error) {
	return splitOptionsJoined(args, asks, nil)
}

func splitOptionsJoined(args []string, asks bool, joined []string) ([]string, map[string]string, error) {
	pos := []string{}
	flags := map[string]string{}
	set := func(k, v string) {
		if old, again := flags[k]; again && slices.Contains(joined, k) && old != "" && v != "" {
			v = old + "," + v
		}
		flags[k] = v
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return append(pos, args[i+1:]...), flags, nil
		}
		if !strings.HasPrefix(a, "--") {
			pos = append(pos, a)
			continue
		}
		k, v, hasEq := strings.Cut(strings.TrimPrefix(a, "--"), "=")
		switch {
		case hasEq:
			set(k, v)
		case i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") && !(onOff[k] || asks && k == "status"):
			set(k, args[i+1])
			i++
		case onOff[k] || asks && k == "status":
			flags[k] = ""
		default:
			flags[k] = ""
			return pos, flags, fmt.Errorf("--%s wants a value", k)
		}
	}
	return pos, flags, nil
}

func die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

// manage edits one record: its description, status, owner and route, and its
// lists either whole or by add, add_to_set and remove deltas the daemon
// resolves against the record as it finds it
// (docs/01-identity-and-authority.md#record-authority).
func manage(args []string) error {
	pos, flags, err := split(args)
	fields := map[string]string{"allow": "allow", "maintainers": "maintainers", "deliver-to": "subs", "tags": "tags"}
	allowed := []string{"descr", "status", "owner", "deliver-to", "roles", "tags"}
	for _, op := range []string{"add", "add-to-set", "remove"} {
		for f := range fields {
			allowed = append(allowed, op+"-"+f)
		}
	}
	if err := only(flags, err, allowed...); err != nil {
		return err
	}
	if len(pos) != 1 {
		return fmt.Errorf("manage wants one record name")
	}
	change := map[string]any{"name": pos[0]}
	for _, k := range []string{"descr", "status", "owner"} {
		if v, ok := flags[k]; ok {
			change[k] = v
		}
	}
	if v, ok := flags["deliver-to"]; ok {
		change["subs"] = terms(v)
	}
	if v, ok := flags["tags"]; ok {
		tags, err := protocol.NormalizeTags([]string{v})
		if err != nil {
			return err
		}
		if tags == nil {
			tags = []string{}
		}
		change["tags"] = tags
	}
	// An Agent's list of the roles it understands; '' clears it.
	if v, ok := flags["roles"]; ok {
		change["roles"] = terms(v)
	}
	for _, op := range []string{"add", "add-to-set", "remove"} {
		delta := map[string][]string{}
		for f, key := range fields {
			if v, ok := flags[op+"-"+f]; ok {
				if f == "tags" {
					tags, err := protocol.NormalizeTags([]string{v})
					if err != nil {
						return err
					}
					delta[key] = tags
				} else if f == "allow" {
					delta[key] = protocol.SplitACLText(v)
				} else {
					delta[key] = terms(v)
				}
			}
		}
		if len(delta) != 0 {
			change[strings.ReplaceAll(op, "-", "_")] = delta
		}
	}
	if len(change) == 1 {
		return fmt.Errorf("manage %s changes nothing; name a field", pos[0])
	}
	return post("/manage", change)
}

// terms is a comma-separated list, blanks dropped; empty is the empty list.
func terms(v string) []string {
	out := []string{}
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// group sets a Group's whole membership, or prints it
// (docs/01-identity-and-authority.md#groups).
func group(args []string) error {
	pos, flags, err := split(args)
	if err := only(flags, err); err != nil {
		return err
	}
	if len(pos) == 0 || !strings.HasPrefix(pos[0], "@") {
		return fmt.Errorf("group wants a group name beginning with @, then its members")
	}
	if len(pos) == 1 {
		body, code, err := call("GET", "/groups", nil, nil)
		if err != nil {
			return err
		}
		if code >= 400 {
			return fmt.Errorf("%s", strings.TrimSpace(string(body)))
		}
		var groups map[string][]string
		if err := json.Unmarshal(body, &groups); err != nil {
			return err
		}
		members, ok := groups[strings.ToLower(pos[0])]
		if !ok {
			return fmt.Errorf("no such group: %s", pos[0])
		}
		for _, m := range members {
			fmt.Println(m)
		}
		return nil
	}
	return post("/group", map[string]any{"name": pos[0], "members": pos[1:]})
}

// `agent-bus start` publishes a shell script as a service.
//
// The script never sees the bus: this process is the inbox's one reader, it
// spawns the script per message, and the script's stdout is the reply. That
// is the whole contract (docs/08-runner-role.md#script-services).
//
// What it adds to the PoC's version is confinement and a handle: each script
// gets one work directory it may write to, is confined when it asks to be,
// and leaves a note that `stop` and `logs` read (service.go).
// Supervision — timeouts, restart with backoff — is still the runner proper.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/sandbox"
)

// A service description, from flags or from JSON on stdin. The two forms
// carry the same fields so that one can be pasted from the other.
type service struct {
	Name      string `json:"name"`
	Algo      string `json:"algo"`      // json: envelope on stdin · args: body as argv[1]
	Script    string `json:"script"`    // the program to run
	Descr     string `json:"descr"`     // what ls and the MCP catalog show
	Instances int    `json:"instances"` // how many may run at once
	Sandbox   string `json:"sandbox"`   // on or off; unset is off — confinement is asked for
	Network   bool   `json:"network"`   // a script that needs one says so; off otherwise

	// Worked out at start rather than stated: how the script is confined,
	// the one directory it may write to, what it must still be able to read,
	// and where its output goes.
	box  ports.Sandbox
	work string
	read []string
	say  io.Writer
}

const (
	// A form names what reaches the child, and that decides the rest. `json`
	// is the envelope on stdin; `args` is the body as argv[1]. `std` is the
	// raw body on stdin and the stream forms keep the process, neither of
	// which is built yet — docs/08-runner-role.md#script-services.
	algoJSON = "json"
	algoArgs = "args"
)

func start(args []string) error {
	svc, err := describe(args)
	if err != nil {
		return err
	}
	// Two readers of one inbox is the mistake the rule exists for
	// (docs/04-messaging.md#one-reader-per-inbox), and a second `start` of
	// the same name here is exactly that — said plainly rather than left to
	// surface as a refused consume a second later.
	if r, err := alive(svc.Name); err == nil {
		return fmt.Errorf("%s is already running here as pid %d; stop it first", r.Name, r.PID)
	}
	if err := postQuiet("/register", protocol.Record{
		Name: svc.Name, Kind: "generic", Addr: svc.Script, Descr: svc.Descr,
	}); err != nil {
		return err
	}
	// From here on this process *is* the service: it reads the service's
	// inbox and answers from it, not from whatever name launched it.
	// The record above keeps the launcher as its owner, which is exactly
	// what lets it collect the service's own credential — a name is bound
	// to its token, so switching names means switching both.
	// See docs/02-access.md#what-a-call-carries.
	tok, err := tokenFor(svc.Name)
	if err != nil {
		return err
	}
	os.Setenv("AGENT_BUS_NAME", svc.Name)
	os.Setenv("AGENT_BUS_TOKEN", tok)

	svc.work = workPath(svc.Name)
	if err := os.MkdirAll(svc.work, 0o700); err != nil {
		return fmt.Errorf("no work directory for %s: %w", svc.Name, err)
	}
	svc.read = scriptDir(svc.Script)
	log, err := openLog(svc.Name)
	if err != nil {
		return err
	}
	defer log.Close()
	// Both, because the person who started it is watching the terminal and
	// whoever runs `logs` later is not.
	svc.say = io.MultiWriter(os.Stderr, log)
	if svc.box, err = sandbox.Pick(svc.Sandbox); err != nil {
		return err
	}
	fmt.Fprintf(svc.say, "%s is %s (%s, %d at a time, sandbox %s, work %s); ctrl-c to stop\n",
		svc.Name, svc.Script, svc.Algo, svc.Instances, svc.box.Name(), svc.work)

	r := running{
		Name: svc.Name, PID: os.Getpid(), Script: svc.Script,
		Sandbox: svc.box.Name(), Log: logPath(svc.Name), Started: time.Now(),
	}
	if err := r.note(); err != nil {
		return fmt.Errorf("could not leave a note for stop and logs: %w", err)
	}
	defer os.Remove(notePath(svc.Name))
	return serve(svc)
}

// openLog is where the service and its scripts write. Append, because a
// service restarted by hand should not throw away what the last run said.
func openLog(name string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(logPath(name)), 0o700); err != nil {
		return nil, err
	}
	return os.OpenFile(logPath(name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
}

// scriptDir is the directory the script itself lives in, which a private
// /tmp would otherwise hide from the child along with everything else there.
// The script is a shell command line, so the program is its first word, and
// something that is not a path (a bare `echo`) has no directory to keep.
func scriptDir(script string) []string {
	fields := strings.Fields(script)
	if len(fields) == 0 {
		return nil
	}
	p, err := filepath.Abs(fields[0])
	if err != nil {
		return nil
	}
	if _, err := os.Stat(p); err != nil {
		return nil
	}
	return []string{filepath.Dir(p)}
}

// describe reads the service either from the command line or, with no name
// given, as JSON on stdin.
func describe(args []string) (service, error) {
	var svc service
	pos, flags := split(args)

	// -5 means five at a time. It is not a --flag, so it arrives as a
	// positional; the first one that looks like a count is taken out before
	// the rest is read, and only the first, so a script named `-5` is still
	// reachable as `./-5`.
	count := 0
	for i, a := range pos {
		if n, err := strconv.Atoi(strings.TrimPrefix(a, "-")); err == nil && strings.HasPrefix(a, "-") && n > 0 {
			count = n
			pos = append(pos[:i:i], pos[i+1:]...)
			break
		}
	}

	if len(pos) == 0 {
		// The JSON form, `-5` on its own included: the flag then overrides
		// what stdin says (docs/08-runner-role.md#script-services).
		if err := json.NewDecoder(os.Stdin).Decode(&svc); err != nil {
			return svc, fmt.Errorf("bad service JSON on stdin: %w", err)
		}
	} else {
		// The script is one argument, and it is a shell command line, so
		// joining several would silently lose their quoting.
		if len(pos) > 2 {
			return svc, fmt.Errorf("start wants one script; quote it if it is a command line: %q",
				strings.Join(pos[1:], " "))
		}
		svc.Name, svc.Algo, svc.Descr = pos[0], flags["algo"], flags["descr"]
		if len(pos) == 2 {
			svc.Script = pos[1]
		}
	}
	if count > 0 {
		svc.Instances = count
	}

	if svc.Name == "" || svc.Script == "" {
		return svc, fmt.Errorf("a service needs a name and a script")
	}
	if svc.Algo == "" {
		svc.Algo = algoJSON
	}
	if svc.Algo != algoJSON && svc.Algo != algoArgs {
		return svc, fmt.Errorf("--algo is %s or %s", algoJSON, algoArgs)
	}
	if svc.Instances <= 0 {
		svc.Instances = 1
	}
	// Overridden by the flag the same way -N overrides the JSON form.
	if v := flags["sandbox"]; v != "" {
		svc.Sandbox = v
	}
	if has(flags, "network") {
		svc.Network = true
	}
	return svc, nil
}

// serve is the one reader of the service's inbox. It consumes in this
// goroutine — one unfiltered read, as the design requires
// (docs/04-messaging.md#one-reader-per-inbox) — and runs the script in
// others, up to Instances at once.
//
// Stopping is graceful and nothing more: the consume is cut short, no further
// message is taken, and the scripts already running are waited for — however
// long they take. There is no supervision here to do anything else.
func serve(svc service) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slots := make(chan struct{}, svc.Instances)
	var running sync.WaitGroup
	defer running.Wait()

	q := url.Values{"wait": {"55s"}}
	for {
		// The slot is taken *before* the consume, so a message is only ever
		// taken off the daemon when there is a worker free to run it. Taking
		// it after would hold one message inside this process, out of reach
		// of anything else if the service then died.
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			fmt.Fprintf(svc.say, "%s: stopping\n", svc.Name)
			return nil
		}

		e, _, got, err := next(ctx, q)
		if err != nil || !got {
			<-slots
			if err == nil {
				continue // nothing arrived before the deadline
			}
			if ctx.Err() != nil {
				fmt.Fprintf(svc.say, "%s: stopping\n", svc.Name)
				return nil
			}
			return err
		}

		running.Add(1)
		go func(e protocol.Envelope) {
			defer running.Done()
			defer func() { <-slots }()
			handle(svc, e)
		}(e)
	}
}

// next is the one consume: long-poll, tell "nothing arrived" apart from a
// refusal, and decode. Every reader here goes through it — the service loop,
// the `consume` verb and the wait inside `call` — so "no message" cannot come
// to mean three things. The raw body comes back too, because a reader that
// prints is printing what the daemon said, not a re-encoding of it.
func next(ctx context.Context, q url.Values) (protocol.Envelope, []byte, bool, error) {
	var e protocol.Envelope
	body, code, err := callCtx(ctx, "GET", "/consume", q, nil)
	switch {
	case err != nil:
		return e, nil, false, err
	case code == http.StatusNoContent:
		return e, nil, false, nil
	case code >= 400:
		return e, nil, false, fmt.Errorf("%s", strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, &e); err != nil {
		return e, body, false, fmt.Errorf("the daemon sent something that is not an envelope: %w", err)
	}
	return e, body, true, nil
}

// handle runs the script once and answers with what it printed. A non-zero
// exit sends no reply — the caller waits and times out. That says the script
// failed, not that it did nothing: it may have got half way first.
//
// A script that succeeds and prints nothing sends `done` instead. Without it
// the caller has an `ack` and then silence forever, which is the one case
// `done` exists for; a script that answers skips it, because a reply has
// plainly finished. See docs/04-messaging.md#receipts.
func handle(svc service, e protocol.Envelope) {
	// Receipts and the answer all go where the request said they should,
	// which is the sender unless it named a third party.
	// See docs/04-messaging.md#reply-routing.
	// Nobody is waiting any more, so the work is not worth doing — that is
	// the whole reason the deadline travels. Checked before the ack, because
	// an ack for a request that will never be run says the opposite of the
	// truth. See docs/04-messaging.md#request-and-reply.
	if e.TooLate(time.Now()) {
		fmt.Fprintf(svc.say, "%s: %s arrived after its caller gave up at %s; not run\n",
			svc.Name, e.ID, e.Deadline.Format(time.RFC3339))
		return
	}
	back, topic, tag := e.From, e.Topic, e.Tag
	if e.ReplyTo != nil {
		back, topic, tag = e.ReplyTo.Service, e.ReplyTo.Topic, e.ReplyTo.Tag
	}
	say := func(kind string) {
		if err := postQuiet("/send", protocol.Envelope{
			To: back, Topic: topic, Tag: tag, Receipt: kind, Re: e.ID,
		}); err != nil {
			fmt.Fprintf(svc.say, "%s: could not %s %s: %v\n", svc.Name, kind, e.ID, err)
		}
	}
	// ack first: the service has the message, whatever happens next.
	say(protocol.ReceiptAck)

	argv := []string{"sh", "-c", svc.Script}
	if svc.Algo == algoArgs {
		argv = []string{"sh", "-c", svc.Script + ` "$@"`, "sh", e.Body}
	}
	// The envelope is in the environment either way, so a script can route on
	// it without parsing anything — and it is STATED rather than exported,
	// because a sandboxed child starts from the manager's environment and
	// inherits nothing of ours (docs/08-runner-role.md#sandboxing).
	env := []string{
		"AGENT_BUS_MESSAGE_ID=" + e.ID,
		"AGENT_BUS_FROM=" + e.From,
		"AGENT_BUS_TO=" + e.To,
		"AGENT_BUS_TOPIC=" + e.Topic,
		"AGENT_BUS_TAG=" + e.Tag,
		"AGENT_BUS_WORK=" + svc.work,
	}
	line := svc.box.Wrap(ports.Job{
		Argv: argv, Work: svc.work, Read: svc.read, Env: env, Net: svc.Network,
	})
	cmd := exec.Command(line[0], line[1:]...)
	if svc.Algo != algoArgs {
		cmd.Stdin = strings.NewReader(string(mustJSON(e)) + "\n")
	}
	cmd.Dir = svc.work
	cmd.Env = append(os.Environ(), env...)
	cmd.Stderr = svc.say

	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(svc.say, "%s: %s exited badly for %s: %v\n", svc.Name, svc.Script, e.ID, err)
		return
	}
	// Nothing printed is not an empty answer: a script that only does
	// something says so by staying quiet — and `done` is how the caller
	// hears that it finished rather than waiting out its deadline.
	answer := strings.TrimRight(string(out), "\n")
	if answer == "" {
		say(protocol.ReceiptDone)
		return
	}
	if err := postQuiet("/send", protocol.Envelope{
		To: back, Topic: topic, Tag: tag, Body: answer,
	}); err != nil {
		fmt.Fprintf(svc.say, "%s: could not answer %s: %v\n", svc.Name, e.ID, err)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// `agent-bus start` publishes a shell script as a service.
//
// The script never sees the bus: this process is the inbox's one reader, it
// spawns the script per message, and the script's stdout is the reply. That
// is the whole contract — no sandbox, no supervision, no restart, which is
// what makes it a PoC feature rather than the runner
// (docs/08-runner-role.md#script-services).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A service description, from flags or from JSON on stdin. The two forms
// carry the same fields so that one can be pasted from the other.
type service struct {
	Name      string `json:"name"`
	Algo      string `json:"algo"`      // std: envelope on stdin · args: body as argv[1]
	Script    string `json:"script"`    // the program to run
	Descr     string `json:"descr"`     // what ls and the MCP catalog show
	Instances int    `json:"instances"` // how many may run at once
}

const (
	algoStd  = "std"
	algoArgs = "args"
)

func start(args []string) error {
	svc, err := describe(args)
	if err != nil {
		return err
	}
	if err := postQuiet("/register", protocol.Record{
		Name: svc.Name, Kind: "generic", Addr: svc.Script, Descr: svc.Descr,
	}); err != nil {
		return err
	}
	// From here on this process *is* the service: it reads the service's
	// inbox and answers from it, not from whatever name launched it.
	// The record above keeps the launcher as its owner.
	os.Setenv("AGENT_BUS_NAME", svc.Name)
	fmt.Fprintf(os.Stderr, "%s is %s (%s, %d at a time); ctrl-c to stop\n",
		svc.Name, svc.Script, svc.Algo, svc.Instances)
	return serve(svc)
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
		svc.Algo = algoStd
	}
	if svc.Algo != algoStd && svc.Algo != algoArgs {
		return svc, fmt.Errorf("--algo is %s or %s", algoStd, algoArgs)
	}
	if svc.Instances <= 0 {
		svc.Instances = 1
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
			fmt.Fprintf(os.Stderr, "%s: stopping\n", svc.Name)
			return nil
		}

		e, _, got, err := next(ctx, q)
		if err != nil || !got {
			<-slots
			if err == nil {
				continue // nothing arrived before the deadline
			}
			if ctx.Err() != nil {
				fmt.Fprintf(os.Stderr, "%s: stopping\n", svc.Name)
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
	say := func(kind string) {
		if err := postQuiet("/send", protocol.Envelope{
			To: e.From, Topic: e.Topic, Tag: e.Tag, Receipt: kind, Re: e.ID,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "%s: could not %s %s: %v\n", svc.Name, kind, e.ID, err)
		}
	}
	// ack first: the service has the message, whatever happens next.
	say(protocol.ReceiptAck)

	cmd := exec.Command("sh", "-c", svc.Script)
	if svc.Algo == algoArgs {
		cmd = exec.Command("sh", "-c", svc.Script+` "$@"`, "sh", e.Body)
	} else {
		cmd.Stdin = strings.NewReader(string(mustJSON(e)) + "\n")
	}
	// The envelope is in the environment either way, so a script can route on
	// it without parsing anything.
	cmd.Env = append(os.Environ(),
		"AGENT_BUS_MESSAGE_ID="+e.ID,
		"AGENT_BUS_FROM="+e.From,
		"AGENT_BUS_TO="+e.To,
		"AGENT_BUS_TOPIC="+e.Topic,
		"AGENT_BUS_TAG="+e.Tag,
	)
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s exited badly for %s: %v\n", svc.Name, svc.Script, e.ID, err)
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
		To: e.From, Topic: e.Topic, Tag: e.Tag, Body: answer,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "%s: could not answer %s: %v\n", svc.Name, e.ID, err)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// `agent-bus start` publishes a shell script as a service.
//
// The script never sees the bus: this process is the inbox's one reader, it
// spawns the script per message, and the script's stdout is the reply. That
// is the whole contract — no sandbox, no supervision, no restart, which is
// what makes it a PoC feature rather than the runner
// (docs/08-runner-role.md#script-services).
package main

import (
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
	fmt.Fprintf(os.Stderr, "%s is %s (%s, %d at a time); ctrl-c to stop\n",
		svc.Name, svc.Script, svc.Algo, svc.Instances)
	return serve(svc)
}

// describe reads the service either from the command line or, with no
// arguments at all, as JSON on stdin.
func describe(args []string) (service, error) {
	var svc service
	if len(args) == 0 {
		if err := json.NewDecoder(os.Stdin).Decode(&svc); err != nil {
			return svc, fmt.Errorf("bad service JSON on stdin: %w", err)
		}
	} else {
		pos, flags := split(args)
		// -5 means five at a time. It is not a --flag, so it arrives as a
		// positional and is taken out before the rest is read.
		kept := pos[:0]
		for _, a := range pos {
			if n, err := strconv.Atoi(strings.TrimPrefix(a, "-")); err == nil && strings.HasPrefix(a, "-") && n > 0 {
				svc.Instances = n
				continue
			}
			kept = append(kept, a)
		}
		pos = kept
		if len(pos) < 1 {
			return svc, fmt.Errorf("start wants <name> and a script, or JSON on stdin")
		}
		svc.Name, svc.Algo, svc.Descr = pos[0], flags["algo"], flags["descr"]
		if len(pos) > 1 {
			svc.Script = strings.Join(pos[1:], " ")
		}
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
func serve(svc service) error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	slots := make(chan struct{}, svc.Instances)
	var running sync.WaitGroup
	defer running.Wait()

	q := url.Values{"wait": {"55s"}}
	for {
		select {
		case <-stop:
			fmt.Fprintf(os.Stderr, "%s: stopping\n", svc.Name)
			return nil
		default:
		}

		body, code, err := call("GET", "/consume", q, nil)
		if err != nil {
			return err
		}
		if code == http.StatusNoContent {
			continue // nothing arrived before the deadline
		}
		if code >= 400 {
			return fmt.Errorf("%s", strings.TrimSpace(string(body)))
		}
		var e protocol.Envelope
		if err := json.Unmarshal(body, &e); err != nil {
			return fmt.Errorf("the daemon sent something that is not an envelope: %w", err)
		}

		// Taking the slot before the goroutine is what bounds concurrency:
		// with all N busy this blocks, and nothing is consumed meanwhile.
		slots <- struct{}{}
		running.Add(1)
		go func(e protocol.Envelope) {
			defer running.Done()
			defer func() { <-slots }()
			handle(svc, e)
		}(e)
	}
}

// handle runs the script once and answers with what it printed. A non-zero
// exit means no reply — the caller waits and times out, which is the honest
// outcome when the work did not happen.
func handle(svc service, e protocol.Envelope) {
	// ack first: the service has the message, whatever happens next.
	if err := postQuiet("/send", protocol.Envelope{
		To: e.From, Topic: e.Topic, Tag: e.Tag, Receipt: "ack", Re: e.ID,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "%s: could not ack %s: %v\n", svc.Name, e.ID, err)
	}

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
	if err := postQuiet("/send", protocol.Envelope{
		To: e.From, Topic: e.Topic, Tag: e.Tag, Body: strings.TrimRight(string(out), "\n"),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "%s: could not answer %s: %v\n", svc.Name, e.ID, err)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

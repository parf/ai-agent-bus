// `agent-bus start` publishes a shell script as an agent.
//
// The script never sees the bus: this process is the inbox's one reader, it
// spawns the script per message, and the script's stdout is the reply. That
// is the whole contract (docs/06-runner-role.md#script-agents).
//
// Each script gets one work directory it may write to, is confined when it
// asks to be, and leaves a note that `stop` and `logs` read (agent.go).
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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/proctitle"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/sandbox"
)

// An agent description, from flags or from JSON on stdin. The two forms
// carry the same fields so that one can be pasted from the other.
type agent struct {
	Name      string   `json:"name"`
	Allow     []string `json:"allow,omitempty"`
	Personal  bool     `json:"personal,omitempty"`
	Algo      string   `json:"algo"`      // json: envelope on stdin · args: body as argv[1]
	Script    string   `json:"script"`    // the program to run
	Descr     string   `json:"descr"`     // what ls and the MCP catalog show
	Instances int      `json:"instances"` // how many may run at once
	Sandbox   string   `json:"sandbox"`   // on or off; unset is off — confinement is asked for
	Network   bool     `json:"network"`   // a script that needs one says so; off otherwise
	// Roles is the list of roles this script understands, registered on the
	// Agent for its Owner to know what to assign; informational
	// (Plans/R1.0-Release/roles.md#roles-are-for-agents).
	Roles []string `json:"roles,omitempty"`

	// Worked out at start rather than stated: how the script is confined,
	// the one directory it may write to, what it must still be able to read,
	// and where its output goes.
	box  ports.Sandbox
	work string
	read []string
	say  io.Writer
	stop context.Context // ends on ctrl-c or `stop`; an answer retried while the daemon is away gives up then
}

const (
	// A form names what reaches the child, and that decides the rest. `json`
	// is the envelope on stdin; `args` is the body as argv[1]. `std` is the
	// raw body on stdin and the stream forms keep the process, neither of
	// which is built yet — docs/06-runner-role.md#script-agents.
	algoJSON = "json"
	algoArgs = "args"
)

func start(args []string) error {
	ag, err := describe(args)
	if err != nil {
		return err
	}
	// Two readers of one inbox is the mistake the rule exists for
	// (docs/04-messaging.md#one-reader-per-inbox), and a second `start` of
	// the same name here is exactly that — said plainly rather than left to
	// surface as a refused consume a second later.
	if r, err := alive(ag.Name); err == nil {
		return fmt.Errorf("%s is already running here as pid %d; stop it first", r.Name, r.PID)
	}
	if err := postQuiet("/register", protocol.Record{
		Name: ag.Name, Kind: protocol.KindAgent, Script: ag.Script, Descr: ag.Descr,
		Allow: ag.Allow, Personal: ag.Personal, Roles: ag.Roles,
	}); err != nil {
		return err
	}
	// From here on this process *is* the agent: it reads the agent's
	// inbox and answers from it, not from whatever name launched it.
	// The record above keeps the launcher as its owner, which is exactly
	// what lets it collect the agent's own credential — a name is bound
	// to its token, so switching names means switching both.
	// See docs/02-access.md#what-a-call-carries.
	tok, err := tokenFor(ag.Name)
	if err != nil {
		return err
	}
	os.Setenv("AGENT_BUS_NAME", ag.Name)
	useToken(tok)

	ag.work = workPath(ag.Name)
	if err := os.MkdirAll(ag.work, 0o700); err != nil {
		return fmt.Errorf("no work directory for %s: %w", ag.Name, err)
	}
	ag.read = scriptDir(ag.Script)
	log, err := openLog(ag.Name)
	if err != nil {
		return err
	}
	defer log.Close()
	// Both, because the person who started it is watching the terminal and
	// whoever runs `logs` later is not.
	ag.say = io.MultiWriter(os.Stderr, log)
	if ag.box, err = sandbox.Pick(ag.Sandbox); err != nil {
		return err
	}
	fmt.Fprintf(ag.say, "%s is %s (%s, %d at a time, sandbox %s, work %s); ctrl-c to stop\n",
		ag.Name, ag.Script, ag.Algo, ag.Instances, ag.box.Name(), ag.work)

	r := running{
		Name: ag.Name, PID: os.Getpid(), Script: ag.Script,
		Sandbox: ag.box.Name(), Log: logPath(ag.Name), Started: time.Now(),
	}
	if err := r.note(); err != nil {
		return fmt.Errorf("could not leave a note for stop and logs: %w", err)
	}
	defer os.Remove(notePath(ag.Name))
	serveErr := serve(ag)
	// A deliberate exit leaves no name behind: when nothing is waiting, the
	// runner unregisters itself, so the next `start` begins clean. A daemon
	// that refuses — messages still queued — keeps the name. Only the
	// deliberate stop unregisters: an error exit (a refused read, a daemon
	// that went away) keeps the name too, so a transient fault does not
	// take the registration with it, which is also the answer for a crash:
	// no graceful exit, no unregister (docs/06-runner-role.md#script-agents).
	if serveErr == nil {
		if err := unregisterStopped(ag.Name); err != nil {
			fmt.Fprintf(ag.say, "%s stays registered: %v\n", ag.Name, err)
		} else {
			fmt.Fprintf(ag.say, "%s unregistered\n", ag.Name)
		}
	}
	return serveErr
}

// A cancelled HTTP consume can finish before its daemon waiter is released.
// Retry only a confirmed busy refusal, after observing this inbox empty and
// idle. A transport failure may have removed the record and is never replayed.
func unregisterStopped(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for {
		out, code, err := callCtx(ctx, "POST", "/unregister", nil, map[string]string{"name": name})
		if err != nil {
			return err
		}
		if code < http.StatusBadRequest {
			return nil
		}
		refusal := fmt.Errorf("%s", strings.TrimSpace(string(out)))
		if code != http.StatusConflict {
			return refusal
		}
		for {
			out, code, err = callCtx(ctx, "GET", "/lookup", url.Values{"name": {name}}, nil)
			if err != nil || code >= http.StatusBadRequest {
				return refusal
			}
			var record protocol.Record
			if json.Unmarshal(out, &record) != nil || record.Name != name || record.Readers == nil || record.Queued != 0 {
				return refusal
			}
			if *record.Readers == 0 && !record.Reading {
				break
			}
			select {
			case <-ctx.Done():
				return refusal
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
}

// openLog is where the agent and its scripts write. Append, because a
// an agent restarted by hand should not throw away what the last run said.
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

// describe reads the agent either from the command line or, with no name
// given, as JSON on stdin.
func describe(args []string) (agent, error) {
	var ag agent
	pos, flags := split(args)
	if err := only(flags, "algo", "descr", "sandbox", "allow", "personal", "network", "roles"); err != nil {
		return ag, err
	}

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
		// what stdin says (docs/06-runner-role.md#script-agents).
		decoder := json.NewDecoder(os.Stdin)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&ag); err != nil {
			return ag, fmt.Errorf("bad agent JSON on stdin: %w", err)
		}
	} else {
		// The script is one argument, and it is a shell command line, so
		// joining several would silently lose their quoting.
		if len(pos) > 2 {
			return ag, fmt.Errorf("start wants one script; quote it if it is a command line: %q",
				strings.Join(pos[1:], " "))
		}
		ag.Name, ag.Algo, ag.Descr = pos[0], flags["algo"], flags["descr"]
		if len(pos) == 2 {
			ag.Script = pos[1]
		}
	}
	if count > 0 {
		ag.Instances = count
	}

	// The runner publishes an agent, and an agent's name begins with #; a
	// plainly written `start hi@demo` names the same thing
	// (docs/06-runner-role.md#script-agents). Anything else is left alone:
	// the daemon's refusal names the rule better than a rewrite here could.
	if ag.Name != "" && !strings.HasPrefix(ag.Name, "#") {
		ag.Name = "#" + ag.Name
	}

	if ag.Name == "" || ag.Script == "" {
		return ag, fmt.Errorf("an agent needs a name and a script")
	}
	// A path is absolute or it is refused: the runner starts scripts in its
	// work directory, so any relative path — ./hi.sh or bin/hi.sh alike —
	// would name something else there. A bare command (echo) is found on
	// PATH and is not a path at all.
	if fields := strings.Fields(ag.Script); len(fields) > 0 && strings.Contains(fields[0], "/") && !filepath.IsAbs(fields[0]) {
		return ag, fmt.Errorf("use an absolute script path: the runner starts scripts in its work directory, not the launch directory")
	}
	if ag.Algo == "" {
		ag.Algo = algoJSON
	}
	if ag.Algo != algoJSON && ag.Algo != algoArgs {
		return ag, fmt.Errorf("--algo is %s or %s", algoJSON, algoArgs)
	}
	if ag.Instances <= 0 {
		ag.Instances = 1
	}
	// Overridden by the flag the same way -N overrides the JSON form.
	if v := flags["sandbox"]; v != "" {
		ag.Sandbox = v
	}
	if has(flags, "allow") {
		ag.Allow = allow(flags)
	}
	if has(flags, "personal") {
		ag.Personal = true
	}
	if has(flags, "network") {
		ag.Network = true
	}
	if has(flags, "roles") {
		ag.Roles = nil
		for _, r := range strings.Split(flags["roles"], ",") {
			if r = strings.TrimSpace(r); r != "" {
				ag.Roles = append(ag.Roles, r)
			}
		}
	}
	return ag, nil
}

// serve is the one reader of the agent's inbox. It consumes in this
// goroutine — one unfiltered read, as the design requires
// (docs/04-messaging.md#one-reader-per-inbox) — and runs the script in
// others, up to Instances at once.
//
// Stopping is graceful and nothing more: the consume is cut short, no further
// message is taken, and the scripts already running are waited for — however
// long they take. There is no supervision here to do anything else.
func serve(ag agent) error {
	var calls atomic.Uint64
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ag.stop = ctx
	bus := &link{say: ag.say, name: ag.Name}
	defer proctitle.StartNoted("agent-bus-runner", ag.Name, &calls, bus.note)()

	slots := make(chan struct{}, ag.Instances)
	var running sync.WaitGroup
	defer running.Wait()

	const poll = 55 * time.Second
	q := url.Values{"wait": {poll.String()}}
	for {
		// The slot is taken *before* the consume, so a message is only ever
		// taken off the daemon when there is a worker free to run it. Taking
		// it after would hold one message inside this process, out of reach
		// of anything else if the agent then died.
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			fmt.Fprintf(ag.say, "%s: stopping\n", ag.Name)
			return nil
		}

		// A long poll with no answer well past its wait is a link that died
		// without saying so — a dropped network, a half-open forward.
		read, done := context.WithTimeout(ctx, poll+readGrace)
		e, _, got, err := next(read, q)
		done()
		if err != nil || !got {
			<-slots
			if err == nil {
				bus.up()
				continue // nothing arrived before the deadline
			}
			if ctx.Err() != nil {
				fmt.Fprintf(ag.say, "%s: stopping\n", ag.Name)
				return nil
			}
			// The daemon being away is not the agent failing: wait for it
			// and read again. A refusal ends the runner, as before.
			// Just after the link dropped, a second reader is most likely our
			// own last long poll, still waiting on a daemon that never saw
			// the link go; it ends within one poll.
			if !isAway(err, 0) && !(isTwoReads(err) && bus.recovering(poll+readGrace)) {
				return err
			}
			if !bus.down(ctx, err) {
				fmt.Fprintf(ag.say, "%s: stopping\n", ag.Name)
				return nil
			}
			continue
		}
		bus.up()

		calls.Add(1)
		running.Add(1)
		go func(e protocol.Envelope) {
			defer running.Done()
			defer func() { <-slots }()
			handle(ag, e)
		}(e)
	}
}

// next is the one consume: long-poll, tell "nothing arrived" apart from a
// refusal, and decode. Every reader here goes through it — the agent loop,
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
		err := fmt.Errorf("%s", strings.TrimSpace(string(body)))
		if isAway(nil, code) {
			err = away{err}
		}
		if code == http.StatusConflict {
			err = twoReads{err}
		}
		return e, nil, false, err
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
func handle(ag agent, e protocol.Envelope) {
	// Receipts and the answer all go where the request said they should,
	// which is the sender unless it named a third party.
	// See docs/04-messaging.md#reply-routing.
	// Nobody is waiting any more, so the work is not worth doing — that is
	// the whole reason the deadline travels. Checked before the ack, because
	// an ack for a request that will never be run says the opposite of the
	// truth. See docs/04-messaging.md#request-and-reply.
	if e.TooLate(time.Now()) {
		fmt.Fprintf(ag.say, "%s: %s arrived after its caller gave up at %s; not run\n",
			ag.Name, e.ID, e.Deadline.Format(time.RFC3339))
		return
	}
	back, topic, tag := e.From, e.Topic, e.Tag
	if e.ReplyTo != nil {
		back, topic, tag = e.ReplyTo.Name, e.ReplyTo.Topic, e.ReplyTo.Tag
	}
	// While the daemon is away, a receipt or answer is retried until the
	// caller stops waiting — its deadline, or answerFor without one — so a
	// short restart loses nothing that was already worked out.
	until := time.Now().Add(answerFor)
	if !e.Deadline.IsZero() {
		until = e.Deadline
	}
	stop := ag.stop
	if stop == nil {
		stop = context.Background()
	}
	say := func(kind string) {
		// The ack is tried once: waiting on it would hold the work up.
		by := until
		if kind == protocol.ReceiptAck {
			by = time.Now()
		}
		if err := deliver(stop, by, "/send", protocol.Envelope{
			To: back, Topic: topic, Tag: tag, Receipt: kind, Re: e.ID,
		}); err != nil {
			fmt.Fprintf(ag.say, "%s: could not %s %s: %v\n", ag.Name, kind, e.ID, err)
		}
	}
	// ack first: the agent has the message, whatever happens next.
	say(protocol.ReceiptAck)

	argv := scriptArgv(ag.Script, ag.Algo, e.Body, e.Roles)
	// The envelope is in the environment either way, so a script can route on
	// it without parsing anything — and it is STATED rather than exported,
	// because a sandboxed child starts from the manager's environment and
	// inherits nothing of ours (docs/06-runner-role.md#sandboxing).
	env := []string{
		"AGENT_BUS_MESSAGE_ID=" + e.ID,
		"AGENT_BUS_FROM=" + e.From,
		"AGENT_BUS_TO=" + e.To,
		"AGENT_BUS_TOPIC=" + e.Topic,
		"AGENT_BUS_TAG=" + e.Tag,
		"AGENT_BUS_WORK=" + ag.work,
	}
	line := ag.box.Wrap(ports.Job{
		Argv: argv, Work: ag.work, Read: ag.read, Env: env, Net: ag.Network,
	})
	cmd := exec.Command(line[0], line[1:]...)
	if ag.Algo != algoArgs {
		cmd.Stdin = strings.NewReader(string(mustJSON(e)) + "\n")
	}
	cmd.Dir = ag.work
	// None inherited: an AB_ROLE_* in the runner's own environment would be
	// a role nobody holds.
	cmd.Env = append(withoutRoles(os.Environ()), env...)
	cmd.Stderr = ag.say

	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(ag.say, "%s: %s exited badly for %s: %v\n", ag.Name, ag.Script, e.ID, err)
		return
	}
	// Nothing printed is not an empty answer but a quiet success, and `done`
	// is how the caller hears of it (see above).
	answer := strings.TrimRight(string(out), "\n")
	if answer == "" {
		say(protocol.ReceiptDone)
		return
	}
	if err := deliver(stop, until, "/send", protocol.Envelope{
		To: back, Topic: topic, Tag: tag, Body: answer,
	}); err != nil {
		fmt.Fprintf(ag.say, "%s: could not answer %s: %v\n", ag.Name, e.ID, err)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// scriptArgv is the child's command line: the script, with the body as its
// one argument for --algo args, preceded by the sender's roles. The child's
// own shell unsets every AB_ROLE_* it started with and then exports only the
// roles the daemon worked out, because a sandboxed child starts from the
// user manager's environment, which the runner can neither see nor strip
// (Plans/R1.0-Release/roles.md#roles-are-for-agents).
func scriptArgv(script, algo, body string, roles []string) []string {
	prelude := rolePrelude(roles)
	if algo == algoArgs {
		return []string{"sh", "-c", prelude + script + ` "$@"`, "sh", body}
	}
	return []string{"sh", "-c", prelude + script}
}

// rolePrelude clears inherited roles and sets the held ones. A role name is
// lowercase letters, digits and _, so upper-cased it is a shell name already;
// anything else is skipped rather than quoted into the command.
func rolePrelude(roles []string) string {
	var b strings.Builder
	// export -p is the shell's own list, so this needs no program a
	// confined child may not have. Its words include pieces of values, so a
	// word is only a candidate: one that is not a shell name is skipped, and
	// unsetting an AB_ROLE_* that was never set costs nothing. Globbing is off
	// while it runs, and the loop's variable is AB_ROLE_ itself, cleared with
	// the rest, so no variable of the script's is touched.
	b.WriteString(`set -f; for AB_ROLE_ in $(export -p); do case ${AB_ROLE_%%=*} in AB_ROLE_*[!A-Za-z0-9_]*) ;; AB_ROLE_?*) unset "${AB_ROLE_%%=*}";; esac; done; unset AB_ROLE_; set +f; `)
	for _, kv := range roleEnv(roles) {
		b.WriteString("export " + kv + "; ")
	}
	return b.String()
}

// roleEnv is AB_ROLE_<NAME>=1 for each role held, the name upper-cased.
func roleEnv(roles []string) []string {
	out := make([]string, 0, len(roles))
	for _, r := range roles {
		if !validRole(r) {
			continue
		}
		out = append(out, "AB_ROLE_"+strings.ToUpper(r)+"=1")
	}
	return out
}

// withoutRoles is env with every AB_ROLE_* taken out.
func withoutRoles(env []string) []string {
	out := env[:0:0]
	for _, kv := range env {
		if !strings.HasPrefix(kv, "AB_ROLE_") {
			out = append(out, kv)
		}
	}
	return out
}

// validRole is a role as the daemon writes one: lowercase letters, digits and _.
func validRole(r string) bool {
	if r == "" || len(r) > 64 {
		return false
	}
	for _, c := range r {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}

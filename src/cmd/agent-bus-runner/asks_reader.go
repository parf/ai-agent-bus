package main

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/agentmeta"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// RT25(b): the runner reads asks, and only asks, for a name that has no
// ordinary reader of the runner's own: an entry that is refused or
// unconfigured and has a token, and a kept agent until its process starts. It
// answers `status` from what the runner observes and the help asks from the
// manifest, `unsupported: <ask>` for the rest, never runs the script and never
// takes an ordinary message. Once a kept agent's process runs, the process is
// the reader and the runner's goes (RD118: a running agent's status reaches
// the script).

// asker is one name's asks-only reader, stopped by its cancel.
type asker struct{ cancel context.CancelFunc }

const askPoll = 2 * time.Second

// readerless is the names this runner answers asks for, with their entries.
// It holds r.mu.
func (r *runner) readerless() map[string]installed {
	out := map[string]installed{}
	for _, in := range r.installed {
		if !in.enabled() {
			continue
		}
		name := r.agentName(in)
		if in.Refused != "" || in.Unconfigured {
			if _, err := os.Stat(r.tokenFile(name)); err == nil {
				out[name] = in
			}
			continue
		}
		if s := r.served[name]; s != nil && s.kept != nil {
			if n, _ := s.kept.snapshot(); n == 0 {
				out[name] = in
			}
		}
	}
	return out
}

// syncAskers starts a reader for each name that needs one and ends the rest.
// It holds r.mu.
func (r *runner) syncAskers(ctx context.Context) {
	if r.askers == nil {
		r.askers = map[string]*asker{}
	}
	want := r.readerless()
	for name, a := range r.askers {
		if _, ok := want[name]; !ok {
			a.cancel()
			delete(r.askers, name)
		}
	}
	for name := range want {
		if r.askers[name] != nil {
			continue
		}
		as := r.bus
		if s := r.served[name]; s != nil {
			as = s.bus
		} else if b, err := os.ReadFile(r.tokenFile(name)); err == nil {
			as = r.bus.as(strings.TrimSpace(string(b)))
		} else {
			continue
		}
		child, cancel := context.WithCancel(ctx)
		a := &asker{cancel: cancel}
		r.askers[name] = a
		go r.askLoop(child, name, a, as)
	}
}

// askLoop reads asks for one name until it is not wanted any more.
func (r *runner) askLoop(ctx context.Context, name string, self *asker, as *client) {
	defer func() {
		r.mu.Lock()
		if r.askers[name] == self {
			delete(r.askers, name)
		}
		r.mu.Unlock()
	}()
	for ctx.Err() == nil {
		r.mu.Lock()
		_, wanted := r.readerless()[name]
		r.mu.Unlock()
		if !wanted {
			return
		}
		e, got, err := as.nextAsks(ctx, askPoll)
		if err != nil {
			select {
			case <-time.After(askPoll):
			case <-ctx.Done():
			}
			continue
		}
		if got {
			r.answerReaderless(name, as, e)
		}
	}
}

// answerReaderless answers one ask for a name with no running process of its own.
func (r *runner) answerReaderless(name string, as *client, e protocol.Envelope) {
	if e.Receipt != "" || e.Ask == "" || e.TooLate(time.Now()) {
		return
	}
	r.mu.Lock()
	var in installed
	for _, candidate := range r.installed {
		if r.agentName(candidate) == name {
			in = candidate
			break
		}
	}
	kept := r.served[name]
	handlers := map[string]agentmeta.AskHandler{}
	if in.Agent.Asks != "none" && agentmeta.Manifest(in.Agent).Check() == nil {
		handlers = agentmeta.Manifest(in.Agent).AskHandlers(r.codeDir(in), e.Roles, name)
	}
	failed := in.Refused != "" || in.Unconfigured
	r.mu.Unlock()
	state := "starting: waiting for reader"
	switch {
	case failed:
		state = failedStatus(in)
	case kept != nil:
		if observed := r.observedStatus(kept); observed != "" {
			state = observed
		}
	}
	handlers[protocol.AskStatus] = func() (string, error) { return state, nil }
	body, _, err := agentmeta.DispatchAsk(e.Ask, handlers)
	if err != nil {
		body = "error: " + err.Error()
	}
	r.answerAs(as, e, body)
}

// syncAskersNow is syncAskers once an entry has just become refused. It holds r.mu.
func (r *runner) syncAskersNow() {
	if r.ctx != nil && r.ctx.Err() == nil {
		r.syncAskers(r.ctx)
	}
}

// askersAgain is syncAskers for a kept process that has just exited: until it
// starts again the runner answers asks for the name.
func (r *runner) askersAgain() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ctx != nil && r.ctx.Err() == nil {
		r.syncAskers(r.ctx)
	}
}

package main

import (
	"context"
	"io"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// Against a real daemon and its #bus reader: what the runner observes of an
// agent reaches that agent's own log through the real write verb, and the
// no-wait write leaves the agent's inbox empty after the daemon has completed
// it, so nothing the runner reads on the agent's behalf is ever an answer to it.
func TestRunnerEventsReachTheAgentsLogAndLeaveItsInboxEmpty(t *testing.T) {
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens(), "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	server := api.New(b, tokens, "owner@h")
	t.Cleanup(server.Close)
	if err := b.EstablishDaemonAgent("node.example"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "#a@h", Kind: protocol.KindAgent, Owner: "owner@h", Allow: []string{"*"}}); err != nil {
		t.Fatal(err)
	}
	stop := server.ServeDaemonAgent(context.Background())
	defer stop()
	httpServer := httptest.NewServer(server.Handler())
	t.Cleanup(httpServer.Close)
	tok, err := tokens.Issue("#a@h")
	if err != nil {
		t.Fatal(err)
	}
	agent := &client{http: httpServer.Client(), base: httpServer.URL, token: tok}

	// The runner's real event path: its sink, its real call to #bus, the
	// agent's own client.
	r := &runner{ctx: context.Background(), say: io.Discard, name: "#runner@h", served: map[string]*served{}}
	r.events = newEventSink(r)
	r.events.start()
	s := &served{name: "#a@h", bus: agent}
	r.event(s, levelNotice, "runner started serving this agent")
	w := r.stderrOf(s, []string{"API_KEY=sekret-value-123"})
	io.WriteString(w, "connecting with sekret-value-123\n")
	r.event(s, levelAlert, "process exited unexpectedly, code 3")
	r.events.close() // drains the queue through the real call

	// The completion fence: a write that waits, on the same single #bus reader,
	// is answered only after every earlier call has been completed.
	var accepted protocol.Envelope
	fence := protocol.Envelope{To: "#bus", Topic: "fence", Tag: "f1", Body: "write #a@h notice fence", Wait: "5s"}
	if err := agent.post("/send", fence, &accepted); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	first, ok, err := agent.consume(ctx, 5*time.Second, url.Values{"wait": {"5s"}})
	if err != nil || !ok {
		t.Fatalf("no answer to the waiting write: %v %v", ok, err)
	}
	if first.Re != accepted.ID || strings.HasPrefix(first.Body, "error:") {
		t.Fatalf("the first message in the agent's inbox is %+v, want only the answer to the fence: the runner's writes answered into it", first)
	}
	if rec, found := b.Lookup("owner@h", "#a@h"); !found || rec.Queued != 0 {
		t.Fatalf("the agent's inbox holds %d messages after the runner's writes (found %v)", rec.Queued, found)
	}

	// And the observations are in the agent's own log, masked.
	log, err := b.EventRead("owner@h", "#a@h", "")
	if err != nil {
		t.Fatal(err)
	}
	var notices, warnings, alerts []string
	for _, e := range log.Rings["notice"] {
		notices = append(notices, e.Text)
	}
	for _, e := range log.Rings["warning"] {
		warnings = append(warnings, e.Text)
	}
	for _, e := range log.Rings["alert"] {
		alerts = append(alerts, e.Text)
	}
	// Counts, so a duplicated event fails: the start notice and the fence, one
	// masked warning, one alert.
	if len(notices) != 2 || !contains(notices, "runner started serving this agent") || len(warnings) != 1 || warnings[0] != "connecting with ***" || len(alerts) != 1 || alerts[0] != "process exited unexpectedly, code 3" {
		t.Fatalf("the agent's log holds notices %v, warnings %v, alerts %v", notices, warnings, alerts)
	}
	for _, text := range warnings {
		if strings.Contains(text, "sekret") {
			t.Fatalf("a secret reached the log: %q", text)
		}
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

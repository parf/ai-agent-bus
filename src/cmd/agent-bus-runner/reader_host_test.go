package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A served agent's reads state the runner's host and, when agents.json says
// share, say they are one of a pool: its listing shows the host, and a second
// unfiltered reader that also shares is let in beside it while one that does not
// is refused.
func TestRunnerReadsStateTheirHostAndShare(t *testing.T) {
	r, b, _ := configurationFixture(t, "", "")
	write := func(entry map[string]any) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(r.host, "agents.json"), mustJSON(map[string]any{"fixture": entry}), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// The first reader is stopped and its poll dropped before the pool reader starts.
	write(map[string]any{"sandbox": "off", "enabled": false})
	r.reload()
	for end := time.Now().Add(5 * time.Second); b.Status().Waiting != 0; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(end) {
			t.Fatalf("%d readers still waiting after the agent was disabled", b.Status().Waiting)
		}
	}
	r.readHost = "pool-host"
	write(map[string]any{"sandbox": "off", "share": true})
	r.reload()
	var hosts []string
	for end := time.Now().Add(5 * time.Second); len(hosts) == 0; time.Sleep(10 * time.Millisecond) {
		rec, _ := b.Lookup("owner@h", "#fixture@h")
		for _, h := range rec.Hosts {
			hosts = append(hosts, h.Name)
		}
		if time.Now().After(end) {
			t.Fatal("the runner's read never stated its host")
		}
	}
	if len(hosts) != 1 || hosts[0] != "pool-host" {
		t.Fatalf("hosts %v, want pool-host", hosts)
	}
	wait := func(share bool) error {
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()
		_, err := b.ConsumeAs(ctx, "owner@h", "#fixture@h", "", "", false, share)
		return err
	}
	if err := wait(true); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a second sharing reader beside the pool member: %v, want it admitted and left waiting", err)
	}
	if err := wait(false); !errors.Is(err, core.ErrTwoReads) {
		t.Fatalf("a second reader that does not share: %v, want it refused as a second reader", err)
	}
}

// Two runners serve one name, as a pool: the second runner is handed the
// name's token once, each says it shares and states its own host, and the
// messages are shared between them, each exactly once, while the listing shows
// both hosts (M.6 acceptance). A handler holds until a second handler has
// started (bounded), so the split does not depend on timing: one runner serves
// one message at a time, so the next must go to the other.
func TestTwoRunnersServeOneNameSharingMessagesAndTheListingShowsBothHosts(t *testing.T) {
	started := t.TempDir()
	body := "touch \"" + started + "/$$\"\n" +
		"for i in $(seq 1 100); do [ \"$(ls \"" + started + "\" | wc -l)\" -ge 2 ] && break; sleep 0.05; done\n" +
		"echo \"$AGENT_BUS_MESSAGE_ID\" >> \"$AGENT_BUS_WORK/handled\"\necho done\n"
	r1, b, _, tokens := configurationFixtureTokens(t, "", body, io.Discard)
	write := func(r *runner, entry map[string]any) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(r.host, "agents.json"), mustJSON(map[string]any{"fixture": entry}), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// The first runner's plain reader is stopped before the pool readers start.
	write(r1, map[string]any{"sandbox": "off", "enabled": false})
	r1.reload()
	for end := time.Now().Add(5 * time.Second); b.Status().Waiting != 0; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(end) {
			t.Fatalf("%d readers still waiting after the agent was disabled", b.Status().Waiting)
		}
	}
	// The second runner: its own account directory and runner agent, sharing agent.d.
	if _, err := b.Register(protocol.Record{Name: "#runner2@h", Kind: protocol.KindAgent, Owner: "owner@h"}); err != nil {
		t.Fatal(err)
	}
	tok2, err := tokens.Issue("#runner2@h")
	if err != nil {
		t.Fatal(err)
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	r2 := &runner{ctx: ctx2, name: "#runner2@h", person: "owner@h", realm: "h", host: filepath.Join(t.TempDir(), "runner2"), agents: r1.agents, say: io.Discard, served: make(map[string]*served)}
	r2.bus = &client{http: r1.bus.http, base: r1.bus.base, token: tok2}
	r2.addr = r1.addr
	t.Cleanup(func() { cancel2(); r2.stopKept() })
	if err := os.MkdirAll(filepath.Join(r2.host, "tokens"), 0o700); err != nil {
		t.Fatal(err)
	}
	kept, err := os.ReadFile(r1.tokenFile("#fixture@h"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r2.tokenFile("#fixture@h"), kept, 0o600); err != nil { // handed over once, by the Owner
		t.Fatal(err)
	}
	r1.readHost, r2.readHost = "host-one", "host-two"
	write(r1, map[string]any{"sandbox": "off", "share": true})
	write(r2, map[string]any{"sandbox": "off", "share": true})
	r1.reload()
	r2.reload()
	waitFor := func(what string, ok func() bool) {
		t.Helper()
		for end := time.Now().Add(8 * time.Second); !ok(); time.Sleep(10 * time.Millisecond) {
			if time.Now().After(end) {
				t.Fatalf("timed out waiting for %s", what)
			}
		}
	}
	waitFor("both runners reading", func() bool { return b.Status().Waiting >= 2 })
	const sent = 8
	want := map[string]bool{}
	for i := 0; i < sent; i++ {
		e, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: fmt.Sprintf("job %d", i)})
		if err != nil {
			t.Fatal(err)
		}
		want[e.ID] = true
	}
	handled := func(r *runner) []string {
		r.mu.Lock()
		s := r.served["#fixture@h"]
		r.mu.Unlock()
		if s == nil {
			return nil
		}
		raw, _ := os.ReadFile(filepath.Join(s.work, "handled"))
		return strings.Fields(string(raw))
	}
	waitFor("every message handled", func() bool { return len(handled(r1))+len(handled(r2)) >= sent })
	time.Sleep(200 * time.Millisecond)
	one, two := handled(r1), handled(r2)
	if len(one) == 0 || len(two) == 0 {
		t.Fatalf("runner one handled %d, runner two %d of %d: want both to serve", len(one), len(two), sent)
	}
	seen := map[string]int{}
	for _, id := range append(append([]string{}, one...), two...) {
		seen[id]++
	}
	if len(seen) != sent {
		t.Fatalf("%d distinct messages handled of %d sent: %v", len(seen), sent, seen)
	}
	for id, n := range seen {
		if !want[id] || n != 1 {
			t.Fatalf("message %s handled %d times (sent here: %v), want each message once", id, n, want[id])
		}
	}
	rec, _ := b.Lookup("owner@h", "#fixture@h")
	var hosts []string
	for _, h := range rec.Hosts {
		hosts = append(hosts, h.Name)
	}
	if strings.Join(hosts, ",") != "host-one,host-two" {
		t.Fatalf("the listing shows hosts %v, want host-one and host-two", hosts)
	}
}

// An agent that does not say share reads as one reader: a second plain read of
// its name is refused, whatever else the runner states on it.
func TestRunnerReadsOfAnAgentWithoutShareAreOneReader(t *testing.T) {
	r, b, _ := configurationFixture(t, "", "")
	r.readHost = "solo-host"
	r.reload()
	for end := time.Now().Add(5 * time.Second); b.Status().Waiting == 0; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(end) {
			t.Fatal("the runner's read never started waiting")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if _, err := b.ConsumeAs(ctx, "owner@h", "#fixture@h", "", "", false, false); !errors.Is(err, core.ErrTwoReads) {
		t.Fatalf("a second plain read beside an agent that does not share: %v, want it refused as a second reader", err)
	}
}

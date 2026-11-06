package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// replies collects the runner's answers to owner@h by topic, so a test can
// wait for one call's answer and count what has been answered so far.
type replies struct {
	t    *testing.T
	in   chan protocol.Envelope
	seen map[string]protocol.Envelope
}

func collectReplies(t *testing.T, b *core.Bus) *replies {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	rs := &replies{t: t, in: make(chan protocol.Envelope, 64), seen: map[string]protocol.Envelope{}}
	go func() {
		for {
			e, err := b.Consume(ctx, "owner@h", "", "", false, false)
			if err != nil {
				return
			}
			if e.Receipt == "" {
				rs.in <- e
			}
		}
	}()
	return rs
}

// wait returns the answer on topic, failing when it has not come within d.
func (rs *replies) wait(topic string, d time.Duration) protocol.Envelope {
	rs.t.Helper()
	end := time.After(d)
	for {
		if e, ok := rs.seen[topic]; ok {
			return e
		}
		select {
		case e := <-rs.in:
			rs.seen[e.Topic] = e
		case <-end:
			rs.t.Fatalf("no answer on %s within %v", topic, d)
		}
	}
}

// settle collects what arrives within d and returns the topics answered.
func (rs *replies) settle(d time.Duration) map[string]protocol.Envelope {
	end := time.After(d)
	for {
		select {
		case e := <-rs.in:
			rs.seen[e.Topic] = e
		case <-end:
			return rs.seen
		}
	}
}

func ownerCall(topic, body string) protocol.Envelope {
	return protocol.Envelope{ID: topic, From: "owner@h", To: "#runner@h", Topic: topic, Body: body, Roles: []string{"owner"}}
}

// dispatch hands one call to the inbox loop's handler and fails when the
// handler itself does not return at once: the loop must never wait on a control.
func dispatch(t *testing.T, r *runner, e protocol.Envelope) {
	t.Helper()
	done := make(chan struct{})
	go func() { r.handle(e); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("%q blocked the inbox loop", e.Body)
	}
}

func fixtureState(t *testing.T, body string) string {
	t.Helper()
	var st statusAnswer
	if err := json.Unmarshal([]byte(body), &st); err != nil {
		t.Fatalf("not a status answer: %v: %s", err, body)
	}
	for _, a := range st.Agents {
		if a.Key == "fixture" {
			return a.State
		}
	}
	t.Fatalf("no fixture row: %s", body)
	return ""
}

// blockedInstall starts an install of fixture by call and parks it under
// reloadMu, at its last check before the publish; the control worker is busy
// with it until the returned gate opens.
func blockedInstall(t *testing.T, r *runner) *hookGate {
	t.Helper()
	r.bundled = t.TempDir()
	directoryAgent(t, r.bundled, "fixture", "deployed")
	g := newHookGate(t)
	r.afterInstallCheck = g.hook
	dispatch(t, r, ownerCall("install", "install --replace fixture"))
	select {
	case <-g.reached:
	case <-time.After(10 * time.Second):
		t.Fatal("the install never reached its publish")
	}
	return g
}

// F1: a control waiting behind an install's publish does not hold up the
// inbox loop, and status answers at once that the package is installing.
func TestStatusAnswersAtOnceAndSaysInstallingWhileControlsWait(t *testing.T) {
	r, b, _ := configurationFixture(t, "", "printf old")
	rs := collectReplies(t, b)
	g := blockedInstall(t, r)
	dispatch(t, r, ownerCall("disable", "disable fixture"))
	dispatch(t, r, ownerCall("status", "status"))
	if got := fixtureState(t, rs.wait("status", time.Second).Body); got != "installing" {
		t.Fatalf("status during the install says %q, want installing", got)
	}
	g.open()
	if got := rs.wait("install", 10*time.Second).Body; strings.HasPrefix(got, "error:") {
		t.Fatal(got)
	}
	rs.wait("disable", 10*time.Second)
}

// F1: controls queued behind an install apply after it, in the order they
// were accepted, each answered on its own route.
func TestControlsQueuedBehindAnInstallApplyInOrder(t *testing.T) {
	r, b, _ := configurationFixture(t, "", "printf old")
	rs := collectReplies(t, b)
	g := blockedInstall(t, r)
	dispatch(t, r, ownerCall("disable", "disable fixture"))
	dispatch(t, r, ownerCall("enable", "enable fixture"))
	if early := rs.settle(200 * time.Millisecond); len(early) != 0 {
		t.Fatalf("answered before the install published: %v", early)
	}
	g.open()
	if got := rs.wait("install", 10*time.Second).Body; strings.HasPrefix(got, "error:") {
		t.Fatal(got)
	}
	if got := fixtureState(t, rs.wait("disable", 10*time.Second).Body); got != "disabled" {
		t.Fatalf("disable answered %q", got)
	}
	if got := fixtureState(t, rs.wait("enable", 10*time.Second).Body); got == "disabled" || got == "installing" {
		t.Fatalf("enable answered %q", got)
	}
	if got := directoryRow(t, r, "fixture"); !got.Enabled {
		t.Fatalf("the later enable did not win: %+v", got)
	}
}

// F1: over the bound a control is refused as busy at once, never blocking.
func TestAFullControlQueueRefusesBusy(t *testing.T) {
	r, b, _ := configurationFixture(t, "", "printf old")
	rs := collectReplies(t, b)
	blockedInstall(t, r)
	for i := 0; i < controlQueueMax; i++ {
		dispatch(t, r, ownerCall("queued-"+string(rune('a'+i)), "enable fixture"))
	}
	dispatch(t, r, ownerCall("over", "enable fixture"))
	if got := rs.wait("over", time.Second).Body; !strings.HasPrefix(got, "error: busy: ") {
		t.Fatalf("over the bound: %q", got)
	}
	if answered := rs.settle(200 * time.Millisecond); len(answered) != 1 {
		t.Fatalf("answered while the worker was held: %d calls, want only the busy one", len(answered))
	}
}

// F1: a stopping runner refuses every queued control by name, never drops one.
func TestShutdownRefusesQueuedControls(t *testing.T) {
	r, b, _ := configurationFixture(t, "", "printf old")
	ctx, cancel := context.WithCancel(r.ctx)
	r.ctx = ctx
	rs := collectReplies(t, b)
	g := blockedInstall(t, r)
	dispatch(t, r, ownerCall("first", "disable fixture"))
	dispatch(t, r, ownerCall("second", "enable fixture"))
	cancel()
	r.refuseQueued() // as run does when its context ends, before stopKept
	for _, topic := range []string{"first", "second"} {
		if got := rs.wait(topic, time.Second).Body; got != "error: runner stopping" {
			t.Fatalf("%s while stopping: %q", topic, got)
		}
	}
	g.open()
	dispatch(t, r, ownerCall("late", "enable fixture"))
	if got := rs.wait("late", time.Second).Body; got != "error: runner stopping" {
		t.Fatalf("a control after the stop: %q", got)
	}
}

// fetchGate parks an install's git fetch until the test opens it, then writes
// a package labelled label.
func fetchGate(t *testing.T, r *runner, label string) *hookGate {
	g := newHookGate(t)
	r.fetch = func(ctx context.Context, in installRequest, dest string) error {
		g.hook()
		writeFile(t, filepath.Join(dest, "agent.json"), `{"description":"Directory fixture","command":"./answer.sh","form":"args","deadline":"2s"}`)
		writeFile(t, filepath.Join(dest, "env.dist"), "FLAVOUR="+label+"\n")
		writeFile(t, filepath.Join(dest, "answer.sh"), "#!/bin/sh\nprintf '%s' \"$FLAVOUR\"\n")
		return os.Chmod(filepath.Join(dest, "answer.sh"), 0755)
	}
	return g
}

var gitInstall = "install --replace fixture git https://fixture.invalid/repo " + installPin

// F1: a stop or a remove carried out while an install is fetched supersedes
// it: the later call wins, and the install's caller is told so.
func TestAStopOrRemoveDuringAFetchSupersedesTheInstall(t *testing.T) {
	for _, verb := range []string{"stop", "remove"} {
		t.Run(verb, func(t *testing.T) {
			r, b, _ := configurationFixture(t, "", "printf old")
			rs := collectReplies(t, b)
			before, _ := os.ReadFile(filepath.Join(r.agents, "fixture", "agent.json"))
			g := fetchGate(t, r, "deployed")
			dispatch(t, r, ownerCall("install", gitInstall))
			select {
			case <-g.reached:
			case <-time.After(10 * time.Second):
				t.Fatal("the fetch never began")
			}
			dispatch(t, r, ownerCall(verb, verb+" fixture"))
			if got := rs.wait(verb, 10*time.Second).Body; strings.HasPrefix(got, "error:") {
				t.Fatal(got)
			}
			g.open()
			if got := rs.wait("install", 10*time.Second).Body; !strings.HasPrefix(got, "error: superseded by "+verb+":") {
				t.Fatalf("the install after a %s: %q", verb, got)
			}
			after, err := os.ReadFile(filepath.Join(r.agents, "fixture", "agent.json"))
			if verb == "remove" && !os.IsNotExist(err) {
				t.Fatalf("the dropped install brought back a removed package: %v", err)
			}
			if verb == "stop" && string(after) != string(before) {
				t.Fatal("the dropped install changed the live package")
			}
		})
	}
}

// F1: a stop after an install published is an ordinary stop of the new package.
func TestAStopAfterThePublishStopsTheInstalledPackage(t *testing.T) {
	r, b, _ := configurationFixture(t, "", "printf old")
	rs := collectReplies(t, b)
	g := fetchGate(t, r, "deployed")
	g.open()
	dispatch(t, r, ownerCall("install", gitInstall))
	if got := rs.wait("install", 10*time.Second).Body; strings.HasPrefix(got, "error:") {
		t.Fatal(got)
	}
	dispatch(t, r, ownerCall("stop", "stop fixture"))
	if got := fixtureState(t, rs.wait("stop", 10*time.Second).Body); got != "stopped" {
		t.Fatalf("stop after the publish answered %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(r.agents, "fixture", "env.dist")); string(got) != "FLAVOUR=deployed\n" {
		t.Fatalf("the published package is not live: %q", got)
	}
}

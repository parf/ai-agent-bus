package main

import (
	"bytes"
	"context"
	"io"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// Real daemon handlers/inboxes, temporary scripts and credentials only.
func configurationFixture(t *testing.T, serves, body string) (*runner, *core.Bus, string) {
	t.Helper()
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens(), "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	server := api.New(b, tokens, "owner@h")
	t.Cleanup(server.Close)
	for _, name := range []string{"#runner@h", "#fixture@h"} {
		if _, err := b.Register(protocol.Record{Name: name, Kind: protocol.KindAgent, Owner: "owner@h"}); err != nil {
			t.Fatal(err)
		}
	}
	httpServer := httptest.NewServer(server.Handler())
	t.Cleanup(httpServer.Close)
	ctx, cancel := context.WithCancel(context.Background())
	root := t.TempDir()
	r := &runner{ctx: ctx, name: "#runner@h", person: "owner@h", realm: "h", host: filepath.Join(root, "runner"), agents: filepath.Join(root, "agent.d"), say: io.Discard, served: make(map[string]*served)}
	tok, err := tokens.Issue(r.name)
	if err != nil {
		t.Fatal(err)
	}
	r.bus = &client{http: httpServer.Client(), base: httpServer.URL, token: tok}
	r.addr = httpServer.URL
	dir := filepath.Join(r.agents, "fixture")
	for _, path := range []string{dir, filepath.Join(r.host, "tokens")} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	script := filepath.Join(dir, "agent.sh")
	if err := os.WriteFile(script, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "agent.json"), mustJSON(map[string]any{"description":"Test agent","command": "/bin/sh " + script, "serves": serves, "processes": 1, "deadline": "100ms"}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.host, "agents.json"), []byte(`{"fixture":{"sandbox":"off"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	tok, err = tokens.Issue("#fixture@h")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.tokenFile("#fixture@h"), []byte(tok), 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); r.stopKept() })
	r.reload()
	return r, b, script
}

func awaitConfigurationState(t *testing.T, r *runner, state string) agentStatus {
	t.Helper()
	end := time.Now().Add(2 * time.Second)
	for {
		st := r.status()
		if len(st.Agents) == 1 && st.Agents[0].State == state {
			return st.Agents[0]
		}
		if time.Now().After(end) {
			t.Fatalf("wanted state %q, got %+v", state, st)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func configurationReply(t *testing.T, b *core.Bus) protocol.Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		e, err := b.Consume(ctx, "owner@h", "", "", false, false)
		if err != nil {
			t.Fatalf("no script answer: %v", err)
		}
		if e.Receipt == "" {
			return e
		}
	}
}

func TestKeptExit78StopsUntilReloadWithLastStderrReason(t *testing.T) {
	r, _, script := configurationFixture(t, "itself", "echo started >> \"$AGENT_BUS_WORK/starts\"\nprintf 'older detail\nmissing token' >&2\nexit 78\n")
	st := awaitConfigurationState(t, r, "unconfigured")
	if st.Reason != "missing token" {
		t.Fatalf("last stderr reason: %q", st.Reason)
	}
	r.mu.Lock()
	if r.served["#fixture@h"] != nil {
		t.Error("unconfigured keeper remains served")
	}
	retired := r.retiring["fixture"]
	r.mu.Unlock()
	if retired != nil {
		select {
		case <-retired:
		case <-time.After(time.Second):
			t.Fatal("exit78 keeper still restarting")
		}
	}
	starts := filepath.Join(r.host, "work", "fixture", "starts")
	data, err := os.ReadFile(starts)
	if err != nil || strings.Count(string(data), "started") != 1 {
		t.Fatalf("exit78 restarted: %q %v", data, err)
	}
	if err := os.WriteFile(script, []byte("echo started >> \"$AGENT_BUS_WORK/starts\"\nsleep 5\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// Merely editing the file cannot resume a stopped keeper.
	if got := r.status().Agents[0].State; got != "unconfigured" {
		t.Fatalf("retried without reload: %s", got)
	}
	r.reload()
	awaitConfigurationState(t, r, "running")
}

func TestScriptExit78AnswersAndLeavesNextMessageQueuedUntilReload(t *testing.T) {
	r, b, script := configurationFixture(t, "runner", "printf 'older detail\nmissing configuration\n' >&2\nexit 78\n")
	// A transient zero-reader count between polls is not proof of shutdown.
	// Observe cancellation as well as the real daemon inbox below.
	r.mu.Lock()
	s := r.served["#fixture@h"]
	stopped := make(chan struct{})
	cancel := s.cancel
	s.cancel = func() { cancel(); close(stopped) }
	r.mu.Unlock()
	// Pin an actual outstanding read before the triggering message.
	deadline := time.Now().Add(time.Second)
	for {
		rec, _ := b.Lookup("owner@h", "#fixture@h")
		if rec.Readers != nil && *rec.Readers == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("script never read its inbox")
		}
		time.Sleep(2 * time.Millisecond)
	}
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: "first"}); err != nil {
		t.Fatal(err)
	}
	answer := configurationReply(t, b)
	if answer.From != "#fixture@h" || answer.Body != "error: #fixture@h is not configured: missing configuration" {
		t.Fatalf("configuration answer: %+v", answer)
	}
	st := awaitConfigurationState(t, r, "unconfigured")
	if st.Reason != "missing configuration" {
		t.Fatalf("status reason: %q", st.Reason)
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("exit78 did not cancel its reader")
	}
	deadline = time.Now().Add(time.Second)
	for {
		rec, _ := b.Lookup("owner@h", "#fixture@h")
		if rec.Readers != nil && *rec.Readers == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("exit78 kept reading its inbox")
		}
		time.Sleep(2 * time.Millisecond)
	}
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: "second"}); err != nil {
		t.Fatal(err)
	}
	if rec, _ := b.Lookup("owner@h", "#fixture@h"); rec.Queued != 1 {
		t.Fatalf("next message was read: %+v", rec)
	}
	if err := os.WriteFile(script, []byte("printf configured\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r.reload()
	if got := configurationReply(t, b).Body; got != "configured" {
		t.Fatalf("reload did not serve queued message: %q", got)
	}
}

func TestOtherScriptExitKeepsFailureBehavior(t *testing.T) {
	r, s, replies, _ := deadlineFixture(t, "printf ordinary >&2\nexit 77\n", "1s")
	r.runOne(s, protocol.Envelope{ID: "fixture", From: "caller@h"})
	replies.mu.Lock()
	defer replies.mu.Unlock()
	for _, e := range replies.replies {
		if e.Receipt == "" {
			t.Fatalf("ordinary failure gained an answer: %+v", e)
		}
	}
}

func TestExit78WithNoStderrStillAnswers(t *testing.T) {
	r, s, replies, _ := deadlineFixture(t, "exit 78\n", "1s")
	r.runOne(s, protocol.Envelope{ID: "fixture", From: "caller@h"})
	replies.mu.Lock()
	defer replies.mu.Unlock()
	for _, e := range replies.replies {
		if e.Receipt == "" && e.Body == "error: #fixture@h is not configured: " {
			return
		}
	}
	t.Fatalf("no-stderr exit78 answer missing: %+v", replies.replies)
}

func TestKeptOrdinaryExitStillRestarts(t *testing.T) {
	r, _, _ := configurationFixture(t, "itself", "echo started >> \"$AGENT_BUS_WORK/starts\"\nexit 77\n")
	end := time.Now().Add(2500 * time.Millisecond)
	for {
		data, _ := os.ReadFile(filepath.Join(r.host, "work", "fixture", "starts"))
		if strings.Count(string(data), "started") >= 2 {
			break
		}
		if time.Now().After(end) {
			t.Fatalf("ordinary exit stopped restarting: %q %+v", data, r.status())
		}
		time.Sleep(2 * time.Millisecond)
	}
	if st := r.status().Agents[0]; st.State == "unconfigured" {
		t.Fatalf("ordinary exit was configuration failure: %+v", st)
	}
}

func TestLastStderrLineAcrossWritesAndBoundedCapture(t *testing.T) {
	for _, tc := range []struct {
		chunks []string
		reason string
	}{
		{[]string{"first\nse", "cond\n"}, "second"},
		{[]string{"first\nsecond"}, "second"},
		{[]string{"first\n\n"}, ""},
		{[]string{"first\n", "last\r", "\n"}, "last"},
		{[]string{""}, ""},
	} {
		var log bytes.Buffer
		out := &stderrLine{out: &log}
		for _, chunk := range tc.chunks {
			if _, err := out.Write([]byte(chunk)); err != nil {
				t.Fatal(err)
			}
		}
		if out.reason() != tc.reason || log.String() != strings.Join(tc.chunks, "") {
			t.Fatalf("stderr retained=%q log=%q, want %q", out.reason(), log.String(), tc.reason)
		}
	}
	out := &stderrLine{out: io.Discard}
	out.Write([]byte(strings.Repeat("x", 2*maxOutput)))
	if len(out.line) > maxOutput {
		t.Fatalf("stderr capture grew to %d", len(out.line))
	}
	out.Write([]byte("\nfinal reason\n"))
	if out.reason() != "final reason" {
		t.Fatalf("lost final reason after large stderr: %q", out.reason())
	}
}

func TestRetiringKeeperConfigurationFailureCannotWaitOnItself(t *testing.T) {
	r, s, began, release, _ := retiringFixture(t, safeRetiringName)
	r.installed = []installed{s.in}
	r.mu.Lock()
	old := s.kept
	s.kept = nil // the existing command/env reload retirement sequence
	r.retire(s.in.Key, old, func() {})
	r.mu.Unlock()
	awaitRetirement(t, began)
	done := make(chan struct{})
	go func() { r.notConfigured(s, old, "obsolete failure"); close(done) }()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		release()
		awaitRetirement(t, done)
		t.Fatal("old keeper configuration failure waited on its own retirement")
	}
	r.mu.Lock()
	if r.served[s.name] != s || r.installed[0].Unconfigured {
		t.Error("retired keeper changed the current serving")
	}
	r.mu.Unlock()
	release()
}

func TestConfigurationExitWithoutCaptureDoesNotPanic(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 78")
	cmd.Stderr = io.Discard
	err := cmd.Run()
	if err == nil {
		t.Fatal("fixture must exit 78")
	}
	defer func() {
		if p := recover(); p != nil {
			t.Errorf("configuration exit with another stderr writer panicked: %v", p)
		}
	}()
	reason, unconfigured := configurationExit(cmd, err)
	if unconfigured || reason != "" {
		t.Errorf("an uncaptured exit must keep ordinary failure handling: %q, %v", reason, unconfigured)
	}
}

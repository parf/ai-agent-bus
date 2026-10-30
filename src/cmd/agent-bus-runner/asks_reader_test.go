package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func sendAsk(t *testing.T, b *core.Bus, ask string) {
	t.Helper()
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Ask: ask}); err != nil {
		t.Fatal(err)
	}
}

func queuedFor(b *core.Bus, name string) int {
	rec, _ := b.Lookup("owner@h", name)
	return rec.Queued
}

// askAnswer sends an ask and waits for the one reply, ignoring receipts.
func askAnswer(t *testing.T, b *core.Bus, ask string) string {
	t.Helper()
	sendAsk(t, b, ask)
	return configurationReply(t, b).Body
}

// An entry that is refused or unconfigured and has a token answers status and
// the help asks, and takes no ordinary message: the queue keeps it.
func TestReaderlessEntryAnswersAsksAndLeavesOrdinaryMessagesQueued(t *testing.T) {
	r, b, _ := configurationFixture(t, "itself", "printf 'missing token' >&2\nexit 78\n")
	awaitConfigurationState(t, r, "unconfigured")
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: "work"}); err != nil {
		t.Fatal(err)
	}
	if got := askAnswer(t, b, protocol.AskStatus); got != "failed: missing token" {
		t.Fatalf("status of an unconfigured entry: %q", got)
	}
	if got := askAnswer(t, b, protocol.AskHelp); !strings.Contains(got, "Test agent") {
		t.Fatalf("help from the manifest: %q", got)
	}
	if got := askAnswer(t, b, protocol.AskMCP); got != `{"tools":[]}` {
		t.Fatalf("mcp: %q", got)
	}
	if n := queuedFor(b, "#fixture@h"); n != 1 {
		t.Fatalf("the ordinary message must stay queued, %d queued", n)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	e, err := b.ConsumeAs(ctx, "owner@h", "#fixture@h", "", "", false, false)
	if err != nil || e.Body != "work" || e.Ask != "" {
		t.Fatalf("the queue keeps the ordinary message for its reader: %+v %v", e, err)
	}
}

// A package whose manifest cannot be read answers status only; every other ask
// is unsupported, and the script is never run.
func TestReaderlessEntryWithABadManifestAnswersStatusOnly(t *testing.T) {
	r, b, script := configurationFixture(t, "itself", "exit 0\n")
	marker := filepath.Join(filepath.Dir(script), "ran")
	if err := os.WriteFile(script, []byte("touch "+marker+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(script), "agent.json"), []byte(`{"description":`), 0600); err != nil {
		t.Fatal(err)
	}
	r.reload()
	end := time.Now().Add(3 * time.Second)
	for time.Now().Before(end) {
		if st := r.status(); len(st.Agents) == 1 && st.Agents[0].State == "refused" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := askAnswer(t, b, protocol.AskStatus); !strings.HasPrefix(got, "failed: ") {
		t.Fatalf("status: %q", got)
	}
	if got := askAnswer(t, b, protocol.AskHelp); got != "unsupported: help" {
		t.Fatalf("help of an unreadable manifest: %q", got)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("an ask ran the script")
	}
}

// The runner's asks-only reader is an outstanding consume request, so a listing counts it in
// readers; it takes no ordinary message, so reading (an unfiltered read) stays false.
func TestAsksOnlyReaderCountsInReadersAndIsNotReading(t *testing.T) {
	r, b, _ := configurationFixture(t, "itself", "printf 'missing token' >&2\nexit 78\n")
	awaitConfigurationState(t, r, "unconfigured")
	deadline := time.Now().Add(5 * time.Second)
	for {
		rec, _ := b.Lookup("owner@h", "#fixture@h")
		if rec.Readers != nil && *rec.Readers == 1 {
			if rec.Reading {
				t.Fatalf("an asks-only reader is reading: %+v", rec)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the asks-only reader never showed in readers: %+v", rec)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// A kept agent with no process answers status until its process starts; then
// the runner's reader is gone and the ask reaches the process.
func TestKeeperAnswersAsksOnlyUntilItsProcessRuns(t *testing.T) {
	r, b, script := configurationFixture(t, "itself", "exit 3\n")
	// The asks reader attaches a moment after the entry exists, and under load later than a fixed wait:
	// ask again whenever a second passes without a reply, and read until the deadline.
	var got string
	sendAsk(t, b, protocol.AskStatus)
	deadline := time.Now().Add(8 * time.Second)
	for !strings.HasPrefix(got, "failed: kept process exited") && time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		e, err := b.Consume(ctx, "owner@h", "", "", false, false)
		cancel()
		if err != nil {
			sendAsk(t, b, protocol.AskStatus)
			continue
		}
		if e.Receipt != "" {
			continue
		}
		if got = e.Body; !strings.HasPrefix(got, "failed: kept process exited") {
			sendAsk(t, b, protocol.AskStatus)
		}
	}
	if !strings.HasPrefix(got, "failed: kept process exited") {
		t.Fatalf("a keeper whose process is not running answers status: %q", got)
	}
	// An ordinary message waits through the handover and is the process's first.
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: "work"}); err != nil {
		t.Fatal(err)
	}
	// Now a process that stays: the runner's reader goes, whatever the process reads.
	if err := os.WriteFile(script, []byte("sleep 30\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r.reload()
	end := time.Now().Add(5 * time.Second)
	for {
		r.mu.Lock()
		running, readers := 0, len(r.askers)
		if s := r.served["#fixture@h"]; s != nil && s.kept != nil {
			running, _ = s.kept.snapshot()
		}
		r.mu.Unlock()
		if running > 0 && readers == 0 {
			break
		}
		if time.Now().After(end) {
			t.Fatalf("the process runs (%d) and the runner's asks-only readers (%d) must be gone", running, readers)
		}
		time.Sleep(10 * time.Millisecond)
	}
	// The process is the reader: the test plays it with the agent's own token and answers.
	process, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	taken := make(chan protocol.Envelope, 2)
	go func() {
		for i := 0; i < 2; i++ {
			e, _ := b.ConsumeAs(process, "#fixture@h", "#fixture@h", "", "", false, false)
			taken <- e
		}
	}()
	time.Sleep(100 * time.Millisecond)
	sendAsk(t, b, protocol.AskStatus)
	for _, want := range []func(protocol.Envelope) bool{
		func(e protocol.Envelope) bool { return e.Body == "work" && e.Ask == "" },
		func(e protocol.Envelope) bool { return e.Ask == protocol.AskStatus },
	} {
		select {
		case e := <-taken:
			if !want(e) {
				t.Fatalf("the process takes the ordinary message first, then the ask: %+v", e)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("the process did not get what the runner's reader left it")
		}
	}
}

// The runner's asks-only read and the agent's own unfiltered reader wait on one
// inbox together: an ordinary message goes to the unfiltered reader.
func TestAsksOnlyReaderAndOrdinaryReaderCoexist(t *testing.T) {
	r, b, _ := configurationFixture(t, "itself", "exit 78\n")
	awaitConfigurationState(t, r, "unconfigured")
	for end := time.Now().Add(3 * time.Second); b.Status().Waiting == 0 && time.Now().Before(end); time.Sleep(5 * time.Millisecond) {
	}
	if b.Status().Waiting == 0 {
		t.Fatal("the runner's asks-only reader never waited")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got := make(chan protocol.Envelope, 1)
	go func() {
		e, _ := b.ConsumeAs(ctx, "#fixture@h", "#fixture@h", "", "", false, false)
		got <- e
	}()
	time.Sleep(100 * time.Millisecond)
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: "work"}); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-got:
		if e.Body != "work" {
			t.Fatalf("the unfiltered reader takes the ordinary message: %+v", e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the unfiltered reader was refused or starved beside the asks-only reader")
	}
}

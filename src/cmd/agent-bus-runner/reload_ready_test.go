package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestReloadAnswerWaitsForReplacementReader(t *testing.T) {
	body := func(gated bool) string {
		setup := ""
		if gated {
			setup = `
open(os.environ['AGENT_BUS_WORK']+'/new-started','w').close()
while not os.path.exists(os.environ['AGENT_BUS_WORK']+'/attach'): time.sleep(.005)
`
		}
		return `exec python3 - <<'PY'
import os,time,urllib.request
` + setup + `
while True:
 request=urllib.request.Request(os.environ['AGENT_BUS_ADDR']+'/consume?wait=1s',headers={'X-Agent-Bus-Token':os.environ['AGENT_BUS_TOKEN']})
 with urllib.request.urlopen(request) as response: response.read()
PY
`
	}
	r, b, script := configurationFixture(t, "itself", body(false))
	until := func(ready func() bool) {
		t.Helper()
		end := time.Now().Add(2 * time.Second)
		for !ready() {
			if time.Now().After(end) {
				t.Fatal("fixture did not reach state")
			}
			time.Sleep(time.Millisecond)
		}
	}
	until(func() bool { record, _ := b.Lookup("owner@h", "#fixture@h"); return record.Reading })
	if err := os.WriteFile(script, []byte(body(true)), 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan protocol.Envelope, 1)
	dispatched := make(chan struct{})
	go func() {
		r.handle(protocol.Envelope{From: "owner@h", Body: "reload", Roles: []string{"owner"}})
		close(dispatched)
	}()
	select {
	case <-dispatched:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("reload blocks the control call loop")
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		e, _ := b.Consume(ctx, "owner@h", "", "", false, false)
		done <- e
	}()
	r.mu.Lock()
	work := r.served["#fixture@h"].work
	r.mu.Unlock()
	until(func() bool { _, err := os.Stat(filepath.Join(work, "new-started")); return err == nil })
	select {
	case <-done:
		t.Fatal("reload answered before the replacement reader attached")
	default:
	}
	statusDone := make(chan struct{})
	go func() { r.status(); r.reloadMu.Lock(); r.reloadMu.Unlock(); close(statusDone) }()
	select {
	case <-statusDone:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("reload readiness blocks status/control serialization")
	}
	if err := os.WriteFile(filepath.Join(work, "attach"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	var answer protocol.Envelope
	select {
	case answer = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("reload did not acknowledge attached replacement")
	}
	var status statusAnswer
	if err := json.Unmarshal([]byte(answer.Body), &status); err != nil {
		t.Fatal(err)
	}
	record, _ := b.Lookup("owner@h", "#fixture@h")
	if !record.Reading || status.Agents[0].State != "running" {
		t.Fatalf("replacement not ready: %+v", status)
	}
}

func TestReloadReportsStartingAtReadinessBound(t *testing.T) {
	r, _, script := configurationFixture(t, "itself", "exec sleep 5")
	if err := os.WriteFile(script, []byte("exec sleep 6"), 0600); err != nil {
		t.Fatal(err)
	}
	begin := time.Now()
	status := r.reloadStatus(context.Background(), 80*time.Millisecond)
	if elapsed := time.Since(begin); elapsed > 400*time.Millisecond {
		t.Fatalf("readiness exceeded bound: %s", elapsed)
	}
	if status.Agents[0].State != "starting" || status.Agents[0].Reason == "" {
		t.Fatalf("unattached reader reported ready: %+v", status)
	}
}

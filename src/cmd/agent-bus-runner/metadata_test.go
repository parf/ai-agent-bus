package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/agentmeta"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestRunnerAnswersPackageAsksWithoutScript(t *testing.T) {
	r, b, _ := configurationFixture(t, "", "printf script-was-run")
	s := r.served["#fixture@h"]
	s.in.Agent.Description = "Manifest fixture"
	s.in.Agent.Methods = map[string]agentmeta.Method{"read": {Help: "Read facts"}}
	if err := os.WriteFile(filepath.Join(s.dir, "env.dist"), []byte("# Token purpose\nTOKEN=sentinel-no-output\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, ask := range []string{protocol.AskHelp, protocol.AskHelpConfig, protocol.AskHelpSecret, protocol.AskHelpJSON, protocol.AskMCP} {
		r.runOne(s, protocol.Envelope{ID: "fixture", From: "owner@h", To: s.name, Ask: ask})
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		e, err := b.Consume(ctx, "owner@h", "", "", false, false)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		if e.Receipt != "" || strings.Contains(e.Body, "script-was-run") || strings.Contains(e.Body, "sentinel-no-output") {
			t.Fatalf("package ask ran script or leaked: %+v", e)
		}
		if json.Valid([]byte(e.Body)) != (ask == protocol.AskHelpJSON || ask == protocol.AskMCP) {
			t.Fatalf("wrong answer form for %s: %s", ask, e.Body)
		}
		want := map[string]string{"help": "Manifest fixture", "help-config": "No Configuration keys", "help-secret": "Token purpose", "help-json": `"methods"`, "mcp": `{"tools":[]}`}[ask]
		if !strings.Contains(e.Body, want) {
			t.Fatalf("%s: %s", ask, e.Body)
		}
	}
}

func TestRunnerForwardsNativeHelpAsks(t *testing.T) {
	r, b, _ := configurationFixture(t, "", `printf 'native:%s' "$AGENT_BUS_ASK"`)
	s := r.served["#fixture@h"]
	s.in.Agent.Help = "native"
	for _, ask := range []string{"help", "help-json", "help-config", "help-secret", "mcp"} {
		r.runOne(s, protocol.Envelope{ID: "native", From: "owner@h", To: s.name, Ask: ask})
		answer := configurationReply(t, b)
		if answer.Body != "native:"+ask {
			t.Fatalf("native %s intercepted: %s", ask, answer.Body)
		}
	}
}
func TestSelfServingDistDescribesWithoutInjectingOrRequiring(t *testing.T) {
	r, _, _ := configurationFixture(t, "", "true")
	s := r.served["#fixture@h"]
	s.in.Agent.Serves = "itself"
	if err := os.WriteFile(filepath.Join(s.dir, "env.dist"), []byte("# Required but self-serving\nREQUIRED=\nDEFAULT=metadata-only\n"), 0600); err != nil {
		t.Fatal(err)
	}
	env, why := r.environment(s.in, s.bus)
	if why != "" {
		t.Fatalf("metadata requirements enforced: %s", why)
	}
	for _, kv := range env {
		if strings.HasPrefix(kv, "REQUIRED=") || strings.HasPrefix(kv, "DEFAULT=") {
			t.Fatalf("metadata default injected: %s", kv)
		}
	}
}

func TestRunnerNoAsksAndStatusPrecedence(t *testing.T) {
	r, b, _ := configurationFixture(t, "", `echo run >> "$AGENT_BUS_WORK/ask-marker"; printf 'script:%s:' "$AGENT_BUS_ASK"; python3 -c 'import json,sys; print(json.load(sys.stdin).get("body",""))'`)
	s := r.served["#fixture@h"]
	s.in.Agent.Asks = "none"
	for _, ask := range []string{"help", "help-config", "help-secret", "help-json", "mcp", "status", "unknown"} {
		r.runOne(s, protocol.Envelope{ID: "no-asks", From: "owner@h", To: s.name, Ask: ask, Body: "must-be-empty"})
		e := configurationReply(t, b)
		if ask == "status" || ask == "unknown" {
			if e.Body != "unsupported: "+ask {
				t.Fatal(e.Body)
			}
		}
		if _, err := os.Stat(filepath.Join(s.work, "ask-marker")); !os.IsNotExist(err) {
			t.Fatalf("ask %s ran script", ask)
		}
	}
	for _, state := range []string{"starting", "stopping", "failed"} {
		r.ctx = context.Background()
		r.installed[0].Refused = ""
		r.retiring = map[string]chan struct{}{}
		switch state {
		case "starting":
			r.retiring[s.in.Key] = make(chan struct{})
		case "failed":
			r.installed[0].Refused = "needs KEY"
		case "stopping":
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			r.ctx = ctx
		}
		r.runOne(s, protocol.Envelope{ID: state, From: "owner@h", To: s.name, Ask: "status"})
		e := configurationReply(t, b)
		if state == "starting" {
			close(r.retiring[s.in.Key])
		}
		if !strings.HasPrefix(e.Body, state+":") {
			t.Fatalf("%s: %s", state, e.Body)
		}
	}
	r.ctx = context.Background()
	r.retiring = nil
	r.installed[0].Refused = ""
	r.installed[0].LastExit = &exitStatus{Code: 1, Reason: "needs KEY"}
	r.installed[0].Unconfigured = true
	r.runOne(s, protocol.Envelope{ID: "unconfigured", From: "owner@h", To: s.name, Ask: "status"})
	if e := configurationReply(t, b); e.Body != "failed: needs KEY" {
		t.Fatal(e.Body)
	}
	r.installed[0].LastExit = nil
	if state := r.observedStatus(s); state != "failed: agent is not configured" {
		t.Fatal(state)
	}
	r.installed[0].Unconfigured = false
	r.installed[0].LastExit = &exitStatus{Code: 1, Reason: "bad request"}
	r.runOne(s, protocol.Envelope{ID: "ordinary-exit", From: "owner@h", To: s.name, Ask: "status"})
	if e := configurationReply(t, b); e.Body != "unsupported: status" {
		t.Fatal(e.Body)
	}
	probe := &served{in: s.in, kept: &keeper{running: map[*exec.Cmd]keptProcess{}}}
	if state := r.observedStatus(probe); state != "failed: kept process exited" {
		t.Fatal(state)
	}
	r.installed[0].LastExit = nil
	if state := r.observedStatus(probe); !strings.HasPrefix(state, "starting:") {
		t.Fatal(state)
	}
	s.in.Agent.Asks = ""
	r.runOne(s, protocol.Envelope{ID: "forward-status", From: "owner@h", To: s.name, Ask: "status", Body: "discard"})
	if e := configurationReply(t, b); e.Body != "script:status:" {
		t.Fatal(e.Body)
	}
}

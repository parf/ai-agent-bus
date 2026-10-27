package main

import (
	"encoding/json"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"strings"
	"testing"
)

func TestStatusLineUsesRealAgentStates(t *testing.T) {
	st := statusAnswer{Agents: []agentStatus{
		{Key: "chat", Enabled: true, State: "running", Processes: &processes{Running: 1, Min: 1}},
		{Key: "ai", Enabled: true, State: "idle", Processes: &processes{Min: 0}},
		{Key: "off", Enabled: false, State: "disabled"},
	}}
	if got := statusLine(st); got != "ok: 1/1 running, 1 on demand (ai)" {
		t.Fatal(got)
	}
	st.Agents[0].State = "starting"
	st.Agents[0].Processes.Running = 0
	if got := statusLine(st); got != "degraded: 0/1 running, 1 on demand (ai); chat (starting)" {
		t.Fatal(got)
	}
	st.Agents[1].State = "refused"
	st.Agents[1].Processes = nil
	if got := statusLine(st); !strings.HasPrefix(got, "failed:") || !strings.Contains(got, "ai (refused)") {
		t.Fatal(got)
	}
	st.Error = "broken inventory"
	if got := statusLine(st); got != "failed: no agents loaded" {
		t.Fatal(got)
	}
}
func TestStatusAskIsOneLineAndStatusMethodRemainsJSON(t *testing.T) {
	r, b, _ := configurationFixture(t, "", "true")
	r.handle(protocol.Envelope{ID: "ask", From: "owner@h", Ask: "status"})
	line := configurationReply(t, b).Body
	if strings.ContainsAny(line, "\n\r") || !(strings.HasPrefix(line, "ok:") || strings.HasPrefix(line, "degraded:") || strings.HasPrefix(line, "failed:")) {
		t.Fatalf("status ask is not a health line: %s", line)
	}
	r.handle(protocol.Envelope{ID: "method", From: "owner@h", Body: "status"})
	var full statusAnswer
	if err := json.Unmarshal([]byte(configurationReply(t, b).Body), &full); err != nil || len(full.Agents) != 1 {
		t.Fatalf("status method lost JSON: %v %+v", err, full)
	}
}

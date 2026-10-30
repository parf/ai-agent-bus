package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// The normal shell gate makes an assertion about the runner's actual reply.
// Exercise its exact grep expression against a real reply, without rerunning
// every preceding shell fixture just to discover a stale command list.
func TestRunnerUnknownCallMatchesOwningSmokeAssertion(t *testing.T) {
	r, b, _ := configurationFixture(t, "", "printf ready")
	r.handle(protocol.Envelope{From: "owner@h", Body: "frobnicate", Roles: []string{"owner"}})
	reply := configurationReply(t, b)
	if !strings.HasPrefix(reply.Body, "error:") || !strings.Contains(reply.Body, `not "frobnicate"`) {
		t.Fatalf("unknown call did not refuse: %s", reply.Body)
	}
	script, err := os.ReadFile("../../smoke.sh")
	if err != nil {
		t.Fatal(err)
	}
	var pattern string
	for _, line := range strings.Split(string(script), "\n") {
		if strings.HasPrefix(line, `has "the runner refuses a call it does not answer" `) {
			end := strings.LastIndex(line, "'")
			if end < 0 {
				t.Fatal("smoke assertion lost its quoted pattern")
			}
			start := strings.LastIndex(line[:end], "'")
			if start < 0 {
				t.Fatal("smoke assertion lost its pattern opener")
			}
			pattern = line[start+1 : end]
		}
	}
	if pattern == "" {
		t.Fatal("owning unknown-call assertion missing")
	}
	data, err := json.Marshal([]protocol.Envelope{reply})
	if err != nil {
		t.Fatal(err)
	}
	grep := exec.Command("grep", "-qE", "--", pattern)
	grep.Stdin = bytes.NewReader(data)
	if err := grep.Run(); err != nil {
		t.Fatalf("owning smoke assertion does not match runner's actual refusal: pattern=%s reply=%s", pattern, data)
	}
}

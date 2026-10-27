package main

import (
	"context"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReloadSwitchesServingModeBothWays(t *testing.T) {
	r, b, script := configurationFixture(t, "runner", `printf per-message`)
	send := func(want string) {
		t.Helper()
		if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: "probe"}); err != nil {
			t.Fatal(err)
		}
		if got := configurationReply(t, b).Body; got != want {
			t.Fatalf("answer %q, want %q", got, want)
		}
	}
	send("per-message")
	// A real self-serving child consumes and answers using only its own credential.
	body := `exec python3 - <<'PY'
import os,json,urllib.request
base=os.environ['AGENT_BUS_ADDR']
headers={'X-Agent-Bus-Token':os.environ['AGENT_BUS_TOKEN'],'Content-Type':'application/json'}
while True:
 with urllib.request.urlopen(urllib.request.Request(base+'/consume?wait=1s',headers=headers)) as response:
  raw=response.read()
 if not raw: continue
 message=json.loads(raw)
 if message.get('receipt'): continue
 answer={'to':message['from'],'body':'self-serving'}
 with urllib.request.urlopen(urllib.request.Request(base+'/send',data=json.dumps(answer).encode(),headers=headers)) as response: response.read()
PY
`
	if err := os.WriteFile(script, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(filepath.Dir(script), "agent.json")
	setMode := func(mode string) {
		t.Helper()
		if err := os.WriteFile(manifest, mustJSON(map[string]any{"description": "fixture", "command": "/bin/sh " + script, "serves": mode, "processes": 1, "deadline": "100ms"}), 0600); err != nil {
			t.Fatal(err)
		}
		// Keep interpreter startup outside the two-second message reply budget.
		r.reloadStatus(context.Background(), 5*time.Second)
	}
	setMode("itself")
	send("self-serving")
	if err := os.WriteFile(script, []byte(`printf per-message-again`), 0600); err != nil {
		t.Fatal(err)
	}
	setMode("runner")
	send("per-message-again")
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.served["#fixture@h"].kept != nil {
		t.Fatal("old self-serving mode remains")
	}
}

package tokencmd

import (
	"strings"
	"testing"
)

func TestRotateWithAKeyRotatesAfterTheProof(t *testing.T) {
	log := daemon(t, "agent-bus")
	key, _ := keygen(t)
	out, err := stdout(t, func() error { return issue([]string{"alice@test", "--key=" + key, "--rotate"}) })
	if err != nil || out != "ROTATED-TOKEN" {
		t.Fatalf("issue printed %q, %v; want the rotated token", out, err)
	}
	calls := log()
	if len(calls) != 3 || calls[2].path != "/token" {
		t.Fatalf("the daemon saw %+v, want the proof then POST /token", calls)
	}
	if calls[2].token != "PROOF-TOKEN" || !strings.Contains(calls[2].body, `"rotate":true`) {
		t.Fatalf("the rotation was asked as %+v", calls[2])
	}
	// Without --rotate the proof's own token is the answer and nothing is rotated.
	log2 := daemon(t, "agent-bus")
	out, err = stdout(t, func() error { return issue([]string{"alice@test", "--key=" + key}) })
	if err != nil || out != "PROOF-TOKEN" || len(log2()) != 2 {
		t.Fatalf("without --rotate: %q, %v, %d calls", out, err, len(log2()))
	}
}

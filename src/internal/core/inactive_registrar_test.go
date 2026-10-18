package core

import (
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// An Agent set inactive registers nothing for its Owner, though the Owner is
// active: the authority is asked of the caller under the write's hold, not
// only at the edge (issues/2026-10/2026-10-17.md#A27).
func TestAnInactiveAgentRegistersNothing(t *testing.T) {
	b := New()
	known(t, b, "alice@h")
	provision(t, b, nil, protocol.Record{Name: "#a@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict})
	// Active, it may: the refusal below is about its state, not its right.
	if _, err := b.Register(protocol.Record{Name: "#first@h", Kind: protocol.KindAgent, Owner: "#a@h"}); err != nil {
		t.Fatalf("an active Agent registering for its Owner: %v", err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "#a@h", Status: ptr(protocol.StatusInactive)}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "#child@h", Kind: protocol.KindAgent, Owner: "#a@h"}); err == nil {
		t.Fatal("an inactive Agent registered #child@h")
	}
	if _, ok := b.Lookup("alice@h", "#child@h"); ok {
		t.Fatal("#child@h exists")
	}
}

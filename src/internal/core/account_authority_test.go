package core

import (
	"errors"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestAccountMappingCannotGrantAnotherPrincipalsAuthority(t *testing.T) {
	b := accountFixture(t)
	for _, r := range []protocol.Record{
		{Name: "#foreign@h", Kind: protocol.KindAgent, Owner: "bob@h"},
		{Name: "#mine@h", Kind: protocol.KindAgent, Owner: "alice@h"},
	} {
		if _, err := b.Register(r); err != nil {
			t.Fatal(err)
		}
	}
	for _, principal := range []string{"owner@h", "alice@h", "#foreign@h"} {
		if _, err := b.SetAccount("alice@h", "new-os", principal, false); !errors.Is(err, ErrNotOwner) {
			t.Fatalf("administrator granted %s: %v", principal, err)
		}
	}
	for _, principal := range []string{"owner@h", "alice@h", "#foreign@h"} {
		if _, err := b.SetAccount("owner@h", "owner-choice", principal, false); err != nil {
			t.Fatalf("owner mapping %s: %v", principal, err)
		}
	}
	for _, principal := range []string{"bob@h", "#mine@h"} {
		if _, err := b.SetAccount("alice@h", "new-os", principal, false); err != nil {
			t.Fatalf("allowed mapping %s: %v", principal, err)
		}
	}
}

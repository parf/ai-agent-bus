package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Only a destination the caller manages goes on a 📣 list: a queue or a 📣
// that refuses bob is not reached by bob listing it on a channel of his own,
// a hidden one is no such name, and an agent or a group is the same.
// See docs/04-messaging.md#deliver-to.
func TestADeliverToTermNeedsItsDestinationsManager(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "bob@h", "carol@h")
	provision(t, b, nil,
		protocol.Record{Name: "alerts@h", Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"alice@h"}, Subs: []string{"#a@h"}},
		protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict},
		protocol.Record{Name: "#a@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict},
		protocol.Record{Name: "relay@h", Kind: protocol.KindPubSub, Owner: "bob@h", Allow: []string{"*"}},
	)
	if err := b.SetGroup("admin@h", "@team", []string{"#a@h"}); err != nil {
		t.Fatal(err)
	}
	for term, want := range map[string]error{
		"alerts@h": ErrUnknown, // bob may not see it
		"jobs@h":   ErrNotOwner,
		"#a@h":     ErrNotOwner,
		"@team":    ErrNotOwner,
	} {
		_, err := b.Manage("bob@h", Management{Name: "relay@h", Subs: &[]string{term}})
		if !errors.Is(err, want) {
			t.Fatalf("bob listed %s: %v, want %v", term, err, want)
		}
		if want == ErrUnknown && strings.Contains(err.Error(), "own") {
			t.Fatalf("the refusal for a hidden name says more than no such name: %v", err)
		}
	}
	if r, _ := b.Lookup("bob@h", "relay@h"); len(r.Subs) != 0 {
		t.Fatalf("a refused list stored %v", r.Subs)
	}
	if _, err := b.Send(protocol.Envelope{From: "bob@h", To: "relay@h", Body: "in"}); err == nil {
		t.Fatal("bob's channel reached a destination he does not manage")
	}
	if q, _, _, _ := counts(b, "#a@h"); q != 0 {
		t.Fatalf("#a@h holds %d copies from bob", q)
	}
	// A Maintainer of each may; so may the daemon Owner.
	maintainedBy(t, b, "bob@h", "jobs@h", "#a@h", "@team")
	if _, err := b.Manage("bob@h", Management{Name: "relay@h", Subs: &[]string{"jobs@h", "#a@h", "@team"}}); err != nil {
		t.Fatalf("a Maintainer of every destination: %v", err)
	}
	if _, err := b.Manage("admin@h", Management{Name: "relay@h", Subs: &[]string{"alerts@h"}}); err != nil {
		t.Fatalf("the daemon Owner: %v", err)
	}
}

// Control is asked when a term is added, never again: a list stored earlier
// keeps a term its manager no longer controls, an edit that keeps it is not
// refused for it, and taking it off means asking again to put it back.
func TestADeliverToTermIsCheckedWhenItIsAdded(t *testing.T) {
	b := New()
	known(t, b, "alice@h", "bob@h")
	provision(t, b, nil,
		protocol.Record{Name: "#a@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict},
		protocol.Record{Name: "#b@h", Kind: protocol.KindAgent, Owner: "bob@h", Allow: []string{"*"}, Full: protocol.OverflowStrict},
		protocol.Record{Name: "relay@h", Kind: protocol.KindPubSub, Owner: "bob@h", Allow: []string{"*"}, Subs: []string{"#a@h"}},
	)
	if _, err := b.Manage("bob@h", Management{Name: "relay@h", Subs: &[]string{"#a@h", "#b@h"}}); err != nil {
		t.Fatalf("an edit keeping a stored term: %v", err)
	}
	// From alice: bob's own send carries his owner role, which no copy
	// takes to a record he does not answer for.
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "relay@h", Body: "x"}); err != nil {
		t.Fatal(err)
	}
	if q, _, _, _ := counts(b, "#a@h"); q != 1 {
		t.Fatalf("the stored term took %d copies, want one", q)
	}
	if _, err := b.Manage("bob@h", Management{Name: "relay@h", Subs: &[]string{"#b@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("bob@h", Management{Name: "relay@h", Subs: &[]string{"#b@h", "#a@h"}}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("a term taken off came back without its manager: %v", err)
	}
	// A registration restating the list is asked the same.
	if _, err := b.Register(protocol.Record{Name: "relay@h", Kind: protocol.KindPubSub, Owner: "bob@h", Allow: []string{"*"}, Subs: []string{"#a@h"}}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("a registration added a term its caller does not manage: %v", err)
	}
}

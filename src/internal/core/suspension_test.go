package core

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Q63 asks whether ConsumeAs should refuse a read of an inactive name's *own*
// inbox, the way Send already refuses to deliver to one. It is the owner's
// question and it is open (Plans/MVP/QUESTIONS.md#open-questions), so H.5.7
// had to leave that behaviour exactly as it found it. Adding owner suspension
// to this path very nearly answered it by accident — the first draft called a
// predicate whose leading branch is `!active(name)` — so this pins the
// boundary rather than trusting the spelling.
func TestSuspensionDidNotSettleTheDrainQuestion(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	b.Masters([]string{"admin@h"})
	for _, who := range []string{"alice@h", "sender@h"} {
		if _, err := b.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.Send(protocol.Envelope{From: "sender@h", To: "alice@h", Body: "to the person"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetUserState("admin@h", "alice@h", "paused"); err != nil {
		t.Fatal(err)
	}

	// alice's record is self-owned, so there is no separate owner to be
	// suspended. Whether her own paused state should refuse this read is Q63,
	// and the answer here must still be the one that was there before.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	e, err := b.ConsumeAs(ctx, "admin@h", "alice@h", "", "", false, false)
	if err != nil {
		t.Fatalf("a master could no longer drain a paused person's inbox: %v", err)
	}
	if !strings.Contains(e.Body, "to the person") {
		t.Fatalf("wrong message: %+v", e)
	}
}

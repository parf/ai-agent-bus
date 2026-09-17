package core

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// H.5.7 preserved this behavior; the owner confirmed it when settling Q63:
// an active, authorized caller may drain an inactive name's own inbox.
// See docs/01-identity-and-roles.md#user-states.
func TestSuspensionDidNotSettleTheDrainQuestion(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	b.Masters([]string{"admin@h"})
	for _, who := range []string{"alice@h", "sender@h"} {
		if _, err := b.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.Manage("alice@h", Management{Name: "alice@h", Allow: ptr([]string{"sender@h"})}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "sender@h", To: "alice@h", Body: "to the person"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetUserState("admin@h", "alice@h", "paused"); err != nil {
		t.Fatal(err)
	}

	// Alice's record is self-owned, so there is no separate owner to be
	// suspended. Her own inactive state does not prevent an authorized drain.
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

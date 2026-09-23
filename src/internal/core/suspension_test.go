package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// An inactive User's own record is no such inbox: nobody drains it, however
// explicitly they were admitted, and the queued work stays for the User's
// return (docs/constitution.md#common-record-fields). This replaces Q63's
// drain permission.
func TestAnInactiveUsersInboxIsDrainedByNobodyAndKeepsItsWork(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "#sender@h")
	if _, err := b.SetUser("admin@h", protocol.User{Name: "alice@h"}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "alice@h", Allow: ptr([]string{"admin@h"})}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "#sender@h", To: "alice@h", Body: "to the person"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetUserState("admin@h", "alice@h", protocol.StatusInactive); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := b.ConsumeAs(ctx, "admin@h", "alice@h", "", "", false, false); !errors.Is(err, ErrUnknown) {
		t.Fatalf("an inactive user's inbox was drained: %v", err)
	}
	if _, err := b.SetUserState("admin@h", "alice@h", protocol.StatusActive); err != nil {
		t.Fatal(err)
	}
	e, err := b.ConsumeAs(ctx, "admin@h", "alice@h", "", "", false, false)
	if err != nil || !strings.Contains(e.Body, "to the person") {
		t.Fatalf("the work queued before the deactivation was lost: %+v %v", e, err)
	}
}

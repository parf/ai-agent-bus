package core

import (
	"errors"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// A reader that a write would refuse is refused only once that write commits:
// a write the store refuses is rolled back and has refused nobody (K.25).
func TestAFailedCommitLeavesWaitingReadersWaiting(t *testing.T) {
	st := memory.NewState()
	b := durabilityFixture(t, st)
	// bob reads #svc@h through @readers; the writes below would take that.
	for _, c := range []struct {
		name  string
		write func() error
	}{
		{"an allow edit", func() error {
			_, err := b.Manage("alice@h", Management{Name: "#svc@h", Allow: ptr([]string{"alice@h"})})
			return err
		}},
		{"a deactivation", func() error {
			_, err := b.Manage("alice@h", Management{Name: "#svc@h", Status: ptr("inactive")})
			return err
		}},
		{"a group edit", func() error { return b.SetGroup("admin@h", "@readers", []string{"friend@h"}) }},
		{"a user's suspension", func() error {
			_, err := b.SetUserState("admin@h", "bob@h", "inactive")
			return err
		}},
	} {
		done := blockedRead(t, b, "bob@h", "#svc@h")
		boom := errors.New("disk full")
		st.Err = boom
		if err := c.write(); !errors.Is(err, boom) {
			t.Fatalf("%s: the refused commit answered %v", c.name, err)
		}
		st.Err = nil
		select {
		case err := <-done:
			t.Fatalf("%s: a failed commit refused the waiting reader: %v", c.name, err)
		case <-time.After(50 * time.Millisecond):
		}
		// Falsifiable: the same write, committed, releases it.
		if err := c.write(); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		released(t, done, c.name)
		// Back to a node where bob reads #svc@h again.
		b = durabilityFixture(t, st)
	}
}

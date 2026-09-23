package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func ptr[T any](v T) *T { return &v }

func TestPolicyChangesCancelBlockedReaders(t *testing.T) {
	// "owner" is the one where the reader itself is untouched: reader@h is
	// active and still on the ACL, and the read ends anyway because the
	// service stopped answering under it
	// (docs/01-identity-and-roles.md#user-states).
	for _, change := range []string{"deactivate", "acl", "empty-acl", "membership", "user", "owner"} {
		t.Run(change, func(t *testing.T) {
			b := New()
			b.SetDaemonOwner("admin@h")
			known(t, b, "owner@h")
			if _, err := b.SetUser("admin@h", protocol.User{Name: "reader@h"}, true); err != nil {
				t.Fatal(err)
			}
			if err := b.SetGroup("admin@h", "@readers", []string{"reader@h"}); err != nil {
				t.Fatal(err)
			}
			if _, err := b.Register(protocol.Record{Name: "queue@h", Owner: "owner@h", Kind: protocol.KindQueue, Allow: []string{"@readers"}}); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			result := make(chan error, 1)
			go func() { _, err := b.ConsumeAs(ctx, "reader@h", "queue@h", "", "", false, false); result <- err }()
			deadline := time.Now().Add(time.Second)
			for b.Status().Waiting != 1 {
				if time.Now().After(deadline) {
					t.Fatal("reader never waited")
				}
				time.Sleep(time.Millisecond)
			}
			var err error
			switch change {
			case "deactivate":
				_, err = b.Manage("owner@h", Management{Name: "queue@h", Status: ptr(protocol.StatusInactive)})
			case "acl":
				_, err = b.Manage("owner@h", Management{Name: "queue@h", Allow: ptr([]string{"owner@h"})})
			case "empty-acl":
				_, err = b.Manage("owner@h", Management{Name: "queue@h", Allow: ptr([]string{})})
			case "user":
				_, err = b.SetUserState("admin@h", "reader@h", "inactive")
			case "owner":
				_, err = b.SetUserState("admin@h", "owner@h", "inactive")
			case "membership":
				err = b.SetGroup("admin@h", "@readers", nil)
			}
			if err != nil {
				t.Fatal(err)
			}
			// The reader that stopped acting is told so; a queue that stopped
			// being an entity — itself or through its Owner — is no such
			// inbox now (docs/constitution.md#common-record-fields).
			want := ErrNotAllow
			switch change {
			case "user":
				want = ErrInactive
			case "owner", "deactivate":
				want = ErrUnknown
			}
			if got := <-result; !errors.Is(got, want) {
				t.Fatalf("blocked read: got %v, want %v", got, want)
			}
			if b.Status().Waiting != 0 {
				t.Fatal("revoked waiter retained")
			}
		})
	}
}

func TestManagementRejectsPartialInvalidChanges(t *testing.T) {
	b := New()
	known(t, b, "owner@h")
	b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "owner@h", Descr: "original"})
	if _, err := b.Manage("owner@h", Management{Name: "#svc@h", Descr: ptr("lost"), Bound: ptr(-1)}); !errors.Is(err, ErrBound) {
		t.Fatal(err)
	}
	got, _ := b.Lookup("owner@h", "#svc@h")
	if got.Descr != "original" {
		t.Fatal("invalid change partly applied")
	}
}

// A deactivation can wake a blocked reader while later requests reactivate and
// unregister the now-idle address. The reader's cleanup must not recreate it.
func TestCanceledReaderCannotRecreateRemovedInbox(t *testing.T) {
	b := New()
	known(t, b, "owner@h")
	b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "owner@h"})
	w := &waiter{caller: "#svc@h", ch: make(chan protocol.Envelope, 1), stopped: make(chan error, 1)}
	b.inboxes["#svc@h"].waiters = append(b.inboxes["#svc@h"].waiters, w)
	if _, err := b.Manage("owner@h", Management{Name: "#svc@h", Status: ptr(protocol.StatusInactive)}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("owner@h", Management{Name: "#svc@h", Status: ptr(protocol.StatusActive)}); err != nil {
		t.Fatal(err)
	}
	if err := b.Unregister("#svc@h", "owner@h"); err != nil {
		t.Fatal(err)
	}
	b.settle("#svc@h", w)
	if _, exists := b.inboxes["#svc@h"]; exists {
		t.Fatal("canceled reader recreated an unregistered inbox")
	}
}

func TestChannelManagersCanRemoveButStrangersCannot(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "owner@h")
	b.Register(protocol.Record{Name: "news@h", Owner: "owner@h", Allow: []string{"*"}, Kind: protocol.KindPubSub})
	known(t, b, "#subscriber@h", "stranger@h", "maint@h")
	if _, err := b.Manage("owner@h", Management{Name: "news@h", Subs: ptr([]string{"#subscriber@h"})}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.RemoveSubscriber("stranger@h", "news@h", "#subscriber@h"); !errors.Is(err, ErrNotOwner) {
		t.Fatal("stranger removed subscription", err)
	}
	if err := b.SetGroup("admin@h", "@ops", []string{"maint@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("owner@h", Management{Name: "news@h", Maintainers: ptr(protocol.MaintainerList{"@ops"})}); err != nil {
		t.Fatal(err)
	}
	r, err := b.RemoveSubscriber("maint@h", "news@h", "#subscriber@h")
	if err != nil || len(r.Subs) != 0 {
		t.Fatal("channel maintainer could not remove subscription", err)
	}
}

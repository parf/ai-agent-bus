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
	for _, change := range []string{"disable", "acl", "membership", "user"} {
		t.Run(change, func(t *testing.T) {
			b := New()
			b.Administrator("admin@h")
			known(t, b, "owner@h")
			if _, err := b.SetUser("admin@h", protocol.User{Name: "reader@h"}, true); err != nil {
				t.Fatal(err)
			}
			if err := b.SetGroup("admin@h", "@readers", []string{"reader@h"}, false); err != nil {
				t.Fatal(err)
			}
			if _, err := b.Register(protocol.Record{Name: "queue@h", Owner: "owner@h", Kind: "topic", Allow: []string{"@readers"}}); err != nil {
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
			case "disable":
				_, err = b.Manage("owner@h", Management{Name: "queue@h", Disabled: ptr(true)})
			case "acl":
				_, err = b.Manage("owner@h", Management{Name: "queue@h", Allow: ptr([]string{"owner@h"})})
			case "user":
				_, err = b.SetUserState("admin@h", "reader@h", "paused")
			case "membership":
				err = b.SetGroup("admin@h", "@readers", nil, false)
			}
			if err != nil {
				t.Fatal(err)
			}
			want := ErrNotAllow
			if change == "user" {
				want = ErrInactive
			}
			if change == "disable" {
				want = ErrDisabled
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
	b.Register(protocol.Record{Name: "svc@h", Owner: "owner@h", Descr: "original"})
	if _, err := b.Manage("owner@h", Management{Name: "svc@h", Descr: ptr("lost"), Bound: ptr(-1)}); !errors.Is(err, ErrBound) {
		t.Fatal(err)
	}
	got, _ := b.Lookup("owner@h", "svc@h")
	if got.Descr != "original" {
		t.Fatal("invalid change partly applied")
	}
}

// A disable can wake a blocked reader while a concurrent request unregisters
// the now-idle address. The reader's cancellation cleanup must not recreate it.
func TestCanceledReaderCannotRecreateRemovedInbox(t *testing.T) {
	b := New()
	known(t, b, "owner@h")
	b.Register(protocol.Record{Name: "svc@h", Owner: "owner@h"})
	w := &waiter{caller: "svc@h", ch: make(chan protocol.Envelope, 1), stopped: make(chan error, 1)}
	b.inboxes["svc@h"].waiters = append(b.inboxes["svc@h"].waiters, w)
	if _, err := b.Manage("owner@h", Management{Name: "svc@h", Disabled: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	if err := b.Unregister("svc@h", "owner@h"); err != nil {
		t.Fatal(err)
	}
	b.settle("svc@h", w)
	if _, exists := b.inboxes["svc@h"]; exists {
		t.Fatal("canceled reader recreated an unregistered inbox")
	}
}

func TestChannelManagersCanRemoveButStrangersCannot(t *testing.T) {
	b := New()
	b.Administrator("admin@h")
	known(t, b, "owner@h")
	b.Register(protocol.Record{Name: "news@h", Owner: "owner@h", Kind: "topic", Mode: "pubsub"})
	known(t, b, "subscriber@h", "stranger@h", "maint@h")
	if _, err := b.Subscribe("subscriber@h", "news@h", true); err != nil {
		t.Fatal(err)
	}
	if _, err := b.RemoveSubscriber("stranger@h", "news@h", "subscriber@h"); !errors.Is(err, ErrNotOwner) {
		t.Fatal("stranger removed subscription", err)
	}
	if err := b.SetGroup("admin@h", "@ops", []string{"maint@h"}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("owner@h", Management{Name: "news@h", Maintainers: ptr("@ops")}); err != nil {
		t.Fatal(err)
	}
	r, err := b.RemoveSubscriber("maint@h", "news@h", "subscriber@h")
	if err != nil || len(r.Subs) != 0 {
		t.Fatal("channel maintainer could not remove subscription", err)
	}
}

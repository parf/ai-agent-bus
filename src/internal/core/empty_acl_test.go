package core

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func restrictedFixture(t *testing.T, restored bool) *Bus {
	t.Helper()
	b := New()
	b.SetDaemonOwner("admin@h")
	for _, name := range []string{"alice@h", "maintainer@h", "outsider@h", "master@h", "paused@h"} {
		if _, err := b.SetUser("admin@h", protocol.User{Name: name}, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.SetGroup("admin@h", "@support", []string{"maintainer@h"}); err != nil {
		t.Fatal(err)
	}
	for _, r := range []protocol.Record{
		{Name: "svc@h", Owner: "alice@h", Maintainers: "@support"},
		{Name: "peer@h", Owner: "outsider@h"},
	} {
		if _, err := b.Register(r); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.Manage("alice@h", Management{Name: "svc@h", Maintainers: ptr("@support")}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("admin@h", "@readers", []string{"outsider@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetUserState("admin@h", "paused@h", "paused"); err != nil {
		t.Fatal(err)
	}
	if restored {
		// The pre-change snapshot shape has no access-policy version or migration.
		// An existing empty ACL must remain empty and adopt the same restriction.
		data, err := json.Marshal(b.Snapshot())
		if err != nil {
			t.Fatal(err)
		}
		var snapshot ports.Snapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			t.Fatal(err)
		}
		b = New()
		b.Restore(snapshot)
		b.SetDaemonOwner("admin@h")
	}
	b.Masters([]string{"admin@h", "master@h"})
	return b
}

func TestEmptyACLRestrictsUseAndVisibilityOnNewAndRestoredRecords(t *testing.T) {
	for _, restored := range []bool{false, true} {
		label := "new"
		if restored {
			label = "restored"
		}
		t.Run(label, func(t *testing.T) {
			b := restrictedFixture(t, restored)
			for _, who := range []string{"alice@h", "maintainer@h", "svc@h", "outsider@h", "peer@h", "admin@h", "master@h"} {
				t.Run(who, func(t *testing.T) {
					allowed := who == "alice@h" || who == "maintainer@h" || who == "svc@h"
					r, visible := b.Lookup(who, "svc@h")
					if visible != (allowed || who == "admin@h") {
						t.Fatalf("lookup visible=%v, want %v", visible, allowed || who == "admin@h")
					}
					if visible && len(r.Allow) != 0 {
						t.Fatal("empty ACL silently rewritten")
					}
					listed := false
					for _, r := range b.List(who, "") {
						if r.Name == "svc@h" {
							listed = true
						}
					}
					if listed != (allowed || who == "admin@h") {
						t.Fatalf("listing contains service=%v, want %v", listed, allowed || who == "admin@h")
					}
					_, err := b.Send(protocol.Envelope{From: who, To: "svc@h", Body: who})
					if allowed {
						if err != nil {
							t.Fatal(err)
						}
						ctx, cancel := context.WithTimeout(context.Background(), time.Second)
						defer cancel()
						e, err := b.ConsumeAs(ctx, who, "svc@h", "", "", false, false)
						if err != nil || e.Body != who {
							t.Fatalf("authorized consume: %+v, %v", e, err)
						}
					} else {
						if !errors.Is(err, ErrNotAllow) {
							t.Fatalf("send: %v, want refusal", err)
						}
						ctx, cancel := context.WithTimeout(context.Background(), time.Second)
						defer cancel()
						if _, err := b.ConsumeAs(ctx, who, "svc@h", "", "", false, false); !errors.Is(err, ErrNotAllow) {
							t.Fatalf("consume: %v, want refusal", err)
						}
					}
				})
			}
		})
	}
}

func TestExplicitGrantsStillShareAndRemovingWildcardClosesAccess(t *testing.T) {
	b := restrictedFixture(t, false)
	for _, allow := range [][]string{{"outsider@h"}, {"@readers"}, {"*"}} {
		if _, err := b.Manage("alice@h", Management{Name: "svc@h", Allow: &allow}); err != nil {
			t.Fatal(err)
		}
		who := "outsider@h"
		if _, err := b.Send(protocol.Envelope{From: who, To: "svc@h"}); err != nil {
			t.Fatal(err)
		}
		// Explicit ACLs retain the configured master grant; empty ACLs do not.
		if _, err := b.Send(protocol.Envelope{From: "master@h", To: "svc@h"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.Send(protocol.Envelope{From: "peer@h", To: "svc@h"}); err != nil {
		t.Fatalf("wildcard excluded a service principal: %v", err)
	}
	for _, who := range []string{"unknown@h", "paused@h"} {
		if _, err := b.Send(protocol.Envelope{From: who, To: "svc@h"}); err == nil {
			t.Fatalf("wildcard admitted %s", who)
		}
	}
	empty := []string{}
	if _, err := b.Manage("alice@h", Management{Name: "svc@h", Allow: &empty}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "outsider@h", To: "svc@h"}); !errors.Is(err, ErrNotAllow) {
		t.Fatalf("removed wildcard still grants: %v", err)
	}
}

func TestOwnInboxReadDoesNotNeedAnACLEntryButStillObeysState(t *testing.T) {
	for _, allow := range [][]string{nil, {"peer@h"}} {
		for _, state := range []string{"active", "disabled", "owner-paused"} {
			b := restrictedFixture(t, false)
			if _, err := b.Manage("alice@h", Management{Name: "svc@h", Allow: &allow}); err != nil {
				t.Fatal(err)
			}
			if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "svc@h", Body: "own-inbox"}); err != nil {
				t.Fatal(err)
			}
			var want error
			switch state {
			case "disabled":
				if _, err := b.Manage("alice@h", Management{Name: "svc@h", Disabled: ptr(true)}); err != nil {
					t.Fatal(err)
				}
				want = ErrDisabled
			case "owner-paused":
				if _, err := b.SetUserState("admin@h", "alice@h", "paused"); err != nil {
					t.Fatal(err)
				}
				want = ErrInactive
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			e, err := b.ConsumeAs(ctx, "svc@h", "svc@h", "", "", false, false)
			cancel()
			if !errors.Is(err, want) || want == nil && e.Body != "own-inbox" {
				t.Fatalf("allow=%v state=%s: %+v %v, want %v", allow, state, e, err, want)
			}
		}
	}
}

func TestRegistrationRefreshPreservesACLUnlessExplicitlyReplaced(t *testing.T) {
	b := restrictedFixture(t, false)
	_, err := b.Register(protocol.Record{Name: "svc@h", Owner: "alice@h", Allow: []string{"outsider@h"}, NoMaster: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "svc@h", Owner: "svc@h", Descr: "restarted"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.Lookup("outsider@h", "svc@h"); !ok {
		t.Fatal("metadata refresh erased the explicit grant")
	}
	if _, ok := b.Lookup("master@h", "svc@h"); ok {
		t.Fatal("metadata refresh lifted master refusal")
	}
	if _, err := b.Register(protocol.Record{Name: "svc@h", Owner: "alice@h", Allow: []string{"peer@h"}, NoMaster: true}); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.Lookup("peer@h", "svc@h"); !ok {
		t.Fatal("explicit replacement ACL was ignored")
	}
	if _, ok := b.Lookup("outsider@h", "svc@h"); ok {
		t.Fatal("replaced ACL still grants its former recipient")
	}
	if _, err := b.Manage("alice@h", Management{Name: "svc@h", Allow: ptr([]string{})}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "svc@h", Owner: "svc@h"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.Lookup("peer@h", "svc@h"); ok {
		t.Fatal("restart restored a deliberately cleared grant")
	}
	if _, err := b.Manage("alice@h", Management{Name: "svc@h", Allow: ptr([]string{"peer@h"}), NoMaster: ptr(false)}); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.Lookup("master@h", "svc@h"); !ok {
		t.Fatal("explicit master-refusal clear did not take effect")
	}
	if _, err := b.Register(protocol.Record{Name: "svc@h", Owner: "svc@h", NoMaster: true}); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.Lookup("master@h", "svc@h"); ok {
		t.Fatal("refresh ignored an explicit master refusal")
	}
	if _, ok := b.Lookup("peer@h", "svc@h"); !ok {
		t.Fatal("tightening master refusal erased the peer grant")
	}
}

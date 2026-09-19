package core

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func personalFixture(t *testing.T) *Bus {
	t.Helper()
	b := New()
	b.SetDaemonOwner("admin@h")
	for _, name := range []string{"alice@h", "bob@h", "maint@h"} {
		if _, err := b.SetUser("admin@h", protocol.User{Name: name}, true); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []protocol.Record{
		{Name: "peer@h", Owner: "alice@h", Kind: protocol.KindAgent},
		{Name: "jobs@h", Owner: "alice@h", Kind: protocol.KindQueue},
	} {
		if _, err := b.Register(r); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.SetGroup("admin@h", "@ops", []string{"maint@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("admin@h", "@services", []string{"peer@h"}); err != nil {
		t.Fatal(err)
	}
	// A profile may sit on a self-owned record whose historical kind is
	// generic. Personal validation asks about the profile, not its spelling or
	// record kind, so this remains a user rather than becoming a service.
	provision(t, b, protocol.Record{Name: "profile@h", Kind: protocol.KindAgent})
	if _, err := b.SetUser("admin@h", protocol.User{Name: "profile@h"}, false); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPersonalChangesValidateTheFinalRecordAtomically(t *testing.T) {
	b := personalFixture(t)
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "reports@h", Owner: "alice@h", Allow: []string{"bob@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "reports@h", Maintainers: ptr(protocol.MaintainerList{"@ops"})}); err != nil {
		t.Fatal(err)
	}

	if _, err := b.Manage("alice@h", Management{Name: "reports@h", Personal: ptr(true)}); !errors.Is(err, ErrPersonal) {
		t.Fatalf("enabling without stripping sharing: %v", err)
	}
	got, _ := b.Lookup("alice@h", "reports@h")
	if got.Personal || !reflect.DeepEqual(got.Maintainers, protocol.MaintainerList{"@ops"}) || len(got.Allow) != 1 || got.Allow[0] != "bob@h" {
		t.Fatalf("failed change partly applied: %+v", got)
	}

	empty := protocol.MaintainerList{}
	peerOnly := []string{"peer@h"}
	got, err := b.Manage("alice@h", Management{
		Name: "reports@h", Personal: ptr(true), Maintainers: &empty, Allow: &peerOnly,
	})
	if err != nil || !got.Personal || len(got.Maintainers) != 0 || len(got.Allow) != 1 || got.Allow[0] != "peer@h" {
		t.Fatalf("strip sharing and enable Personal atomically: %+v, %v", got, err)
	}

	userOnly := []string{"bob@h"}
	if _, err := b.Manage("alice@h", Management{Name: "reports@h", Allow: &userOnly}); !errors.Is(err, ErrPersonal) {
		t.Fatalf("adding sharing while Personal: %v", err)
	}
	got, _ = b.Lookup("alice@h", "reports@h")
	if !got.Personal || len(got.Allow) != 1 || got.Allow[0] != "peer@h" {
		t.Fatalf("failed sharing change partly applied: %+v", got)
	}

	maintainers := protocol.MaintainerList{"@ops"}
	got, err = b.Manage("alice@h", Management{
		Name: "reports@h", Personal: ptr(false), Maintainers: &maintainers, Allow: &userOnly,
	})
	if err != nil || got.Personal || !reflect.DeepEqual(got.Maintainers, protocol.MaintainerList{"@ops"}) || len(got.Allow) != 1 || got.Allow[0] != "bob@h" {
		t.Fatalf("disable Personal and add sharing atomically: %+v, %v", got, err)
	}
}

func TestPersonalAcceptsOnlyDirectRegisteredAgents(t *testing.T) {
	b := personalFixture(t)
	for i, tc := range []struct {
		name  string
		allow []string
	}{
		{"wildcard", []string{"*"}},
		{"service-only-group", []string{"@services"}},
		{"user", []string{"bob@h"}},
		{"profile-backed", []string{"profile@h"}},
		{"channel", []string{"jobs@h"}},
		{"unknown", []string{"missing@h"}},
		{"self", []string{"self-7@h"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := "candidate-" + string(rune('a'+i)) + "@h"
			if tc.name == "self" {
				name = "self-7@h"
			}
			_, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: name, Owner: "alice@h", Personal: true, Allow: tc.allow})
			if !errors.Is(err, ErrPersonal) {
				t.Fatalf("registered invalid Personal ACL %v: %v", tc.allow, err)
			}
		})
	}
	if got, err := b.Register(protocol.Record{Kind: protocol.KindAgent, 
		Name: "valid@h", Owner: "alice@h", Personal: true, Allow: []string{"peer@h"},
	}); err != nil || !got.Personal {
		t.Fatalf("direct agent ACL refused: %+v, %v", got, err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "valid@h", Maintainers: ptr(protocol.MaintainerList{"@ops"})}); !errors.Is(err, ErrPersonal) {
		t.Fatalf("Personal service accepted Maintainers: %v", err)
	}
}

// Personal is an agent-only owner choice: it groups an owner's own agents in
// their web view, and the other four kinds have nobody to be personal to.
func TestPersonalIsAnAgentOnlyOwnerChoice(t *testing.T) {
	b := personalFixture(t)
	for _, r := range []protocol.Record{
		{Name: "personal-queue@h", Owner: "alice@h", Kind: protocol.KindQueue, Personal: true},
		{Name: "personal-pubsub@h", Owner: "alice@h", Kind: protocol.KindPubSub, Personal: true},
		{Name: "personal-service@h", Owner: "alice@h", Kind: protocol.KindService, Addr: "h:1", Proto: "https", Personal: true},
		{Name: "personal-user@h", Owner: "alice@h", Kind: protocol.KindUser, Personal: true},
	} {
		if _, err := b.Register(r); !errors.Is(err, ErrPersonal) {
			t.Fatalf("registered Personal %s: %v", r.Kind, err)
		}
	}
	// Falsifiable the other way: the one kind that may be Personal is.
	if got, err := b.Register(protocol.Record{
		Name: "personal-agent@h", Owner: "alice@h", Kind: protocol.KindAgent, Personal: true,
	}); err != nil || !got.Personal {
		t.Fatalf("an agent was refused Personal: %+v, %v", got, err)
	}
	if _, err := b.Manage("profile@h", Management{Name: "profile@h", Personal: ptr(true)}); !errors.Is(err, ErrPersonal) {
		t.Fatalf("profile-backed record became Personal: %v", err)
	}

	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "owned@h", Owner: "alice@h", Allow: []string{"peer@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "owned@h", Maintainers: ptr(protocol.MaintainerList{"@ops"})}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("maint@h", Management{Name: "owned@h", Personal: ptr(true)}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("Maintainer changed owner's classification: %v", err)
	}
}

func TestPersonalRefreshPreservesOwnerPolicyAndRejectsInvalidReplacement(t *testing.T) {
	b := personalFixture(t)
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, 
		Name: "worker@h", Owner: "alice@h", Personal: true, Allow: []string{"peer@h"},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "worker@h", Owner: "worker@h", Descr: "restarted"})
	if err != nil || !got.Personal || got.Descr != "restarted" || len(got.Allow) != 1 || got.Allow[0] != "peer@h" {
		t.Fatalf("refresh lost Personal policy: %+v, %v", got, err)
	}
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "worker@h", Owner: "worker@h", Allow: []string{"bob@h"}}); !errors.Is(err, ErrPersonal) {
		t.Fatalf("refresh installed invalid Personal ACL: %v", err)
	}
	got, _ = b.Lookup("alice@h", "worker@h")
	if got.Descr != "restarted" || !got.Personal || len(got.Allow) != 1 || got.Allow[0] != "peer@h" {
		t.Fatalf("refused refresh changed stored policy: %+v", got)
	}
}

func TestPersonalPersistsWithoutChangingAuthorization(t *testing.T) {
	b := personalFixture(t)
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, 
		Name: "private-tool@h", Owner: "alice@h", Personal: true, Allow: []string{"peer@h"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "peer@h", To: "private-tool@h", Body: "allowed"}); err != nil {
		t.Fatalf("Personal changed an explicit service grant: %v", err)
	}
	if _, err := b.Send(protocol.Envelope{From: "bob@h", To: "private-tool@h", Body: "refused"}); !errors.Is(err, ErrNotAllow) {
		t.Fatalf("Personal changed an unrelated caller's access: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if got, err := b.Consume(ctx, "private-tool@h", "", "", false, false); err != nil || got.Body != "allowed" {
		t.Fatalf("service lost its own inbox: %+v, %v", got, err)
	}

	raw, err := json.Marshal(b.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	var saved ports.Snapshot
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	restarted := New()
	restarted.Restore(saved)
	got, visible := restarted.Lookup("alice@h", "private-tool@h")
	if !visible || !got.Personal || len(got.Allow) != 1 || got.Allow[0] != "peer@h" {
		t.Fatalf("restored Personal record: visible=%v %+v", visible, got)
	}

	if err := restarted.Unregister("peer@h", "alice@h"); err != nil {
		t.Fatal(err)
	}
	got, visible = restarted.Lookup("alice@h", "private-tool@h")
	if !visible || !got.Personal || len(got.Allow) != 1 || got.Allow[0] != "peer@h" {
		t.Fatalf("removed ACL target retroactively changed Personal record: visible=%v %+v", visible, got)
	}
}

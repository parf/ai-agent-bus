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

// Personal is an audience — the Owner and the Agents that Owner owns — on
// every kind. Its allow and maintainers lists may name that cohort, directly
// or by @owner and @agent, and nothing wider, on every save; it restricts
// nothing else. See docs/03-records.md#personal-and-shared.

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
		{Name: "#peer@h", Owner: "alice@h", Kind: protocol.KindAgent},
		{Name: "#bobs@h", Owner: "bob@h", Kind: protocol.KindAgent},
		{Name: "jobs@h", Owner: "alice@h", Kind: protocol.KindQueue},
	} {
		if _, err := b.Register(r); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.SetGroup("admin@h", "@ops", []string{"maint@h"}); err != nil {
		t.Fatal(err)
	}
	return b
}

// Every term inside the Owner's cohort is accepted, in both lists, and every
// term outside it is refused on its own — on registration and on management.
func TestPersonalAdmitsTheOwnersCohortAndNothingWider(t *testing.T) {
	b := personalFixture(t)
	inside := []string{"alice@h", "#peer@h", OwnerGroup, AgentTerm}
	if got, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#mine@h", Owner: "alice@h", Personal: true, Allow: inside}); err != nil || !got.Personal {
		t.Fatalf("the cohort was refused in the ACL: %+v, %v", got, err)
	}
	maintainers := protocol.MaintainerList{"alice@h", "#peer@h", OwnerGroup, AgentTerm}
	if _, err := b.Manage("alice@h", Management{Name: "#mine@h", Maintainers: &maintainers}); err != nil {
		t.Fatalf("the cohort was refused as Maintainers: %v", err)
	}
	for name, term := range map[string]string{
		"another user":         "bob@h",
		"another user's agent": "#bobs@h",
		"a group":              "@ops",
		"the wildcard":         "*",
	} {
		if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#candidate@h", Owner: "alice@h", Personal: true, Allow: []string{term}}); !errors.Is(err, ErrPersonal) {
			t.Errorf("%s was accepted on a Personal registration: %v", name, err)
		}
		allow := []string{term}
		if _, err := b.Manage("alice@h", Management{Name: "#mine@h", Allow: &allow}); !errors.Is(err, ErrPersonal) {
			t.Errorf("%s was accepted in a Personal ACL by management: %v", name, err)
		}
		list := protocol.MaintainerList{term}
		if term == "*" {
			continue // not a Maintainer term at all
		}
		if _, err := b.Manage("alice@h", Management{Name: "#mine@h", Maintainers: &list}); !errors.Is(err, ErrPersonal) {
			t.Errorf("%s was accepted as a Personal Maintainer: %v", name, err)
		}
	}
	got, _ := b.Lookup("alice@h", "#mine@h")
	if len(got.Allow) != len(inside) || len(got.Maintainers) != len(maintainers) {
		t.Fatalf("a refused change touched the record: %+v", got)
	}
}

// Every kind may be Personal, and a User's own record always is.
func TestEveryKindMayBePersonalAndAUserAlwaysIs(t *testing.T) {
	b := personalFixture(t)
	for _, r := range []protocol.Record{
		{Name: "personal-queue@h", Owner: "alice@h", Kind: protocol.KindQueue, Personal: true},
		{Name: "personal-pubsub@h", Owner: "alice@h", Kind: protocol.KindPubSub, Personal: true},
		{Name: "personal-service@h", Owner: "alice@h", Kind: protocol.KindService, Addr: "h:1", Proto: "https", Personal: true},
		{Name: "#personal-agent@h", Owner: "alice@h", Kind: protocol.KindAgent, Personal: true},
	} {
		if got, err := b.Register(r); err != nil || !got.Personal {
			t.Errorf("a Personal %s was refused: %+v, %v", r.Kind, got, err)
		}
	}
	if got, ok := b.Lookup("alice@h", "alice@h"); !ok || !got.Personal {
		t.Fatalf("a user's own record is not Personal: %+v", got)
	}
	if _, err := b.Manage("alice@h", Management{Name: "alice@h", Personal: ptr(false)}); !errors.Is(err, ErrPersonal) {
		t.Fatalf("a user's own record was made shared: %v", err)
	}
}

// Checked against the whole record a change leaves behind, so sharing can be
// taken away and Personal set in one edit, and the other way round.
func TestPersonalChangesValidateTheFinalRecordAtomically(t *testing.T) {
	b := personalFixture(t)
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#reports@h", Owner: "alice@h", Allow: []string{"bob@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "#reports@h", Personal: ptr(true)}); !errors.Is(err, ErrPersonal) {
		t.Fatalf("enabling without stripping sharing: %v", err)
	}
	if got, _ := b.Lookup("alice@h", "#reports@h"); got.Personal || len(got.Allow) != 1 || got.Allow[0] != "bob@h" {
		t.Fatalf("a refused change partly applied: %+v", got)
	}
	peerOnly := []string{"#peer@h"}
	if got, err := b.Manage("alice@h", Management{Name: "#reports@h", Personal: ptr(true), Allow: &peerOnly}); err != nil || !got.Personal {
		t.Fatalf("strip sharing and enable Personal in one edit: %+v, %v", got, err)
	}
	userOnly := []string{"bob@h"}
	if got, err := b.Manage("alice@h", Management{Name: "#reports@h", Personal: ptr(false), Allow: &userOnly}); err != nil || got.Personal || got.Allow[0] != "bob@h" {
		t.Fatalf("disable Personal and add sharing in one edit: %+v, %v", got, err)
	}
}

// Only the Owner changes the classification; a Maintainer may not.
func TestOnlyTheOwnerChangesPersonal(t *testing.T) {
	b := personalFixture(t)
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#owned@h", Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "#owned@h", Maintainers: ptr(protocol.MaintainerList{"@ops"})}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("maint@h", Management{Name: "#owned@h", Personal: ptr(true)}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("a Maintainer changed the owner's classification: %v", err)
	}
	// And an agent refreshing its own record keeps the owner's choice.
	if _, err := b.Manage("alice@h", Management{Name: "#owned@h", Maintainers: &protocol.MaintainerList{}, Personal: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	got, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#owned@h", Owner: "#owned@h", Descr: "restarted", Personal: false})
	if err != nil || !got.Personal || got.Descr != "restarted" {
		t.Fatalf("a refresh changed the owner's classification: %+v, %v", got, err)
	}
}

// Personal restricts nothing but the two lists: delivery, Deliver-To and an
// explicit grant behave as on a shared record, and it persists.
func TestPersonalRestrictsNothingElse(t *testing.T) {
	b := personalFixture(t)
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent,
		Name: "#private-tool@h", Owner: "alice@h", Personal: true, Allow: []string{"#peer@h"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "#peer@h", To: "#private-tool@h", Body: "allowed"}); err != nil {
		t.Fatalf("Personal changed an explicit grant: %v", err)
	}
	if _, err := b.Send(protocol.Envelope{From: "bob@h", To: "#private-tool@h", Body: "refused"}); !errors.Is(err, ErrNotAllow) {
		t.Fatalf("Personal changed an unrelated caller's access: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if got, err := b.Consume(ctx, "#private-tool@h", "", "", false, false); err != nil || got.Body != "allowed" {
		t.Fatalf("the agent lost its own inbox: %+v, %v", got, err)
	}
	// A Personal pub/sub may deliver to anyone the ordinary rules allow.
	if _, err := b.Register(protocol.Record{Name: "mine@h", Owner: "alice@h", Kind: protocol.KindPubSub, Personal: true, Subs: []string{"#bobs@h"}}); err != nil {
		t.Fatalf("Personal restricted a Deliver-To list: %v", err)
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
	if got, visible := restarted.Lookup("alice@h", "#private-tool@h"); !visible || !got.Personal {
		t.Fatalf("restored Personal record: visible=%v %+v", visible, got)
	}
}

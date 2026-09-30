package core

import (
	"errors"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A snapshot is a file an operator can edit, so a stored record is asked the
// same question a registration is. A secret belongs to the kinds the field
// table gives one — an Agent, a Service and a Group — and a queue holding one
// is a record this version could not have written: ignored and reported.
// See docs/constitution.md#-private-values.
func TestRestoreIgnoresASecretOnAKindThatHoldsNone(t *testing.T) {
	b := New()
	rep := &reports{}
	b.Journal(rep)
	b.Restore(withUsers(ports.Snapshot{Clean: true, Records: []protocol.Record{
		{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "owner@h", Secret: "TOKEN=abc"},
		{Name: "digest@h", Kind: protocol.KindQueue, Owner: "owner@h", SecretSHA: "deadbeef"},
	}}, "owner@h"))
	if err := b.EstablishDaemonOwner("owner@h"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"jobs@h", "digest@h"} {
		if _, loaded := b.Lookup("owner@h", name); loaded {
			t.Errorf("a queue holding a secret was loaded: %s", name)
		}
		// Named, so that whoever reads the report knows which record to edit.
		if !rep.has("stored record "+name+" is ignored") || !rep.has("holds no secret") {
			t.Errorf("the ignored %s was not reported with its reason: %v", name, rep.lines)
		}
	}
}

// The controls: the kinds that hold one keep what they were given, or the
// refusal above would be passing by refusing everything.
func TestRestoreKeepsAServiceAndAnAgentSecret(t *testing.T) {
	b := New()
	b.Restore(withUsers(ports.Snapshot{Clean: true, Records: []protocol.Record{
		{Name: "db@h", Kind: protocol.KindService, Owner: "owner@h",
			Addr: "host:5432", Proto: "postgresql", Secret: "PGPASSWORD=kept"},
		{Name: "#worker@h", Kind: protocol.KindAgent, Owner: "owner@h", Secret: "TOKEN=abc", Full: protocol.OverflowStrict},
	}}, "owner@h"))
	if err := b.EstablishDaemonOwner("owner@h"); err != nil {
		t.Fatalf("refused records holding secrets: %v", err)
	}
	got, err := b.Secret("db@h", "owner@h")
	if err != nil || got != "PGPASSWORD=kept" {
		t.Fatalf("the restored service secret: %q, %v", got, err)
	}
	// An Agent's own principal reads its secret, and so does its Owner.
	for _, who := range []string{"#worker@h", "owner@h"} {
		if got, err := b.Secret("#worker@h", who); err != nil || got != "TOKEN=abc" {
			t.Fatalf("%s reading the agent's secret: %q, %v", who, got, err)
		}
	}
}

// Private values are the record's Owner's and Maintainers' to read and write,
// and an Agent reads its own; the allow list, a Group's membership and the
// daemon Owner's office grant neither (docs/constitution.md#-private-values).
func TestPrivateValuesAreTheOwnersAndMaintainers(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "mia@h", "user@h")
	provision(t, b, nil,
		protocol.Record{Name: "db@h", Kind: protocol.KindService, Owner: "alice@h", Addr: "db:1", Proto: "pg",
			Maintainers: protocol.MaintainerList{"mia@h"}, Allow: []string{"*"}},
		protocol.Record{Name: "#bot@h", Kind: protocol.KindAgent, Owner: "alice@h", Full: protocol.OverflowStrict},
	)
	for _, who := range []string{"alice@h", "mia@h"} {
		if _, err := b.SetSecret("db@h", who, "K="+who); err != nil {
			t.Fatalf("%s could not write the service's secret: %v", who, err)
		}
		if got, err := b.Secret("db@h", who); err != nil || got != "K="+who {
			t.Fatalf("%s read %q, %v", who, got, err)
		}
	}
	// Both see the record, the one on its allow list and the daemon Owner by
	// office, so each is told the secret is private, and refused the write.
	for _, who := range []string{"user@h", "admin@h"} {
		if _, err := b.Secret("db@h", who); !errors.Is(err, ErrPrivate) {
			t.Errorf("%s read the service's secret through the ACL or office: %v", who, err)
		}
		if _, err := b.SetSecret("db@h", who, "K=x"); !errors.Is(err, ErrNotOwner) {
			t.Errorf("%s wrote the service's secret: %v", who, err)
		}
	}
	if _, err := b.SetSecret("#bot@h", "alice@h", "T=1"); err != nil {
		t.Fatal(err)
	}
	if got, err := b.Secret("#bot@h", "#bot@h"); err != nil || got != "T=1" {
		t.Fatalf("the agent could not read its own secret: %q, %v", got, err)
	}
	if _, err := b.SetSecret("#bot@h", "#bot@h", "T=2"); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("the agent wrote its own secret: %v", err)
	}
}

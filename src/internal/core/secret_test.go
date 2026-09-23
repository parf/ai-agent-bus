package core

import (
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
	// An Agent's own principal reads its secret; its Owner does not.
	if got, err := b.Secret("#worker@h", "#worker@h"); err != nil || got != "TOKEN=abc" {
		t.Fatalf("the agent reading its own secret: %q, %v", got, err)
	}
	if _, err := b.Secret("#worker@h", "owner@h"); err == nil {
		t.Fatal("an agent's owner read the agent's secret")
	}
}

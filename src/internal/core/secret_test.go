package core

import (
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A snapshot is a file an operator can edit, so a stored record is asked the
// same question a registration is. A secret belongs to the one kind that is
// reached somewhere else; on any other it is a field saying something untrue,
// and coming up on it would serve state the daemon cannot describe.
// See docs/06-services.md#secrets.
func TestRestoreRefusesASecretOnAKindWithAQueue(t *testing.T) {
	b := New()
	b.Restore(ports.Snapshot{Clean: true, Records: []protocol.Record{
		{Name: "worker@h", Kind: protocol.KindAgent, Owner: "owner@h", Secret: "TOKEN=abc"},
	}})
	err := b.EstablishDaemonOwner("owner@h")
	if err == nil {
		t.Fatal("restored an agent holding a secret")
	}
	// Named, so that whoever reads the refusal knows which record to edit.
	if !strings.Contains(err.Error(), "worker@h") {
		t.Errorf("refusal does not name the record: %v", err)
	}
	if !strings.Contains(err.Error(), "secret") {
		t.Errorf("refusal does not say what is wrong with it: %v", err)
	}
}

// The digest is derived, so a snapshot claiming one without the bytes is the
// same lie in the other direction: it would say a credential is held.
func TestRestoreRefusesASecretDigestOnAKindWithAQueue(t *testing.T) {
	b := New()
	b.Restore(ports.Snapshot{Clean: true, Records: []protocol.Record{
		{Name: "worker@h", Kind: protocol.KindAgent, Owner: "owner@h", SecretSHA: "deadbeef"},
	}})
	if err := b.EstablishDaemonOwner("owner@h"); err == nil {
		t.Fatal("restored an agent claiming to hold a secret")
	}
}

// The control: the kind that is reached elsewhere keeps what it was given,
// or the refusal above would be passing by refusing everything.
func TestRestoreKeepsAServiceSecret(t *testing.T) {
	b := New()
	b.Restore(ports.Snapshot{Clean: true, Records: []protocol.Record{
		{Name: "db@h", Kind: protocol.KindService, Owner: "owner@h",
			Addr: "host:5432", Proto: "postgresql", Secret: "PGPASSWORD=kept"},
	}})
	if err := b.EstablishDaemonOwner("owner@h"); err != nil {
		t.Fatalf("refused a service holding a secret: %v", err)
	}
	got, err := b.Secret("db@h", "owner@h")
	if err != nil {
		t.Fatalf("reading the restored secret: %v", err)
	}
	if got != "PGPASSWORD=kept" {
		t.Errorf("restored secret is %q", got)
	}
}

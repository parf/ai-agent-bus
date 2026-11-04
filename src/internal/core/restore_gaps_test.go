package core

import (
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A stored group naming a nested group nothing holds is incorrect: no write
// allows it, and the grant would pass to whoever created the name. It is
// ignored and reported, and so is a group nesting only that one; a group
// nesting a stored group loads.
func TestRestoreIgnoresAGroupNestingAMissingGroup(t *testing.T) {
	b := New()
	j := &reports{}
	b.Journal(j)
	assertRestoreRefused(t, b, canonicalFixture(withUsers(ports.Snapshot{Clean: true, Records: []protocol.Record{
		{Name: "@crew@h", Kind: protocol.KindGroup, Owner: "alice@h", Allow: []string{"@gone@h"}},
		{Name: "@outer@h", Kind: protocol.KindGroup, Owner: "alice@h", Allow: []string{"@crew@h"}},
		{Name: "@team@h", Kind: protocol.KindGroup, Owner: "alice@h", Allow: []string{"bob@h"}},
		{Name: "@fine@h", Kind: protocol.KindGroup, Owner: "alice@h", Allow: []string{"@team@h"}},
	}}, "alice@h", "bob@h"), ""))
}

// A stored account mapping whose principal is no User or Agent is ignored and
// reported, its socket is not served, and removing it is an ordinary edit.
func TestRestoreIgnoresAnAccountMappingForNobody(t *testing.T) {
	b := New()
	j := &reports{}
	b.Journal(j)
	assertRestoreRefused(t, b, canonicalFixture(withUsers(ports.Snapshot{
		Clean: true, OwnerEstablished: true, Owner: "admin@h", AccountsEstablished: true,
		Accounts: []protocol.AccountMapping{
			{Account: "ghost", Principal: "#gone@h"},
			{Account: "ops", Principal: "alice@h"},
		},
		Records: []protocol.Record{{
			Name: AdministratorsGroup, Kind: protocol.KindGroup, Owner: "admin@h", Allow: []string{"admin@h"},
		}},
	}, "admin@h", "alice@h"), ""))
}

// A record ignored for sharing an ID keeps its credential, as an incorrect
// record does: the operator who repairs the row must not find the token gone.
func TestARecordIgnoredForADuplicateIDKeepsItsCredential(t *testing.T) {
	b := New()
	j := &reports{}
	b.Journal(j)
	assertRestoreRefused(t, b, canonicalFixture(withUsers(ports.Snapshot{Clean: true, Records: []protocol.Record{
		{ID: 50, Name: "#a@h", Kind: protocol.KindAgent, Owner: "alice@h", Full: protocol.OverflowStrict},
		{ID: 50, Name: "#b@h", Kind: protocol.KindAgent, Owner: "alice@h", Full: protocol.OverflowStrict},
	}}, "alice@h"), ""))
}

// An account ignored at load and mapped again once its principal exists is
// still unserved by this start, so the edit asks for the restart that serves
// it: the runner's upgrade path, a mapping restored before its User.
func TestAnIgnoredAccountMappedAgainNeedsTheRestart(t *testing.T) {
	b := New()
	assertRestoreRefused(t, b, canonicalFixture(withUsers(ports.Snapshot{
		Clean: true, OwnerEstablished: true, Owner: "admin@h", AccountsEstablished: true,
		Accounts: []protocol.AccountMapping{{Account: "agent-bus-runner", Principal: "agent-bus-runner@h"}},
		Records: []protocol.Record{{
			Name: AdministratorsGroup, Kind: protocol.KindGroup, Owner: "admin@h", Allow: []string{"admin@h"},
		}},
	}, "admin@h"), ""))
}

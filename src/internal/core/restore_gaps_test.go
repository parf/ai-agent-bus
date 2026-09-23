package core

import (
	"errors"
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
	b.Restore(withUsers(ports.Snapshot{Clean: true, Records: []protocol.Record{
		{Name: "@crew@h", Kind: protocol.KindGroup, Owner: "alice@h", Allow: []string{"@gone@h"}},
		{Name: "@outer@h", Kind: protocol.KindGroup, Owner: "alice@h", Allow: []string{"@crew@h"}},
		{Name: "@team@h", Kind: protocol.KindGroup, Owner: "alice@h", Allow: []string{"bob@h"}},
		{Name: "@fine@h", Kind: protocol.KindGroup, Owner: "alice@h", Allow: []string{"@team@h"}},
	}}, "alice@h", "bob@h"))
	for _, gone := range []string{"@crew@h", "@outer@h"} {
		if _, loaded := b.records[gone]; loaded {
			t.Fatalf("%s loaded though it nests a group nothing holds", gone)
		}
		if !j.has("stored record " + gone + " is ignored") {
			t.Fatalf("%s was not reported: %v", gone, j.lines)
		}
	}
	for _, kept := range []string{"@team@h", "@fine@h"} {
		if _, loaded := b.records[kept]; !loaded {
			t.Fatalf("%s was ignored: %v", kept, j.lines)
		}
	}
}

// A stored account mapping whose principal is no User or Agent is ignored and
// reported, its socket is not served, and removing it is an ordinary edit.
func TestRestoreIgnoresAnAccountMappingForNobody(t *testing.T) {
	b := New()
	j := &reports{}
	b.Journal(j)
	b.Restore(withUsers(ports.Snapshot{
		Clean: true, OwnerEstablished: true, Owner: "admin@h", AccountsEstablished: true,
		Accounts: []protocol.AccountMapping{
			{Account: "ghost", Principal: "#gone@h"},
			{Account: "ops", Principal: "alice@h"},
		},
		Records: []protocol.Record{{
			Name: AdministratorsGroup, Kind: protocol.KindGroup, Owner: "admin@h", Allow: []string{"admin@h"},
		}},
	}, "admin@h", "alice@h"))
	if err := b.EstablishAccounts(map[string]string{"ghost": "#gone@h", "ops": "alice@h"}); err != nil {
		t.Fatal(err)
	}
	if _, loaded := b.accounts["ghost"]; loaded {
		t.Fatal("a mapping for nobody was loaded")
	}
	if !j.has("stored local account ghost is ignored") {
		t.Fatalf("the ignored mapping was not reported: %v", j.lines)
	}
	if !b.Unserved("#gone@h") || b.Unserved("alice@h") {
		t.Fatalf("unserved: #gone@h=%v alice@h=%v", b.Unserved("#gone@h"), b.Unserved("alice@h"))
	}
	view, err := b.Accounts("admin@h")
	if err != nil || view.RestartRequired || len(view.Mappings) != 1 {
		t.Fatalf("the listing counted the ignored mapping: %+v %v", view, err)
	}
	if _, err := b.SetAccount("admin@h", "ghost", "", true); err != nil {
		t.Fatalf("the ignored mapping could not be removed: %v", err)
	}
	if b.Unserved("#gone@h") {
		t.Fatal("the removed mapping is still held as ignored")
	}
	if _, err := b.SetAccount("admin@h", "ghost", "", true); !errors.Is(err, ErrUnknown) {
		t.Fatalf("a second removal found something: %v", err)
	}
}

// A record ignored for sharing an ID keeps its credential, as an incorrect
// record does: the operator who repairs the row must not find the token gone.
func TestARecordIgnoredForADuplicateIDKeepsItsCredential(t *testing.T) {
	b := New()
	j := &reports{}
	b.Journal(j)
	b.Restore(withUsers(ports.Snapshot{Clean: true, Records: []protocol.Record{
		{ID: 50, Name: "#a@h", Kind: protocol.KindAgent, Owner: "alice@h", Full: protocol.OverflowStrict},
		{ID: 50, Name: "#b@h", Kind: protocol.KindAgent, Owner: "alice@h", Full: protocol.OverflowStrict},
	}}, "alice@h"))
	if _, loaded := b.records["#b@h"]; loaded || !j.has("share internal ID 50; #b@h is ignored") {
		t.Fatalf("#b@h was not ignored for the shared ID: %v", j.lines)
	}
	if got := b.Ownerless([]string{"#b@h", "#never@h"}); len(got) != 1 || got[0] != "#never@h" {
		t.Fatalf("the ownerless sweep would take %v; a duplicate-ID record's credential stays", got)
	}
}

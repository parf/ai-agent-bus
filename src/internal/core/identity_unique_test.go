package core

import (
	"errors"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Email, GitHub login and Twitter/X name each identify one User; a person's
// name identifies nobody. Checked on add, edit, import and restore.
// See docs/01-identity-and-roles.md#users-and-profiles.
func TestIdentifyingFieldsBelongToOneUser(t *testing.T) {
	b := New()
	b.SetDaemonOwner("owner@h")
	if _, err := b.SetUser("owner@h", protocol.User{Name: "alice@h", PersonName: "Sam", Email: "a@example.com", GithubUser: "alice-gh", GithubTwitterUsername: "Alice_X"}, true); err != nil {
		t.Fatal(err)
	}
	for field, u := range map[string]protocol.User{
		"email":          {Name: "bob@h", Email: " A@Example.com "},
		"GitHub login":   {Name: "bob@h", GithubUser: "Alice-GH"},
		"Twitter/X name": {Name: "bob@h", GithubTwitterUsername: "@alice_x"},
	} {
		if _, err := b.SetUser("owner@h", u, true); !errors.Is(err, ErrProfile) {
			t.Errorf("adding a user with alice's %s: %v", field, err)
		}
	}
	// A person's name is shared freely.
	if _, err := b.SetUser("owner@h", protocol.User{Name: "bob@h", PersonName: "Sam"}, true); err != nil {
		t.Fatalf("a shared person name was refused: %v", err)
	}
	// Editing into a clash is refused whole: the other field in the same
	// write does not land either.
	if _, err := b.SetUser("owner@h", protocol.User{Name: "bob@h", PersonName: "Bob", GithubTwitterUsername: "ALICE_X"}, false); !errors.Is(err, ErrProfile) {
		t.Fatalf("editing bob into alice's Twitter/X name: %v", err)
	}
	if u := userNamed(t, b, "bob@h"); u.PersonName != "Sam" || u.GithubTwitterUsername != "" {
		t.Fatalf("a refused edit changed bob: %+v", u)
	}
	if _, err := b.EditOwnEmail("bob@h", "a@example.com"); !errors.Is(err, ErrProfile) {
		t.Fatalf("bob took alice's email for himself: %v", err)
	}
	if _, err := b.SetUser("owner@h", protocol.User{Name: "bob@h", GithubTwitterUsername: "not a handle!"}, false); !errors.Is(err, ErrProfile) {
		t.Fatalf("an invalid Twitter/X name was stored: %v", err)
	}
}

// An import fills an identifying field only where no other User holds it.
func TestAGithubImportNeverSharesAnIdentity(t *testing.T) {
	d := &trackingGithub{profile: ports.DirectoryProfile{TwitterUsername: "@Bob_X", Email: "bob@example.com", Company: "Imported"}}
	b := githubProfileFixture(t, d)
	if _, err := b.SetUser("owner@h", protocol.User{Name: "bob@h", Email: "bob@example.com", GithubTwitterUsername: "bob_x"}, false); err != nil {
		t.Fatal(err)
	}
	got, err := b.SetUser("owner@h", protocol.User{Name: "alice@h", GithubUser: "alice-gh"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.GithubTwitterUsername != "" || got.Email == "bob@example.com" {
		t.Fatalf("the import gave alice bob's identity: twitter %q email %q", got.GithubTwitterUsername, got.Email)
	}
	if got.GithubCompany != "Imported" {
		t.Fatalf("the rest of the import was lost: %+v", got)
	}
}

// Two stored Users sharing an identifying field is damage no write of this
// version makes: the later one is ignored and reported, with its records.
func TestRestoreIgnoresAUserSharingAnIdentity(t *testing.T) {
	for field, second := range map[string]protocol.User{
		"email":          {Name: "bob@h", ID: 2, Email: "a@example.com"},
		"GitHub login":   {Name: "bob@h", ID: 2, GithubUser: "alice-gh"},
		"Twitter/X name": {Name: "bob@h", ID: 2, GithubTwitterUsername: "ALICE_X"},
	} {
		b := New()
		rep := &reports{}
		b.Journal(rep)
		b.Restore(ports.Snapshot{Clean: true,
			Users: []protocol.User{
				{Name: "alice@h", ID: 1, Email: "a@example.com", GithubUser: "alice-gh", GithubTwitterUsername: "alice_x", Status: "active"},
				second,
			},
			Records: []protocol.Record{
				{Name: "alice@h", ID: 10, Kind: protocol.KindUser, Owner: "alice@h", Personal: true, Full: protocol.OverflowStrict},
				{Name: "bob@h", ID: 11, Kind: protocol.KindUser, Owner: "bob@h", Personal: true, Full: protocol.OverflowStrict},
				{Name: "#bobs@h", ID: 12, Kind: protocol.KindAgent, Owner: "bob@h", Full: protocol.OverflowStrict},
			},
		})
		if !b.IsPerson("alice@h") || b.IsPerson("bob@h") {
			t.Errorf("%s: the earlier user must stay and the later go", field)
		}
		if !rep.has("stored user bob@h is ignored: its " + field) {
			t.Errorf("%s: the shared identity was not reported: %v", field, rep.lines)
		}
		if _, ok := b.RecordName(12); ok {
			t.Errorf("%s: the ignored user's agent was loaded", field)
		}
		if _, ok := b.RecordName(10); !ok {
			t.Errorf("%s: the kept user's record was not loaded", field)
		}
	}
}

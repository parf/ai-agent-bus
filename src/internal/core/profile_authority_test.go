package core

import (
	"errors"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func profileAuthorityFixture(t *testing.T) *Bus {
	t.Helper()
	b := New()
	b.SetDaemonOwner("owner@h")
	enableGithubProfiles(b)
	for _, u := range []protocol.User{
		{Name: "admin@h", PersonName: "Admin"},
		{Name: "peer@h", PersonName: "Peer"},
		{Name: "alice@h", PersonName: "Alice", Email: "old@example.com", GithubUser: "alice-gh"},
		{Name: "bob@h", PersonName: "Bob", Email: "bob@example.com", GithubUser: "bob-gh"},
	} {
		if _, err := b.SetUser("owner@h", u, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.SetGroup("owner@h", AdministratorsGroup, []string{"owner@h", "admin@h", "peer@h"}); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestUserEditsOnlyTheirOwnEmail(t *testing.T) {
	b := profileAuthorityFixture(t)
	got, err := b.EditOwnEmail("alice@h", " New@Example.COM ")
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != "new@example.com" || got.PersonName != "Alice" || got.GithubUser != "alice-gh" || got.Status != "active" {
		t.Fatalf("self edit changed protected fields or missed normalization: %+v", got)
	}
	if !got.CanSetEmail || got.CanEdit {
		t.Fatalf("self and administrative capabilities were merged: %+v", got)
	}
	if _, err := b.EditOwnEmail("alice@h", "BOB@example.com"); !errors.Is(err, ErrProfile) {
		t.Fatalf("duplicate self email: %v", err)
	}
	if got := b.Users("alice@h", nil)[0]; got.Email != "new@example.com" {
		t.Fatalf("refused duplicate changed email: %+v", got)
	}
	if got, err := b.EditOwnEmail("alice@h", ""); err != nil || got.Email != "" || got.PersonName != "Alice" {
		t.Fatalf("user could not clear optional email without changing protected fields: %+v, %v", got, err)
	}
	if _, err := b.EditOwnEmail("alice@h", "new@example.com"); err != nil {
		t.Fatalf("user could not set email again after clearing it: %v", err)
	}
	if _, err := b.EditOwnEmail("alice@h", "not an address"); !errors.Is(err, ErrProfile) {
		t.Fatalf("invalid self email: %v", err)
	}
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.EditOwnEmail("#svc@h", "svc@example.com"); !errors.Is(err, ErrUnknown) {
		t.Fatalf("record-only identity edited a profile: %v", err)
	}
	if _, err := b.SetUserState("admin@h", "alice@h", "inactive"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.EditOwnEmail("alice@h", "paused@example.com"); !errors.Is(err, ErrInactive) {
		t.Fatalf("inactive user edited their profile: %v", err)
	}
	if _, err := b.EditOwnEmail("unknown@h", "unknown@example.com"); !errors.Is(err, ErrNoPrincipal) {
		t.Fatalf("unknown identity edited a profile: %v", err)
	}
}

func TestAdministratorUnbansOnlyOrdinaryUsers(t *testing.T) {
	b := profileAuthorityFixture(t)
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#alice-svc@h", Owner: "alice@h", Allow: []string{"admin@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetUserState("admin@h", "alice@h", "inactive"); err != nil {
		t.Fatal(err)
	}
	if err := b.Authenticate("#alice-svc@h"); !errors.Is(err, ErrInactive) {
		t.Fatalf("banned owner's service stayed active: %v", err)
	}
	views := b.Users("admin@h", nil)
	for _, u := range views {
		if u.Name == "alice@h" && !u.CanActivate {
			t.Fatal("administrator was not told it may unban an ordinary user")
		}
	}
	if _, err := b.SetUserState("admin@h", "alice@h", "active"); err != nil {
		t.Fatalf("administrator could not unban ordinary user: %v", err)
	}
	if err := b.Authenticate("#alice-svc@h"); err != nil {
		t.Fatalf("unban did not restore directly owned service: %v", err)
	}

	if _, err := b.SetUserState("owner@h", "peer@h", "inactive"); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"peer@h", "admin@h", "owner@h"} {
		if _, err := b.SetUserState("admin@h", target, "active"); !errors.Is(err, ErrNotOwner) {
			t.Fatalf("administrator unbanned protected %s: %v", target, err)
		}
	}

	if _, err := b.SetUserState("admin@h", "bob@h", "inactive"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetUser("admin@h", protocol.User{Name: "bob@h", PersonName: "Bob", Email: "bob@example.com", GithubUser: "bob-gh", Status: "active"}, false); err != nil {
		t.Fatalf("administrator could not unban through administrative profile update: %v", err)
	}
}

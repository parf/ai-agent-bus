package core

import (
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A snapshot is a file an operator can edit and a partial write can damage, so
// it is an input rather than a trusted store. These pin that the derived
// fields — what a visitor may do and what the daemon worked out — are cleared
// on the way in, and so cannot survive into the next snapshot.

// forged is a user carrying every derived field set, alongside durable profile
// and lifecycle state that a restore exists to bring back.
func forged() protocol.User {
	return protocol.User{
		Name: "alice@h", PersonName: "Alice", Email: "alice@example.com",
		Status: "inactive", GithubUser: "alice", GithubCompany: "ACME",
		GithubLocation: "Boston", GithubTwitterUsername: "alice",
		GithubProfileAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		GithubAvatarURL: "https://example.invalid/a.png", GithubGravatarID: "abc",
		PhotoPNG: []byte{1, 2, 3}, PhotoSource: "github",
		PhotoFetchedAt: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),

		Kind: "forged", Administrator: true, DaemonOwner: true,
		CanEdit: true, CanSetEmail: true, CanActivate: true, CanRemove: true,
		Groups: []string{"@administrators"}, Services: []string{"#svc@h"},
	}
}

func derivedClaims(u protocol.User) []string {
	var claimed []string
	for label, set := range map[string]bool{
		"Kind": u.Kind != "", "Administrator": u.Administrator,
		"DaemonOwner": u.DaemonOwner, "CanEdit": u.CanEdit,
		"CanSetEmail": u.CanSetEmail, "CanActivate": u.CanActivate,
		"CanRemove": u.CanRemove, "Groups": len(u.Groups) > 0,
		"Services": len(u.Services) > 0,
	} {
		if set {
			claimed = append(claimed, label)
		}
	}
	return claimed
}

func TestRestoredDerivedFieldsAreStrippedAndDoNotReachTheNextSnapshot(t *testing.T) {
	b := New()
	b.Restore(ports.Snapshot{Clean: true, Users: []protocol.User{forged()}, Records: []protocol.Record{userRecord("alice@h")}})

	b.mu.Lock()
	stored := b.users["alice@h"]
	b.mu.Unlock()
	if claimed := derivedClaims(stored); len(claimed) > 0 {
		t.Errorf("restore stored derived fields %v", claimed)
	}

	// The durable question: a forged value must not come back out of the file
	// it was written into, or the next run restores it again.
	var out protocol.User
	for _, u := range b.Snapshot().Users {
		if u.Name == "alice@h" {
			out = u
		}
	}
	if out.Name == "" {
		t.Fatal("alice@h did not survive the round trip at all")
	}
	if claimed := derivedClaims(out); len(claimed) > 0 {
		t.Errorf("snapshot carried derived fields %v back out", claimed)
	}
}

// The other half, and the one a careless clear breaks: a restore exists to
// bring durable state back. Over-clearing loses it permanently.
func TestRestoreKeepsDurableProfileLifecycleAndProviderState(t *testing.T) {
	b := New()
	want := forged()
	b.Restore(ports.Snapshot{Clean: true, Users: []protocol.User{want}, Records: []protocol.Record{userRecord("alice@h")}})
	b.mu.Lock()
	got := b.users["alice@h"]
	b.mu.Unlock()

	for _, f := range []struct {
		label     string
		got, want any
	}{
		{"PersonName", got.PersonName, want.PersonName},
		{"Email", got.Email, want.Email},
		{"State", got.Status, want.Status},
		{"GithubUser", got.GithubUser, want.GithubUser},
		{"GithubCompany", got.GithubCompany, want.GithubCompany},
		{"GithubLocation", got.GithubLocation, want.GithubLocation},
		{"GithubTwitterUsername", got.GithubTwitterUsername, want.GithubTwitterUsername},
		{"GithubProfileAt", got.GithubProfileAt, want.GithubProfileAt},
		{"GithubAvatarURL", got.GithubAvatarURL, want.GithubAvatarURL},
		{"GithubGravatarID", got.GithubGravatarID, want.GithubGravatarID},
		{"PhotoSource", got.PhotoSource, want.PhotoSource},
		{"PhotoFetchedAt", got.PhotoFetchedAt, want.PhotoFetchedAt},
		{"len(PhotoPNG)", len(got.PhotoPNG), len(want.PhotoPNG)},
	} {
		if f.got != f.want {
			t.Errorf("restore lost %s: got %v, want %v", f.label, f.got, f.want)
		}
	}
}

// The write path was already closed; this keeps it closed, so a later change
// cannot move the clearing out of one path while believing the other covers it.
func TestAnOrdinaryProfileWriteStillCannotClaimDerivedFields(t *testing.T) {
	b := New()
	b.SetDaemonOwner("owner@h")
	if _, err := b.SetUser("owner@h", forged(), true); err != nil {
		t.Fatal(err)
	}
	b.mu.Lock()
	stored := b.users["alice@h"]
	b.mu.Unlock()
	if claimed := derivedClaims(stored); len(claimed) > 0 {
		t.Errorf("an ordinary write stored derived fields %v", claimed)
	}
}

package core

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

type profileDirectory struct{ entry ports.DirectoryEntry }

func (d *profileDirectory) Lookup(string) (ports.DirectoryEntry, error) { return d.entry, nil }
func (d *profileDirectory) Profile(login string) (ports.DirectoryProfile, error) {
	p := d.entry.Profile
	p.Login, p.FetchedAt = login, time.Now()
	return p, nil
}

func enableGithubProfiles(b *Bus) {
	b.Directories(map[string]ports.Directory{"github": &profileDirectory{}}, nil)
}

type profileSignatures struct{}

func (profileSignatures) Verify(_, _, signature string, _ []string) error {
	if signature != "good" {
		return errors.New("wrong signature")
	}
	return nil
}

func enrolledProfile(t *testing.T, b *Bus, name string) protocol.User {
	t.Helper()
	users := b.Users(name, nil)
	if len(users) != 1 || users[0].Name != name {
		t.Fatalf("missing enrolled profile for %s: %+v", name, users)
	}
	return users[0]
}

func TestEnrolmentRetainsTrustedPersonNameWithKeys(t *testing.T) {
	dir := &profileDirectory{entry: ports.DirectoryEntry{Keys: []string{"key-one"}, Profile: ports.DirectoryProfile{Login: "alice", PersonName: " Alice Provider ", Email: "PUBLIC@example.com", Company: "First Company", PhotoPNG: []byte("first-photo"), PhotoSource: "github"}}}
	b := New()
	b.Directories(map[string]ports.Directory{"github": dir}, profileSignatures{})
	nonce, err := b.Challenge("alice@github")
	if err != nil {
		t.Fatal(err)
	}
	// A provider change after the challenge cannot change either half of the
	// retained directory statement used by this proof.
	dir.entry = ports.DirectoryEntry{Keys: []string{"key-two"}, Profile: ports.DirectoryProfile{Login: "alice", PersonName: "Mallory Later", Email: "mallory@example.com", Company: "Changed Company", PhotoPNG: []byte("changed-photo"), PhotoSource: "github"}}
	if _, err := b.Enrol(nonce, "good"); err != nil {
		t.Fatal(err)
	}
	if got := enrolledProfile(t, b, "alice@github"); got.PersonName != "Alice Provider" || got.Email != "public@example.com" || got.GithubUser != "alice" || got.GithubCompany != "First Company" || string(got.PhotoPNG) != "first-photo" {
		t.Fatalf("enrolment did not retain the trusted profile: %+v", got)
	}
}

func TestEnrolmentDoesNotOverwriteAdministratorPersonName(t *testing.T) {
	dir := &profileDirectory{entry: ports.DirectoryEntry{Keys: []string{"key"}, Profile: ports.DirectoryProfile{Login: "alice", PersonName: "Provider First"}}}
	b := New()
	b.SetDaemonOwner("owner@h")
	b.Directories(map[string]ports.Directory{"github": dir}, profileSignatures{})
	nonce, _ := b.Challenge("alice@github")
	if _, err := b.Enrol(nonce, "good"); err != nil {
		t.Fatal(err)
	}
	u := enrolledProfile(t, b, "alice@github")
	u.PersonName = "Administrator Choice"
	if _, err := b.SetUser("owner@h", u, false); err != nil {
		t.Fatal(err)
	}
	dir.entry.Profile.PersonName = "Provider Later"
	nonce, _ = b.Challenge("alice@github")
	if _, err := b.Enrol(nonce, "good"); err != nil {
		t.Fatal(err)
	}
	if got := enrolledProfile(t, b, "alice@github"); got.PersonName != "Administrator Choice" {
		t.Fatalf("re-enrolment overwrote the explicit profile: %+v", got)
	}
}

func TestInvalidTrustedPersonNameCreatesNothing(t *testing.T) {
	dir := &profileDirectory{entry: ports.DirectoryEntry{Keys: []string{"key"}, Profile: ports.DirectoryProfile{Login: "alice", PersonName: strings.Repeat("x", 201)}}}
	b := New()
	b.SetDaemonOwner("owner@h")
	b.Directories(map[string]ports.Directory{"github": dir}, profileSignatures{})
	nonce, _ := b.Challenge("alice@github")
	if _, err := b.Enrol(nonce, "bad"); err == nil {
		t.Fatal("wrong signature enrolled a profile")
	}
	if got := b.Users("owner@h", nil); len(got) != 1 || got[0].Name != "owner@h" {
		t.Fatalf("wrong signature changed users: %+v", got)
	}
	nonce, _ = b.Challenge("alice@github")
	if _, err := b.Enrol(nonce, "good"); !errors.Is(err, ErrProfile) {
		t.Fatalf("overlong provider name: %v", err)
	}
	if _, known := b.Lookup("owner@h", "alice@github"); known {
		t.Fatal("invalid provider name left a record")
	}
}

func TestDirectoryWithoutPersonNameStillEnrols(t *testing.T) {
	b := New()
	b.Directories(map[string]ports.Directory{"manual": &profileDirectory{entry: ports.DirectoryEntry{Keys: []string{"key"}}}}, profileSignatures{})
	nonce, _ := b.Challenge("alice@manual")
	if _, err := b.Enrol(nonce, "good"); err != nil {
		t.Fatal(err)
	}
	if got := enrolledProfile(t, b, "alice@manual"); got.PersonName != "" {
		t.Fatalf("manual directory invented a person name: %+v", got)
	}
}

func TestEnrolmentSkipsOptionalPublicEmailCollision(t *testing.T) {
	dir := &profileDirectory{entry: ports.DirectoryEntry{Keys: []string{"key"}, Profile: ports.DirectoryProfile{Login: "alice", Email: "BOB@example.com", Company: "Still Imported"}}}
	b := New()
	b.SetDaemonOwner("owner@h")
	if _, err := b.SetUser("owner@h", protocol.User{Name: "bob@h", Email: "bob@example.com"}, true); err != nil {
		t.Fatal(err)
	}
	b.Directories(map[string]ports.Directory{"github": dir}, profileSignatures{})
	nonce, err := b.Challenge("alice@github")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Enrol(nonce, "good"); err != nil {
		t.Fatalf("optional email collision refused enrolment: %v", err)
	}
	if got := enrolledProfile(t, b, "alice@github"); got.Email != "" || got.GithubCompany != "Still Imported" {
		t.Fatalf("optional collision imported email or lost profile: %+v", got)
	}
}

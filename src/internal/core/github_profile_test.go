package core

import (
	"bytes"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

type trackingGithub struct {
	mu      sync.Mutex
	profile ports.DirectoryProfile
	err     error
	calls   int
	started chan struct{}
	release chan struct{}
}

func (d *trackingGithub) Lookup(login string) (ports.DirectoryEntry, error) {
	p, err := d.Profile(login)
	return ports.DirectoryEntry{Keys: []string{"key"}, Profile: p}, err
}

func (d *trackingGithub) Profile(login string) (ports.DirectoryProfile, error) {
	d.mu.Lock()
	d.calls++
	p, err, started, release := d.profile, d.err, d.started, d.release
	d.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if release != nil {
		<-release
	}
	if p.Login == "" {
		p.Login = login
	}
	return p, err
}

func (d *trackingGithub) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.calls
}

func githubProfileFixture(t *testing.T, d *trackingGithub) *Bus {
	t.Helper()
	b := New()
	b.SetDaemonOwner("owner@h")
	for _, u := range []protocol.User{{Name: "alice@h"}, {Name: "bob@h", Email: "bob@example.com"}} {
		if _, err := b.SetUser("owner@h", u, true); err != nil {
			t.Fatal(err)
		}
	}
	b.Directories(map[string]ports.Directory{"github": d}, nil)
	return b
}

func userNamed(t *testing.T, b *Bus, name string) protocol.User {
	t.Helper()
	for _, u := range b.Users("owner@h", nil) {
		if u.Name == name {
			return u
		}
	}
	t.Fatalf("missing user %s", name)
	return protocol.User{}
}

func TestGithubLoginChangeImportsProviderProfileOnce(t *testing.T) {
	at := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	d := &trackingGithub{profile: ports.DirectoryProfile{
		PersonName: " Alice Provider ", Email: " PUBLIC@EXAMPLE.COM ",
		Company: " Example Inc ", Location: " New York ", TwitterUsername: "@alice",
		AvatarURL: "https://avatars.githubusercontent.com/u/1", GravatarID: "legacy", FetchedAt: at,
		PhotoPNG: []byte("normalized-png"), PhotoSource: "github", PhotoFetchedAt: at,
	}}
	b := githubProfileFixture(t, d)
	got, err := b.SetUser("owner@h", protocol.User{Name: "alice@h", GithubUser: "Alice-GH"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.GithubUser != "alice-gh" || got.PersonName != "Alice Provider" || got.Email != "public@example.com" ||
		got.GithubCompany != "Example Inc" || got.GithubLocation != "New York" || got.GithubTwitterUsername != "alice" ||
		got.GithubAvatarURL == "" || got.GithubGravatarID != "legacy" || !got.GithubProfileAt.Equal(at) ||
		got.PhotoSource != "github" || !got.PhotoFetchedAt.Equal(at) || !bytes.Equal(got.PhotoPNG, []byte("normalized-png")) {
		t.Fatalf("provider profile not retained: %+v", got)
	}
	if d.count() != 1 {
		t.Fatalf("provider calls = %d, want 1", d.count())
	}

	// Saving editable profile fields must not perform provider I/O. Provider
	// provenance and image data survive while the AgentBus fields may change.
	got, err = b.SetUser("owner@h", protocol.User{Name: "alice@h", PersonName: "Administrator Name", Email: got.Email, GithubUser: got.GithubUser, GithubCompany: "Local Company", GithubLocation: "Boston", GithubTwitterUsername: "local"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if d.count() != 1 || got.PersonName != "Administrator Name" || got.GithubCompany != "Local Company" || got.GithubLocation != "Boston" || got.GithubTwitterUsername != "local" || !bytes.Equal(got.PhotoPNG, []byte("normalized-png")) {
		t.Fatalf("profile edit fetched or lost trusted decoration: calls=%d user=%+v", d.count(), got)
	}
}

func TestGithubLoginChangeKeepsEditableFieldsButRejectsClaimedProvenance(t *testing.T) {
	at := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	d := &trackingGithub{profile: ports.DirectoryProfile{
		Company: "Provider Company", AvatarURL: "https://avatars.githubusercontent.com/u/1", FetchedAt: at,
	}}
	b := githubProfileFixture(t, d)
	claimedAt := at.Add(-24 * time.Hour)
	got, err := b.SetUser("owner@h", protocol.User{
		Name: "alice@h", GithubUser: "alice",
		GithubCompany: "Caller Claim", GithubProfileAt: claimedAt,
		PhotoPNG: []byte("caller-controlled-image"), PhotoSource: "github", PhotoFetchedAt: claimedAt,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.GithubCompany != "Caller Claim" || !got.GithubProfileAt.Equal(at) || len(got.PhotoPNG) != 0 || got.PhotoSource != "" || !got.PhotoFetchedAt.IsZero() {
		t.Fatalf("editable field or trusted provider boundary is wrong: %+v", got)
	}
}

func TestGithubProfileFailureDoesNotBlockLoginAndEmailCollisionDoesNotBlockImport(t *testing.T) {
	d := &trackingGithub{err: errors.New("provider down")}
	b := githubProfileFixture(t, d)
	got, err := b.SetUser("owner@h", protocol.User{Name: "alice@h", PersonName: "Committed", GithubUser: "alice"}, false)
	if err != nil {
		t.Fatalf("optional profile failure blocked login: %v", err)
	}
	if got.GithubUser != "alice" || got.PersonName != "Committed" || !got.GithubProfileAt.IsZero() || got.GithubCompany != "" {
		t.Fatalf("unavailable provider did not leave an honest unfetched profile: %+v", got)
	}
	beforeRefresh := got
	if _, err := b.RefreshGithub("owner@h", "alice@h"); !errors.Is(err, ErrProfile) {
		t.Fatalf("explicit refresh hid provider failure: %v", err)
	}
	if after := userNamed(t, b, "alice@h"); !reflect.DeepEqual(beforeRefresh, after) {
		t.Fatalf("failed explicit refresh changed profile: before=%+v after=%+v", beforeRefresh, after)
	}

	d.err = nil
	d.profile = ports.DirectoryProfile{PersonName: "Provider", Email: "BOB@EXAMPLE.COM", Company: "Kept", FetchedAt: time.Now()}
	got, err = b.RefreshGithub("owner@h", "alice@h")
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != "" || got.PersonName != "Committed" || got.GithubCompany != "Kept" {
		t.Fatalf("optional duplicate email blocked or imported: %+v", got)
	}
}

func TestGithubLoginCanBeSetWithoutProfileAdapter(t *testing.T) {
	b := New()
	b.SetDaemonOwner("owner@h")
	if _, err := b.SetUser("owner@h", protocol.User{Name: "alice@h"}, true); err != nil {
		t.Fatal(err)
	}
	got, err := b.SetUser("owner@h", protocol.User{Name: "alice@h", GithubUser: "parf"}, false)
	if err != nil {
		t.Fatalf("missing optional adapter blocked login: %v", err)
	}
	if got.GithubUser != "parf" || !got.GithubProfileAt.IsZero() || got.GithubCompany != "" || len(got.PhotoPNG) != 0 {
		t.Fatalf("missing adapter fabricated provider data: %+v", got)
	}
}

func TestProfileDetailPresencePreservesOldWritersAndAllowsExplicitClear(t *testing.T) {
	b := New()
	b.SetDaemonOwner("owner@h")
	if _, err := b.SetUser("owner@h", protocol.User{Name: "alice@h", GithubCompany: "Company", GithubLocation: "Place", GithubTwitterUsername: "handle"}, true); err != nil {
		t.Fatal(err)
	}
	got, err := b.SetUserWithProfileDetails("owner@h", protocol.User{Name: "alice@h", PersonName: "Alice"}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.GithubCompany != "Company" || got.GithubLocation != "Place" || got.GithubTwitterUsername != "handle" {
		t.Fatalf("old writer erased profile details: %+v", got)
	}
	got, err = b.SetUserWithProfileDetails("owner@h", protocol.User{Name: "alice@h", PersonName: "Alice"}, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.GithubCompany != "" || got.GithubLocation != "" || got.GithubTwitterUsername != "" {
		t.Fatalf("explicit clear retained profile details: %+v", got)
	}
}

func TestGithubRefreshPreservesFailedPhotoAndClearsAbsentSources(t *testing.T) {
	at := time.Now().UTC()
	d := &trackingGithub{profile: ports.DirectoryProfile{FetchedAt: at, AvatarURL: "https://avatars.githubusercontent.com/u/1", PhotoPNG: []byte("old"), PhotoSource: "github", PhotoFetchedAt: at}}
	b := githubProfileFixture(t, d)
	if _, err := b.SetUser("owner@h", protocol.User{Name: "alice@h", GithubUser: "alice"}, false); err != nil {
		t.Fatal(err)
	}

	// Changing the login replaces provider facts, but an optional image
	// failure keeps the last normalized local thumbnail as the fallback.
	d.profile = ports.DirectoryProfile{FetchedAt: at.Add(30 * time.Minute), Company: "Changed login", AvatarURL: "https://avatars.githubusercontent.com/u/2"}
	got, err := b.SetUser("owner@h", protocol.User{Name: "alice@h", GithubUser: "alice-new"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.GithubUser != "alice-new" || got.GithubCompany != "Changed login" || !bytes.Equal(got.PhotoPNG, []byte("old")) || got.PhotoSource != "github" {
		t.Fatalf("login change with failed optional photo lost its fallback: %+v", got)
	}

	// A source existed but optional import failed: metadata refreshes and the
	// last usable local image stays.
	d.profile = ports.DirectoryProfile{FetchedAt: at.Add(time.Hour), Company: "New", AvatarURL: "https://avatars.githubusercontent.com/u/2"}
	got, err = b.RefreshGithub("owner@h", "alice@h")
	if err != nil {
		t.Fatal(err)
	}
	if got.GithubCompany != "New" || !bytes.Equal(got.PhotoPNG, []byte("old")) || got.PhotoSource != "github" {
		t.Fatalf("failed optional photo destroyed fallback: %+v", got)
	}

	// A complete answer explicitly naming no source clears provider-owned art.
	d.profile = ports.DirectoryProfile{FetchedAt: at.Add(2 * time.Hour), ClearPhoto: true}
	got, err = b.RefreshGithub("owner@h", "alice@h")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.PhotoPNG) != 0 || got.PhotoSource != "" || !got.PhotoFetchedAt.IsZero() {
		t.Fatalf("removed sources retained photo: %+v", got)
	}

	// Removing the provider login clears mirrored data, but imported identity
	// fields stay ordinary local profile values.
	got, err = b.SetUser("owner@h", protocol.User{Name: "alice@h", PersonName: "Local", Email: "local@example.com"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.GithubUser != "" || got.GithubCompany != "" || !got.GithubProfileAt.IsZero() || got.PersonName != "Local" || got.Email != "local@example.com" {
		t.Fatalf("clearing login had wrong ownership boundary: %+v", got)
	}
}

func TestGithubRefreshRefusesStaleLoginAndSnapshotKeepsPhoto(t *testing.T) {
	started, release := make(chan struct{}, 1), make(chan struct{})
	d := &trackingGithub{profile: ports.DirectoryProfile{FetchedAt: time.Now(), PhotoPNG: []byte("photo"), PhotoSource: "github", PhotoFetchedAt: time.Now()}}
	b := githubProfileFixture(t, d)
	if _, err := b.SetUser("owner@h", protocol.User{Name: "alice@h", GithubUser: "alice"}, false); err != nil {
		t.Fatal(err)
	}
	s := b.Snapshot()
	restored := New()
	restored.Restore(s)
	if got := userNamed(t, restored, "alice@h"); !bytes.Equal(got.PhotoPNG, []byte("photo")) || got.PhotoSource != "github" {
		t.Fatalf("snapshot lost provider photo: %+v", got)
	}

	d.started, d.release = started, release
	done := make(chan error, 1)
	go func() { _, err := b.RefreshGithub("owner@h", "alice@h"); done <- err }()
	<-started
	if _, err := b.SetUser("owner@h", protocol.User{Name: "alice@h"}, false); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-done; !errors.Is(err, ErrBusy) {
		t.Fatalf("stale refresh: %v", err)
	}
	if got := userNamed(t, b, "alice@h"); got.GithubUser != "" {
		t.Fatalf("stale refresh restored cleared login: %+v", got)
	}
}

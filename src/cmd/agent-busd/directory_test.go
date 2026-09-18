package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

type githubFixture struct {
	lookups, profiles int
}

func (f *githubFixture) Lookup(login string) (ports.DirectoryEntry, error) {
	f.lookups++
	return ports.DirectoryEntry{Keys: []string{"fixture-key"}}, nil
}

func (f *githubFixture) Profile(login string) (ports.DirectoryProfile, error) {
	f.profiles++
	return ports.DirectoryProfile{Login: login, PersonName: "Provider Person", FetchedAt: time.Now()}, nil
}

func TestGithubProfilesDoNotBackARealmUnlessConfigured(t *testing.T) {
	b, provider := core.New(), &githubFixture{}
	if err := configureDirectories(b, nil, provider); err != nil {
		t.Fatal(err)
	}
	b.SetDaemonOwner("owner@h")
	if _, err := b.SetUser("owner@h", protocol.User{Name: "alice@h"}, true); err != nil {
		t.Fatal(err)
	}
	got, err := b.SetUser("owner@h", protocol.User{Name: "alice@h", GithubUser: "parf"}, false)
	if err != nil {
		t.Fatalf("default profile adapter unavailable: %v", err)
	}
	if got.GithubUser != "parf" || got.PersonName != "Provider Person" || provider.profiles != 1 {
		t.Fatalf("default profile adapter did not import the login: user=%+v calls=%d", got, provider.profiles)
	}
	if _, err := b.Challenge("alice@github"); !errors.Is(err, core.ErrEnrol) || !strings.Contains(err.Error(), `nothing backs the realm "github"`) {
		t.Fatalf("profile adapter unexpectedly backed the GitHub realm: %v", err)
	}
	if provider.lookups != 0 {
		t.Fatalf("unconfigured GitHub realm reached the enrolment adapter %d times", provider.lookups)
	}
}

func TestConfiguredGithubRealmReusesProfileAdapter(t *testing.T) {
	b, provider := core.New(), &githubFixture{}
	if err := configureDirectories(b, []string{"code=github"}, provider); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Challenge("alice@code"); err != nil {
		t.Fatal(err)
	}
	if provider.lookups != 1 {
		t.Fatalf("configured realm used another adapter: %d lookups", provider.lookups)
	}
	if err := configureDirectories(b, []string{"broken"}, provider); err == nil || !strings.Contains(err.Error(), "--directory wants") {
		t.Fatalf("bad directory setting accepted: %v", err)
	}
}

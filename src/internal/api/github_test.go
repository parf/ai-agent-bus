package api

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

type testGithubDirectory struct{}

func (testGithubDirectory) Lookup(login string) (ports.DirectoryEntry, error) {
	p, err := (testGithubDirectory{}).Profile(login)
	return ports.DirectoryEntry{Keys: []string{"key"}, Profile: p}, err
}

func (testGithubDirectory) Profile(login string) (ports.DirectoryProfile, error) {
	return ports.DirectoryProfile{Login: login, FetchedAt: time.Now()}, nil
}

func enableGithubProfiles(b *core.Bus) {
	b.Directories(map[string]ports.Directory{"github": testGithubDirectory{}}, nil)
}

func TestGithubRefreshRouteIsStrictAndAdministrative(t *testing.T) {
	b := core.New()
	enableGithubProfiles(b)
	s, token := serverFor(t, b, "owner@h")
	if _, err := b.SetUser("owner@h", protocol.User{Name: "alice@h", GithubUser: "alice"}, true); err != nil {
		t.Fatal(err)
	}
	call := func(who, body string, want int) {
		t.Helper()
		r := httptest.NewRequest("POST", "/user/github-refresh", strings.NewReader(body))
		r.Header.Set(HeaderToken, token(who))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s refresh: %d %s, want %d", who, w.Code, w.Body.String(), want)
		}
	}
	call("owner@h", `{"name":"alice@h"}`, 200)
	call("owner@h", `{"name":"alice@h","person_name":"forged"}`, 400)
	call("alice@h", `{"name":"alice@h"}`, 403)
}

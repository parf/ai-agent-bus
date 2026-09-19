package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

type webGithub struct {
	calls   atomic.Int32
	profile ports.DirectoryProfile
}

func (d *webGithub) Lookup(login string) (ports.DirectoryEntry, error) {
	p, err := d.Profile(login)
	return ports.DirectoryEntry{Keys: []string{"key"}, Profile: p}, err
}

func (d *webGithub) Profile(login string) (ports.DirectoryProfile, error) {
	d.calls.Add(1)
	p := d.profile
	p.Login = login
	return p, nil
}

func TestGithubProfileAndPhotoStayLocalAndVisibilityBounded(t *testing.T) {
	photo := []byte("NORMALIZED-LOCAL-PNG")
	provider := &webGithub{profile: ports.DirectoryProfile{
		PersonName: "Alice Provider", Company: "Example Company", Location: "New York",
		TwitterUsername: "alice_x", GravatarID: "legacy", AvatarURL: "https://avatars.githubusercontent.com/u/1",
		FetchedAt: time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC), PhotoPNG: photo, PhotoSource: "github", PhotoFetchedAt: time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC),
	}}
	b := core.New()
	b.SetDaemonOwner("owner@h")
	tokens, err := auth.Load(memory.NewTokens(), "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	b.Directories(map[string]ports.Directory{"identity-provider": provider}, nil)
	for _, u := range []protocol.User{{Name: "alice@h", GithubUser: "alice"}, {Name: "visitor@h"}} {
		if _, err := b.SetUser("owner@h", u, true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "reports@h", Owner: "alice@h", Allow: []string{"visitor@h"}}); err != nil {
		t.Fatal(err)
	}
	backend := httptest.NewServer(api.New(b, tokens, "owner@h").Handler())
	defer backend.Close()
	web := httptest.NewServer(dashboard(&caller{client: backend.Client(), base: backend.URL}, false))
	defer web.Close()
	client := web.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	sessions := map[string]*http.Cookie{}
	for _, who := range []string{"owner@h", "visitor@h"} {
		token, err := tokens.Issue(who)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.PostForm(web.URL+"/signin", url.Values{"token": {token}})
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		sessions[who] = resp.Cookies()[0]
	}
	request := func(who, method, path string, form url.Values, want int) ([]byte, http.Header) {
		t.Helper()
		var body io.Reader
		if form != nil {
			body = strings.NewReader(form.Encode())
		}
		req, _ := http.NewRequest(method, web.URL+path, body)
		req.AddCookie(sessions[who])
		if form != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Origin", web.URL)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != want {
			t.Fatalf("%s %s: %d %s, want %d", method, path, resp.StatusCode, data, want)
		}
		return data, resp.Header
	}

	if provider.calls.Load() != 1 {
		t.Fatalf("profile setup calls = %d, want 1", provider.calls.Load())
	}
	list, listHeader := request("owner@h", "GET", "/users?q=Example+Company", nil, 200)
	if !bytes.Contains(list, []byte("data:image/png;base64,")) || bytes.Contains(list, []byte("/avatar?")) || bytes.Contains(list, []byte("avatars.githubusercontent.com")) || !bytes.Contains(list, []byte("alice@h")) {
		t.Fatalf("directory did not use its bounded local photo answer: %s", list)
	}
	if !strings.Contains(listHeader.Get("Content-Security-Policy"), "img-src 'self' data:") {
		t.Fatalf("local embedded thumbnail is blocked by CSP: %q", listHeader.Get("Content-Security-Policy"))
	}
	detail, _ := request("owner@h", "GET", "/user?name=alice@h", nil, 200)
	for _, want := range []string{"Example Company", "New York", "alice_x", "data:image/png;base64,"} {
		if !bytes.Contains(detail, []byte(want)) {
			t.Fatalf("user detail lacks %q: %s", want, detail)
		}
	}
	// The imported values are editable where every profile value is edited:
	// the one form, which is also the one that adds a person.
	editor, _ := request("owner@h", "GET", "/user/edit?name=alice@h", nil, 200)
	for _, want := range []string{"Example Company", "New York", `name=company`, `name=location`, `name=twitter`, "alice_x"} {
		if !bytes.Contains(editor, []byte(want)) {
			t.Fatalf("the profile form lacks %q: %s", want, editor)
		}
	}
	// The owner removed both at 0.5.83: the refresh control, and the hint
	// that a public GitHub email fills a blank address. The refresh itself
	// is unchanged and is still exercised below through POST /user.
	for _, removed := range []string{"Refresh fields from GitHub", "A public GitHub email fills this"} {
		if bytes.Contains(detail, []byte(removed)) {
			t.Fatalf("user detail still carries %q: %s", removed, detail)
		}
	}
	for _, hidden := range []string{"Photo source", ">Fetched<", "Gravatar ID", "GitHub profile"} {
		if bytes.Contains(detail, []byte(hidden)) {
			t.Fatalf("user detail exposes provider bookkeeping %q: %s", hidden, detail)
		}
	}
	_, _ = request("owner@h", "POST", "/user", url.Values{
		"action": {"save"}, "name": {"alice@h"}, "github_user": {"alice"},
		"company": {"Local Company"}, "location": {"Wellesley"}, "twitter": {"local_handle"},
	}, 303)
	detail, _ = request("owner@h", "GET", "/user?name=alice@h", nil, 200)
	for _, want := range []string{"Local Company", "Wellesley", "local_handle"} {
		if !bytes.Contains(detail, []byte(want)) {
			t.Fatalf("editable AgentBus profile field was not saved: %q", want)
		}
	}
	_, _ = request("owner@h", "POST", "/user", url.Values{
		"action": {"save"}, "name": {"alice@h"}, "github_user": {"alice"},
	}, 303)
	detail, _ = request("owner@h", "GET", "/user?name=alice@h", nil, 200)
	for _, cleared := range []string{"Local Company", "Wellesley", "local_handle"} {
		if bytes.Contains(detail, []byte(cleared)) {
			t.Fatalf("web profile form could not clear editable field %q", cleared)
		}
	}
	avatar, header := request("owner@h", "GET", "/avatar?name=alice@h", nil, 200)
	if !bytes.Equal(avatar, photo) || header.Get("Content-Type") != "image/png" {
		t.Fatalf("avatar did not serve normalized local bytes: %q %q", avatar, header.Get("Content-Type"))
	}
	service, _ := request("owner@h", "GET", "/service?name=reports@h", nil, 200)
	if !bytes.Contains(service, []byte("data:image/png;base64,")) || !bytes.Contains(service, []byte(`/user?name=alice%40h`)) {
		t.Fatalf("visible owner photo missing from service: %s", service)
	}
	hidden, _ := request("visitor@h", "GET", "/service?name=reports@h", nil, 200)
	if bytes.Contains(hidden, []byte("data:image/png;base64,")) || bytes.Contains(hidden, []byte(`/user?name=alice%40h`)) {
		t.Fatalf("service leaked hidden owner profile: %s", hidden)
	}
	if provider.calls.Load() != 1 {
		t.Fatalf("page renders contacted provider %d times", provider.calls.Load())
	}

	provider.profile.Company, provider.profile.Location, provider.profile.TwitterUsername = "Updated Company", "Boston", "refreshed"
	_, _ = request("owner@h", "POST", "/user", url.Values{"action": {"refresh-github"}, "name": {"alice@h"}, "return": {"/users"}}, 303)
	if provider.calls.Load() != 2 {
		t.Fatalf("explicit refresh calls = %d, want 2 total", provider.calls.Load())
	}
	detail, _ = request("owner@h", "GET", "/user?name=alice@h", nil, 200)
	if !bytes.Contains(detail, []byte("Updated Company")) || !bytes.Contains(detail, []byte("Boston")) || !bytes.Contains(detail, []byte("refreshed")) || provider.calls.Load() != 2 {
		t.Fatal("explicit refresh did not update locally or page render refetched")
	}
}

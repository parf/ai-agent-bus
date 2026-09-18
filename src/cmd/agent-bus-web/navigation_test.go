package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func getAs(t *testing.T, m *meanings, path string) (string, int) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, m.web.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(m.session)
	resp, err := m.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return strings.Join(strings.Fields(string(body)), " "), resp.StatusCode
}

func postAs(t *testing.T, m *meanings, path string, form url.Values) (string, int) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, m.web.URL+path, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(m.session)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", m.web.URL)
	resp, err := m.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	return resp.Header.Get("Location"), resp.StatusCode
}

func TestSectionNavigationCountsOnlyCallerVisibleCategories(t *testing.T) {
	m := meaningFixture(t)
	for _, who := range []string{"ordinary@h", "other@h"} {
		if _, err := m.bus.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	m.register(protocol.Record{Name: "viewer@h", Owner: "ordinary@h", Kind: "generic"})
	m.register(protocol.Record{Name: "mine@h", Owner: "viewer@h", Kind: "generic"})
	m.register(protocol.Record{Name: "shared@h", Owner: "other@h", Kind: "generic", Allow: []string{"viewer@h"}})
	m.register(protocol.Record{Name: "hidden@h", Owner: "other@h", Kind: "generic", Allow: []string{"other@h"}, NoMaster: true})
	m.register(protocol.Record{Name: "personal-mine@h", Owner: "viewer@h", Kind: "generic", Personal: true})
	m.register(protocol.Record{Name: "personal-other@h", Owner: "other@h", Kind: "generic", Personal: true, Allow: []string{"viewer@h"}})
	m.register(protocol.Record{Name: "jobs@h", Owner: "other@h", Kind: protocol.KindTopic, Mode: protocol.ModeQueue, Allow: []string{"viewer@h"}})

	ordinary := m.as("viewer@h")
	services := ordinary.get("/services?scope=my&state=inactive")
	for _, want := range []string{
		`href="/services?state=inactive" class="">All (3)</a>`,
		`href="/services?scope=my&amp;state=inactive" aria-current=true class="my-view">My (1)</a>`,
		`href="/personal?state=inactive" class="personal-view">Personal (1)</a>`,
		`href="/services/new" class="">Register service</a>`,
		`href="/services?scope=my">All</a>`,
		`href="/services?scope=my&amp;state=active">Enabled</a>`,
	} {
		if !strings.Contains(services, want) {
			t.Errorf("service navigation missing %q", want)
		}
	}
	if strings.Contains(services, "hidden@h") || strings.Contains(services, "Personal (2)") {
		t.Error("service navigation counted a hidden or another owner's Personal record")
	}
	channels := ordinary.get("/channels")
	if !strings.Contains(channels, `aria-current=true class="">All channels (1)</a>`) || !strings.Contains(channels, `href="/channels/new" class="">Register channel</a>`) {
		t.Error("channel section navigation or count is wrong")
	}
}

func TestOwnedRowsAreMarkedAndEditFollowsDaemonAuthority(t *testing.T) {
	m := meaningFixture(t)
	for _, who := range []string{"alice@h", "bob@h"} {
		if _, err := m.bus.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	m.register(protocol.Record{Name: "own@h", Owner: "alice@h", Allow: []string{"alice@h"}, Proto: "remote"})
	m.register(protocol.Record{Name: "managed@h", Owner: "bob@h", Allow: []string{"alice@h"}})
	maintainers := protocol.MaintainerList{"alice@h"}
	if _, err := m.bus.Manage("bob@h", core.Management{Name: "managed@h", Maintainers: &maintainers}); err != nil {
		t.Fatal(err)
	}
	m.register(protocol.Record{Name: "view@h", Owner: "bob@h", Allow: []string{"alice@h"}})

	page := m.as("alice@h").get("/services")
	own := m.row(page, "own@h")
	managed := m.row(page, "managed@h")
	view := m.row(page, "view@h")
	if !strings.Contains(own, `class="record-name-cell owned-record"`) || strings.Contains(own, "Yours") || !strings.Contains(own, `<td>external`) {
		t.Error("remote owned row lacks its visible ownership treatment")
	}
	if strings.Contains(managed, "owned-record") || strings.Contains(managed, "Yours") {
		t.Error("Maintainer row confused management with ownership")
	}
	if strings.Contains(view, "owned-record") || strings.Contains(view, "Yours") {
		t.Error("view-only row was marked owned")
	}
	if strings.Contains(page, ">Edit</a>") || strings.Contains(page, "<th scope=col>Controls") {
		t.Error("the service name and a duplicate Edit column both route to detail")
	}
	for _, want := range []string{
		`.section-nav .my-view{color:#1d5fa8}`,
		`.section-nav .personal-view{color:#8a5000;font-weight:700}`,
		`.record-name-cell.owned-record{border-left-color:#1d5fa8}`,
		`.record-name-cell.owned-record .record-name{color:#1d5fa8;font-weight:600}`,
		`.record-name-cell.personal-record{border-left-color:#8a5000}`,
		`.record-name-cell.personal-record .record-name,.personal-marker{color:#8a5000;font-weight:700}`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("service-row treatment missing %q", want)
		}
	}
	ownedRule := `.record-name-cell.owned-record{border-left-color:#1d5fa8}`
	personalRule := `.record-name-cell.personal-record{border-left-color:#8a5000}`
	if strings.Index(page, ownedRule) >= strings.Index(page, personalRule) {
		t.Error("Personal styling does not override the owned treatment")
	}
}

func TestRegistrationLivesOnDedicatedSectionPages(t *testing.T) {
	m := meaningFixture(t)
	for _, listing := range []string{"/services", "/channels", "/users", "/groups"} {
		body := m.get(listing)
		if strings.Contains(body, `name=action value=create`) || strings.Contains(body, "<h2>Create group</h2>") {
			t.Errorf("%s still embeds a registration form", listing)
		}
	}

	service := m.get("/services/new")
	if !strings.Contains(service, `name=kind value=generic`) || !strings.Contains(service, `name=kind value=agent`) || strings.Contains(service, `<select name=kind>`) {
		t.Error("service registration does not use plain-valued Kind radios")
	}
	channel := m.get("/channels/new")
	if !strings.Contains(channel, `name=mode value=pubsub`) || !strings.Contains(channel, `name=mode value=queue`) || strings.Contains(channel, `<select name=mode>`) {
		t.Error("channel registration does not use plain-valued Delivery radios")
	}
	if !strings.Contains(m.get("/users/new"), `name=action value=create`) || !strings.Contains(m.get("/user"), `name=action value=create`) {
		t.Error("dedicated or legacy user registration route is missing")
	}
	if !strings.Contains(m.get("/groups/new"), `Register group</button>`) {
		t.Error("dedicated group registration route is missing")
	}

	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "ordinary@h"}, true); err != nil {
		t.Fatal(err)
	}
	ordinary := m.as("ordinary@h")
	users := ordinary.get("/users")
	groups := ordinary.get("/groups")
	if strings.Contains(users, "/users/new") || strings.Contains(groups, "/groups/new") {
		t.Error("ordinary user was offered an administrative registration link")
	}
	for _, path := range []string{"/users/new", "/groups/new"} {
		if _, status := getAs(t, ordinary, path); status != http.StatusForbidden {
			t.Errorf("%s returned %d to an ordinary user, want 403", path, status)
		}
	}

	location, status := postAs(t, m, "/user", url.Values{
		"action": {"create"}, "name": {"new@h"}, "person_name": {"New Person"}, "return": {"/users"},
	})
	if status != http.StatusSeeOther || location != "/user?name=new%40h" {
		t.Fatalf("user creation returned %d %q, want new detail", status, location)
	}
}

func TestSmallURLFiltersAreLinksAndRetainSearch(t *testing.T) {
	m := meaningFixture(t)
	page := m.get("/users?q=alice&kind=users")
	for _, want := range []string{
		`href="/users?q=alice">All`,
		`href="/users?kind=users&amp;q=alice" aria-current=true>👤 Users`,
		`href="/users?kind=other&amp;q=alice">Other`,
		`type=hidden name=kind value="users"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("directory state link lost %q", want)
		}
	}
	if strings.Contains(page, `<select name=kind>`) {
		t.Error("three-value identity filter remained a select")
	}
}

func TestServiceSectionCountsReuseTheSingleListAnswer(t *testing.T) {
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens(), "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	face := api.New(b, tokens, "owner@h").Handler()
	var lists atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/ls" {
			lists.Add(1)
		}
		face.ServeHTTP(w, r)
	}))
	defer backend.Close()
	web := httptest.NewServer(dashboard(&caller{client: backend.Client(), base: backend.URL}, false))
	defer web.Close()
	client := web.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	token, err := tokens.Issue("owner@h")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.PostForm(web.URL+"/signin", url.Values{"token": {token}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(resp.Cookies()) != 1 {
		t.Fatal("sign in did not return a session")
	}
	lists.Store(0)
	req, _ := http.NewRequest(http.MethodGet, web.URL+"/services", nil)
	req.AddCookie(resp.Cookies()[0])
	page, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	page.Body.Close()
	if page.StatusCode != http.StatusOK || lists.Load() != 1 {
		t.Fatalf("service page returned %d with %d /ls calls, want 200 and one", page.StatusCode, lists.Load())
	}
}

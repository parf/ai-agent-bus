package main

import (
	"fmt"
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
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func TestDirectoryShowsJunkWithoutCallingItUsers(t *testing.T) {
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens(), "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	face := api.New(b, tokens, "owner@h").Handler()
	var reads atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			reads.Add(1)
		}
		face.ServeHTTP(w, r)
	}))
	defer backend.Close()
	web := httptest.NewServer(dashboard(&caller{client: backend.Client(), base: backend.URL}, false))
	defer web.Close()
	client := web.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if _, err := b.SetUser("owner@h", protocol.User{Name: "smoke/person@h"}, true); err != nil {
		t.Fatal(err)
	}
	// An administrator is a User first (docs/01-identity-and-roles.md#daemon-administrators).
	if _, err := b.SetUser("owner@h", protocol.User{Name: "maintainer@h"}, true); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("owner@h", core.AdministratorsGroup, []string{"owner@h", "maintainer@h"}); err != nil {
		t.Fatal(err)
	}
	known(t, b, "self@h")
	// #holds@h is a principal without being a person: an Agent, owned by
	// smoke/person@h, holding a credential. Agents are not people, so the
	// directory gives it no row of either kind (docs/01-identity-and-roles.md#names).
	// held@h is a service of the same User and names nobody on its allow
	// list, so no other caller sees it: the hidden-record check needs that.
	b.Restore(ports.Snapshot{Clean: true, Records: []protocol.Record{
		{Name: "#holds@h", Owner: "smoke/person@h", Kind: protocol.KindAgent, Allow: []string{"*"}, Full: protocol.OverflowStrict},
		{Name: "held@h", Owner: "smoke/person@h", Kind: protocol.KindService, Full: protocol.OverflowStrict},
	}})
	if r, ok := b.Lookup("owner@h", "#holds@h"); !ok || r.Owner != "smoke/person@h" {
		t.Fatal("the fixture agent was not restored")
	}
	for i := 0; i < 30; i++ {
		if _, err := tokens.Issue(fmt.Sprintf("unused-%02d@h", i)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tokens.Issue("#holds@h"); err != nil {
		t.Fatal(err)
	}
	cookies := map[string]*http.Cookie{}
	for _, who := range []string{"owner@h", "maintainer@h", "smoke/person@h"} {
		token, err := tokens.Issue(who)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.PostForm(web.URL+"/signin", url.Values{"token": {token}})
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 303 || len(resp.Cookies()) != 1 {
			t.Fatal("fixture sign-in failed")
		}
		cookies[who] = resp.Cookies()[0]
	}
	request := func(who, path, origin string, form url.Values, want int) (string, string) {
		t.Helper()
		method := "GET"
		var body io.Reader
		if form != nil {
			method = "POST"
			body = strings.NewReader(form.Encode())
		}
		r, err := http.NewRequest(method, web.URL+path, body)
		if err != nil {
			t.Fatal(err)
		}
		r.AddCookie(cookies[who])
		r.Header.Set("Origin", origin)
		if form != nil {
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		resp, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != want {
			t.Fatalf("%s %s: HTTP %d want %d", method, path, resp.StatusCode, want)
		}
		return string(data), resp.Header.Get("Location")
	}
	reads.Store(0)
	before := len(tokens.Names())
	page, _ := request("owner@h", "/users", "", nil, 200)
	// The record listing is two reads: /ls answers active records only, and
	// /inactive the rest (docs/constitution.md#common-record-fields).
	if reads.Load() != 5 {
		t.Fatalf("directory fetched %d bus answers, want identity + status + users + the active and inactive record listings", reads.Load())
	}
	if strings.Contains(page, "/avatar?") {
		t.Error("directory reintroduced one avatar request per row")
	}
	if len(tokens.Names()) != before {
		t.Error("GET removed credentials")
	}
	_, tables, ok := strings.Cut(page, "<tbody>")
	if !ok {
		t.Fatal("missing user table")
	}
	people, _, _ := strings.Cut(tables, "</tbody>")
	var adminRow string
	for _, candidate := range strings.Split(people, "<tr>") {
		if strings.Contains(candidate, "maintainer@h") {
			adminRow = candidate
			break
		}
	}
	if !strings.Contains(adminRow, "Daemon administrator") || strings.Contains(people, "Daemon maintainer") {
		t.Fatal("directory does not distinguish daemon Administrator from service Maintainer")
	}
	detail, _ := request("owner@h", "/user?name=maintainer@h", "", nil, 200)
	if !strings.Contains(detail, "Daemon administrator") || strings.Contains(detail, "Daemon maintainer") {
		t.Fatal("identity detail retained the old administrative role label")
	}
	// self@h is a User now, so it is listed with the people; an Agent is not
	// a person and is listed nowhere.
	if !strings.Contains(people, "smoke/person@h") || !strings.Contains(people, "self@h") || strings.Contains(people, "unused-") {
		t.Error("users table hides a real user or labels other identities as users")
	}
	directory := section(t, page, "<main>", "</main>")
	if !strings.Contains(directory, "Other identities — review and cleanup") || !strings.Contains(directory, "unused-00@h") {
		t.Error("non-user identities were hidden")
	}
	if strings.Contains(directory, "holds@h") {
		t.Error("the directory gives an Agent a row")
	}
	if !strings.Contains(page, "Next page") || !strings.Contains(page, "</main>") {
		t.Error("directory is unbounded or its main landmark is unclosed")
	}
	private, _ := request("smoke/person@h", "/users", "", nil, 200)
	private = section(t, private, "<main>", "</main>") // Public header names the daemon owner; directory visibility is unchanged.
	if strings.Contains(private, "unused-00@h") || strings.Contains(private, "#holds@h") || strings.Contains(private, "owner@h") {
		t.Error("ordinary user can enumerate other identities")
	}
	// An Agent has no directory entry to open, so its name finds nothing.
	request("owner@h", "/user?name="+url.QueryEscape("#holds@h"), "", nil, 404)
	// The User who owns the agent is where its records are investigated.
	owner, _ := request("owner@h", "/user?name=smoke/person@h", "", nil, 200)
	owner = section(t, owner, "<main>", "</main>")
	if !strings.Contains(owner, "#holds@h") {
		t.Error("the owning user's detail gives no records to investigate")
	}
	if strings.Contains(owner, "value=remove-credential") {
		t.Error("a user who owns records is offered credential cleanup")
	}
	hiddenRecords, _ := request("maintainer@h", "/user?name=smoke/person@h", "", nil, 200)
	if strings.Contains(section(t, hiddenRecords, "<main>", "</main>"), "held@h") {
		t.Error("directory-visible user detail exposed a record hidden from this caller")
	}
	filtered := "/users?kind=other&page=2&q=unused"
	page, _ = request("owner@h", filtered, "", nil, 200)
	if !strings.Contains(page, "unused-29@h") || strings.Contains(page, "unused-00@h") || strings.Contains(page, "self@h") {
		t.Error("directory search/filter/paging failed")
	}
	page, _ = request("maintainer@h", "/user?name=unused-29@h&return="+url.QueryEscape(filtered), "", nil, 200)
	if !strings.Contains(page, "Review credential removal…") || strings.Contains(page, `name=action value=remove-credential`) {
		t.Fatal("maintainer cannot review cleanup")
	}
	confirmation, _ := request("maintainer@h", "/credential-remove?name=unused-29@h&return="+url.QueryEscape(filtered), "", nil, 200)
	if !strings.Contains(confirmation, "Confirm credential removal") || !strings.Contains(confirmation, `name=action value=remove-credential`) || !strings.Contains(confirmation, "Every browser session") {
		t.Fatal("credential cleanup bypasses or loses its consequence confirmation")
	}
	form := url.Values{"action": {"remove-credential"}, "name": {"unused-29@h"}, "return": {filtered}}
	request("maintainer@h", "/user", "https://foreign.invalid", form, 403)
	_, location := request("maintainer@h", "/user", web.URL, form, 303)
	if location != filtered {
		t.Errorf("cleanup lost filters: %s", location)
	}
	for _, name := range tokens.Names() {
		if name == "unused-29@h" {
			t.Error("cleanup only changed the page, not the daemon")
		}
	}
	form.Set("name", "unused-28@h")
	form.Set("return", "https://foreign.invalid/")
	_, location = request("owner@h", "/user", web.URL, form, 303)
	if location != "/users" {
		t.Error("cleanup permits a foreign return URL")
	}
}

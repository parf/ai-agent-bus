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
	if err := b.SetGroup("owner@h", core.MaintainersGroup, []string{"owner@h", "maintainer@h"}); err != nil {
		t.Fatal(err)
	}
	known(t, b, "self@h")
	// holds@h is a principal without being a person: a self-owned record and
	// no profile. The daemon supports it — it authenticates, it may be handed
	// a record by transfer, and it may register records of its own — so it
	// survives the orphan sweep and the directory has to show it as what it
	// is. Restored rather than registered because that is where such a name
	// comes from; the owner it needs would have to exist first.
	//
	// It owns a service, which is why credential cleanup must not be offered
	// for it: taking the credential of a name that owns records is how orphans
	// get manufactured, and the sweep no longer guards against that
	// (docs/01-identity.md#when-the-owner-is-gone) because nothing reachable
	// creates it. The classification is what keeps it safe.
	b.Restore(ports.Snapshot{Clean: true, Records: []protocol.Record{
		{Name: "holds@h", Owner: "holds@h", Kind: "generic", Full: protocol.OverflowStrict},
		{Name: "service@h", Owner: "holds@h", Kind: "generic", Full: protocol.OverflowStrict},
	}})
	for i := 0; i < 30; i++ {
		if _, err := tokens.Issue(fmt.Sprintf("unused-%02d@h", i)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tokens.Issue("holds@h"); err != nil {
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
	if reads.Load() != 3 {
		t.Fatalf("directory fetched %d bus answers, want identity + status + users once", reads.Load())
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
	if !strings.Contains(people, "smoke/person@h") || strings.Contains(people, "unused-") || strings.Contains(people, "self@h") {
		t.Error("users table hides a real user or labels other identities as users")
	}
	if !strings.Contains(page, "Other identities — review and cleanup") || !strings.Contains(page, "unused-00@h") || !strings.Contains(page, "self@h") {
		t.Error("non-user identities were hidden")
	}
	if !strings.Contains(page, "Next page") || !strings.Contains(page, "</main>") {
		t.Error("directory is unbounded or its main landmark is unclosed")
	}
	private, _ := request("smoke/person@h", "/users", "", nil, 200)
	private = section(t, private, "<main>", "</main>") // Public header names the daemon owner; directory visibility is unchanged.
	if strings.Contains(private, "unused-00@h") || strings.Contains(private, "holds@h") || strings.Contains(private, "owner@h") {
		t.Error("ordinary user can enumerate other identities")
	}
	for _, name := range []string{"self@h", "holds@h"} {
		page, _ := request("owner@h", "/user?name="+name, "", nil, 200)
		if strings.Contains(page, "State: active") || strings.Contains(page, "Save profile") || strings.Contains(page, "value=remove-credential") {
			t.Errorf("non-user %s has fabricated user controls or unsafe cleanup", name)
		}
		if name == "holds@h" && !strings.Contains(page, "service@h") {
			t.Error("retained identity gives no service to investigate")
		}
	}
	filtered := "/users?kind=other&page=2&q=unused"
	page, _ = request("owner@h", filtered, "", nil, 200)
	if !strings.Contains(page, "unused-29@h") || strings.Contains(page, "unused-00@h") || strings.Contains(page, "self@h") {
		t.Error("directory search/filter/paging failed")
	}
	page, _ = request("maintainer@h", "/user?name=unused-29@h&return="+url.QueryEscape(filtered), "", nil, 200)
	if !strings.Contains(page, "Remove credential for unused-29@h") {
		t.Fatal("maintainer cannot review cleanup")
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

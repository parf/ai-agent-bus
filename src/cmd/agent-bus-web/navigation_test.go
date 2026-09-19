package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func TestReaderFiltersKeepMeasuredZeroSeparateFromUnavailable(t *testing.T) {
	zero, two := 0, 2
	for _, tc := range []struct {
		name   string
		value  *int
		filter string
		want   bool
	}{
		{"present", &two, "present", true},
		{"zero-is-not-present", &zero, "present", false},
		{"measured-zero", &zero, "none", true},
		{"missing-is-not-zero", nil, "none", false},
		{"missing", nil, "unavailable", true},
		{"measured-is-available", &zero, "unavailable", false},
		{"all-includes-missing", nil, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchesReaderFilter(tc.value, tc.filter); got != tc.want {
				t.Fatalf("matchesReaderFilter(%v, %q)=%v, want %v", tc.value, tc.filter, got, tc.want)
			}
		})
	}
}

func TestServiceReaderFilterIsIndependentFromDelivery(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	m.attachReader("reading@h")

	present := m.get("/agents?readers=present")
	if !strings.Contains(present, ">reading@h<") || strings.Contains(present, ">quiet@h<") || !strings.Contains(present, `aria-current=true>Reading now</a>`) {
		t.Fatal("positive Readers filter does not isolate the outstanding request")
	}
	none := m.get("/agents?readers=none")
	if strings.Contains(none, ">reading@h<") || !strings.Contains(none, ">quiet@h<") {
		t.Fatal("measured-zero Readers filter collapsed with a positive count")
	}
	disabled := m.get("/agents?readers=none&state=inactive")
	if !strings.Contains(disabled, ">off@h<") || strings.Contains(disabled, ">quiet@h<") || !strings.Contains(disabled, `/agents?readers=none&amp;state=active`) {
		t.Fatal("reader and delivery filters do not compose or retain each other")
	}
}

func TestRecordListsPageAfterFilteringAndRetainURLState(t *testing.T) {
	// A service has no queue here, so its listing carries no reader or delivery
	// filter to retain; the pages that do keep theirs across paging.
	// See docs/03-records.md#five-record-kinds.
	queueFilters := []string{"readers=none", "state=active"}
	for _, tc := range []struct {
		path     string
		kind     string
		stem     string
		personal bool
		filters  []string
	}{
		{"/services", protocol.KindService, "svc", false, nil},
		{"/personal", protocol.KindAgent, "personal", true, queueFilters},
		{"/channels", protocol.KindQueue, "channel", false, queueFilters},
	} {
		t.Run(tc.stem, func(t *testing.T) {
			m := meaningFixture(t)
			for i := 0; i < 30; i++ {
				m.register(protocol.Record{Name: fmt.Sprintf("%s-%02d@h", tc.stem, i), Owner: "admin@h", Kind: tc.kind, Addr: "a", Proto: "p", Personal: tc.personal})
			}
			state := append([]string{"q=" + tc.stem}, tc.filters...)
			state = append(state, "sort=updated")
			sort.Strings(state)
			path := tc.path + "?" + strings.Join(state, "&") + "&page=2"
			before := m.lsCalls.Load()
			page := m.get(path)
			if got := m.lsCalls.Load() - before; got != 1 {
				t.Fatalf("%s listing made %d /ls calls, want one", tc.path, got)
			}
			if !strings.Contains(page, "Showing 26&ndash;30 of 30 matching records") || !strings.Contains(page, "Previous page") || strings.Contains(page, "Next page") || !strings.Contains(page, "Clear filters") {
				t.Fatalf("%s did not render the bounded second page: %s", tc.path, page)
			}
			if !strings.Contains(page, `.record-table caption{display:block;width:100%}`) {
				t.Fatalf("%s table caption collapses at the narrow breakpoint", tc.path)
			}
			if strings.Count(page, `class="record-name-cell`) != 5 {
				t.Fatalf("%s paged before filtering or used the wrong page size", tc.path)
			}
			previous := `href="` + tc.path + `?page=1&amp;` + strings.Join(state, "&amp;") + `">Previous page</a>`
			if !strings.Contains(page, previous) {
				t.Errorf("%s Previous link lost listing state: want %s", tc.path, previous)
			}
			for _, want := range append(append([]string{}, state...), "page=1") {
				if !strings.Contains(page, want) {
					t.Errorf("%s pager lost %q", tc.path, want)
				}
			}
			// The filters a service has no answer for are not offered on it.
			for _, absent := range queueFilters {
				if tc.filters == nil && strings.Contains(page, absent) {
					t.Errorf("%s offers %q, which is a question about a queue it does not have", tc.path, absent)
				}
			}
			category := "All (30)</a>"
			if tc.path == "/personal" {
				category = "Personal (30)</a>"
			}
			if !strings.Contains(page, category) {
				t.Errorf("%s category count was replaced by a page count", tc.path)
			}
			if !strings.Contains(page, `return=`) || !strings.Contains(page, `page%3d2`) {
				t.Errorf("%s detail link lost its exact listing page: %s", tc.path, page)
			}
			clamped := m.get(tc.path + "?q=" + tc.stem + "&page=999")
			if !strings.Contains(clamped, "Showing 26&ndash;30 of 30 matching records") {
				t.Errorf("%s did not bound an out-of-range page", tc.path)
			}
		})
	}
}

func TestServiceDetailReturnIsLocalAndStateful(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Kind: protocol.KindAgent, Name: "return@h", Owner: "admin@h"})
	stateful := m.get("/agent?name=return@h&return=%2Fagents%3Freaders%3Dnone%26page%3D2")
	if !strings.Contains(stateful, `href="/agents?readers=none&amp;page=2"`) {
		t.Fatal("service detail did not retain its local listing URL")
	}
	foreign := m.get("/agent?name=return@h&return=https%3A%2F%2Fevil.example%2Fservices")
	if !strings.Contains(foreign, `href="/agents"`) || strings.Contains(foreign, "evil.example") {
		t.Fatal("service detail accepted a foreign return URL")
	}
}

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

func TestCompactUserEditorExplainsSSHOnboarding(t *testing.T) {
	m := meaningFixture(t)
	page := m.get("/users/new")
	for _, want := range []string{
		`class="editor-card task-card"`, `class=form-grid`, `class="form-field form-field-wide"`,
		`name=person_name`, `type=email name=email`, `name=github_user`,
		`🔑</span> SSH access`, `Public keys are added on the host after the profile is saved.`,
		`agent-bus-admin user add &lt;user@realm&gt; &lt;key.pub&gt;`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("user editor lacks %q", want)
		}
	}
	if strings.Contains(page, `name=ssh`) || strings.Contains(page, `<textarea`) {
		t.Error("profile editor invented a web-writable SSH key field")
	}
}

func TestCompactSelectorsSubmitOnChange(t *testing.T) {
	m := meaningFixture(t)
	agents := m.get("/agents")
	if !strings.Contains(agents, `<span aria-hidden=true>🔛</span> Delivery`) || !strings.Contains(agents, `name=sort data-submit-on-change`) || strings.Contains(agents, `<button>Filter</button>`) {
		t.Fatal("agent filters lack the delivery glyph or retain a visible Filter button")
	}
	// Sorting is a question about any listing; the delivery filter is a
	// question about a queue, and a service has none.
	services := m.get("/services")
	if !strings.Contains(services, `name=sort data-submit-on-change`) || strings.Contains(services, `<button>Filter</button>`) {
		t.Fatal("service sorting does not apply on change, or a visible Filter button remains")
	}
	if strings.Contains(services, `</span> Delivery`) || strings.Contains(services, `aria-label="Reader filter"`) || strings.Contains(services, `aria-label="Queue filter"`) {
		t.Fatal("the services page offers a filter about a queue it does not have")
	}
	activity := m.get("/activity")
	if !strings.Contains(activity, `name=name data-submit-on-change`) || strings.Contains(activity, `<button>Filter</button>`) {
		t.Fatal("activity selector does not apply on change")
	}
	resp, err := m.client.Get(m.web.URL + "/ui.js")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	script, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(script), `control.form.requestSubmit()`) || !strings.Contains(resp.Header.Get("Content-Type"), "text/javascript") {
		t.Fatalf("local selector behavior unavailable: status=%d type=%q body=%q", resp.StatusCode, resp.Header.Get("Content-Type"), script)
	}
}

func TestDensePagesUseCardsAndImmediateHelpWithoutLosingActions(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "alice@h", PersonName: "Alice", GithubUser: "parf"}, true); err != nil {
		t.Fatal(err)
	}
	for path, wants := range map[string][]string{
		"/services/new":                 {`class="editor-card task-card"`, `class=form-grid`, `popovertarget=form-access-help`, `data-tooltip="One identity per line.`},
		"/user?name=alice@h":            {`class=person-layout`, `class=person-sidebar`, `popovertarget=user-access-help`, `>Save profile</button>`, `name=company`, `name=location`, `name=twitter`},
		"/groups":                       {`<table class="record-table group-table">`, `href="/group?name=%40administrators"`, `<th scope=col>Members</th>`},
		"/group?name=%40administrators": {`class="editor-card group-card"`, `textarea name=members rows=8`, `>Save members</button>`},
		"/":                             {`class=dashboard-section`, `popovertarget=overview-help`, `class=node-strip`},
		"/diagnostics":                  {`class=dashboard-section`, `popovertarget=refusals-help`, `popovertarget=exchanges-help`},
		"/account":                      {` Account</h1>`, `popovertarget=account-help`, `>Credentials</h2>`, `agent-bus-token admin@h --rotate`},
		"/service?name=quiet@h":         {`class=service-dashboard`, `Queue &amp; counters`, `popovertarget=policy-help`, `id=settings class=editor-link`},
		"/service/edit?name=quiet@h":    {`id=form-save`, `>Save settings</button>`, `popovertarget=form-access-help`},
	} {
		body := m.get(path)
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Errorf("%s lacks %q", path, want)
			}
		}
	}
}

func TestSectionNavigationCountsOnlyCallerVisibleCategories(t *testing.T) {
	m := meaningFixture(t)
	for _, who := range []string{"ordinary@h", "other@h"} {
		if _, err := m.bus.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	m.register(protocol.Record{Name: "viewer@h", Owner: "ordinary@h", Kind: protocol.KindAgent})
	m.register(protocol.Record{Name: "mine@h", Owner: "viewer@h", Kind: protocol.KindAgent})
	m.register(protocol.Record{Name: "shared@h", Owner: "other@h", Kind: protocol.KindAgent, Allow: []string{"viewer@h"}})
	m.register(protocol.Record{Name: "hidden@h", Owner: "other@h", Kind: protocol.KindAgent, Allow: []string{"other@h"}})
	m.register(protocol.Record{Name: "personal-mine@h", Owner: "viewer@h", Kind: protocol.KindAgent, Personal: true})
	m.register(protocol.Record{Name: "personal-other@h", Owner: "other@h", Kind: protocol.KindAgent, Personal: true, Allow: []string{"viewer@h"}})
	m.register(protocol.Record{Name: "jobs@h", Owner: "other@h", Kind: protocol.KindQueue, Allow: []string{"viewer@h"}})
	// A channel counts with the channels, so the agent totals beside it must
	// not move when one is registered.
	m.register(protocol.Record{Name: "db@h", Owner: "other@h", Kind: protocol.KindService, Addr: "h:1", Proto: "https", Allow: []string{"viewer@h"}})

	ordinary := m.as("viewer@h")
	agents := ordinary.get("/agents?scope=my&state=inactive")
	for _, want := range []string{
		`href="/agents?state=inactive" class="">All (3)</a>`,
		`href="/agents?scope=my&amp;state=inactive" aria-current=true class="my-view">My (1)</a>`,
		`href="/personal?state=inactive" class="personal-view">Personal (1)</a>`,
		`href="/agents/new" class="">Register agent</a>`,
		`href="/agents?scope=my">All</a>`,
		`href="/agents?scope=my&amp;state=active">Enabled</a>`,
	} {
		if !strings.Contains(agents, want) {
			t.Errorf("agent navigation missing %q", want)
		}
	}
	if strings.Contains(agents, "hidden@h") || strings.Contains(agents, "Personal (2)") {
		t.Error("agent navigation counted a hidden or another owner's Personal record")
	}
	for _, elsewhere := range []string{">jobs@h<", ">db@h<"} {
		if strings.Contains(agents, elsewhere) {
			t.Errorf("%s was listed among the agents", elsewhere)
		}
	}
	channels := ordinary.get("/channels")
	if !strings.Contains(channels, `aria-current=true class="">All (1)</a>`) || !strings.Contains(channels, `href="/channels/new" class="">Register channel</a>`) {
		t.Error("channel section navigation or count is wrong")
	}
	if !strings.Contains(channels, ">jobs@h<") || strings.Contains(channels, ">mine@h<") {
		t.Error("the channels page lost its channel or gained an agent")
	}
	if services := ordinary.get("/services"); !strings.Contains(services, ">db@h<") || strings.Contains(services, ">mine@h<") {
		t.Error("the services page lost its external service or gained an agent")
	}
}

func TestOwnedRowsAreMarkedAndEditFollowsDaemonAuthority(t *testing.T) {
	m := meaningFixture(t)
	for _, who := range []string{"alice@h", "bob@h"} {
		if _, err := m.bus.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	m.register(protocol.Record{Kind: protocol.KindAgent, Name: "own@h", Owner: "alice@h", Allow: []string{"alice@h"}, Proto: "remote"})
	m.register(protocol.Record{Kind: protocol.KindAgent, Name: "managed@h", Owner: "bob@h", Allow: []string{"alice@h"}})
	maintainers := protocol.MaintainerList{"alice@h"}
	if _, err := m.bus.Manage("bob@h", core.Management{Name: "managed@h", Maintainers: &maintainers}); err != nil {
		t.Fatal(err)
	}
	m.register(protocol.Record{Kind: protocol.KindAgent, Name: "view@h", Owner: "bob@h", Allow: []string{"alice@h"}})

	page := m.as("alice@h").get("/agents")
	own := m.row(page, "own@h")
	managed := m.row(page, "managed@h")
	view := m.row(page, "view@h")
	if !strings.Contains(own, `class="record-name-cell owned-record"`) || strings.Contains(own, "Yours") {
		t.Error("owned row lacks its visible ownership treatment")
	}
	// own@h carries a protocol and is still an agent, which is the only way
	// this can fail: an agent is on this bus whatever a stray endpoint says,
	// and only a service is reached off it.
	if !strings.Contains(own, `<td data-label=Reached><span class=muted>&mdash;</span>`) {
		t.Errorf("an agent with a protocol is reported as reached externally: %s", own)
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
		`.section-nav .my-view{color:var(--accent);font-weight:600}`,
		`.section-nav .personal-view{color:var(--orange);font-weight:600}`,
		`.record-name-cell.owned-record{border-left-color:var(--accent)}`,
		`.record-name-cell.owned-record .record-name{color:var(--accent);font-weight:600}`,
		`.record-name-cell.personal-record{border-left-color:var(--orange)}`,
		`.record-name-cell.personal-record .record-name,.personal-marker{color:var(--orange);font-weight:600}`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("service-row treatment missing %q", want)
		}
	}
	ownedRule := `.record-name-cell.owned-record{border-left-color:var(--accent)}`
	personalRule := `.record-name-cell.personal-record{border-left-color:var(--orange)}`
	if strings.Index(page, ownedRule) >= strings.Index(page, personalRule) {
		t.Error("Personal styling does not override the owned treatment")
	}
}

func TestServiceListSearchKindSortAndCompactNames(t *testing.T) {
	m := meaningFixture(t)
	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "alice@h"}, true); err != nil {
		t.Fatal(err)
	}
	m.register(protocol.Record{Name: "alpha@h", Owner: "alice@h", Kind: protocol.KindAgent, Descr: "Billing API", Allow: []string{"admin@h"}})
	time.Sleep(time.Millisecond)
	m.register(protocol.Record{Name: "bot@h", Owner: "admin@h", Kind: protocol.KindAgent, Allow: []string{"admin@h"}})
	time.Sleep(time.Millisecond)
	m.register(protocol.Record{Name: "zeta@h", Owner: "admin@h", Kind: protocol.KindAgent, Descr: "Archive", Allow: []string{"admin@h"}})
	if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "alpha@h", Body: "one"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "alpha@h", Body: "two"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "bot@h", Body: "one"}); err != nil {
		t.Fatal(err)
	}

	byDescription := m.get("/agents?q=bILLing")
	if !strings.Contains(byDescription, ">Billing API<") || !strings.Contains(byDescription, ">alpha@h<") || strings.Contains(byDescription, ">bot@h<") {
		t.Fatal("case-insensitive description search did not isolate its record")
	}
	if row := m.row(byDescription, "alpha@h"); strings.Count(row, ">alpha@h<") != 1 {
		t.Fatalf("described service name is visibly duplicated: %s", row)
	}
	byOwner := m.get("/agents?q=ALICE%40H")
	if !strings.Contains(byOwner, ">alpha@h<") || strings.Contains(byOwner, ">bot@h<") {
		t.Fatal("case-insensitive owner search did not isolate its record")
	}

	if row := m.row(m.get("/agents"), "bot@h"); strings.Count(row, ">bot@h<") != 1 {
		t.Fatalf("an agent without a description has a duplicated visible name: %s", row)
	}
	// Agents and services are each one kind, so neither offers a control for
	// narrowing to one.
	if plain := m.get("/agents"); strings.Contains(plain, `aria-label="Kind filter"`) {
		t.Fatal("Agents still offers a Kind filter for the single kind it lists")
	}
	if forced := m.get("/agents?kind=queue"); !strings.Contains(forced, ">alpha@h<") {
		t.Fatal("a kind query the Agents page no longer offers still filtered it")
	}

	updated := m.get("/agents?sort=updated")
	if strings.Index(updated, ">zeta@h<") > strings.Index(updated, ">bot@h<") || strings.Index(updated, ">bot@h<") > strings.Index(updated, ">alpha@h<") {
		t.Fatal("recently-updated sorting is not newest first")
	}
	queued := m.get("/agents?sort=queued")
	if strings.Index(queued, ">alpha@h<") > strings.Index(queued, ">bot@h<") || strings.Index(queued, ">bot@h<") > strings.Index(queued, ">zeta@h<") {
		t.Fatal("queued sorting is not high to low with name ties")
	}

	state := m.get("/agents?q=bot&scope=my&sort=queued&state=active")
	for _, want := range []string{
		`name=q value="bot"`,
		`name=scope value="my"`,
		`name=state value="active"`,
		`value=queued selected`,
		`href="/agents?q=bot&amp;sort=queued&amp;state=active" class="">All`,
		`href="/agents?q=bot&amp;scope=my&amp;sort=queued&amp;state=active" aria-current=true class="my-view">My`,
	} {
		if !strings.Contains(state, want) {
			t.Errorf("listing state lost %q", want)
		}
	}
	// A service has no queue, so sorting by one is not offered and an asked-for
	// queued sort falls back to the name order rather than ranking by nothing.
	services := m.get("/services?sort=queued")
	if strings.Contains(services, "value=queued") {
		t.Error("the services page offers a sort by a queue it does not have")
	}
	if !strings.Contains(services, `<option value="" selected>Name`) {
		t.Errorf("a queued sort asked of Services did not fall back to the name order: %s", services)
	}
}

func TestRegistrationLivesOnDedicatedSectionPages(t *testing.T) {
	m := meaningFixture(t)
	for _, listing := range []string{"/agents", "/services", "/channels", "/users", "/groups"} {
		body := m.get(listing)
		if strings.Contains(body, `name=action value=create`) || strings.Contains(body, "<h2>Create group</h2>") {
			t.Errorf("%s still embeds a registration form", listing)
		}
	}

	// Each registration page offers only what its listing shows, so a form
	// cannot register a record that page would not then display.
	service := m.get("/services/new")
	if !strings.Contains(service, `<input type=hidden name=kind value=service>`) || strings.Contains(service, `name=kind value=agent`) {
		t.Error("service registration still offers a kind the Services page does not list")
	}
	// A service is external, so the two fields that say where it is and how to
	// reach it are the ones the daemon will not register it without.
	if !strings.Contains(service, `<input name=addr required`) || !strings.Contains(service, `<input name=protocol required`) {
		t.Error("service registration does not require the address and protocol a service needs")
	}
	agent := m.get("/agents/new")
	if !strings.Contains(agent, `<input type=hidden name=kind value=agent>`) || strings.Contains(agent, `name=addr`) {
		t.Error("agent registration offers the wrong kind or an address it has no use for")
	}
	// The Channels section has two kinds and they do not ask for the same
	// things, so each has its own page. The section page without a kind links
	// to both rather than registering anything itself.
	chooser := m.get("/channels/new")
	if strings.Contains(chooser, "name=action value=create") {
		t.Errorf("the channel chooser registers something itself: %s", chooser)
	}
	for _, kind := range []string{protocol.KindQueue, protocol.KindPubSub} {
		if !strings.Contains(chooser, `href="/channels/new?kind=`+kind+`"`) {
			t.Errorf("the channel chooser does not offer %s: %s", kind, chooser)
		}
		page := m.get("/channels/new?kind=" + kind)
		if !strings.Contains(page, `<input type=hidden name=kind value=`+kind+`>`) || strings.Contains(page, `<select name=kind>`) {
			t.Errorf("%s registration does not carry its own plain kind: %s", kind, page)
		}
	}
	// A queue declares the policy of the queue it will hold; a pub/sub topic
	// holds nothing, so asking it the same questions would be asking about a
	// queue that will never exist. Both halves, because a page that dropped
	// the fields everywhere would satisfy the second on its own.
	queue := m.get("/channels/new?kind=" + protocol.KindQueue)
	topic := m.get("/channels/new?kind=" + protocol.KindPubSub)
	for _, field := range []string{"name=ttl", "name=bound", "name=overflow"} {
		if !strings.Contains(queue, field) {
			t.Errorf("queue registration cannot declare %s: %s", field, queue)
		}
		if strings.Contains(topic, field) {
			t.Errorf("pub/sub registration asks for %s, which it has no queue for: %s", field, topic)
		}
	}
	channel := queue
	// Delivery is no longer a second answer beside the kind: the kind is the
	// delivery, so a form that still asked twice could disagree with itself.
	if strings.Contains(channel, `name=mode`) {
		t.Error("channel registration still asks for a delivery mode beside the kind")
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
	location, status = postAs(t, m, "/groups", url.Values{
		"action": {"save"}, "new": {"1"}, "name": {"@new-group"}, "members": {"admin@h"},
	})
	if status != http.StatusSeeOther || location != "/group?name=%40new-group" {
		t.Fatalf("group creation returned %d %q, want new detail", status, location)
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

// The two marked authorities are marked where a person reads them, not only
// where the label is built: the directory's Authority cell, the person page,
// the signed-in account, and the record detail that names its maintainers.
func TestTheMarkedAuthoritiesAreMarkedOnEveryPageThatStatesThem(t *testing.T) {
	m := meaningFixture(t)
	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "alice@h"}, true); err != nil {
		t.Fatal(err)
	}
	m.register(protocol.Record{Kind: protocol.KindAgent, Name: "kept@h", Owner: "admin@h", Allow: []string{"alice@h"}})
	maintainers := protocol.MaintainerList{"alice@h"}
	if _, err := m.bus.Manage("admin@h", core.Management{Name: "kept@h", Maintainers: &maintainers}); err != nil {
		t.Fatal(err)
	}
	owner := m.as("admin@h")
	if row := m.row(owner.get("/users"), "admin@h"); !strings.Contains(row, "🔱 Daemon owner") {
		t.Errorf("the directory Authority cell does not mark the daemon owner: %s", row)
	}
	if row := m.row(owner.get("/users"), "alice@h"); strings.Contains(row, "🔱") {
		t.Error("an ordinary user carries the daemon owner's mark")
	}
	for _, page := range []string{"/user?name=admin%40h", "/account"} {
		if got := owner.get(page); !strings.Contains(got, "🔱 Daemon owner") {
			t.Errorf("%s does not mark the daemon owner", page)
		}
	}
	if got := owner.get("/user?name=alice%40h"); strings.Contains(got, "🔱") {
		t.Error("an ordinary user's page carries the daemon owner's mark")
	}
	if got := owner.get("/service?name=kept%40h"); !strings.Contains(got, "👮 Maintainers: alice@h") {
		t.Error("the record detail does not mark its maintainers")
	}
}

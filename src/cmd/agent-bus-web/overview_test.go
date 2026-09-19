package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func TestOverviewIsShortAndDiagnosticsKeepsTheEvidence(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "quiet@h", Body: "ordinary backlog"}); err != nil {
		t.Fatal(err)
	}

	overview := m.get("/")
	for _, want := range []string{
		`class=node-strip`,
		`🏠</span> Overview</h1>`,
		`Node-wide. The lists linked below contain only records visible to you; the two never have to agree.`,
		`href="/services?sort=queued&amp;work=held"`, `href="/channels?sort=queued&amp;work=held"`,
	} {
		if !strings.Contains(overview, want) {
			t.Errorf("Overview lacks %q", want)
		}
	}
	// This fixture raises nothing, and an Overview with nothing to report
	// carries no attention section at all rather than a block saying so.
	// TestNeedsAttentionAppearsOnlyWhenSomethingWasObserved holds both halves.
	for _, gone := range []string{
		`id=attention>Needs attention`,
		`No observed attention conditions in this view`,
		`It is not a statement that everything is working`,
	} {
		if strings.Contains(overview, gone) {
			t.Errorf("a quiet Overview still carries %q", gone)
		}
	}
	// The Find row offers only what the menu cannot: a filtered view of each
	// listing that holds work, agents first. Users and Diagnostics are menu
	// entries and were repeated here.
	find := section(t, overview, `<nav class=overview-links aria-label="Find records">`, "</nav>")
	for _, kept := range []string{"Agents holding work", "Services holding work", "Channels holding work"} {
		if !strings.Contains(find, kept) {
			t.Errorf("the Find row lost %q: %s", kept, find)
		}
	}
	for _, duplicated := range []string{">Users</a>", ">Diagnostics</a>"} {
		if strings.Contains(find, duplicated) {
			t.Errorf("the Find row repeats a menu entry, %q: %s", duplicated, find)
		}
	}
	for _, detail := range []string{"Refusals since this daemon started", "Exchanges in retained history", "Registry", "quiet@h"} {
		if strings.Contains(overview, detail) {
			t.Errorf("Overview still carries detailed Diagnostics content %q", detail)
		}
	}

	diagnostics := m.get("/diagnostics")
	for _, want := range []string{"Refusals since this daemon started", "Inboxes holding messages", "Exchanges in retained history", "Loss by name", `href="/agent?name=quiet%40h"`} {
		if !strings.Contains(diagnostics, want) {
			t.Errorf("Diagnostics lacks %q", want)
		}
	}
	if strings.Contains(diagnostics, "id=registry") || strings.Contains(diagnostics, ">Registry</h2>") {
		t.Error("Diagnostics still duplicates the registry catalogue")
	}
}

// The admitted set, its levels and its precedence are the accepted page spec
// (Plans/MVP/web/pages.md#overview, Plans/MVP/web/glyphs.md#attention-levels).
// Level is the visual meaning, so it drifts silently unless it is asserted.
func TestAttentionItemsAreEnumeratedAndOnePerRecord(t *testing.T) {
	items := attentionItems(core.Status{Unclean: true, Refused: map[string]int{"acl": 2}}, []protocol.Record{
		{Kind: protocol.KindAgent, Name: "ordinary@h", Queued: 8},
		{Kind: protocol.KindAgent, Name: "full@h", Queued: 4, Oldest: "2m", AtBound: true, Disabled: true, Dropped: 3, Expired: 1},
		{Kind: protocol.KindAgent, Name: "off@h", Queued: 2, Disabled: true},
		{Kind: protocol.KindAgent, Name: "lost@h", Dropped: 1},
		{Name: "news@h", Kind: protocol.KindQueue, Queued: 1, AtBound: true, Full: protocol.OverflowRing},
	})
	if len(items) != 6 { // previous stop, one refusal and four exceptional records
		t.Fatalf("attention count = %d, want 6: %#v", len(items), items)
	}
	seen := map[string]attention{}
	kinds := map[string]attention{}
	for _, item := range items {
		kinds[item.Kind] = item
		if item.Name != "" {
			if _, duplicate := seen[item.Name]; duplicate {
				t.Fatalf("record %s yielded more than one attention item", item.Name)
			}
			seen[item.Name] = item
		}
	}
	if _, ok := seen["ordinary@h"]; ok {
		t.Error("an ordinary backlog was labelled as an attention condition")
	}
	// An unclean stop is a fact about this node and is stated red; a lifetime
	// refusal total cannot say anything is wrong now and is informational.
	if kinds["unclean"].Level != "red" {
		t.Errorf("the previous-stop item is %q, want red", kinds["unclean"].Level)
	}
	if kinds["refusal"].Level != "blue" {
		t.Errorf("the cumulative refusal item is %q, want blue (informational)", kinds["refusal"].Level)
	}
	// Losses are cumulative across restarts and may predate this run, so they
	// are notable rather than urgent.
	if lost := seen["lost@h"]; lost.Level != "orange" || lost.Title != "Messages were lost from this inbox" {
		t.Errorf("cumulative loss is %q/%q, want orange and the loss title", lost.Level, lost.Title)
	}
	// Disabled outranks at-bound: nothing is being accepted, so capacity is
	// not what is wrong with the record. The capacity stays as a fact.
	full := seen["full@h"]
	if full.Level != "orange" || full.Title != "Delivery is off and work is held" {
		t.Errorf("a disabled record at its bound is %q/%q, want orange and the disabled title", full.Level, full.Title)
	}
	if !full.AtBound || full.Dropped != 3 || full.Expired != 1 || full.Overflow == "" {
		t.Fatalf("the winning condition hid supporting facts: %#v", full)
	}
	// At capacity names the configured policy as a setting, and says nothing
	// about what the next send will do.
	news := seen["news@h"]
	if news.Level != "red" || news.Title != "Queue at capacity when observed" {
		t.Errorf("an at-capacity record is %q/%q, want red and the capacity title", news.Level, news.Title)
	}
	if news.Overflow != "when full: drop the oldest" {
		t.Errorf("at capacity did not name the configured overflow policy: %q", news.Overflow)
	}
	if seen["off@h"].Overflow != "" {
		t.Errorf("a record that is not at capacity states an overflow policy: %q", seen["off@h"].Overflow)
	}
	if seen["news@h"].Href != "/channel?name=news%40h" {
		t.Fatalf("channel attention link = %q", seen["news@h"].Href)
	}
	// One ordered enum: red, then orange, then informational.
	for i := 1; i < len(items); i++ {
		rank := map[string]int{"red": 0, "orange": 1, "blue": 2}
		if rank[items[i-1].Level] > rank[items[i].Level] {
			t.Fatalf("attention items are out of level order at %d: %q before %q", i, items[i-1].Level, items[i].Level)
		}
	}
}

func TestHoldingWorkLinksAreRealFilters(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "quiet@h", Body: "held"}); err != nil {
		t.Fatal(err)
	}
	page := m.get("/agents?sort=queued&work=held")
	if !strings.Contains(page, `aria-label="Queue filter"`) || !strings.Contains(page, `aria-current=true>Holding work</a>`) || !strings.Contains(page, "quiet@h") {
		t.Fatal("holding-work URL does not render its state and matching record")
	}
	for _, empty := range []string{"reading@h", "off@h", "elsewhere@h"} {
		if strings.Contains(page, ">"+empty+"<") {
			t.Errorf("empty record %s survived the holding-work filter", empty)
		}
	}
}

// Every attention item offers a link, and a link into a section that does not
// exist is a promise the page cannot keep: the reader follows it and lands at
// the top of an unrelated page with nothing about the condition they clicked.
// The unclean-stop card pointed at /diagnostics#node, an anchor Diagnostics
// never had and deliberately removed with the node section.
func TestEveryAttentionLinkPointsAtASectionThatExists(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	m.register(protocol.Record{Name: "news@h", Owner: "admin@h", Kind: protocol.KindPubSub, Allow: []string{"*"}})
	items := attentionItems(core.Status{Unclean: true, Refused: map[string]int{"acl": 1}}, []protocol.Record{
		{Kind: protocol.KindAgent, Name: "quiet@h", Queued: 1, AtBound: true},
		{Name: "news@h", Kind: protocol.KindQueue, Dropped: 1},
	})
	if len(items) != 4 {
		t.Fatalf("fixture produced %d attention items, want 4", len(items))
	}
	for _, item := range items {
		path, fragment, _ := strings.Cut(item.Href, "#")
		if path == "" {
			path = "/"
		}
		// get fails the test on any status but 200, so this also proves the
		// destination page is one this caller can actually open.
		page := m.get(path)
		if fragment == "" { // a record link is the whole page, and needs none
			continue
		}
		if !strings.Contains(page, "id="+fragment+">") && !strings.Contains(page, `id="`+fragment+`"`) {
			t.Errorf("%s attention links to %s, but %s has no id=%s", item.Kind, item.Href, path, fragment)
		}
	}
}

// The whole-page recovery stopped printing err.Error() (truthful_test.go), but
// the envelope section fails on its own and printed the same text: a transport
// failure between /ls and /recent names the socket or TCP address this child
// talks to, straight onto a page anybody signed in can read.
func TestAFailedEnvelopeSectionDoesNotExposeTheBackendAddress(t *testing.T) {
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens(), "admin@h")
	if err != nil {
		t.Fatal(err)
	}
	handler := api.New(b, tokens, "admin@h").Handler()
	// Everything but the feed answers, so the page renders and only its
	// envelope section has to explain itself.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/recent" {
			conn, _, hijackErr := w.(http.Hijacker).Hijack()
			if hijackErr != nil {
				t.Error(hijackErr)
				return
			}
			conn.Close()
			return
		}
		handler.ServeHTTP(w, r)
	}))
	defer backend.Close()
	web := httptest.NewServer(dashboard(&caller{client: backend.Client(), base: backend.URL}, false))
	defer web.Close()
	client := web.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	token, err := tokens.Issue("admin@h")
	if err != nil {
		t.Fatal(err)
	}
	in, err := client.PostForm(web.URL+"/signin", url.Values{"token": {token}})
	if err != nil {
		t.Fatal(err)
	}
	in.Body.Close()
	req, err := http.NewRequest("GET", web.URL+"/diagnostics", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(in.Cookies()[0])
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	page := string(body)
	if resp.StatusCode != 200 || !strings.Contains(page, "Envelope history unavailable") {
		t.Fatalf("a failed feed did not leave the rest of Diagnostics standing: %d", resp.StatusCode)
	}
	address := strings.TrimPrefix(backend.URL, "http://")
	if strings.Contains(page, address) || strings.Contains(page, backend.URL) {
		t.Errorf("the envelope section exposes the backend address %s", address)
	}
	if strings.Contains(page, `{"error"`) {
		t.Error("the envelope section prints the daemon's raw JSON body")
	}
}

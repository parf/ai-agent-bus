package main

import (
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
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// The node strip is where the owner reads the node's numbers, so the call
// counters belong in it rather than in the footer, and the record count has to
// say what it counted. "Records" did not: the value is len(b.records), which is
// every registered record whatever its kind.
func TestTheNodeStripCarriesTheCallCountersAndNamesWhatItCounts(t *testing.T) {
	m := meaningFixture(t)
	for _, record := range []protocol.Record{
		{Name: "svc@h", Owner: "admin@h", Kind: protocol.KindAgent},
		{Name: "bot@h", Owner: "admin@h", Kind: "agent"},
		{Name: "news@h", Owner: "admin@h", Kind: protocol.KindQueue},
	} {
		m.register(record)
	}
	overview := m.get("/")
	strip := section(t, overview, "<div class=node-strip>", "</div></div>")

	// One label naming every page it sums, because a reader comparing it with
	// any one of them must know what else is in the number.
	if !strings.Contains(strip, "<span>Agents + Services + Channels + Users</span>") {
		t.Errorf("the record count does not name the kinds it sums: %s", strip)
	}
	// The figures are read down a column, so they are ranged right in tabular
	// numerals: the owner asked for it after reading them centred.
	if !strings.Contains(overview, ".node-fact strong{") {
		t.Fatal("the node-fact figure rule is gone, so the next check cannot fail")
	}
	figure := section(t, overview, ".node-fact strong{", "}")
	for _, want := range []string{"text-align:right", "font-variant-numeric:tabular-nums"} {
		if !strings.Contains(figure, want) {
			t.Errorf("the strip figures are not ranged right by place value: %q lacks %q", figure, want)
		}
	}
	if strings.Contains(strip, "<span>Records</span>") {
		t.Error("the strip still calls the sum Records without saying what a record is here")
	}
	// Three kinds registered, and the count is all of them.
	if !strings.Contains(strip, "<span>Agents + Services + Channels + Users</span><strong>3</strong>") {
		t.Errorf("the count is not the whole registry: %s", strip)
	}

	// The counters live in the strip now; the states they can be in are
	// exercised separately below, against a bound counter.
	if !strings.Contains(strip, "Calls") {
		t.Errorf("the node strip carries no call counters at all: %s", strip)
	}

	// Moved, not copied. The shared footer keeps the two node facts it
	// publishes to anybody and no longer carries the counters beside them.
	footer := section(t, overview, "<div class=footer-node>", "</div>")
	if !strings.Contains(footer, "<strong>Owner</strong>") || !strings.Contains(footer, "<strong>Uptime</strong>") {
		t.Errorf("the footer lost the node facts it still publishes: %s", footer)
	}
	if strings.Contains(footer, "Calls") {
		t.Errorf("the counters are in the footer and the strip at once: %s", footer)
	}

	// The help beside the strip must not outlive it. It counted the facts
	// ("These four values") while the strip now carries seven, and listed
	// the filtered pages without Agents while the label beside the count
	// names them.
	help := section(t, overview, "<div popover id=node-help class=context-help>", "</div>")
	facts := strings.Count(strip, "<div class=node-fact>")
	if facts < 5 {
		t.Fatalf("the fixture strip carries %d facts, so a stale cardinality would not be wrong here: %s", facts, strip)
	}
	// Asserted as the sentence rather than as a list of wrong numbers: a
	// blocklist of cardinalities lets the next one through, which is the
	// brittleness being removed.
	if !strings.Contains(help, "These values cover the whole daemon.") {
		t.Errorf("the node help counts the strip's %d facts instead of describing them: %s", facts, help)
	}
	// The help names the pages a reader can actually open, so every page it
	// names has to be in the menu. Written from navItems rather than listed
	// again here, so adding or removing a section cannot leave this behind.
	named := 0
	for _, item := range navItems {
		if strings.Contains(help, item.Label+",") || strings.Contains(help, item.Label+" and") || strings.Contains(help, "and "+item.Label) {
			named++
		}
	}
	if named < 4 {
		t.Errorf("the node help names %d menu sections for a count that spans every record: %s", named, help)
	}
	if !strings.Contains(help, "This number is every record on the node") {
		t.Errorf("the node help does not say what its record count covers: %s", help)
	}
	if strings.Contains(help, "Inboxes page") || strings.Contains(help, "Agents page") {
		t.Errorf("the node help sends a reader to a page that does not exist: %s", help)
	}
	if !strings.Contains(help, "Calls") {
		t.Errorf("the node help does not explain the call counters it now sits beside: %s", help)
	}
}

// The generation time is one fact about one page load. It was printed beside
// Refresh, again in the empty state and again on every attention item; the
// owner cut the repeats, then cut Refresh and moved the time to the shared
// footer, where one line covers the whole page.
func TestTheGenerationTimeIsStatedOnceAndOnlyInTheFooter(t *testing.T) {
	m := meaningFixture(t)
	empty := m.get("/")
	footer := section(t, empty, "<footer ", "</footer>")
	at := section(t, footer, "<strong>Generated</strong> ", "</span>")
	if strings.TrimSpace(at) == "" {
		t.Fatal("no page says when it was generated")
	}
	if n := strings.Count(empty, at); n != 1 {
		t.Errorf("a quiet Overview states the generation time %d times, want 1", n)
	}
	body := section(t, empty, "<main>", "</main>")
	if strings.Contains(body, at) || strings.Contains(body, "as of") {
		t.Errorf("the page body still carries its own time: %s", body)
	}
	// Refresh went with it: the owner reads the time in the footer and
	// reloads with the browser.
	if strings.Contains(body, ">Refresh</a>") {
		t.Errorf("Overview still offers a Refresh link: %s", body)
	}

	// Populated: two records that each raise an item, so a per-item repeat
	// would show up as three or more.
	m.register(protocol.Record{Kind: protocol.KindAgent, Name: "tiny@h", Owner: "admin@h", Bound: 1})
	if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "tiny@h", Body: "fills it"}); err != nil {
		t.Fatal(err)
	}
	m.register(protocol.Record{Kind: protocol.KindAgent, Name: "small@h", Owner: "admin@h", Bound: 1})
	if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "small@h", Body: "fills it"}); err != nil {
		t.Fatal(err)
	}
	full := m.get("/")
	if !strings.Contains(full, "Queue at capacity when observed") {
		t.Fatalf("the fixture raised no attention item, so this check proves nothing: %s", full)
	}
	if !strings.Contains(full, ">tiny@h<") || !strings.Contains(full, ">small@h<") {
		t.Fatal("the fixture did not raise both items")
	}
	now := section(t, section(t, full, "<footer ", "</footer>"), "<strong>Generated</strong> ", "</span>")
	if n := strings.Count(full, now); n != 1 {
		t.Errorf("a populated Overview states the generation time %d times, want 1", n)
	}
	// The item keeps its way through; only the repeated time went.
	if !strings.Contains(full, `<p class=muted><a href="/agent?name=tiny%40h">View record</a></p>`) {
		t.Errorf("an attention item lost its link with the repeated time: %s", full)
	}
	// Every page carries the footer, so every page is dated, not just this one.
	for _, route := range []string{"/agents", "/services", "/channels", "/users", "/groups", "/diagnostics", "/activity"} {
		page := m.get(route)
		stamp := section(t, section(t, page, "<footer ", "</footer>"), "<strong>Generated</strong> ", "</span>")
		if strings.TrimSpace(stamp) == "" {
			t.Errorf("%s does not say when it was generated", route)
		}
	}
}

// An Overview with nothing to report says nothing rather than saying so. The
// block existed to make no health claim; the owner's answer is that a heading
// with an explanation under it is itself a claim on the reader's attention.
func TestNeedsAttentionAppearsOnlyWhenSomethingWasObserved(t *testing.T) {
	m := meaningFixture(t)
	quiet := m.get("/")
	// Element forms, not class names: every class in this list also appears
	// in the inline stylesheet that every page carries.
	for _, gone := range []string{">Needs attention<", "<div class=attention-list>", "attention-empty\">", "No observed attention conditions"} {
		if strings.Contains(quiet, gone) {
			t.Errorf("a quiet Overview still carries %q: %s", gone, quiet)
		}
	}
	// The rest of the page is untouched, so the check above cannot pass on
	// an Overview that failed to render at all.
	if !strings.Contains(quiet, "<div class=node-strip>") {
		t.Fatalf("the quiet Overview did not render its node strip: %s", quiet)
	}

	m.register(protocol.Record{Kind: protocol.KindAgent, Name: "tiny@h", Owner: "admin@h", Bound: 1})
	if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "tiny@h", Body: "fills it"}); err != nil {
		t.Fatal(err)
	}
	raised := m.get("/")
	for _, want := range []string{"id=attention>Needs attention", "<div class=attention-list>", "Queue at capacity when observed"} {
		if !strings.Contains(raised, want) {
			t.Errorf("an Overview with an observed condition lacks %q: %s", want, raised)
		}
	}
	if strings.Contains(raised, "attention-empty\">") {
		t.Errorf("a populated Overview rendered the empty state too: %s", raised)
	}
}

// Every menu entry carries its section's own mark, and it is the same mark the
// section's page title uses rather than a second symbol for one thing.
func TestEveryMenuEntryCarriesItsSectionMark(t *testing.T) {
	m := meaningFixture(t)
	page := m.get("/")
	menu := section(t, page, `<nav aria-label="sections">`, "</nav>")
	for _, item := range navItems {
		mark := strings.Join(strings.Fields(string(titleMark(item.Key))), " ")
		if !strings.Contains(menu, mark+item.Label+"</a>") {
			t.Errorf("the %s menu entry does not carry its section mark: %s", item.Label, menu)
		}
		// Decorative, and asserted per entry: the link text beside it
		// already names the section, and one entry can lose this on its
		// own while the rest of the row still carries the attribute.
		if !strings.Contains(mark, "aria-hidden") || strings.Contains(mark, "aria-hidden=false") {
			t.Errorf("the %s menu mark is read out beside the word it repeats: %s", item.Label, mark)
		}
	}
	// Overview's mark is not the bus logo. The header carries that already,
	// and a title repeating it named the product, not the page.
	logo := section(t, page, "<svg class=node-logo", "</svg>")
	if strings.Contains(menu, "node-logo") {
		t.Error("the menu repeats the node logo")
	}
	if mark := string(titleMark("overview")); strings.Contains(mark, "<svg") || !strings.Contains(mark, "🏠") {
		t.Errorf("the Overview mark is still a drawn bus mark: %q", mark)
	}
	if strings.Count(page, "<svg class=node-logo") != 1 || logo == "" {
		t.Error("the header lost the one node logo it should carry")
	}
}

// Calls moved out of the footer and into the strip, and kept the three states
// the footer distinguished: a measured count, a window with no sample yet, and
// a counter this process never bound. An unbound counter is unavailable, never
// a fabricated zero (internal/api/identity.go:13).
func TestTheStripReportsCallsInTheThreeStatesTheDaemonCanBeIn(t *testing.T) {
	for _, probe := range []struct {
		name  string
		stats *protocol.CallStats
		want  []string
		gone  []string
	}{
		{
			name: "measured",
			stats: &protocol.CallStats{Total: 4210, Windows: []protocol.CallWindow{
				{Window: "1m", Available: true, Count: 17},
				{Window: "1h", Available: true, Count: 908},
			}},
			want: []string{
				"<span>Calls, minute</span><strong>17</strong>",
				"<span>Calls, hour</span><strong>908</strong>",
				"<span>Calls, total</span><strong>4,210</strong>",
			},
			gone: []string{"collecting history", "unavailable"},
		},
		{
			name: "one window has no sample yet",
			stats: &protocol.CallStats{Total: 9, Windows: []protocol.CallWindow{
				{Window: "1m", Available: false},
				{Window: "1h", Available: true, Count: 9},
			}},
			want: []string{
				"<span>Calls, minute</span><strong class=node-fact-note>collecting history</strong>",
				"<span>Calls, hour</span><strong>9</strong>",
				"<span>Calls, total</span><strong>9</strong>",
			},
			// A window with no sample must not read as a measured zero.
			gone: []string{"<span>Calls, minute</span><strong>0</strong>"},
		},
		{
			name:  "counter never bound",
			stats: nil,
			want:  []string{"<span>Calls</span><strong class=node-fact-note>unavailable</strong>"},
			gone:  []string{"<span>Calls, total</span>", "<strong>0</strong></div><div class=node-fact><span>Calls"},
		},
	} {
		t.Run(probe.name, func(t *testing.T) {
			m := callsFixture(t, probe.stats)
			strip := section(t, m.get("/"), "<div class=node-strip>", "</div></div>")
			for _, want := range probe.want {
				if !strings.Contains(strip, want) {
					t.Errorf("the strip has no %s: %s", want, strip)
				}
			}
			for _, gone := range probe.gone {
				if strings.Contains(strip, gone) {
					t.Errorf("the strip still says %s: %s", gone, strip)
				}
			}
		})
	}
}

// callsFixture is meaningFixture with the daemon's request counter bound to a
// fixed snapshot, which the ordinary fixture leaves unbound.
func callsFixture(t *testing.T, stats *protocol.CallStats) *meanings {
	t.Helper()
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens(), "admin@h")
	if err != nil {
		t.Fatal(err)
	}
	face := api.New(b, tokens, "admin@h")
	if stats != nil {
		face.Calls(func(time.Time) protocol.CallStats { return *stats })
	}
	backend := httptest.NewServer(face.Handler())
	t.Cleanup(backend.Close)
	web := httptest.NewServer(dashboard(&caller{client: backend.Client(), base: backend.URL}, false))
	t.Cleanup(web.Close)
	client := web.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	token, err := tokens.Issue("admin@h")
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
	return &meanings{t: t, bus: b, tokens: tokens, backend: backend, web: web,
		session: resp.Cookies()[0], client: client, lsCalls: new(atomic.Int64)}
}

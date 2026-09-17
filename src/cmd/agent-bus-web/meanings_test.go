package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// F.13.1. Declared state, observed state and health are three different
// things, and the pages had merged all three into one word
// (Plans/MVP/web/data-dictionary.md#the-rule). These fixtures put every
// distinguishable shape on one page at once, because a label is only wrong
// next to the thing it should have said instead.

type meanings struct {
	t       *testing.T
	bus     *core.Bus
	tokens  *auth.Tokens
	backend *httptest.Server
	web     *httptest.Server
	session *http.Cookie
	client  *http.Client
}

func meaningFixture(t *testing.T) *meanings {
	t.Helper()
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens(), "admin@h")
	if err != nil {
		t.Fatal(err)
	}
	backend := httptest.NewServer(api.New(b, tokens, "admin@h").Handler())
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
	return &meanings{t, b, tokens, backend, web, resp.Cookies()[0], client}
}

// as returns the same fixture signed in as somebody else. Every meaning check
// that ran only as the daemon owner was blind to a label that is right for
// them and wrong for an ordinary caller.
func (m *meanings) as(who string) *meanings {
	m.t.Helper()
	token, err := m.tokens.Issue(who)
	if err != nil {
		m.t.Fatal(err)
	}
	resp, err := m.client.PostForm(m.web.URL+"/signin", url.Values{"token": {token}})
	if err != nil {
		m.t.Fatal(err)
	}
	resp.Body.Close()
	if len(resp.Cookies()) != 1 {
		m.t.Fatalf("%s could not sign in", who)
	}
	next := *m
	next.session = resp.Cookies()[0]
	return &next
}

func (m *meanings) get(path string) string {
	m.t.Helper()
	req, err := http.NewRequest("GET", m.web.URL+path, nil)
	if err != nil {
		m.t.Fatal(err)
	}
	req.AddCookie(m.session)
	resp, err := m.client.Do(req)
	if err != nil {
		m.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		m.t.Fatalf("%s: %d", path, resp.StatusCode)
	}
	// Whitespace collapsed, because every sentence here is prose in a template
	// and a line wrap falling between two words of a phrase is not a change in
	// what the page says. Tests that broke on reflow were testing the margin.
	return strings.Join(strings.Fields(string(body)), " ")
}

func (m *meanings) register(r protocol.Record) {
	m.t.Helper()
	if _, err := m.bus.Register(r); err != nil {
		m.t.Fatal(err)
	}
}

// The four record shapes a reader has to be able to tell apart, on one page.
// Every one of them is "Active/Serving" or "Active/Offline" under the old
// vocabulary, which is why no single-fixture test caught it.
func (m *meanings) shapes() {
	m.t.Helper()
	m.register(protocol.Record{Name: "admin@h", Owner: "admin@h"})
	m.register(protocol.Record{Name: "reading@h", Owner: "admin@h", Descr: "a reader is on it"})
	m.register(protocol.Record{Name: "quiet@h", Owner: "admin@h", Allow: []string{"*"}, Descr: "enabled, nobody reading"})
	// Through Manage, because Register clears the bit (bus.go:226): delivery
	// is turned off by its owner, not declared at registration.
	m.register(protocol.Record{Name: "off@h", Owner: "admin@h", Descr: "the owner turned it off"})
	off := true
	if _, err := m.bus.Manage("admin@h", core.Management{Name: "off@h", Disabled: &off}); err != nil {
		m.t.Fatal(err)
	}
	m.register(protocol.Record{Name: "elsewhere@h", Owner: "admin@h", Descr: "reached another way", Proto: "https"})
}

// A read that is actually blocked, so the count is an observation rather than
// a fixture field. Returns once the daemon reports the wait.
func (m *meanings) attachReader(name string, topic ...string) <-chan protocol.Envelope {
	m.t.Helper()
	// An optional topic filter, for the one shape an unfiltered read cannot hold
	// still for: a read attached to an inbox that also has a backlog. Asked
	// for everything it drains the queue and stops waiting, so the read
	// that leaves a backlog standing is one waiting for something else.
	want := ""
	if len(topic) != 0 {
		want = topic[0]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	m.t.Cleanup(cancel)
	// The result is kept, not discarded: a waiter leaving the list is not the
	// same event as a message reaching it, and a check on the count alone
	// passes against a daemon that drops the message on the floor.
	got := make(chan protocol.Envelope, 1)
	go func() {
		e, err := m.bus.ConsumeAs(ctx, name, name, want, "", want != "", false)
		if err == nil {
			got <- e
		}
	}()
	deadline := time.Now().Add(3 * time.Second)
	for m.bus.Status().Waiting == 0 {
		if time.Now().After(deadline) {
			m.t.Fatal("the reader never blocked, so nothing is being observed")
		}
		time.Sleep(time.Millisecond)
	}
	return got
}

// Disabled says delivery is off and does not say why. "Inactive" reads as
// broken, and the record may be perfectly healthy and deliberately paused.
func TestDeliveryIsEnabledOrDisabledAndNeverInactive(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	// Read out of each record's own row. "Disabled" is also an option in the
	// listing's own delivery filter, so a page-wide match is satisfied by a
	// listing that labels every record Enabled.
	listing := m.get("/services")
	for _, want := range []struct{ name, cell string }{
		{"off@h", "<td>Disabled"}, {"quiet@h", "<td>Enabled"},
	} {
		if !strings.Contains(m.row(listing, want.name), want.cell) {
			t.Errorf("%s has no %q cell: %s", want.name, want.cell, m.row(listing, want.name))
		}
	}
	for _, page := range []string{"/services", "/service?name=off@h"} {
		body := m.get(page)
		if !strings.Contains(body, "Disabled") {
			t.Errorf("%s does not say delivery is disabled", page)
		}
		if strings.Contains(body, "Inactive") {
			t.Errorf("%s calls a disabled record Inactive, which reads as broken", page)
		}
	}
	// The positive control: an enabled record says so on the same page, so
	// "Disabled" is a distinction and not a constant.
	if !strings.Contains(m.get("/service?name=quiet@h"), "Enabled") {
		t.Error("an enabled record is not labelled Enabled")
	}
	// And the bit cannot be read as the owner's decision, because the daemon
	// merges that with the name having stopped being active.
	if !strings.Contains(m.get("/service?name=off@h"), "does not say whether the owner turned it off") {
		t.Error("the page does not say the disabled bit cannot say why")
	}
}

// Readers is an observation about outstanding reads on an inbox. It is not
// health, and a busy process between pulls is not offline.
func TestReadersAreObservedAndNeverCalledOfflineOrServing(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	m.attachReader("reading@h")
	body := m.get("/services")
	for _, banned := range []string{"Serving", "Offline", ">offline<", ">serving<"} {
		if strings.Contains(body, banned) {
			t.Errorf("the listing says %q, which is health language the daemon does not supply", banned)
		}
	}
	if !strings.Contains(m.row(body, "reading@h"), "Enabled<td>1<td>") {
		t.Errorf("the outstanding read is not counted: %s", m.row(body, "reading@h"))
	}
	if !strings.Contains(m.row(body, "quiet@h"), "Enabled<td>0<td>") {
		t.Errorf("the measured zero is not shown: %s", m.row(body, "quiet@h"))
	}
	if !strings.Contains(body, "None of it is health") {
		t.Error("the page does not say this is not health")
	}
}

func TestReaderCountDistinguishesUnavailableFromMeasuredZero(t *testing.T) {
	zero := 0
	two := 2
	if got := readerCount(nil); got != "unavailable" {
		t.Fatalf("absent reader count = %q", got)
	}
	if got := readerCount(&zero); got != "0" {
		t.Fatalf("measured zero reader count = %q", got)
	}
	if got := readerCount(&two); got != "2" {
		t.Fatalf("measured nonzero reader count = %q", got)
	}
}

// external is a caller-supplied hint about how a thing is reached. It was
// occupying the reader column, so an external service could never report
// whether anything was reading it — declared state overwriting an observation,
// which is the merge the rule forbids.
func TestExternalDoesNotStandInForTheReaderObservation(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	m.attachReader("elsewhere@h")
	listing := m.get("/services")
	if !strings.Contains(listing, "external") {
		t.Fatal("the listing does not mark a record reached another way")
	}
	// The row for the external record carries BOTH facts. Sliced to the row,
	// because the words appear elsewhere on a page listing five records.
	row := listing
	if i := strings.Index(row, "elsewhere@h"); i >= 0 {
		row = row[i:]
		if j := strings.Index(row, "</tr>"); j >= 0 {
			row = row[:j]
		}
	}
	if !strings.Contains(row, "external") {
		t.Error("the external record's own row does not say so")
	}
	if !strings.Contains(row, "Enabled<td>1<td>external") {
		t.Errorf("the external record's row does not keep the separate reader count: %s", row)
	}
	if !strings.Contains(m.get("/service?name=elsewhere@h"), "not proof of anything") {
		t.Error("the detail page treats the hint as though it established something")
	}
}

// Accepted and dequeued come back from the snapshot, so they are lifetime
// totals rather than this run's. And handing a message to a reader is not the
// work being done.
func TestQueueCountersSayTheirScopeAndNeverSayCompleted(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "quiet@h", Body: "held"}); err != nil {
		t.Fatal(err)
	}
	// The registry table names its own columns, read out of that table. Two
	// of the four words head a column of the stuck-inbox table above it, so
	// a page-wide match is satisfied by a registry with no headings at all.
	// Three accepted and one dequeued, so accepted and dequeued cannot be
	// swapped without the numbers saying so. Equal counters make the column
	// names unfalsifiable.
	for i := 0; i < 2; i++ {
		if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "quiet@h", Body: "held"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.bus.ConsumeAs(context.Background(), "quiet@h", "quiet@h", "", "", false, false); err != nil {
		t.Fatal(err)
	}
	head := m.section(m.get("/"), "registry")
	for _, col := range []string{"<th scope=col>readers", "<th scope=col>held now",
		"<th scope=col>accepted", "<th scope=col>dequeued"} {
		if !strings.Contains(head, col) {
			t.Errorf("the registry table has no %q column: %s", col, head)
		}
	}
	// held now 2, accepted 3, dequeued 1 — in that order, so the columns are
	// named for the numbers under them rather than the other way round.
	if got := m.row(head, "quiet@h"); !strings.Contains(got, "<td>2<td>3<td>1") {
		t.Errorf("the registry's counters do not line up with their columns: %s", got)
	}
	for _, page := range []string{"/", "/service?name=quiet@h"} {
		body := m.get(page)
		if !strings.Contains(body, "accepted") && !strings.Contains(body, "Accepted") {
			t.Errorf("%s does not name what In counts", page)
		}
		if !strings.Contains(body, "dequeued") && !strings.Contains(body, "Dequeued") {
			t.Errorf("%s does not name what Out counts", page)
		}
		if !strings.Contains(body, "cumulative across restarts") {
			t.Errorf("%s does not say the counters outlive a restart", page)
		}
		if strings.Contains(body, "since start") {
			t.Errorf("%s says the counters are since start, which a restart disproves", page)
		}
	}
	if !strings.Contains(m.get("/service?name=quiet@h"), "not the same as the work being done") &&
		!strings.Contains(m.get("/service?name=quiet@h"), "which is not completed") {
		t.Error("the page does not say dequeued is not completed")
	}
}

// A settings value the record does not carry is not a value to render. The
// three inherit differently and the page has to say which.
func TestUnsetQueueSettingsAreStatedAsInheritanceRatherThanGuessed(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	body := m.get("/service?name=quiet@h")
	for _, want := range []string{
		"uses the daemon default",         // Bound really does resolve one
		"not readable here",               // and the resolved number is not ours to show
		"no queue-imposed expiry",         // TTL has no default to inherit
		"a message may still specify its", // and the sender's half of it
		"When full: refuse",               // normalised at registration
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the detail page does not say %q", want)
		}
	}
	// Overflow is never unset on a registered record — Register normalises an
	// empty one (bus.go:168) — so reporting it as inherited is an invention,
	// and so is copying the daemon's resolved bound in as the record's own.
	if strings.Contains(body, "unset — refuse") {
		t.Error("the page claims an unset overflow, which a registered record cannot have")
	}
	m.register(protocol.Record{Name: "bounded@h", Owner: "admin@h", Allow: []string{"reader@h"}, Bound: 7, TTL: "1m", Full: protocol.OverflowRing})
	// Read where the page states them, not anywhere on it: the management
	// form below carries both values in its inputs, so a page-wide match is
	// satisfied by a detail page that displays neither.
	set := m.get("/service?name=bounded@h")
	for _, want := range []string{"Queue bound: 7", "Retention: 1m", "When full: drop the oldest"} {
		if !strings.Contains(set, want) {
			t.Errorf("a record that carries its own settings does not state %q", want)
		}
	}
	if strings.Contains(set, "uses the daemon default") {
		t.Error("a record with its own bound is described as inheriting one")
	}
	// And a caller who is offered no form at all still reads them, which is
	// the case the form cannot stand in for.
	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "reader@h"}, true); err != nil {
		t.Fatal(err)
	}
	visitor := m.as("reader@h").get("/service?name=bounded@h")
	for _, want := range []string{"Queue bound: 7", "Retention: 1m"} {
		if !strings.Contains(visitor, want) {
			t.Errorf("a read-only visitor is not told %q", want)
		}
	}
}

// A reason the daemon supports and has not counted is a measured zero. The
// page drew only what had happened, so "nothing refused" and "this reason is
// not observable" looked identical.
func TestEverySupportedRefusalReasonIsDrawnIncludingItsZero(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	// One real refusal, so the zeros sit beside a number rather than alone.
	if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "nobody@h", Body: "x"}); err == nil {
		t.Fatal("the fixture produced no refusal")
	}
	// Count through HTTP: a direct core call does not classify a refusal.
	// Both reply's error mapping and early request validation count in the API.
	ask, err := http.NewRequest("POST", m.backend.URL+"/send", strings.NewReader(`{"to":"nobody@h","body":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	cred, err := m.tokens.Issue("admin@h")
	if err != nil {
		t.Fatal(err)
	}
	ask.Header.Set(api.HeaderToken, cred)
	resp, err := m.backend.Client().Do(ask)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	// The control controls nothing unless it was refused, and refused for the
	// reason whose count is being read.
	if resp.StatusCode != 404 {
		t.Fatalf("the fixture's unknown name answered %d, so nothing was counted", resp.StatusCode)
	}
	// A refusal decided before the error map is counted like any other, since
	// H.5.10. The page says every endpoint refusal is in the figures, and
	// this is the path that used to answer and count nothing.
	before := m.bus.Status().Refused["unknown"]
	miss, err := http.NewRequest("GET", m.backend.URL+"/lookup?name=alsomissing@h", nil)
	if err != nil {
		t.Fatal(err)
	}
	miss.Header.Set(api.HeaderToken, cred)
	if got, err := m.backend.Client().Do(miss); err != nil {
		t.Fatal(err)
	} else {
		got.Body.Close()
		if got.StatusCode != 404 {
			t.Fatalf("the uncounted-path control answered %d", got.StatusCode)
		}
	}
	if after := m.bus.Status().Refused["unknown"]; after != before+1 {
		t.Fatalf("a lookup refusal moved the counter %d to %d, want one more", before, after)
	}
	body := m.get("/")
	reasons := api.Reasons()
	if len(reasons) < 5 {
		t.Fatalf("the daemon's reason set looks wrong: %v", reasons)
	}
	for _, reason := range reasons {
		if !strings.Contains(body, "<code>"+reason+"</code>") {
			t.Errorf("the page never mentions the reason %q, so its zero cannot be read", reason)
		}
	}
	// The numbers, not only the names. A table drawing every reason with a
	// zero beside it says nothing that a table drawing none of them did not.
	// Read against what the daemon reports rather than against a literal, so
	// the check is that the page shows the count rather than that the fixture
	// produced a particular number of refusals.
	held := fmt.Sprintf("<td>%d", m.bus.Status().Refused["unknown"])
	if m.bus.Status().Refused["unknown"] == 0 {
		t.Fatal("the fixture produced no counted refusal")
	}
	if got := m.row(body, "unknown"); !strings.HasSuffix(got, held) {
		t.Errorf("a counted reason is not shown with its count (want %s): %s", held, got)
	}
	// And a reason nothing produced is a zero rather than a blank or a dash.
	if got := m.row(body, "malformed"); !strings.Contains(got, "<td>0<") && !strings.HasSuffix(got, "<td>0") {
		t.Errorf("a reason nothing produced is not drawn as a measured zero: %s", got)
	}
	// And the page says what the figures cover, which is now every refusal an
	// endpoint decided — including the ones it once answered silently.
	if !strings.Contains(body, "whatever the caller&rsquo;s standing") {
		t.Error("the page does not say the counts do not depend on who was refused")
	}
	if !strings.Contains(body, "router rejected before any handler ran is not") {
		t.Error("the page does not say what the counts leave out")
	}
	if !strings.Contains(body, "closed") || !strings.Contains(body, "measurement") {
		t.Error("the page does not say a zero here is measured")
	}
	// Not a rate. The value is a lifetime count and carries no window.
	if strings.Contains(body, "climbing") {
		t.Error("the page describes refusals as climbing, which this value cannot say")
	}
}

// The node's totals and any list on the page count different things, and a
// page showing both without saying so has merged them.
func TestNodeTotalsSayTheyAreNodeWideAndListsSayTheyAreYours(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	body := m.get("/")
	if !strings.Contains(body, "node-wide") {
		t.Error("the node figures do not say they are node-wide")
	}
	if !strings.Contains(body, "never have to agree") {
		t.Error("the page does not say the node total and the list it shows need not agree")
	}
	if !strings.Contains(m.get("/services"), "not a count of this node") {
		t.Error("the listing does not scope itself to the caller")
	}
}

// Held is what the queue holds at the moment of the read, and at capacity is
// what was true then. Neither is a prediction, and the read does not prune.
func TestQueueObservationsSayWhenTheyWereTrue(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "quiet@h", Body: "held"}); err != nil {
		t.Fatal(err)
	}
	for _, page := range []string{"/", "/service?name=quiet@h"} {
		body := m.get(page)
		if !strings.Contains(body, "does not prune") {
			t.Errorf("%s does not say held may include messages already past their TTL", page)
		}
	}
	if !strings.Contains(m.get("/"), "what was true when observed") {
		t.Error("the page does not date the capacity observation")
	}
	// And the row itself says it, on a queue that really is at its bound —
	// as an observation, never as a claim about what the next send will do.
	m.register(protocol.Record{Name: "tiny@h", Owner: "admin@h", Bound: 1})
	if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "tiny@h", Body: "fills it"}); err != nil {
		t.Fatal(err)
	}
	// On every surface that shows it. Dropping the label from the listing and
	// the detail page alone left the diagnostics page still saying it.
	for _, where := range []struct{ page, what string }{
		{"/", "the backlog table"}, {"/services", "the listing"},
	} {
		full := m.row(m.get(where.page), "tiny@h")
		if !strings.Contains(full, "at capacity when observed") {
			t.Errorf("%s does not say a queue is at its bound: %s", where.what, full)
		}
		if strings.Contains(full, "refusing") {
			t.Errorf("%s states a prediction rather than an observation: %s", where.what, full)
		}
	}
	detail := m.get("/service?name=tiny@h")
	if !strings.Contains(detail, "at capacity when observed") {
		t.Error("the detail page does not say the queue is at its bound")
	}
	// And a queue below its bound says nothing, so the label is a distinction.
	if strings.Contains(m.row(m.get("/"), "quiet@h"), "at capacity") {
		t.Error("a queue below its bound is called at capacity")
	}
	// An empty age is not a zero age: the daemon says nothing about whether
	// the queue ever held anything. Matched as the template writes it — the
	// first version of this looked for lowercase "oldest held: 0" against a
	// page that says "Oldest held:", so it could not have fired.
	empty := m.get("/service?name=off@h")
	if strings.Contains(empty, "Oldest held: 0") {
		t.Error("an absent oldest is rendered as a measured zero")
	}
	if !strings.Contains(empty, "Oldest held: <span class=muted>&mdash;</span>") {
		t.Errorf("an absent oldest is not shown as absent: %s", empty[max(0, strings.Index(empty, "Oldest held:")):max(0, strings.Index(empty, "Oldest held:"))+60])
	}
	// The positive control, so the field is readable when there is one.
	if !strings.Contains(m.get("/service?name=quiet@h"), "Oldest held: 0s") {
		t.Error("a queue that is holding something does not say how long it has")
	}
}

// row returns one table row by the name in it, so an assertion lands on the
// record it names rather than anywhere on the page. Checking a word against
// the whole body passes on the legend, the form and the heading, none of which
// is the claim being made.
func (m *meanings) row(body, name string) string {
	m.t.Helper()
	// Row by row rather than by offset, because the name is on the page
	// before any table: the caller's own name is in the header of every page
	// they are signed in to, and that is not a row about anything.
	for _, row := range strings.Split(body, "<tr>") {
		cells, _, closed := strings.Cut(row, "</tr>")
		if closed && strings.Contains(cells, ">"+name+"<") {
			return cells
		}
	}
	m.t.Fatalf("%s has no row", name)
	return ""
}

// own gives a name a record of its own and no profile — the shape the
// directory calls a registered name. Put in through the store rather than
// registered, because no live call found makes one: registering refuses a
// self-owned record for a name that answers for nobody (mayOwn), handing one
// over refuses an owner that is not already self-owned (manage.go), and
// re-registration keeps the owner it had. Enrolment does not make one either
// — its branch writes b.users[name] too (bus.go), so an enrolled name is a
// user. What this stands in for is a snapshot carrying the shape, which the
// directory has to render whether or not anything still creates it.
func (m *meanings) own(name string) {
	m.t.Helper()
	m.bus.Restore(ports.Snapshot{Clean: true, Records: []protocol.Record{
		{Name: name, Owner: name, Kind: "agent", Full: protocol.OverflowStrict, Allow: []string{"admin@h"}, At: time.Now()},
	}})
}

// section returns one table, from the heading that names it to the end of the
// table under it. A diagnostics page stacks several tables that share column
// words, so a claim about one of them has to be read out of that one.
func (m *meanings) section(body, id string) string {
	m.t.Helper()
	at := strings.Index(body, "id="+id+">")
	if at < 0 {
		m.t.Fatalf("the page has no %s section", id)
	}
	end := strings.Index(body[at:], "</table>")
	if end < 0 {
		m.t.Fatalf("the %s section has no table", id)
	}
	return body[at : at+end]
}

// Three kinds of identity, and the directory had one word for all of them
// (Plans/MVP/done/web-review.md W06). Every one of them is on the page at
// once, because a label only lies next to the thing it should have said.
//
// Each check reads the row for its own name. The page also carries a legend
// naming all three kinds in prose, so a body-wide search for any of the three
// passes whether or not a single row is labelled at all.
func TestTheDirectoryNamesThreeKindsOfIdentityAndGuessesNone(t *testing.T) {
	m := meaningFixture(t)
	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "person@h"}, true); err != nil {
		t.Fatal(err)
	}
	// A record of its own and no profile. Self-owned, because that is what
	// makes a name an identity rather than one of its owner's services: a
	// record owned by somebody else belongs on /services and is not here.
	m.own("named@h")
	// A name with neither a profile nor a record, which has to be minted
	// rather than registered: that is precisely what nothing else creates.
	if _, err := m.tokens.Issue("junk@h"); err != nil {
		t.Fatal(err)
	}
	body := m.get("/users")
	for _, kind := range []struct{ name, says string }{
		{"person@h", "👤 User"},
		{"named@h", "🤖 Agent"},
		{"junk@h", "<td>Credential with no registered name</td>"},
	} {
		if !strings.Contains(m.row(body, kind.name), kind.says) {
			t.Errorf("%s is not named as %q: %s", kind.name, kind.says, m.row(body, kind.name))
		}
	}
	if row := m.row(body, "junk@h"); strings.Contains(row, "👤") || strings.Contains(row, "🤖") || strings.Contains(row, "⚙️") {
		t.Errorf("credential-only identity was given a guessed glyph: %s", row)
	}
	// Named from what the daemon holds, never read off the spelling. A name
	// with a slash in it is a runtime session on this bus and is a registered
	// name like any other.
	m.own("claude/one@h")
	if row := m.row(m.get("/users"), "claude/one@h"); !strings.Contains(row, "<td>Registered name</td>") || !strings.Contains(row, "🤖 Agent") {
		t.Error("a slashed name is not named the same way as any other record")
	}
	if strings.Contains(m.get("/users"), "Unclassified") {
		t.Error("the directory invents a category for a name it can classify")
	}
}

func TestEntityLabelsUseDaemonKindsAndStayOutOfEditableSyntax(t *testing.T) {
	m := meaningFixture(t)
	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "person@h"}, true); err != nil {
		t.Fatal(err)
	}
	m.register(protocol.Record{Name: "svc@h", Owner: "admin@h", Kind: "generic", Allow: []string{"peer@h"}})
	m.register(protocol.Record{Name: "bot@h", Owner: "admin@h", Kind: "agent"})
	m.register(protocol.Record{Name: "jobs@h", Owner: "admin@h", Kind: protocol.KindTopic, Mode: protocol.ModeQueue})

	users := m.get("/users?kind=users")
	if row := m.row(users, "person@h"); !strings.Contains(row, "👤 User") {
		t.Errorf("registered user has no identity label: %s", row)
	}
	if !strings.Contains(users, `value=users selected>👤 Users`) || strings.Contains(users, `value="👤`) {
		t.Errorf("directory filter mixed its displayed label into the URL value: %s", users)
	}

	services := m.get("/services")
	for name, want := range map[string]string{"svc@h": "⚙️ Service", "bot@h": "🤖 Agent"} {
		if row := m.row(services, name); !strings.Contains(row, want) {
			t.Errorf("%s has no %q label: %s", name, want, row)
		}
	}
	for _, plain := range []string{`value=generic>⚙️ Service`, `value=agent>🤖 Agent`} {
		if !strings.Contains(services, plain) {
			t.Errorf("create form does not keep a plain kind value beside %q", plain)
		}
	}
	if strings.Contains(services, `value="⚙️`) || strings.Contains(services, `value="🤖`) {
		t.Error("a display glyph entered a form value")
	}

	detail := m.get("/service?name=svc@h")
	if !strings.Contains(detail, "Type: <strong>⚙️ Service</strong>") || !strings.Contains(detail, `name=allow value="peer@h"`) {
		t.Errorf("detail lost its label or plain ACL value: %s", detail)
	}
	if strings.Contains(detail, `name=allow value="⚙️`) || strings.Contains(detail, `name=allow value="🤖`) {
		t.Error("a display glyph entered the editable ACL")
	}

	diagnostics := m.get("/")
	for name, want := range map[string]string{"svc@h": "⚙️ Service", "bot@h": "🤖 Agent", "jobs@h": "Channel"} {
		if row := m.row(diagnostics, name); !strings.Contains(row, want) {
			t.Errorf("diagnostics labels %s inconsistently: %s", name, row)
		}
	}
}

// A figure the face worked out and a figure the daemon reported are not the
// same kind of fact, and a page showing them side by side without saying which
// has merged them exactly as declared and observed must not be merged.
func TestFaceComputedCountsAreMarkedAsTheFacesOwn(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Name: "named@h", Owner: "admin@h"})
	body := m.get("/users")
	if !strings.Contains(body, "by this page") {
		t.Error("the directory counts do not say the face computed them")
	}
	if !strings.Contains(body, "not figures the daemon reported") {
		t.Error("the directory counts are not distinguished from daemon answers")
	}
	if !strings.Contains(body, "not a count of the credential store") {
		t.Error("the directory counts do not state their scope")
	}
}

// The same daemon, three callers, three different pages. A meaning check run
// only as the daemon owner cannot see a label that is right for them and wrong
// for everybody else.
func TestOneFixtureReadsDifferentlyForOrdinaryMaintainerAndOwner(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	for _, who := range []string{"maint@h", "plain@h"} {
		if _, err := m.bus.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.bus.SetGroup("admin@h", core.AdministratorsGroup, []string{"admin@h", "maint@h"}); err != nil {
		t.Fatal(err)
	}
	// Something only its owner and a maintainer may see, so "visible to you"
	// is a different set for each of the three.
	m.register(protocol.Record{Name: "private@h", Owner: "admin@h", Allow: []string{"admin@h"}, NoMaster: true})

	seen := map[string]bool{}
	for _, who := range []string{"admin@h", "maint@h", "plain@h"} {
		body := m.as(who).get("/services")
		seen[who] = strings.Contains(body, "private@h")
		// Whoever is reading, the words for the three facts are the same. A
		// label that changed with rank would be saying something about the
		// caller rather than about the record.
		if !strings.Contains(body, "Delivery") || !strings.Contains(body, "Reader") {
			t.Errorf("%s does not get the same columns", who)
		}
		if strings.Contains(body, "Serving") || strings.Contains(body, "Inactive") {
			t.Errorf("%s is shown the old vocabulary", who)
		}
		if !strings.Contains(body, "not a count of this node") {
			t.Errorf("%s is not told the listing is theirs rather than the node's", who)
		}
	}
	if !seen["admin@h"] {
		t.Error("the owner cannot see their own restricted record")
	}
	if seen["plain@h"] {
		t.Error("an ordinary caller sees a record restricted away from them")
	}
	// Manage controls follow authority, and the words do not. Read off the
	// row for one record everybody can see, because an ordinary caller has a
	// record of their own on the same page that they may rightly manage.
	// Authority over a record is the record's own: its owner, the name
	// itself, and the group the record names. Standing in the daemon's
	// maintainer group is not one of them (core/manage.go manages), so a
	// maintainer reads admin@h's record exactly as any other caller does —
	// which is the point, since the words did not change either.
	for _, who := range []struct{ caller, offered string }{
		{"admin@h", ">Manage"}, {"maint@h", ">View"}, {"plain@h", ">View"},
	} {
		row := m.row(m.as(who.caller).get("/services"), "quiet@h")
		if !strings.HasSuffix(row, who.offered) {
			t.Errorf("%s is offered the wrong control over a record owned by admin@h: %s", who.caller, row)
		}
	}
	// And the caller's own record is theirs to manage, whoever they are, so
	// the check above is about authority rather than about rank.
	if !strings.HasSuffix(m.row(m.as("plain@h").get("/services"), "plain@h"), ">Manage") {
		t.Error("an ordinary caller is offered no control over their own record")
	}
}

// Pub/sub and queue are what the record declares about delivery, and a
// service declares neither. "Delivery" elsewhere on these pages is the
// disabled bit, which is a different fact about a different thing, so both
// have to be readable at once without either standing in for the other.
func TestPubSubAndQueueDeliveryAreNamedAndNeitherIsGuessed(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Name: "fanout@h", Owner: "admin@h", Kind: "topic", Mode: "pubsub"})
	m.register(protocol.Record{Name: "onebyone@h", Owner: "admin@h", Kind: "topic", Mode: "queue"})
	m.register(protocol.Record{Name: "plain@h", Owner: "admin@h"})
	if !strings.Contains(m.get("/service?name=fanout@h"), "a copy to each subscriber") {
		t.Error("a pub/sub topic does not say every subscriber gets a copy")
	}
	if !strings.Contains(m.get("/service?name=onebyone@h"), "one at a time") {
		t.Error("a queue topic does not say one subscriber takes each message")
	}
	// A record that declares no mode is shown none. The daemon's own default
	// is not the face's to state, and a service is not a topic at all.
	for _, guess := range []string{"a copy to each subscriber", "one at a time"} {
		if strings.Contains(m.get("/service?name=plain@h"), guess) {
			t.Errorf("a record that declares no delivery mode is shown %q", guess)
		}
	}
	// Both records are enabled, so the declared mode cannot be being read off
	// the disabled bit that the listing calls Delivery.
	for _, name := range []string{"fanout@h", "onebyone@h"} {
		if !strings.Contains(m.row(m.get("/channels"), name), "<td>Enabled") {
			t.Errorf("%s is not enabled, so its mode and its delivery state are not separable here", name)
		}
	}
}

// Readers counts every outstanding consume request. A filtered waiter may
// coexist with a backlog and must still appear in the same numeric column.
func TestReadersCountsFilteredAndUnfilteredWaits(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "quiet@h", Body: "held"}); err != nil {
		t.Fatal(err)
	}
	m.attachReader("reading@h")
	// The diagnostics page lists a queue with no read on it.
	body := m.get("/")
	if !strings.Contains(m.row(body, "quiet@h"), "<td>0<td>1<td>") {
		t.Errorf("the backlog does not show readers=0 and held=1: %s", m.row(body, "quiet@h"))
	}
	if !strings.Contains(m.row(m.get("/services"), "reading@h"), "Enabled<td>1<td>") {
		t.Error("an unfiltered read is not counted")
	}
	// A read restricted to a topic is counted while a nonmatching backlog stays.
	n := meaningFixture(t)
	n.shapes()
	if _, err := n.bus.Send(protocol.Envelope{From: "admin@h", To: "reading@h", Body: "held"}); err != nil {
		t.Fatal(err)
	}
	taken := n.attachReader("reading@h", "other")
	row := n.row(n.get("/"), "reading@h")
	if !strings.Contains(row, "<td>1<td>1<td>") {
		t.Errorf("the filtered read and held message are not both reported: %s", row)
	}
	for _, phrase := range []string{"filtered and unfiltered together", "positive count promises neither"} {
		if !strings.Contains(n.get("/"), phrase) {
			t.Errorf("the page omits the count boundary %q", phrase)
		}
	}
	// And it does not say they will not take anything, which is false: a
	// matching filtered waiter is served ahead of an unfiltered one.
	for _, page := range []string{"/", "/services", "/service?name=reading@h"} {
		if strings.Contains(n.get(page), "will not take") {
			t.Errorf("%s says a filtered read will not take a message, which deliver disproves", page)
		}
	}
	// The proof, on the fixture: the counted reader receives its match. Receipt,
	// not only the count falling, proves the waiter was served.
	if _, err := n.bus.Send(protocol.Envelope{From: "admin@h", To: "reading@h", Topic: "other", Body: "only this body"}); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-taken:
		if e.Body != "only this body" {
			t.Errorf("the excluded reader received something else: %q", e.Body)
		}
	case <-time.After(3 * time.Second):
		t.Error("the excluded reader never received the message addressed to its topic")
	}
}

// The unclassified category is permanent and its occupancy is not. A page that
// only names the category when one exists tells an operator nothing about the
// empty case, which is the case they are usually looking at.
func TestTheUnclassifiedCategoryReadsTheSameWayEmptyAsPopulated(t *testing.T) {
	m := meaningFixture(t)
	empty := m.get("/users")
	if !strings.Contains(empty, "credential with no registered name") {
		t.Error("with none of them present the directory does not name the category at all")
	}
	if !strings.Contains(empty, "No other identities on this page") {
		t.Error("an empty section is dropped rather than said to be empty")
	}
	// And the same words survive one arriving, so the empty page is not a
	// different page with a different vocabulary.
	if _, err := m.tokens.Issue("junk@h"); err != nil {
		t.Fatal(err)
	}
	full := m.get("/users")
	if !strings.Contains(full, "credential with no registered name") {
		t.Error("the category is named only while empty")
	}
	if strings.Contains(full, "No other identities on this page") {
		t.Error("the section still says it is empty while holding a row")
	}
}

// The delivery setting is not an answer about the next send. `visible` merges
// the stored bit with the name having stopped being active, and that is all it
// merges: a suspended OWNER is checked separately, at Send. So a record can
// read Enabled while every send to it is refused, and the page has to say so
// rather than presenting the setting as availability.
func TestTheDeliverySettingDoesNotClaimASendWouldBeAccepted(t *testing.T) {
	m := meaningFixture(t)
	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "alice@h"}, true); err != nil {
		t.Fatal(err)
	}
	m.register(protocol.Record{Name: "svc@h", Owner: "alice@h", Allow: []string{"admin@h"}})
	if _, err := m.bus.SetUserState("admin@h", "alice@h", "paused"); err != nil {
		t.Fatal(err)
	}
	// The fact the page must not contradict.
	if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "svc@h", Body: "x"}); err == nil {
		t.Fatal("the fixture's send was accepted, so there is nothing to misreport")
	}
	if !strings.Contains(m.get("/service?name=svc@h"), "Delivery: <strong>Enabled</strong>") {
		t.Fatal("the fixture no longer produces an enabled record whose sends are refused")
	}
	if !strings.Contains(m.row(m.get("/services"), "svc@h"), "<td>Enabled") {
		t.Fatal("the listing no longer shows the record as enabled")
	}
	// On both pages. Correcting the detail page and leaving the listing's
	// legend saying the withdrawn thing is the same shape as correcting one
	// face and leaving its sibling behind.
	for _, page := range []string{"/service?name=svc@h", "/services"} {
		body := m.get(page)
		if strings.Contains(body, "takes delivery now") {
			t.Errorf("%s says the record takes delivery now, which this send disproves", page)
		}
		if !strings.Contains(body, "does not establish that a send will be accepted") {
			t.Errorf("%s presents the delivery setting as an answer about the next send", page)
		}
		if !strings.Contains(body, "owner") {
			t.Errorf("%s does not name the owner's access as one of the other checks", page)
		}
	}
}

func TestACLFormsExplainRestrictedEmptyLists(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Name: "private@h", Owner: "admin@h"})
	for _, page := range []string{"/services", "/service?name=private@h"} {
		body := m.get(page)
		if !strings.Contains(body, "Empty allows only the owner and assigned Maintainers. Add names or * to share.") {
			t.Errorf("%s does not explain the restricted default and explicit sharing", page)
		}
		if strings.Contains(body, "Empty allows every authenticated caller") {
			t.Errorf("%s still promises open access", page)
		}
	}
}

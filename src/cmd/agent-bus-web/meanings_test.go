package main

import (
	"context"
	"fmt"
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
	"github.com/parf/ai-agent-bus/internal/display"
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
	lsCalls *atomic.Int64
}

func meaningFixture(t *testing.T) *meanings {
	t.Helper()
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens(), "admin@h")
	if err != nil {
		t.Fatal(err)
	}
	m := &meanings{t: t, bus: b, tokens: tokens, lsCalls: new(atomic.Int64)}
	apiHandler := api.New(b, tokens, "admin@h").Handler()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ls" {
			m.lsCalls.Add(1)
		}
		apiHandler.ServeHTTP(w, r)
	}))
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
	m.backend, m.web, m.session, m.client = backend, web, resp.Cookies()[0], client
	return m
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
	// A fixture that states no kind means a name on this bus, not an external
	// service: the daemon's own default is the external case.
	if r.Kind == "" {
		r.Kind = protocol.KindAgent
	}
	if _, err := m.bus.Register(r); err != nil {
		m.t.Fatal(err)
	}
}

// The four record shapes a reader has to be able to tell apart, on one page.
// Every one of them is "Active/Serving" or "Active/Offline" under the old
// vocabulary, which is why no single-fixture test caught it.
func (m *meanings) shapes() {
	m.t.Helper()
	// The daemon owner is a User and already has its own user record.
	m.register(protocol.Record{Kind: protocol.KindAgent, Name: "#reading@h", Owner: "admin@h", Descr: "a reader is on it"})
	m.register(protocol.Record{Kind: protocol.KindAgent, Name: "#quiet@h", Owner: "admin@h", Allow: []string{"*"}, Descr: "active, nobody reading"})
	// Through Manage: a record is deactivated by whoever manages it, not
	// declared inactive at registration.
	m.register(protocol.Record{Kind: protocol.KindAgent, Name: "#off@h", Owner: "admin@h", Descr: "the owner deactivated it"})
	off := protocol.StatusInactive
	if _, err := m.bus.Manage("admin@h", core.Management{Name: "#off@h", Status: &off}); err != nil {
		m.t.Fatal(err)
	}
	m.register(protocol.Record{Kind: protocol.KindService, Name: "elsewhere@h", Owner: "admin@h", Descr: "reached another way", Addr: "elsewhere.example", Proto: "https"})
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

// Status is Active or Inactive, and an inactive record is gone from /ls: the
// listing shows it only because the face reads /inactive, and its page is
// the read-only view built from that answer rather than a 404.
func TestStatusIsActiveOrInactiveAndInactiveStaysVisible(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	// Read out of each record's own row. "Inactive" is also an option in the
	// listing's own status filter, so a page-wide match is satisfied by a
	// listing that labels every record Active.
	listing := m.get("/agents")
	for _, want := range []struct{ name, cell string }{
		{"#off@h", "aria-label=Inactive"}, {"#quiet@h", "aria-label=Active"},
	} {
		if !strings.Contains(m.row(listing, want.name), want.cell) {
			t.Errorf("%s has no %q cell: %s", want.name, want.cell, m.row(listing, want.name))
		}
	}
	// The filter splits them, each way.
	if active := m.get("/agents?state=active"); strings.Contains(active, ">#off@h<") || !strings.Contains(active, ">#quiet@h<") {
		t.Error("the Active filter does not hold exactly the active records")
	}
	if inactive := m.get("/agents?state=inactive"); !strings.Contains(inactive, ">#off@h<") || strings.Contains(inactive, ">#quiet@h<") {
		t.Error("the Inactive filter does not hold exactly the inactive records")
	}
	// The page of an inactive record is its read-only view, not the daemon's
	// unknown, and it offers reactivation to whoever manages it.
	detail := section(t, m.get("/agent?name=%23off@h"), "<main>", "</main>")
	for _, want := range []string{"aria-label=Inactive", `name=action value=reactivate`, "returns with its owner"} {
		if !strings.Contains(detail, want) {
			t.Errorf("the inactive record page lacks %q: %s", want, detail)
		}
	}
	if strings.Contains(detail, "Deactivate…") || strings.Contains(detail, "Edit settings") {
		t.Error("the inactive record page offers an edit the daemon answers as unknown")
	}
	// The positive control: an active record says so on its own page, with
	// the opposite transition, so neither is a constant.
	quiet := section(t, m.get("/agent?name=%23quiet@h"), "<main>", "</main>")
	if !strings.Contains(quiet, "aria-label=Active") || !strings.Contains(quiet, "Deactivate…") || strings.Contains(quiet, `value=reactivate`) {
		t.Error("an active record is not labelled Active with its one transition")
	}
}

// Deactivating a record is confirmed first, and only by whoever manages it.
func TestRecordDeactivationIsConfirmedByItsManagerOnly(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "plain@h"}, true); err != nil {
		t.Fatal(err)
	}
	confirm := section(t, m.get("/service-deactivate?name=%23quiet@h"), "<main>", "</main>")
	if !strings.Contains(confirm, "Confirm deactivation") || !strings.Contains(confirm, `name=action value=deactivate`) {
		t.Fatalf("the confirmation lacks its final action: %s", confirm)
	}
	// Opening it changed nothing.
	if _, ok := m.bus.Lookup("admin@h", "#quiet@h"); !ok {
		t.Fatal("the confirmation page deactivated the record")
	}
	// #quiet@h admits everyone, so plain@h reads it and manages nothing.
	plain := m.as("plain@h")
	if _, status := getAs(t, plain, "/service-deactivate?name=%23quiet@h"); status != http.StatusForbidden {
		t.Errorf("a reader who does not manage the record opened its deactivation: %d", status)
	}
	if body, _ := getAs(t, plain, "/agent?name=%23quiet@h"); strings.Contains(body, "Deactivate…") {
		t.Error("a reader who does not manage the record is offered its deactivation")
	}
}

// Readers is an observation about outstanding reads on an inbox. It is not
// health, and a busy process between pulls is not offline.
func TestReadersAreObservedAndNeverCalledOfflineOrServing(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	m.attachReader("#reading@h")
	body := m.get("/agents")
	for _, banned := range []string{"Serving", "Offline", ">offline<", ">serving<"} {
		if strings.Contains(body, banned) {
			t.Errorf("the listing says %q, which is health language the daemon does not supply", banned)
		}
	}
	if !strings.Contains(m.row(body, "#reading@h"), "aria-label=Active title=Active>🔛</span><td class=num data-label=Readers>1") {
		t.Errorf("the outstanding read is not counted: %s", m.row(body, "#reading@h"))
	}
	if !strings.Contains(m.row(body, "#quiet@h"), "aria-label=Active title=Active>🔛</span><td class=num data-label=Readers>0") {
		t.Errorf("the measured zero is not shown: %s", m.row(body, "#quiet@h"))
	}
	if !strings.Contains(body, "None of it is health") {
		t.Error("the page does not say this is not health")
	}
}

func TestNumericTableColumnsAreRightAligned(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "#quiet@h", Body: "table alignment"}); err != nil {
		t.Fatal(err)
	}
	m.bus.SampleActivity(time.Now())
	for path, wants := range map[string][]string{
		"/agents":   {`th.num,td.num{text-align:right`, `<th scope=col class=num>Readers`, `<th scope=col class=num>Queued`, `<td class=num data-label=Readers>0`},
		"/activity": {`<th scope=col class=num>Accepted`, `<th scope=col class=num>Refused`, `<td class=num>0`},
		"/diagnostics": {`<th scope=col class=num>count`, `<th scope=col class=num>readers`, `<th scope=col class=num>held now`,
			`<th scope=col class=num>oldest held`, `<th scope=col class=num>Envelopes`,
			`<th scope=col class=num>dropped`, `<th scope=col class=num>expired`, `<td class=num>0`, `<td class=num data-label="Envelopes">1`},
	} {
		body := m.get(path)
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Errorf("%s does not right-align numeric table markup %q", path, want)
			}
		}
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

// A service is external, so the page has no reader observation to make about
// it and does not invent one. The reader column belongs to the records that
// have a queue. See docs/03-records.md#record-kinds.
func TestExternalDoesNotStandInForTheReaderObservation(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	m.attachReader("#reading@h")
	agent := m.row(m.get("/agents"), "#reading@h")
	if !strings.Contains(agent, `<td class=num data-label=Readers>1`) {
		t.Errorf("an agent being read reports its reader: %s", agent)
	}
	service := m.row(m.get("/services"), "elsewhere@h")
	for _, want := range []string{"<code>elsewhere.example</code>", "<code>https</code>"} {
		if !strings.Contains(service, want) {
			t.Errorf("the service row does not say where it is or how to speak to it, wanted %q: %s", want, service)
		}
	}
	for _, absent := range []string{"data-label=Readers", "data-label=Queued", "data-label=Accepted", "data-label=Dequeued", "data-label=Delivery"} {
		if strings.Contains(service, absent) {
			t.Errorf("the service row carries %q, which is an observation of a queue it does not have: %s", absent, service)
		}
	}
	detail := m.get("/service?name=elsewhere@h")
	if !strings.Contains(detail, "<h2>Where it is</h2>") || !strings.Contains(detail, "The daemon neither reaches the thing nor checks that it is there") {
		t.Errorf("the service detail does not state what the record actually says: %s", detail)
	}
	for _, absent := range []string{"<h2>Queue &amp; counters</h2>", "<h2>Delivery</h2>", "Disable delivery", "name=bound", "name=ttl", "name=overflow"} {
		if strings.Contains(detail, absent) {
			t.Errorf("the service detail offers %q, which is about a queue it does not have", absent)
		}
	}
}

// Accepted and dequeued come back from the snapshot, so they are lifetime
// totals rather than this run's. And handing a message to a reader is not the
// work being done.
func TestQueueCountersSayTheirScopeAndNeverSayCompleted(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "#quiet@h", Body: "held"}); err != nil {
		t.Fatal(err)
	}
	// The Services table now owns these record counters; Diagnostics no longer
	// duplicates the registry catalogue.
	// Three accepted and one dequeued, so accepted and dequeued cannot be
	// swapped without the numbers saying so. Equal counters make the column
	// names unfalsifiable.
	for i := 0; i < 2; i++ {
		if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "#quiet@h", Body: "held"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.bus.ConsumeAs(context.Background(), "#quiet@h", "#quiet@h", "", "", false, false); err != nil {
		t.Fatal(err)
	}
	head := m.get("/agents")
	for _, col := range []string{"<th scope=col class=num>Readers", "<th scope=col class=num>Queued",
		"<th scope=col class=num>Accepted", "<th scope=col class=num>Dequeued"} {
		if !strings.Contains(head, col) {
			t.Errorf("the Services table has no %q column: %s", col, head)
		}
	}
	// held now 2, accepted 3, dequeued 1 — in that order, so the columns are
	// named for the numbers under them rather than the other way round.
	if got := m.row(head, "#quiet@h"); !strings.Contains(got, "data-label=Queued>2<td class=num data-label=Accepted>3<td class=num data-label=Dequeued>1") {
		t.Errorf("the Services counters do not line up with their columns: %s", got)
	}
	for _, page := range []string{"/agents", "/agent?name=%23quiet@h"} {
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
	if !strings.Contains(m.get("/service?name=%23quiet@h"), "not the same as the work being done") &&
		!strings.Contains(m.get("/service?name=%23quiet@h"), "which is not completed") {
		t.Error("the page does not say dequeued is not completed")
	}
}

// A settings value the record does not carry is not a value to render. The
// three inherit differently and the page has to say which.
func TestUnsetQueueSettingsAreStatedAsInheritanceRatherThanGuessed(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	body := m.get("/service?name=%23quiet@h")
	for _, want := range []string{
		"uses the daemon default",         // Bound really does resolve one
		"not readable here",               // and the resolved number is not ours to show
		"no queue-imposed expiry",         // TTL has no default to inherit
		"a message may still specify its", // and the sender's half of it
		"<dt>When full<dd>refuse",         // normalised at registration
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
	m.register(protocol.Record{Kind: protocol.KindAgent, Name: "#bounded@h", Owner: "admin@h", Allow: []string{"reader@h"}, Bound: 7, TTL: "1m", Full: protocol.OverflowRing})
	// Read where the page states them, not anywhere on it: the management
	// form below carries both values in its inputs, so a page-wide match is
	// satisfied by a detail page that displays neither.
	set := m.get("/service?name=%23bounded@h")
	for _, want := range []string{"<dt>Queue bound<dd>7", "<dt>Retention<dd>1m", "<dt>When full<dd>drop the oldest"} {
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
	visitor := m.as("reader@h").get("/service?name=%23bounded@h")
	for _, want := range []string{"<dt>Queue bound<dd>7", "<dt>Retention<dd>1m"} {
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
	body := m.get("/diagnostics")
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
	held := fmt.Sprintf("<td class=num>%d", m.bus.Status().Refused["unknown"])
	if m.bus.Status().Refused["unknown"] == 0 {
		t.Fatal("the fixture produced no counted refusal")
	}
	if got := m.row(body, "unknown"); !strings.HasSuffix(got, held) {
		t.Errorf("a counted reason is not shown with its count (want %s): %s", held, got)
	}
	// And a reason nothing produced is a zero rather than a blank or a dash.
	if got := m.row(body, "malformed"); !strings.Contains(got, "<td class=num>0<") && !strings.HasSuffix(got, "<td class=num>0") {
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
	if !strings.Contains(m.get("/agents"), "not a count of this node") {
		t.Error("the listing does not scope itself to the caller")
	}
}

// Held is what the queue holds at the moment of the read, and at capacity is
// what was true then. Neither is a prediction, and the read does not prune.
func TestQueueObservationsSayWhenTheyWereTrue(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "#quiet@h", Body: "held"}); err != nil {
		t.Fatal(err)
	}
	for _, page := range []string{"/diagnostics", "/service?name=%23quiet@h"} {
		body := m.get(page)
		if !strings.Contains(body, "does not prune") {
			t.Errorf("%s does not say held may include messages already past their TTL", page)
		}
	}
	if !strings.Contains(m.get("/diagnostics"), "what was true when observed") {
		t.Error("the page does not date the capacity observation")
	}
	// And the row itself says it, on a queue that really is at its bound —
	// as an observation, never as a claim about what the next send will do.
	m.register(protocol.Record{Kind: protocol.KindAgent, Name: "#tiny@h", Owner: "admin@h", Bound: 1})
	if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "#tiny@h", Body: "fills it"}); err != nil {
		t.Fatal(err)
	}
	// On every surface that shows it. Dropping the label from the listing and
	// the detail page alone left the diagnostics page still saying it.
	for _, where := range []struct{ page, what string }{
		{"/diagnostics", "the backlog table"}, {"/agents", "the listing"},
	} {
		full := m.row(m.get(where.page), "#tiny@h")
		if !strings.Contains(full, "at capacity when observed") {
			t.Errorf("%s does not say a queue is at its bound: %s", where.what, full)
		}
		if strings.Contains(full, "refusing") {
			t.Errorf("%s states a prediction rather than an observation: %s", where.what, full)
		}
	}
	detail := m.get("/service?name=%23tiny@h")
	if !strings.Contains(detail, "at capacity when observed") {
		t.Error("the detail page does not say the queue is at its bound")
	}
	// And a queue below its bound says nothing, so the label is a distinction.
	if strings.Contains(m.row(m.get("/diagnostics"), "#quiet@h"), "at capacity") {
		t.Error("a queue below its bound is called at capacity")
	}
	// An empty age is not a zero age: the daemon says nothing about whether
	// the queue ever held anything. Matched as the template writes it — the
	// first version of this looked for lowercase "oldest held: 0" against a
	// page that says "Oldest held:", so it could not have fired.
	empty := m.get("/service?name=%23reading@h")
	if strings.Contains(empty, "<dt>Oldest held<dd>0") {
		t.Error("an absent oldest is rendered as a measured zero")
	}
	if !strings.Contains(empty, "<dt>Oldest held<dd><span class=muted>&mdash;</span>") {
		t.Errorf("an absent oldest is not shown as absent")
	}
	// The positive control, so the field is readable when there is one.
	if !strings.Contains(m.get("/service?name=%23quiet@h"), "<dt>Oldest held<dd>0s") {
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

// Kinds of identity, and the directory had one word for all of them
// (Plans/MVP/done/web-review.md W06). From 0.7 a directory row is a User or a
// credential with nothing behind it; an Agent is not a person and has no row,
// and a self-owned record no longer exists to be one. Every one of them is on
// the page at once, because a label only lies next to the thing it should
// have said.
//
// Each check reads the row for its own name. The page also carries a legend
// naming the kinds in prose, so a body-wide search for any of them passes
// whether or not a single row is labelled at all.
func TestTheDirectoryNamesThreeKindsOfIdentityAndGuessesNone(t *testing.T) {
	m := meaningFixture(t)
	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "person@h"}, true); err != nil {
		t.Fatal(err)
	}
	// An Agent that holds a credential: a principal, but nobody's profile.
	m.register(protocol.Record{Name: "#named@h", Owner: "admin@h", Kind: protocol.KindAgent, Allow: []string{"admin@h"}})
	if _, err := m.tokens.Issue("#named@h"); err != nil {
		t.Fatal(err)
	}
	// A name with neither a profile nor a record, which has to be minted
	// rather than registered: that is precisely what nothing else creates.
	if _, err := m.tokens.Issue("junk@h"); err != nil {
		t.Fatal(err)
	}
	body := m.get("/users")
	for _, kind := range []struct{ name, says string }{
		{"person@h", `aria-label="👤 User">👤</span> <a`},
		{"junk@h", "<td>Credential with no registered name</td>"},
	} {
		if !strings.Contains(m.row(body, kind.name), kind.says) {
			t.Errorf("%s is not named as %q: %s", kind.name, kind.says, m.row(body, kind.name))
		}
	}
	if row := m.row(body, "junk@h"); !strings.HasPrefix(row, `<td><a `) {
		t.Errorf("credential-only identity was given a prefix before its stated name: %s", row)
	}
	if strings.Contains(body, "<code>#named@h</code>") {
		t.Error("the directory gives an Agent a row")
	}
	// Named from what the daemon holds, never read off the spelling. A name
	// with a slash in it is a User like any other once it has a profile.
	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "claude/one@h"}, true); err != nil {
		t.Fatal(err)
	}
	if row := m.row(m.get("/users"), "claude/one@h"); !strings.Contains(row, `aria-label="👤 User">👤</span> <a`) {
		t.Errorf("a slashed name is not named the same way as any other user: %s", row)
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
	m.register(protocol.Record{Name: "#svc@h", Owner: "admin@h", Kind: protocol.KindAgent, Allow: []string{"peer@h"}})
	m.register(protocol.Record{Name: "#bot@h", Owner: "admin@h", Kind: "agent"})
	m.register(protocol.Record{Name: "jobs@h", Owner: "admin@h", Kind: protocol.KindQueue})

	users := m.get("/users?kind=users")
	if row := m.row(users, "person@h"); !strings.Contains(row, `aria-label="👤 User">👤</span> <a`) || strings.Contains(row, `<span class=muted>👤 User</span>`) {
		t.Errorf("registered user has no identity label: %s", row)
	}
	if !strings.Contains(users, `href="/users?kind=users" aria-current=true>👤 Users`) || strings.Contains(users, `kind=%F0`) {
		t.Errorf("directory filter mixed its displayed label into the URL value: %s", users)
	}

	m.register(protocol.Record{Name: "db@h", Owner: "admin@h", Kind: protocol.KindService, Addr: "db.example:5432", Proto: "postgresql"})
	if row := m.row(m.get("/services"), "db@h"); !strings.Contains(row, "📡 Service") {
		t.Errorf("db@h has no service label: %s", row)
	}
	// Each kind is labelled where it is listed.
	if row := m.row(m.get("/agents"), "#bot@h"); !strings.Contains(row, "👾 Agent") {
		t.Errorf("#bot@h has no agent label: %s", row)
	}
	if row := m.row(m.get("/queues"), "jobs@h"); !strings.Contains(row, "📮 Queue") {
		t.Errorf("jobs@h has no queue label: %s", row)
	}
	// Each channel kind registers on its own page, and each carries its kind
	// as the daemon's plain word.
	for kind, path := range map[string]string{protocol.KindQueue: "/queues/new", protocol.KindPubSub: "/pubsub/new"} {
		create := m.get(path)
		if !strings.Contains(create, "name=kind value="+kind) {
			t.Errorf("the %s form does not keep a plain kind value: %s", kind, create)
		}
		// The other half of the same fact: the word above is what the form
		// carries, and no attribute value anywhere on the page carries the
		// glyph the reader sees instead.
		for _, k := range protocol.Kinds {
			if glyph := display.EntityGlyph(k); glyph != "" && strings.Contains(create, "value="+glyph) {
				t.Errorf("the %s form carries %s as a value: %s", kind, glyph, create)
			}
			if glyph := display.EntityGlyph(k); glyph != "" && strings.Contains(create, "value=\""+glyph) {
				t.Errorf("the %s form carries %s as a quoted value: %s", kind, glyph, create)
			}
		}
	}

	if detail := m.get("/service?name=%23svc@h"); !strings.Contains(detail, "class=fact-pill>👾 Agent") {
		t.Error("detail lost its label")
	}
	// The editable value is on the settings form, and it is the daemon's
	// plain syntax there: the glyph belongs to the label above.
	editor := m.get("/agent/edit?name=%23svc@h")
	if !strings.Contains(editor, `<textarea name=allow rows=5 placeholder="#agent@realm&#10;user@realm&#10;@group&#10;@owner&#10;*" aria-invalid="false" aria-describedby="">peer@h</textarea>`) {
		t.Errorf("the settings form lost its plain ACL value: %s", editorOf(t, editor))
	}
	for _, k := range protocol.Kinds {
		if glyph := display.EntityGlyph(k); glyph != "" && strings.Contains(editorOf(t, editor), glyph) {
			t.Errorf("a display glyph entered the editable settings form: %s", glyph)
		}
	}

}

// A figure the face worked out and a figure the daemon reported are not the
// same kind of fact, and a page showing them side by side without saying which
// has merged them exactly as declared and observed must not be merged.
func TestFaceComputedCountsAreMarkedAsTheFacesOwn(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Kind: protocol.KindAgent, Name: "#named@h", Owner: "admin@h"})
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
	m.register(protocol.Record{Kind: protocol.KindAgent, Name: "#private@h", Owner: "admin@h", Allow: []string{"admin@h"}})

	seen := map[string]bool{}
	for _, who := range []string{"admin@h", "maint@h", "plain@h"} {
		body := m.as(who).get("/agents")
		seen[who] = strings.Contains(body, "#private@h")
		// Whoever is reading, the words for the three facts are the same. A
		// label that changed with rank would be saying something about the
		// caller rather than about the record.
		if !strings.Contains(body, "<th scope=col>Status") || !strings.Contains(body, "Reader") {
			t.Errorf("%s does not get the same columns", who)
		}
		if strings.Contains(body, "Serving") || strings.Contains(body, "Delivery") || strings.Contains(body, "Enabled") {
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
	// Management controls live on detail rather than being duplicated in the
	// list. The detail still follows authority, and the words do not.
	// Authority over a record is the record's own: its owner, the name
	// itself, and the group the record names. Standing in the daemon's
	// maintainer group is not one of them (core/manage.go manages), so a
	// maintainer reads admin@h's record exactly as any other caller does —
	// which is the point, since the words did not change either.
	for _, who := range []struct {
		caller       string
		getsSettings bool
	}{
		{"admin@h", true}, {"maint@h", false}, {"plain@h", false},
	} {
		as := m.as(who.caller)
		row := m.row(as.get("/agents"), "#quiet@h")
		if strings.Contains(row, ">Edit</a>") {
			t.Errorf("%s sees the duplicate list Edit action", who.caller)
		}
		detail := as.get("/service?name=%23quiet@h")
		if strings.Contains(detail, `id=settings`) != who.getsSettings {
			t.Errorf("%s receives the wrong detail authority", who.caller)
		}
	}
	// And the caller's own record is theirs to manage, whoever they are, so
	// the check above is about authority rather than about rank. A queue
	// plain@h registered is listed as theirs.
	m.register(protocol.Record{Kind: protocol.KindQueue, Name: "plain-jobs@h", Owner: "plain@h", Allow: []string{"*"}})
	plain := m.as("plain@h")
	if row := m.row(plain.get("/queues"), "plain-jobs@h"); !strings.Contains(row, `class="record-name-cell owned-record"`) || strings.Contains(row, "Yours") || !strings.Contains(plain.get("/queue?name=plain-jobs@h"), `id=settings`) {
		t.Error("an ordinary caller is offered no control over their own record")
	}
	// Registering a user creates that user's own record (core/users.go). It is
	// Personal and is listed on no shared page, but its own page is still
	// theirs to manage. The signed-in identity is named in the header of
	// every page, so absence is asked of the listing's own row link.
	for _, list := range []string{"/services", "/queues", "/pubsub", "/agents"} {
		if strings.Contains(plain.get(list), `class=record-name href="`+detailPathFor(protocol.KindUser)+`?name=plain%40h`) {
			t.Errorf("a user's own record is still listed on %s", list)
		}
	}
	if !strings.Contains(plain.get("/queue?name=plain@h"), `id=settings`) {
		t.Error("an ordinary caller is offered no control over their own user record")
	}
}

// Pub/sub and queue are what the record declares about delivery, and a
// service declares neither. Status beside it on these pages is a different
// fact about a different thing, so both
// have to be readable at once without either standing in for the other.
func TestPubSubAndQueueDeliveryAreNamedAndNeitherIsGuessed(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Name: "fanout@h", Owner: "admin@h", Kind: protocol.KindPubSub})
	m.register(protocol.Record{Name: "onebyone@h", Owner: "admin@h", Kind: protocol.KindQueue})
	m.register(protocol.Record{Name: "db@h", Owner: "admin@h", Kind: protocol.KindService, Addr: "h:1", Proto: "https"})
	if !strings.Contains(m.get("/service?name=fanout@h"), "a copy to each subscriber") {
		t.Error("a pub/sub topic does not say every subscriber gets a copy")
	}
	if !strings.Contains(m.get("/service?name=onebyone@h"), "one at a time") {
		t.Error("a queue topic does not say one subscriber takes each message")
	}
	// A service delivers nothing here, so it is shown no delivery at all: it
	// is external, and neither answer would be about this bus.
	for _, guess := range []string{"a copy to each subscriber", "one at a time"} {
		if strings.Contains(m.get("/service?name=db@h"), guess) {
			t.Errorf("an external service is shown the delivery %q", guess)
		}
	}
	// Both records are active, so the declared mode cannot be being read off
	// the status the listing shows beside it.
	for name, list := range map[string]string{"fanout@h": "/pubsub", "onebyone@h": "/queues"} {
		if !strings.Contains(m.row(m.get(list), name), "aria-label=Active") {
			t.Errorf("%s is not active, so its mode and its status are not separable here", name)
		}
	}
}

// Readers counts every outstanding consume request. A filtered waiter may
// coexist with a backlog and must still appear in the same numeric column.
func TestReadersCountsFilteredAndUnfilteredWaits(t *testing.T) {
	m := meaningFixture(t)
	m.shapes()
	if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "#quiet@h", Body: "held"}); err != nil {
		t.Fatal(err)
	}
	m.attachReader("#reading@h")
	// The diagnostics page lists a queue with no read on it.
	body := m.get("/diagnostics")
	if !strings.Contains(m.row(body, "#quiet@h"), "<td class=num>0<td class=num>1<td class=num>") {
		t.Errorf("the backlog does not show readers=0 and held=1: %s", m.row(body, "#quiet@h"))
	}
	if !strings.Contains(m.row(m.get("/agents"), "#reading@h"), "aria-label=Active title=Active>🔛</span><td class=num data-label=Readers>1") {
		t.Error("an unfiltered read is not counted")
	}
	// A read restricted to a topic is counted while a nonmatching backlog stays.
	n := meaningFixture(t)
	n.shapes()
	if _, err := n.bus.Send(protocol.Envelope{From: "admin@h", To: "#reading@h", Body: "held"}); err != nil {
		t.Fatal(err)
	}
	taken := n.attachReader("#reading@h", "other")
	row := n.row(n.get("/diagnostics"), "#reading@h")
	if !strings.Contains(row, "<td class=num>1<td class=num>1<td class=num>") {
		t.Errorf("the filtered read and held message are not both reported: %s", row)
	}
	for _, phrase := range []string{"filtered and unfiltered together", "positive count promises neither"} {
		if !strings.Contains(n.get("/diagnostics"), phrase) {
			t.Errorf("the page omits the count boundary %q", phrase)
		}
	}
	// And it does not say they will not take anything, which is false: a
	// matching filtered waiter is served ahead of an unfiltered one.
	for _, page := range []string{"/diagnostics", "/agents", "/agent?name=%23reading@h"} {
		if strings.Contains(n.get(page), "will not take") {
			t.Errorf("%s says a filtered read will not take a message, which deliver disproves", page)
		}
	}
	// The proof, on the fixture: the counted reader receives its match. Receipt,
	// not only the count falling, proves the waiter was served.
	if _, err := n.bus.Send(protocol.Envelope{From: "admin@h", To: "#reading@h", Topic: "other", Body: "only this body"}); err != nil {
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
	if !strings.Contains(empty, "No other identities match this view") {
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
	if strings.Contains(full, "No other identities match this view") {
		t.Error("the section still says it is empty while holding a row")
	}
}

// Active is not an answer about the next send. The status says the record is
// an entity; the ACL, the owner's access and queue capacity are checked at
// Send. So a record can read Active while a send to it is refused, and the
// page has to say so rather than presenting the status as availability.
func TestTheActiveStatusDoesNotClaimASendWouldBeAccepted(t *testing.T) {
	m := meaningFixture(t)
	for _, name := range []string{"alice@h", "bob@h"} {
		if _, err := m.bus.SetUser("admin@h", protocol.User{Name: name}, true); err != nil {
			t.Fatal(err)
		}
	}
	m.register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "alice@h", Allow: []string{"admin@h"}})
	// The fact the page must not contradict: bob is not on the allow list.
	if _, err := m.bus.Send(protocol.Envelope{From: "bob@h", To: "#svc@h", Body: "x"}); err == nil {
		t.Fatal("the fixture's send was accepted, so there is nothing to misreport")
	}
	if !strings.Contains(m.get("/agent?name=%23svc@h"), "aria-label=Active title=Active>🔛") {
		t.Fatal("the fixture no longer produces an active record whose sends are refused")
	}
	if !strings.Contains(m.row(m.get("/agents"), "#svc@h"), "aria-label=Active") {
		t.Fatal("the listing no longer shows the record as active")
	}
	// On both pages. Correcting the detail page and leaving the listing's
	// legend saying the withdrawn thing is the same shape as correcting one
	// face and leaving its sibling behind.
	for _, page := range []string{"/agent?name=%23svc@h", "/agents"} {
		body := m.get(page)
		if strings.Contains(body, "takes delivery now") {
			t.Errorf("%s says the record takes delivery now, which this send disproves", page)
		}
		if !strings.Contains(body, "does not establish that a send will be accepted") {
			t.Errorf("%s presents the status as an answer about the next send", page)
		}
		if !strings.Contains(body, "owner") {
			t.Errorf("%s does not name the owner's access as one of the other checks", page)
		}
	}
}

func TestACLFormsExplainRestrictedEmptyLists(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Kind: protocol.KindAgent, Name: "#private@h", Owner: "admin@h"})
	// Both halves of the one form: registering and editing ask the same
	// question, so they carry the same explanation of it.
	for _, page := range []string{"/services/new", "/agent/edit?name=%23private@h"} {
		body := m.get(page)
		if !strings.Contains(body, "Empty allows only the owner and assigned Maintainers.") || !strings.Contains(body, "shares with every admitted principal") {
			t.Errorf("%s does not explain the restricted default and explicit sharing", page)
		}
		if !strings.Contains(body, "adds records directly owned by this record") || !strings.Contains(body, "runtime ACL syntax, not an editable group") {
			t.Errorf("%s does not explain the runtime @owner term", page)
		}
		if strings.Contains(body, "Empty allows every authenticated caller") {
			t.Errorf("%s still promises open access", page)
		}
		if strings.Contains(body, "Master access") || strings.Contains(body, "no_master") {
			t.Errorf("%s still renders the retired master layer", page)
		}
	}
}

func TestGroupPagesExplainThatOwnerIsRuntimeACLSyntax(t *testing.T) {
	m := meaningFixture(t)
	// The Groups collection carries the explanation as standing help, where
	// somebody reading about groups will meet it.
	if body := m.get("/groups"); !strings.Contains(body, "@owner") || !strings.Contains(body, "runtime ACL syntax") {
		t.Error("/groups presents @owner as an ordinary group name")
	}
	// The register form does not: the owner removed a standing warning about
	// one reserved word from a form that accepts every other name. It is
	// said when it applies, which is when that name is what was submitted.
	blank := m.get("/groups/new")
	if strings.Contains(blank, "runtime ACL syntax") {
		t.Errorf("the empty register form warns about a name nobody typed: %s", blank)
	}
	attempted, _ := postBody(t, m, "/groups", url.Values{
		"action": {"save"}, "new": {"1"}, "name": {"@owner"}, "members": {"admin@h"},
	})
	if !strings.Contains(attempted, "runtime ACL syntax") {
		t.Errorf("registering @owner was refused without saying why: %s", attempted)
	}
	// An ordinary refused name gets its own error and not this one.
	other, _ := postBody(t, m, "/groups", url.Values{
		"action": {"save"}, "new": {"1"}, "name": {"not-a-group"}, "members": {"admin@h"},
	})
	if strings.Contains(other, "runtime ACL syntax") {
		t.Errorf("an unrelated refused name was blamed on @owner: %s", other)
	}
}

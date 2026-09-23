package main

import (
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
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func TestActivityViewUsesActualTimeAndOneScale(t *testing.T) {
	start := time.Date(2026, time.September, 17, 12, 0, 0, 0, time.UTC)
	points := []core.ActivityPoint{
		{At: start, Counts: core.Counts{In: 2, Out: 1}},
		{At: start.Add(10 * time.Minute), Counts: core.Counts{In: 4, Dropped: 1}},
		{At: start.Add(40 * time.Minute), Counts: core.Counts{Out: 3, Refused: 2}},
	}
	detail := activityView(points, "#svc@h", "42m", false)
	full := activityView(points, "#svc@h", "42m", true)
	if detail.Max != 4 || full.Max != 4 {
		t.Fatalf("shared maximum = %d / %d, want 4", detail.Max, full.Max)
	}
	if len(detail.Series) != 4 || strings.Join(detail.Zero, ",") != "Expired" {
		t.Fatalf("nonzero/zero split: series=%+v zero=%v", detail.Series, detail.Zero)
	}
	if len(full.Series) != len(detail.Series) {
		t.Fatal("detail and full view changed the displayed series for the same answer")
	}
	for i := range detail.Series {
		if detail.Series[i] != full.Series[i] {
			t.Fatalf("series %d differs between detail and full view", i)
		}
	}
	if got := detail.Series[0].Points; got != "50.0,75.0 190.0,25.0 610.0,125.0" {
		t.Fatalf("Accepted positions = %q; middle point must reflect 10/40 of the actual interval", got)
	}
	if detail.Series[1].Label != "Dequeued" || detail.Series[1].Class != "activity-output" || detail.Series[1].Points != "50.0,100.0 190.0,125.0 610.0,50.0" {
		t.Fatalf("Dequeued did not retain the shared scale and distinct presentation: %+v", detail.Series[1])
	}
	if detail.Start != "Sep 17 12:00:00" || detail.End != "Sep 17 12:40:00" || detail.Observed != "40m0s" {
		t.Fatalf("observed interval: %+v", detail)
	}
	if detail.ShowTable || !full.ShowTable {
		t.Fatal("sample table scope reversed")
	}
}

func TestActivityViewMarksOnePartialSample(t *testing.T) {
	v := activityView([]core.ActivityPoint{{At: time.Now(), Counts: core.Counts{In: 2}}}, "#svc@h", "12s", false)
	if v.Observed != "one partial sample" || len(v.Series) != 1 || !v.Series[0].Single || v.Series[0].X != "330.0" || v.Series[0].Y != "25.0" {
		t.Fatalf("one partial sample has no visible point: %+v", v)
	}
}

func TestRecordActivityEmbedsChartAndFullViewKeepsValues(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "admin@h"})
	m.bus.SampleActivity(time.Now().Add(-2 * time.Minute))
	if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "#svc@h", Body: "not rendered"}); err != nil {
		t.Fatal(err)
	}
	detail := m.get("/service?name=%23svc@h")
	for _, want := range []string{
		"<h2>Activity</h2>", `aria-label="About this activity history"`, `<li>The daemon keeps about 24 hours`, `Samples are usually about ten minutes apart`, `class=activity-chart`, "Accepted: 1 in the shown samples",
		`<circle class="activity-point activity-accepted"`,
		"Shared scale: 0–1 per sample over the displayed nonzero series.",
		"Dequeued means handed to a reader, not completed.",
		`href="/activity?name=%23svc%40h">View all activity and sample values</a>`,
	} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail lacks %q", want)
		}
	}
	if strings.Contains(detail, "<summary>Sample values</summary>") || strings.Contains(detail, "not rendered") {
		t.Fatal("compact detail exposed the full table or a message body")
	}
	full := m.get("/activity?name=%23svc%40h")
	for _, want := range []string{`selected>#svc@h</option>`, "<summary>Sample values</summary>", "<th scope=col class=num>Accepted", "About activity history"} {
		if !strings.Contains(full, want) {
			t.Errorf("full Activity view lacks %q", want)
		}
	}
	all := m.get("/activity")
	if !strings.Contains(all, "Scope: visible records") || !strings.Contains(all, "Refused is node-wide for the daemon Owner and covers visible records for other callers") {
		t.Fatal("unfiltered activity did not state its caller-dependent refusal scope")
	}
}

func TestRecordActivityDistinguishesAbsentAndMeasuredZero(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "admin@h"})
	absent := m.get("/service?name=%23svc@h")
	if !strings.Contains(absent, "Activity history is not observed yet. Collecting the first sample after this daemon restart") || !strings.Contains(absent, "no zero series is inferred") || strings.Contains(absent, "class=activity-chart") {
		t.Fatal("fresh history was rendered as zero, broken or charted")
	}
	m.bus.SampleActivity(time.Now().Add(-time.Minute))
	zero := m.get("/service?name=%23svc@h")
	if !strings.Contains(zero, "All five series: <strong>0</strong> in this window") || strings.Contains(zero, "Measured zero:") || strings.Contains(zero, "class=activity-chart") {
		t.Fatal("measured zero was confused with absent history")
	}
}

func TestRecordActivityAttemptsOnceAndDegradesWithoutLeakingHiddenRecord(t *testing.T) {
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens(), "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	face := api.New(b, tokens, "owner@h").Handler()
	for _, who := range []string{"other@h"} {
		if _, err := b.SetUser("owner@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "owner@h", Allow: []string{"owner@h"}}); err != nil {
		t.Fatal(err)
	}
	var activityCalls atomic.Int32
	failActivity := true
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/activity" {
			activityCalls.Add(1)
			if failActivity {
				http.Error(w, `{"error":"fixture unavailable"}`, http.StatusServiceUnavailable)
				return
			}
		}
		face.ServeHTTP(w, r)
	}))
	defer backend.Close()
	web := httptest.NewServer(dashboard(&caller{client: backend.Client(), base: backend.URL}, false))
	defer web.Close()
	client := web.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	signIn := func(who string) *http.Cookie {
		t.Helper()
		token, err := tokens.Issue(who)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.PostForm(web.URL+"/signin", url.Values{"token": {token}})
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		return resp.Cookies()[0]
	}
	get := func(cookie *http.Cookie) (string, int) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, web.URL+"/service?name=%23svc%40h", nil)
		req.AddCookie(cookie)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return string(body), resp.StatusCode
	}
	owner, other := signIn("owner@h"), signIn("other@h")
	if body, status := get(owner); status != http.StatusOK || !strings.Contains(body, "Activity unavailable") || !strings.Contains(body, "#svc@h") || activityCalls.Load() != 1 {
		t.Fatalf("failed activity read: status=%d calls=%d body=%s", status, activityCalls.Load(), body)
	}
	failActivity = false
	if body, status := get(owner); status != http.StatusOK || strings.Contains(body, "Activity unavailable") || activityCalls.Load() != 2 {
		t.Fatalf("successful activity read: status=%d calls=%d", status, activityCalls.Load())
	}
	if body, status := get(other); status != http.StatusNotFound || activityCalls.Load() != 2 || strings.Contains(body, "#svc@h") {
		t.Fatalf("hidden lookup reached activity or leaked the record: status=%d calls=%d body=%s", status, activityCalls.Load(), body)
	}
}

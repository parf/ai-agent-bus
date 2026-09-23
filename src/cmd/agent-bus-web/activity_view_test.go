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

// dayFrom is a daemon's answer: 144 ten-minute slots from start, with counts
// where set says.
func dayFrom(start time.Time, set map[int]core.Counts) []core.ActivityPoint {
	points := make([]core.ActivityPoint, 144)
	for i := range points {
		points[i] = core.ActivityPoint{At: start.Add(time.Duration(i) * 10 * time.Minute), Counts: set[i]}
	}
	return points
}

func TestActivityViewDrawsAFixedDayWithHourTicks(t *testing.T) {
	start := time.Date(2026, time.September, 22, 15, 20, 0, 0, time.UTC)
	points := dayFrom(start, map[int]core.Counts{0: {In: 2, Out: 1}, 4: {In: 4, Dropped: 1}, 143: {Out: 3, Refused: 2}})
	detail := activityView(points, "#svc@h", "42m", false)
	full := activityView(points, "#svc@h", "42m", true)
	if detail.Max != 4 || full.Max != 4 {
		t.Fatalf("shared maximum = %d / %d, want 4", detail.Max, full.Max)
	}
	if len(detail.Series) != 4 || strings.Join(detail.Zero, ",") != "Expired" {
		t.Fatalf("nonzero/zero split: series=%+v zero=%v", detail.Series, detail.Zero)
	}
	for i := range detail.Series {
		if detail.Series[i] != full.Series[i] {
			t.Fatalf("series %d differs between detail and full view", i)
		}
	}
	accepted := strings.Fields(detail.Series[0].Points)
	if len(accepted) != 144 || accepted[0] != "50.0,75.0" || accepted[1] != "53.9,125.0" || accepted[4] != "65.7,25.0" || accepted[143] != "610.0,125.0" {
		t.Fatalf("Accepted is not one continuous point per slot on a fixed axis: %d points %v", len(accepted), accepted)
	}
	if detail.Start != "Sep 22 15:20" || detail.End != "Sep 23 15:20" {
		t.Fatalf("day: %s to %s", detail.Start, detail.End)
	}
	if len(detail.Ticks) != 24 || detail.Ticks[0].X != "65.7" || detail.Ticks[0].Label != "" || detail.Ticks[2].Label != "18:00" || detail.Ticks[1].Label != "" {
		t.Fatalf("hour ticks: %+v", detail.Ticks)
	}
	if detail.ShowTable || !full.ShowTable {
		t.Fatal("slot table scope reversed")
	}
}

func TestRecordActivityEmbedsChartAndFullViewListsSlots(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "admin@h"})
	if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "#svc@h", Body: "not rendered"}); err != nil {
		t.Fatal(err)
	}
	detail := m.get("/service?name=%23svc@h")
	for _, want := range []string{
		"<h2>Activity</h2>", `aria-label="About this activity history"`, `<li>The last 24 hours in ten-minute slots of the node's clock`, `<li>A time the daemon was down reads as zero`, `<svg class=activity-chart`, "Accepted: 1 in the last day",
		`<path class=activity-tick d="M`, `<text class=activity-hour x=`, `<polyline class="activity-line activity-accepted"`,
		"Shared scale: 0–1 per ten-minute slot over the displayed nonzero series; a tick at every hour.",
		"Dequeued means handed to a reader, not completed.",
		`href="/activity?name=%23svc%40h">View all activity and slot values</a>`,
	} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail lacks %q", want)
		}
	}
	if n := strings.Count(detail, `<path class=activity-tick d="M`); n != 24 {
		t.Errorf("%d hour ticks, want 24", n)
	}
	if strings.Contains(detail, "<summary>Slot values</summary>") || strings.Contains(detail, "not rendered") {
		t.Fatal("compact detail exposed the full table or a message body")
	}
	full := m.get("/activity?name=%23svc%40h")
	for _, want := range []string{`selected>#svc@h</option>`, "<summary>Slot values</summary>", "<th scope=col>Slot<th scope=col class=num>Accepted", "About activity history"} {
		if !strings.Contains(full, want) {
			t.Errorf("full Activity view lacks %q", want)
		}
	}
	if n := strings.Count(full, "<tr><td>"); n != 144 {
		t.Errorf("slot table has %d rows, want 144", n)
	}
	all := m.get("/activity")
	if !strings.Contains(all, "Scope: visible records") || !strings.Contains(all, "Refused is node-wide for the daemon Owner and covers visible records for other callers") {
		t.Fatal("unfiltered activity did not state its caller-dependent refusal scope")
	}
}

func TestRecordActivityOfAQuietDayIsZero(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "admin@h"})
	zero := m.get("/service?name=%23svc@h")
	if !strings.Contains(zero, "All five series: <strong>0</strong> in the last day") || strings.Contains(zero, "Zero all day:") || strings.Contains(zero, "<svg class=activity-chart") {
		t.Fatal("a quiet day was not one plain zero")
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

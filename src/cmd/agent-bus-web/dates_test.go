package main

import (
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
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// The registry says when each record was last written, and the pages that
// show a record show it: "registered at some point" and "registered an hour
// ago" are different answers to the question somebody is actually asking.
func TestListingAndDetailCarryTheRecordDate(t *testing.T) {
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens(), "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	backend := httptest.NewServer(api.New(b, tokens, "owner@h").Handler())
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
	session := resp.Cookies()[0]

	before := time.Now()
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "owner@h", Descr: "service"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "chan@h", Owner: "owner@h", Kind: protocol.KindQueue}); err != nil {
		t.Fatal(err)
	}

	get := func(path string) string {
		t.Helper()
		req, _ := http.NewRequest("GET", web.URL+path, nil)
		req.AddCookie(session)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			t.Fatalf("GET %s: %d", path, resp.StatusCode)
		}
		return string(body)
	}

	// Detail retains the full write time. Lists use the compact form tested
	// below, where this new record is "now" rather than a second timestamp.
	stamp := before.Format("2006-01-02 15:04")
	if body := get("/service?name=%23svc@h"); !strings.Contains(body, stamp) {
		t.Fatalf("detail does not carry the record's date %q", stamp)
	}
	for _, page := range []string{"/agents", "/channels"} {
		if body := get(page); !strings.Contains(body, "<td data-label=Updated>now") {
			t.Fatalf("%s does not render the new record as updated now", page)
		}
	}
	// Named as what it is rather than as "date": the record's own registration
	// update, which is not an observation time and not a sample time
	// (Plans/MVP/web/data-dictionary.md#time).
	listing := get("/agents")
	if !strings.Contains(listing, "<th scope=col>Updated") {
		t.Fatal("the listing has no column for the date it now shows")
	}
	// A column added to the table has to be added to its empty row too, or
	// "no matching records" stops spanning the table it is in. Counted rather
	// than written down, so widening the table cannot quietly pass this.
	want := fmt.Sprintf("colspan=%d", strings.Count(listing, "<th scope=col"))
	if body := get("/agents?scope=my&state=inactive"); strings.Contains(body, "No matching records") && !strings.Contains(body, want) {
		t.Fatalf("the empty row does not span the widened table: want %s", want)
	}
}

func TestRegistrationUpdatedUsesAgeThenCompactCalendarDate(t *testing.T) {
	now := time.Date(2026, time.September, 17, 20, 0, 0, 0, time.FixedZone("EDT", -4*60*60))
	for _, test := range []struct {
		name string
		at   time.Time
		want string
	}{
		{"zero", time.Time{}, ""},
		{"future skew", now.Add(time.Minute), "now"},
		{"future year", now.AddDate(1, 0, 0), "now"},
		{"seconds", now.Add(-59 * time.Second), "now"},
		{"minutes", now.Add(-17 * time.Minute), "17m ago"},
		{"hours", now.Add(-9 * time.Hour), "9h ago"},
		{"days", now.Add(-29 * 24 * time.Hour), "29d ago"},
		{"thirty day boundary", now.Add(-30 * 24 * time.Hour), "Aug 18"},
		{"same year", time.Date(2026, time.January, 1, 8, 0, 0, 0, time.UTC), "Jan 1"},
		{"other year", time.Date(2025, time.January, 12, 8, 0, 0, 0, time.UTC), "Jan 12, 2025"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := registrationUpdatedAt(test.at, now); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestHumanNumbersUseThousandsSeparators(t *testing.T) {
	for _, test := range []struct {
		value any
		want  string
	}{
		{0, "0"}, {999, "999"}, {1000, "1,000"}, {123456789, "123,456,789"},
		{int64(-12000), "-12,000"}, {uint64(5000000), "5,000,000"},
	} {
		if got := number(test.value); got != test.want {
			t.Errorf("number(%v) = %q, want %q", test.value, got, test.want)
		}
	}
	if got := number("1000"); got != "" {
		t.Fatalf("unsupported machine-shaped value was reformatted as %q", got)
	}
}

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
	if _, err := b.Register(protocol.Record{Name: "svc@h", Owner: "owner@h", Descr: "service"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "chan@h", Owner: "owner@h", Kind: protocol.KindTopic, Mode: "pubsub"}); err != nil {
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

	// The minute the record was written, as the page spells it. Reading the
	// clock again would be a different test on a minute boundary.
	stamp := before.Format("2006-01-02 15:04")
	for _, page := range []string{"/services", "/channels", "/service?name=svc@h"} {
		body := get(page)
		if !strings.Contains(body, stamp) {
			t.Fatalf("%s does not carry the record's date %q", page, stamp)
		}
	}
	// Named as what it is rather than as "date": the record's own registration
	// update, which is not an observation time and not a sample time
	// (Plans/MVP/web/data-dictionary.md#time).
	listing := get("/services")
	if !strings.Contains(listing, "<th scope=col>Registration updated") {
		t.Fatal("the listing has no column for the date it now shows")
	}
	// A column added to the table has to be added to its empty row too, or
	// "no matching records" stops spanning the table it is in. Counted rather
	// than written down, so widening the table cannot quietly pass this.
	want := fmt.Sprintf("colspan=%d", strings.Count(listing, "<th scope=col>"))
	if body := get("/services?scope=my&state=inactive"); strings.Contains(body, "No matching records") && !strings.Contains(body, want) {
		t.Fatalf("the empty row does not span the widened table: want %s", want)
	}
}

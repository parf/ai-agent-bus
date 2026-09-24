package api

import (
	"encoding/json"
	"github.com/parf/ai-agent-bus/internal/callstats"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/version"
)

func TestPublicNodeIdentityIsOnlyThePublishedFacts(t *testing.T) {
	b := core.New()
	s, _ := serverFor(t, b, "owner@h")
	var calls atomic.Uint64
	history := callstats.New(&calls)
	history.Sample(time.Now().Add(-2 * time.Minute))
	calls.Add(7)
	s.Calls(history.Snapshot)
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#private-inbox@h", Owner: "owner@h", Allow: []string{"owner@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#private-inbox@h", Body: "secret body"}); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"", "invalid-credential"} {
		r := httptest.NewRequest("GET", "/identity", nil)
		r.Header.Set(HeaderToken, token)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("public identity: %d %s", w.Code, w.Body.String())
		}
		var fields map[string]json.RawMessage
		var got protocol.NodeIdentity
		if err := json.Unmarshal(w.Body.Bytes(), &fields); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		host, err := os.Hostname()
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := fields["load"]; ok {
			t.Fatal("OS load still published")
		}
		if _, ok := fields["messages"]; ok {
			t.Fatal("message counters still published")
		}
		if len(fields) != 6 || got.Owner != "owner@h" || got.Version != version.String || got.Build != version.Build || got.Host != host || got.Calls == nil || len(got.Calls.Windows) != 2 || got.Calls.Total != 7 {
			t.Fatalf("public projection differs: %v", got)
		}
		for _, window := range got.Calls.Windows {
			if !window.Available || window.Count != 7 {
				t.Fatalf("public request totals not wired to the process counter: %+v", window)
			}
		}
		if strings.Contains(w.Body.String(), "private-inbox") || strings.Contains(w.Body.String(), "secret body") {
			t.Fatal("public total leaked contributing records or bodies")
		}
		if up, err := time.ParseDuration(got.Up); err != nil || up < 0 || up > time.Minute {
			t.Fatalf("not this daemon's uptime: %q", got.Up)
		}
	}
	for _, path := range []string{"/status", "/users", "/ls", "/names", "/recent", "/activity", "/activity/days?from=260924&to=260924"} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 401 {
			t.Errorf("%s became public: %d", path, w.Code)
		}
	}
}

func TestUnwiredCallCounterIsUnavailable(t *testing.T) {
	s, _ := serverFor(t, core.New(), "owner@h")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/identity", nil))
	var got protocol.NodeIdentity
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Calls != nil {
		t.Fatal("unwired process counter invented zero calls")
	}
}

// A range answers one day per date with its 144 slots; a range the daemon does
// not read is a refusal, not an empty answer.
func TestActivityDaysAnswersARange(t *testing.T) {
	bus := core.New()
	s, tok := serverFor(t, bus, "owner@h")
	bus.SetDaemonOwner("owner@h")
	get := func(q string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/activity/days?"+q, nil)
		r.Header.Set("X-Agent-Bus-Token", tok("owner@h"))
		s.Handler().ServeHTTP(w, r)
		return w
	}
	w := get("from=260922&to=260924")
	var days []protocol.ActivityDay
	if err := json.Unmarshal(w.Body.Bytes(), &days); err != nil || w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if len(days) != 3 || days[0].Day != 260922 || days[2].Day != 260924 || len(days[1].Slots) != 144 {
		t.Fatalf("range: %d days, first %d", len(days), days[0].Day)
	}
	for _, bad := range []string{"from=260924&to=260922", "from=x&to=260924", "from=260101&to=260924", "from=260931&to=260931"} {
		if w := get(bad); w.Code != 400 {
			t.Errorf("%s answered %d", bad, w.Code)
		}
	}
	if w := get("name=%23nobody@h&from=260924&to=260924"); w.Code != 404 {
		t.Errorf("an unknown name answered %d", w.Code)
	}
}

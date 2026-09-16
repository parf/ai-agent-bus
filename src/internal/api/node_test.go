package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/version"
)

func TestPublicNodeIdentityIsOnlyThePublishedFacts(t *testing.T) {
	b := core.New()
	s, _ := serverFor(t, b, "owner@h")
	if _, err := b.Register(protocol.Record{Name: "private-inbox@h", Owner: "owner@h", Allow: []string{"owner@h"}}); err != nil {
		t.Fatal(err)
	}
	b.SampleActivity(time.Now().Add(-2 * time.Minute))
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "private-inbox@h", Body: "secret body"}); err != nil {
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
		if len(fields) != 7 || got.Owner != "owner@h" || got.Version != version.String || got.Build != version.Build || got.Host != host || len(got.Messages) != 3 {
			t.Fatalf("public projection differs: %v", got)
		}
		for _, window := range got.Messages {
			if !window.Available || window.Accepted != 1 || window.Dequeued != 0 {
				t.Fatalf("public message totals not wired to daemon traffic: %+v", window)
			}
		}
		if strings.Contains(w.Body.String(), "private-inbox") || strings.Contains(w.Body.String(), "secret body") {
			t.Fatal("public total leaked contributing records or bodies")
		}
		if up, err := time.ParseDuration(got.Up); err != nil || up < 0 || up > time.Minute {
			t.Fatalf("not this daemon's uptime: %q", got.Up)
		}
	}
	for _, path := range []string{"/status", "/users", "/ls", "/names", "/recent", "/activity"} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 401 {
			t.Errorf("%s became public: %d", path, w.Code)
		}
	}
}

func TestHostLoadDistinguishesZeroFromUnavailable(t *testing.T) {
	for _, raw := range []string{"", "1 2", "NaN 0 0", "0 +Inf 0", "0 0 -1", "not 0 0"} {
		if parseLoad([]byte(raw)) != nil {
			t.Errorf("invented host load from %q", raw)
		}
	}
	got := parseLoad([]byte("0.00 1.25 2.50 1/50 1234\n"))
	if got == nil || *got != [3]float64{0, 1.25, 2.5} {
		t.Fatalf("lost real load: %v", got)
	}
}

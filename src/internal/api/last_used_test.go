package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// The listing says when each name's credential last authenticated a call,
// and puts the most recently used first, so a session someone is in reads
// apart from one left behind (docs/05-discovery.md#what-a-listing-answers).
func TestListingIsMostRecentlyUsedFirst(t *testing.T) {
	b := core.New()
	s, token := serverFor(t, b, "owner@h")
	known(t, b, "#0@h", "#a@h", "#b@h")
	h := s.Handler()
	call := func(tok, path string) []byte {
		t.Helper()
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set(HeaderToken, tok)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("GET %s: HTTP %d", path, w.Code)
		}
		return w.Body.Bytes()
	}
	// By name the order would be #0, #a, #b; by use it is the reverse.
	ta, tb, owner := token("#a@h"), token("#b@h"), token("owner@h")
	_ = token("#0@h") // issued, never used
	call(ta, "/status")
	time.Sleep(2 * time.Millisecond)
	call(tb, "/status")
	time.Sleep(2 * time.Millisecond)
	var recs []protocol.Record
	if err := json.Unmarshal(call(owner, "/ls?kind=agent"), &recs); err != nil {
		t.Fatal(err)
	}
	var order []string
	at := map[string]*time.Time{}
	for _, r := range recs {
		order = append(order, r.Name)
		at[r.Name] = r.LastUsed
	}
	if len(order) != 3 || order[0] != "#b@h" || order[1] != "#a@h" || order[2] != "#0@h" {
		t.Fatalf("order %v, want #b@h (used last), #a@h, then #0@h (never used)", order)
	}
	if at["#a@h"] == nil || at["#b@h"] == nil || !at["#b@h"].After(*at["#a@h"]) {
		t.Fatalf("last_used a=%v b=%v, want both set and b after a", at["#a@h"], at["#b@h"])
	}
	if at["#0@h"] != nil {
		t.Fatalf("a credential never used has last_used %v", at["#0@h"])
	}
	// And it is never stored: a registration carrying it keeps none.
	stamp := time.Unix(1, 0)
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#0@h", Owner: "owner@h", LastUsed: &stamp}); err != nil {
		t.Fatal(err)
	}
	for _, r := range b.List("owner@h", "agent") {
		if r.LastUsed != nil {
			t.Fatalf("%s kept a last_used through the core: %v", r.Name, r.LastUsed)
		}
	}
}

package api

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// GET /ls?tag= keeps the records carrying one label, the label normalised as
// a stored one is; `no-tags` is the records with no label, `personal` the
// personal ones, and a value that normalises to nothing matches nothing
// rather than lifting the filter (docs/03-records.md#tags).
func TestListingFiltersByTag(t *testing.T) {
	bus := core.New()
	s, tok := serverFor(t, bus, "owner@h")
	if _, err := bus.SetUser("owner@h", protocol.User{Name: "alice@h"}, true); err != nil {
		t.Fatal(err)
	}
	for _, r := range []protocol.Record{
		{Name: "tagged@h", Kind: protocol.KindQueue, Owner: "alice@h", Tags: []string{"build-notes"}},
		{Name: "other@h", Kind: protocol.KindQueue, Owner: "alice@h", Tags: []string{"runner"}},
		{Name: "bare@h", Kind: protocol.KindQueue, Owner: "alice@h"},
	} {
		if _, err := bus.Register(r); err != nil {
			t.Fatal(err)
		}
	}
	alice := tok("alice@h")
	list := func(query string) []string {
		t.Helper()
		code, body := send(s, alice, "GET", "/ls"+query, "", "")
		if code != http.StatusOK {
			t.Fatalf("GET /ls%s: %d %s", query, code, body)
		}
		var recs []protocol.Record
		if err := json.Unmarshal([]byte(body), &recs); err != nil {
			t.Fatalf("GET /ls%s: %v %s", query, err, body)
		}
		var names []string
		for _, r := range recs {
			if r.Name == "tagged@h" || r.Name == "other@h" || r.Name == "bare@h" || r.Name == "alice@h" {
				names = append(names, r.Name)
			}
		}
		slices.Sort(names)
		return names
	}
	for query, want := range map[string][]string{
		// The control: unfiltered, all four are there to be filtered.
		"":                 {"alice@h", "bare@h", "other@h", "tagged@h"},
		"?tag=build-notes": {"tagged@h"},
		"?tag=BUILD-NOTES": {"tagged@h"},
		"?tag=no-tags":     {"bare@h"},
		"?tag=personal":    {"alice@h"},
		"?tag=123":         nil,
	} {
		if got := list(query); !slices.Equal(got, want) {
			t.Errorf("GET /ls%s listed %v, want %v", query, got, want)
		}
	}
}

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

// A comma-delimited tag query keeps only the records carrying every label
// listed, in any order; one label is the single-tag request, an unknown label
// leaves nothing, and `no-tags` beside a real label is refused
// (docs/03-records.md#tags).
func TestListingFiltersByEveryListedTag(t *testing.T) {
	bus := core.New()
	s, tok := serverFor(t, bus, "owner@h")
	if _, err := bus.SetUser("owner@h", protocol.User{Name: "alice@h"}, true); err != nil {
		t.Fatal(err)
	}
	for _, r := range []protocol.Record{
		{Name: "both@h", Kind: protocol.KindQueue, Owner: "alice@h", Tags: []string{"build", "ops"}},
		{Name: "three@h", Kind: protocol.KindQueue, Owner: "alice@h", Tags: []string{"build", "ops", "web"}},
		{Name: "buildonly@h", Kind: protocol.KindQueue, Owner: "alice@h", Tags: []string{"build"}},
		{Name: "opsonly@h", Kind: protocol.KindQueue, Owner: "alice@h", Tags: []string{"ops"}},
		{Name: "bare@h", Kind: protocol.KindQueue, Owner: "alice@h"},
	} {
		if _, err := bus.Register(r); err != nil {
			t.Fatal(err)
		}
	}
	alice := tok("alice@h")
	list := func(query string) (int, []string) {
		t.Helper()
		code, body := send(s, alice, "GET", "/ls"+query, "", "")
		if code != http.StatusOK {
			return code, nil
		}
		var recs []protocol.Record
		if err := json.Unmarshal([]byte(body), &recs); err != nil {
			t.Fatalf("GET /ls%s: %v %s", query, err, body)
		}
		var names []string
		for _, r := range recs {
			if r.Name != "alice@h" {
				names = append(names, r.Name)
			}
		}
		slices.Sort(names)
		return code, names
	}
	for query, want := range map[string][]string{
		"?tag=build,ops":       {"both@h", "three@h"},
		"?tag=ops,build":       {"both@h", "three@h"},
		"?tag=OPS,Build":       {"both@h", "three@h"},
		"?tag=build,ops,web":   {"three@h"},
		"?tag=build,ops,ops":   {"both@h", "three@h"},
		"?tag=build":           {"both@h", "buildonly@h", "three@h"},
		"?tag=build,nosuchtag": nil,
		"?tag=no-tags":         {"bare@h"},
		"?tag=no-tags,no-tags": {"bare@h"},
	} {
		code, got := list(query)
		if code != http.StatusOK || !slices.Equal(got, want) {
			t.Errorf("GET /ls%s: %d, listed %v, want %v", query, code, got, want)
		}
	}
	for _, query := range []string{"?tag=no-tags,build", "?tag=build,no-tags"} {
		if code, _ := list(query); code != http.StatusBadRequest {
			t.Errorf("GET /ls%s: %d, want 400: no-tags cannot be combined with a tag", query, code)
		}
	}
}

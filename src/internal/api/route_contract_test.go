package api

import (
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

// The route inventory of docs/09-daemon-api.md#every-route-and-its-class is the
// contract's index: each route the daemon registers has one row, stable (its
// fields and codes are in the client contract) or unstable and admin-only.
// A new route without a row, or a row for a route that is gone, fails here.

var registered = regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+) (/[^"]*)"`)
var row = regexp.MustCompile("(?m)^\\| `([A-Z]+ /[^`]*)` \\| (stable|unstable) \\| (.+) \\|$")

func registeredRoutes(t *testing.T) []string {
	t.Helper()
	src, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, m := range registered.FindAllStringSubmatch(string(src), -1) {
		out = append(out, m[1]+" "+m[2])
	}
	if len(out) < 50 {
		t.Fatalf("found only %d registered routes; the pattern no longer reads server.go", len(out))
	}
	sort.Strings(out)
	return out
}

func contractRows(t *testing.T, doc string) map[string][2]string {
	t.Helper()
	i := strings.Index(doc, "### Every route and its class")
	j := strings.Index(doc, "## The client contract")
	if i < 0 || j < i {
		t.Fatal("docs/09-daemon-api.md has no route inventory before the client contract")
	}
	rows := map[string][2]string{}
	for _, m := range row.FindAllStringSubmatch(doc[i:j], -1) {
		if _, dup := rows[m[1]]; dup {
			t.Errorf("route %s has two rows", m[1])
		}
		rows[m[1]] = [2]string{m[2], m[3]}
	}
	return rows
}

func TestEveryRegisteredRouteHasAContractRow(t *testing.T) {
	doc, err := os.ReadFile("../../../docs/09-daemon-api.md")
	if err != nil {
		t.Fatal(err)
	}
	rows := contractRows(t, string(doc))
	routes := registeredRoutes(t)
	for _, r := range routes {
		got, ok := rows[r]
		if !ok {
			t.Errorf("registered route %s has no row in docs/09-daemon-api.md#every-route-and-its-class: add it as stable (with its contract section) or unstable", r)
			continue
		}
		if got[0] == "stable" && !strings.Contains(got[1], "](") {
			t.Errorf("stable route %s names no contract section", r)
		}
	}
	for r := range rows {
		if !slices.Contains(routes, r) {
			t.Errorf("docs/09-daemon-api.md lists %s, which the daemon does not register", r)
		}
	}
}

// A stable route is documented in the client contract itself: its path appears
// there, not only in the inventory.
func TestEveryStableRoutePathIsInTheClientContract(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/09-daemon-api.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	contract := doc[strings.Index(doc, "## The client contract"):]
	if end := strings.Index(contract, "## Client capability matrix"); end > 0 {
		contract = contract[:end]
	}
	for route, got := range contractRows(t, doc) {
		if got[0] != "stable" || strings.Contains(got[1], "05-discovery.md") {
			continue // /lookup and /ls are specified by the discovery page
		}
		path := route[strings.Index(route, " ")+1:]
		if !strings.Contains(contract, path) {
			t.Errorf("stable route %s is not described in the client contract", route)
		}
	}
}

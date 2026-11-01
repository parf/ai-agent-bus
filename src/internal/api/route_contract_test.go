package api

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

var row = regexp.MustCompile("(?m)^\\| `([A-Z]+ /[^`]*)` \\| (stable|unstable) \\| (.+) \\|$")

// registration finds every call that adds a handler to a mux: Handle or HandleFunc.
var registration = regexp.MustCompile(`\.(Handle|HandleFunc)\(\s*`)
var routeLiteral = regexp.MustCompile(`^"([A-Z]+) (/[^"]*)"`)

// scanRoutes reads one source file for routes. A registration it cannot read as
// HandleFunc("METHOD /path", ...) is a problem, never a skipped line: a new
// registration form must go red here so the inventory cannot silently miss it.
func scanRoutes(file string, src string) (routes, problems []string) {
	for _, at := range registration.FindAllStringSubmatchIndex(src, -1) {
		line := strings.Count(src[:at[0]], "\n") + 1
		call := src[at[2]:at[3]]
		rest := src[at[1]:]
		m := routeLiteral.FindStringSubmatch(rest)
		switch {
		case call == "Handle":
			problems = append(problems, fmt.Sprintf("%s:%d registers with Handle: use HandleFunc(\"METHOD /path\", ...) so the route can be read", file, line))
		case m == nil && strings.HasPrefix(rest, `"/`):
			problems = append(problems, fmt.Sprintf("%s:%d registers a route with no method: write \"METHOD /path\"", file, line))
		case m == nil:
			problems = append(problems, fmt.Sprintf("%s:%d registers a route whose first argument is not a \"METHOD /path\" literal", file, line))
		default:
			routes = append(routes, m[1]+" "+m[2])
		}
	}
	return routes, problems
}

// registeredRoutes reads every non-test Go file under src, since a route may be
// registered outside package api.
func registeredRoutes(t *testing.T) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir("../..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "testdata" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		routes, problems := scanRoutes(path, string(src))
		for _, p := range problems {
			t.Error(p)
		}
		out = append(out, routes...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 50 {
		t.Fatalf("found only %d registered routes; the scan no longer reads the sources", len(out))
	}
	sort.Strings(out)
	return out
}

// whole says whether text holds path as a whole path: not the front of a longer
// one (/lock inside /lock/extend) nor the tail of one.
func whole(text, path string) bool {
	return regexp.MustCompile(`(^|[^A-Za-z0-9_/-])` + regexp.QuoteMeta(path) + `($|[^A-Za-z0-9_/-])`).MatchString(text)
}

func TestScanRoutesReadsOnlyLiteralMethodRoutes(t *testing.T) {
	routes, problems := scanRoutes("x.go", "mux.HandleFunc(\"GET /a\", f)\nmux.HandleFunc(\n\t\"POST /b/c\", g)\n")
	if !slices.Equal(routes, []string{"GET /a", "POST /b/c"}) || len(problems) != 0 {
		t.Fatalf("routes %v, problems %v", routes, problems)
	}
	for name, src := range map[string]string{
		"Handle":       `mux.Handle("GET /a", h)`,
		"no method":    `mux.HandleFunc("/a", f)`,
		"a variable":   `mux.HandleFunc(pattern, f)`,
		"lower method": `mux.HandleFunc("get /a", f)`,
		"http.Handle":  `http.Handle("GET /a", h)`,
	} {
		routes, problems := scanRoutes("x.go", src)
		if len(problems) != 1 || len(routes) != 0 {
			t.Errorf("%s: routes %v, problems %v: want exactly one problem and no route", name, routes, problems)
		}
	}
}

func TestWholePathIsNotAPrefixOrSuffix(t *testing.T) {
	for text, want := range map[string]bool{
		"`POST /lock` takes it":  true,
		"/lock":                  true,
		"/lock.":                 true,
		"only /lock/extend here": false,
		"only /try-lock here":    false,
		"a/lock":                 false,
		"/lock-more":             false,
		"see /lock and /lock/x":  true,
	} {
		if got := whole(text, "/lock"); got != want {
			t.Errorf("whole(%q, /lock) = %v, want %v", text, got, want)
		}
	}
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
		if !whole(contract, path) {
			t.Errorf("stable route %s is not described in the client contract", route)
		}
	}
}

package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// Discover the actual registrations, including registrations added in a new
// source file. An unclassified new route fails instead of escaping the table.
func registeredAPIRoutes(t *testing.T) []string {
	t.Helper()
	pkgs, err := parser.ParseDir(token.NewFileSet(), ".", func(f fs.FileInfo) bool { return !strings.HasSuffix(f.Name(), "_test.go") }, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				method, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || method.Sel.Name != "HandleFunc" {
					return true
				}
				literal, ok := call.Args[0].(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					t.Fatal("classify dynamic API route before registering it")
					return false
				}
				route, err := strconv.Unquote(literal.Value)
				if err != nil {
					t.Fatal(err)
				}
				seen[route] = true
				return true
			})
		}
	}
	routes := make([]string, 0, len(seen))
	for route := range seen {
		routes = append(routes, route)
	}
	sort.Strings(routes)
	if len(routes) == 0 {
		t.Fatal("no registered API routes found")
	}
	return routes
}

func TestDaemonAgentReservationAcrossRegisteredAPIRoutes(t *testing.T) {
	// Explicit exceptions are public reads or operations on the authenticated
	// connection/node, not on a target record. New routes have no implicit skip.
	nonRecord := map[string]string{
		"GET /{$}": "dashboard redirect", "GET /identity": "public node identity", "GET /status": "caller-visible status",
		"GET /accounts": "node-wide account configuration", "GET /groups": "caller-visible group listing", "GET /users": "caller-visible user listing",
		"GET /activity": "caller-visible activity", "GET /activity/days": "caller-visible activity", "GET /ls": "caller-visible records",
		"GET /inactive": "caller-visible inactive records", "GET /lookup": "public record metadata", "GET /manages": "public authority query",
		"GET /recent": "caller-visible traffic", "GET /names": "caller-visible names", "GET /may-send": "read-only target admission",
		"GET /debug": "node-wide diagnostic setting", "POST /debug": "node-wide diagnostic setting", "POST /profile": "caller's own email",
		"POST /send": "ordinary messages to #bus are allowed", "POST /session": "caller's browser session", "DELETE /session": "caller's browser session",
	}
	requests := map[string]string{
		"POST /records/export": `{"include":["name:#bus"],"download":true}`,
		"POST /records/import": `{"data":{"format":1,"records":[{"name":"#bus","kind":"agent","owner":"owner@h"}]}}`,
		"POST /register":       `{"name":"#bus","kind":"agent","descr":"takeover"}`,
		"POST /unregister":     `{"name":"#bus"}`,
		"POST /manage":         `{"name":"#bus","descr":"takeover"}`,
		"POST /owner":          `{"owner":"#bus"}`,
		"POST /account":        `{"account":"fixture","principal":"#bus"}`,
		"POST /lock":           `{"record":"#bus","name":"probe","ttl":"1m"}`,
		"POST /try-lock":       `{"record":"#bus","name":"probe","ttl":"1m"}`,
		"POST /release":        `{"record":"#bus","name":"probe"}`,
		"POST /release-force":  `{"record":"#bus","name":"probe"}`,
		"POST /extend":         `{"record":"#bus","name":"probe","ttl":"1m"}`,
		"GET /holders":         "record=%23bus",
		"GET /kv":              "record=%23bus&name=probe", "GET /kv/list": "record=%23bus",
		"POST /kv/set":              `{"record":"#bus","name":"probe","kind":"string","value":"takeover"}`,
		"POST /kv/delete":           `{"record":"#bus","name":"probe"}`,
		"POST /kv/inc":              `{"record":"#bus","name":"probe","n":1}`,
		"POST /kv/json":             `{"record":"#bus","name":"probe","ops":[{"op":"set","key":"probe","value":1}]}`,
		"POST /user":                `{"name":"#bus","create":true,"person_name":"takeover"}`,
		"POST /user/github-refresh": `{"name":"#bus"}`,
		"POST /user/state":          `{"name":"#bus","status":"inactive"}`,
		"POST /user/key":            `{"name":"#bus","ssh_key":"ssh-ed25519 invalid"}`,
		"POST /identity/remove":     `{"name":"#bus"}`,
		"POST /group":               `{"name":"#bus","members":[]}`,
		"POST /subscribe":           `{"channel":"#bus","off":true}`,
		"POST /deliver-to/remove":   `{"channel":"#bus","recipient":"ordinary@h"}`,
		"POST /subscriber/remove":   `{"channel":"#bus","subscriber":"ordinary@h"}`,
		"POST /configure":           `{"name":"#bus","config":{"takeover":true}}`,
		"GET /config":               "name=%23bus", "POST /secret": `{"name":"#bus","secret":"TOKEN=takeover"}`, "GET /secret": "name=%23bus",
		"GET /consume": "inbox=%23bus&wait=0s",
		"POST /token":  `{"name":"#bus"}`,
		"POST /enrol":  `{"name":"#bus"}`,
	}
	for _, route := range registeredAPIRoutes(t) {
		if nonRecord[route] != "" {
			continue
		}
		payload, known := requests[route]
		if !known {
			t.Errorf("new route %s needs a #bus reservation case or an explicit non-record reason", route)
			continue
		}
		for _, who := range []string{"owner@h", "administrator@h", "ordinary@h"} {
			t.Run(who+route, func(t *testing.T) {
				b := core.New()
				s, token := serverFor(t, b, "owner@h")
				b.Persistence(memory.NewState())
				for _, name := range []string{"administrator@h", "ordinary@h"} {
					if _, err := b.SetUser("owner@h", protocol.User{Name: name}, true); err != nil {
						t.Fatal(err)
					}
				}
				if err := b.SetGroup("owner@h", core.AdministratorsGroup, []string{"owner@h", "administrator@h"}); err != nil {
					t.Fatal(err)
				}
				if err := b.EstablishDaemonAgent("node.example"); err != nil {
					t.Fatal(err)
				}
				s.LocalAccounts("protected", func(string) error { return nil })
				method, path, _ := strings.Cut(route, " ")
				body := payload
				if method == "GET" {
					path += "?" + payload
					body = ""
				}
				r := httptest.NewRequest(method, path, strings.NewReader(body))
				r.Header.Set(HeaderToken, token(who))
				w := httptest.NewRecorder()
				s.Handler().ServeHTTP(w, r)
				if who == "owner@h" && route == "GET /config" {
					// The bus setup is the daemon Owner's to read (TestBusSetupOverHTTP).
					if w.Code != 200 {
						t.Fatalf("daemon Owner cannot read the bus setup: %d %s", w.Code, w.Body.String())
					}
					return
				}
				if w.Code < 400 || w.Code >= 500 {
					t.Fatalf("#bus not refused: %s as %s: %d %s", route, who, w.Code, w.Body.String())
				}
				rec, ok := b.Lookup(who, core.DaemonAgent)
				if !ok || rec.Owner != "owner@h" || rec.Descr != "The daemon on the bus: help, status, version and ls" || rec.CanManage {
					t.Fatalf("reservation changed: %+v", rec)
				}
			})
		}
	}
}

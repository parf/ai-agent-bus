package api

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// recorder is a journal that keeps what it was given.
type recorder struct {
	mu       sync.Mutex
	audit    []ports.AuditEntry
	reports  []string
	requests []ports.RequestLine
	debug    bool
}

func (r *recorder) Audit(e ports.AuditEntry) { r.mu.Lock(); r.audit = append(r.audit, e); r.mu.Unlock() }
func (r *recorder) Report(s ports.Severity, m string) {
	r.mu.Lock()
	r.reports = append(r.reports, s.String()+" "+m)
	r.mu.Unlock()
}
// Request keeps every line it is handed, on or off: whether the API asks
// while the log is off is what the tests check.
func (r *recorder) Request(l ports.RequestLine) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, l)
}
func (r *recorder) SetDebug(on bool) error { r.mu.Lock(); r.debug = on; r.mu.Unlock(); return nil }
func (r *recorder) DebugOn() bool         { r.mu.Lock(); defer r.mu.Unlock(); return r.debug }

func journalFixture(t *testing.T) (*Server, func(string) string, *recorder) {
	t.Helper()
	bus := core.New()
	s, tok := serverFor(t, bus, "admin@h")
	for _, who := range []string{"alice@h", "bob@h"} {
		if _, err := bus.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	rec := &recorder{}
	s.Journal(rec)
	return s, tok, rec
}

func send(s *Server, token, method, path, body, remote string) (int, string) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(HeaderToken, token)
	// A unix-socket request carries no address; httptest's default is a TCP one.
	req.RemoteAddr = "@"
	if remote != "" {
		req.RemoteAddr = remote
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

const (
	canaryConfig = "CANARY-CONFIG-9b1e"
	canarySecret = "CANARY-SECRET-77f0"
	canaryBody   = "CANARY-BODY-3c2a"
)

// Every administrative action and entity edit writes one entry naming who
// did what to which name with what outcome; ordinary traffic writes none,
// and no value the caller sent is in any of it.
func TestAuditEntriesForEditsAndNoneForTraffic(t *testing.T) {
	s, tok, rec := journalFixture(t)
	alice := tok("alice@h")
	calls := []struct{ path, body, op, target, result string }{
		{"/register", `{"kind":"agent","name":"svc@h","allow":["bob@h"]}`, "register", "svc@h", "ok"},
		{"/manage", `{"name":"svc@h","descr":"edited"}`, "manage", "svc@h", "ok"},
		{"/configure", `{"name":"svc@h","config":{"k":"` + canaryConfig + `"}}`, "configure", "svc@h", "ok"},
		{"/register", `{"name":"db@h","addr":"db:5432","protocol":"postgresql"}`, "register", "db@h", "ok"},
		{"/secret", `{"name":"db@h","secret":"PW=` + canarySecret + `"}`, "set-secret", "db@h", "ok"},
		{"/user/state", `{"name":"bob@h","state":"paused"}`, "user-state", "bob@h", "refused 403"},
	}
	for _, c := range calls {
		send(s, alice, "POST", c.path, c.body, "192.0.2.7:4000")
	}
	// Traffic and credentials: no entry.
	send(s, alice, "POST", "/send", `{"to":"svc@h","body":"`+canaryBody+`"}`, "")
	send(s, tok("svc@h"), "GET", "/consume?wait=0s", "", "")
	send(s, alice, "POST", "/token", `{"name":"alice@h"}`, "")
	send(s, alice, "GET", "/ls", "", "")
	if len(rec.audit) != len(calls) {
		t.Fatalf("%d entries for %d edits: %+v", len(rec.audit), len(calls), rec.audit)
	}
	for i, c := range calls {
		e := rec.audit[i]
		if e.Actor != "alice@h" || e.Operation != c.op || e.Target != c.target || e.Result != c.result || e.ClientIP != "192.0.2.7" {
			t.Errorf("entry %d is %+v, want %s %s %s", i, e, c.op, c.target, c.result)
		}
	}
	all := fmt.Sprintf("%+v %+v", rec.audit, rec.requests)
	for _, canary := range []string{canaryConfig, canarySecret, canaryBody, alice} {
		if strings.Contains(all, canary) {
			t.Fatalf("a caller's value reached the audit log: %s", canary)
		}
	}
}

// A status change is an edit like any other, so suspending somebody is never
// silent — and nor is lifting it.
func TestSuspensionAndReactivationAreAudited(t *testing.T) {
	s, tok, rec := journalFixture(t)
	admin := tok("admin@h")
	send(s, admin, "POST", "/user/state", `{"name":"bob@h","state":"paused"}`, "")
	send(s, admin, "POST", "/user/state", `{"name":"bob@h","state":"active"}`, "")
	if len(rec.audit) != 2 || rec.audit[0].Operation != "user-state" || rec.audit[1].Result != "ok" {
		t.Fatalf("%+v", rec.audit)
	}
	// A unix-socket call has no address, and none is invented.
	if rec.audit[0].ClientIP != "" {
		t.Fatalf("invented %q", rec.audit[0].ClientIP)
	}
}

// Only the daemon Owner switches the debug log, and it writes a line per
// request only while it is on.
func TestDebugLogIsTheOwnersAndOnDemand(t *testing.T) {
	s, tok, rec := journalFixture(t)
	send(s, tok("alice@h"), "GET", "/status", "", "")
	if len(rec.requests) != 0 {
		t.Fatal("a request line was written with the debug log off")
	}
	if code, _ := send(s, tok("alice@h"), "POST", "/debug", `{"on":true}`, ""); code != 403 {
		t.Fatalf("a non-owner switching the debug log answered %d", code)
	}
	if rec.DebugOn() {
		t.Fatal("a refused switch changed the state")
	}
	if code, body := send(s, tok("admin@h"), "POST", "/debug", `{"on":true}`, ""); code != 200 || !strings.Contains(body, `"on":true`) {
		t.Fatalf("owner switch answered %d %s", code, body)
	}
	send(s, tok("alice@h"), "POST", "/send", `{"to":"nobody@h","body":"`+canaryBody+`"}`, "")
	// The switch that turned it on is the first line; the send is the second.
	if len(rec.requests) != 2 || rec.requests[1].Caller != "alice@h" || rec.requests[1].Path != "/send" || rec.requests[1].Status != 404 {
		t.Fatalf("request lines %+v", rec.requests)
	}
	if strings.Contains(fmt.Sprintf("%+v", rec.requests), canaryBody) {
		t.Fatal("a message body reached the debug log")
	}
	// Switching it is itself an administrative action.
	last := rec.audit[len(rec.audit)-1]
	if last.Operation != "debug-log" || last.Actor != "admin@h" {
		t.Fatalf("the switch was not audited: %+v", rec.audit)
	}
}

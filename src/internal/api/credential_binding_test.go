package api

import (
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/sqlite"
)

// A credential names a User, and an agent's also names the Agent whose Owner
// that User is (docs/02-access.md#what-a-call-carries). These run against the
// database the daemon uses, and restart it, because "in the ownership change's
// own commit" is a claim about what a restart reads back.

type alerts struct {
	mu    sync.Mutex
	lines []string
}

func (a *alerts) Audit(ports.AuditEntry) {}
func (a *alerts) Report(s ports.Severity, m string) {
	a.mu.Lock()
	a.lines = append(a.lines, s.String()+": "+m)
	a.mu.Unlock()
}
func (a *alerts) Request(ports.RequestLine) {}
func (a *alerts) SetDebug(bool) error       { return nil }
func (a *alerts) DebugOn() bool             { return false }
func (a *alerts) has(sub string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, l := range a.lines {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}
func (a *alerts) all() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return strings.Join(a.lines, "\n")
}

type node struct {
	t      *testing.T
	path   string
	st     *sqlite.Store
	bus    *core.Bus
	tokens *auth.Tokens
	srv    *Server
	log    *alerts
}

func startNode(t *testing.T, path string) *node {
	t.Helper()
	st, err := sqlite.Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	n := &node{t: t, path: path, st: st, bus: core.New(), log: &alerts{}}
	n.bus.Journal(n.log)
	snap, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	n.bus.Restore(snap)
	n.bus.Persistence(st)
	if err := n.bus.EstablishDaemonOwner("admin@h"); err != nil {
		t.Fatal(err)
	}
	if n.tokens, err = auth.Load(st.Tokens(), "admin@h"); err != nil {
		t.Fatal(err)
	}
	n.srv = New(n.bus, n.tokens, "admin@h")
	return n
}

func (n *node) restart() *node {
	n.t.Helper()
	n.st.Close()
	return startNode(n.t, n.path)
}

func (n *node) code(token string) int {
	req := httptest.NewRequest("GET", "/status", nil)
	req.Header.Set(HeaderToken, token)
	w := httptest.NewRecorder()
	n.srv.Handler().ServeHTTP(w, req)
	return w.Code
}

func (n *node) issue(caller, name string) (string, error) {
	return n.bus.IssueFor(caller, name, n.tokens.IssuePair)
}

func (n *node) stored(name string) (ports.Credential, bool) {
	n.t.Helper()
	creds, err := n.st.Tokens().Load()
	if err != nil {
		n.t.Fatal(err)
	}
	for _, c := range creds {
		if c.Name == name {
			return c, true
		}
	}
	return ports.Credential{}, false
}

func newNode(t *testing.T) *node {
	n := startNode(t, filepath.Join(t.TempDir(), "bus.db"))
	for _, u := range []string{"alice@h", "bob@h"} {
		if _, err := n.bus.SetUser("admin@h", protocol.User{Name: u}, true); err != nil {
			t.Fatal(err)
		}
	}
	return n
}

// Only a User and an Agent speak on the bus, so only they hold credentials.
func TestOnlyUsersAndAgentsAreIssuedCredentials(t *testing.T) {
	n := newNode(t)
	for _, r := range []protocol.Record{
		{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "alice@h"},
		{Name: "news@h", Kind: protocol.KindPubSub, Owner: "alice@h"},
		{Name: "db@h", Kind: protocol.KindService, Owner: "alice@h", Addr: "db:5432", Proto: "postgresql"},
	} {
		if _, err := n.bus.Register(r); err != nil {
			t.Fatal(err)
		}
		if _, err := n.issue("alice@h", r.Name); !errors.Is(err, core.ErrKind) {
			t.Errorf("a %s was issued a credential: %v", r.Kind, err)
		}
		if _, held := n.stored(r.Name); held {
			t.Errorf("a refused %s credential was stored", r.Kind)
		}
	}
	if _, err := n.bus.Register(protocol.Record{Name: "#worker@h", Kind: protocol.KindAgent, Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	tok, err := n.issue("alice@h", "#worker@h")
	if err != nil || n.code(tok) != 200 {
		t.Fatalf("an agent's credential: %v, status %d", err, n.code(tok))
	}
	c, _ := n.stored("#worker@h")
	if c.UserID == 0 || c.AgentID == 0 {
		t.Fatalf("the agent's credential was stored without its pair: %+v", c.CredentialPair)
	}
}

// A transfer rebinds the agent's credential to the new Owner in the transfer's
// own commit: it keeps working, before and after a restart, and the stored
// row names the new Owner. A transfer whose commit fails moves neither.
func TestATransferRebindsTheAgentsCredentialInItsOwnCommit(t *testing.T) {
	n := newNode(t)
	if _, err := n.bus.Register(protocol.Record{Name: "#worker@h", Kind: protocol.KindAgent, Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	tok, err := n.issue("alice@h", "#worker@h")
	if err != nil {
		t.Fatal(err)
	}
	before, _ := n.stored("#worker@h")
	bobID, _ := n.bus.PairFor("bob@h")

	bob := "bob@h"
	if _, err := n.bus.Manage("alice@h", core.Management{Name: "#worker@h", Owner: &bob}); err != nil {
		t.Fatal(err)
	}
	if got := n.code(tok); got != 200 {
		t.Fatalf("the transferred agent's credential answers %d", got)
	}
	after, _ := n.stored("#worker@h")
	if after.UserID != bobID.UserID || after.AgentID != before.AgentID || after.Current != tok {
		t.Fatalf("the stored credential after the transfer is %+v, want user %d agent %d", after.CredentialPair, bobID.UserID, before.AgentID)
	}
	n = n.restart()
	if got := n.code(tok); got != 200 {
		t.Fatalf("after a restart the transferred agent's credential answers %d: %s", got, n.log.all())
	}
	if n.log.has("ignored") {
		t.Fatalf("a restart after a committed transfer ignored something: %s", n.log.all())
	}
}

func TestAFailedTransferKeepsTheOwnerAndTheCredential(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bus.db")
	n := startNode(t, path)
	for _, u := range []string{"alice@h", "bob@h"} {
		if _, err := n.bus.SetUser("admin@h", protocol.User{Name: u}, true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := n.bus.Register(protocol.Record{Name: "#worker@h", Kind: protocol.KindAgent, Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	tok, err := n.issue("alice@h", "#worker@h")
	if err != nil {
		t.Fatal(err)
	}
	before, _ := n.stored("#worker@h")
	// A store that refuses the commit: the transfer is abandoned whole.
	failing := &refusingStore{Store: n.st, err: errors.New("disk is full")}
	n.bus.Persistence(failing)
	bob := "bob@h"
	if _, err := n.bus.Manage("alice@h", core.Management{Name: "#worker@h", Owner: &bob}); err == nil {
		t.Fatal("a transfer whose commit failed was reported done")
	}
	if r, _ := n.bus.Lookup("alice@h", "#worker@h"); r.Owner != "alice@h" {
		t.Fatalf("a failed transfer moved the record to %s", r.Owner)
	}
	if got := n.code(tok); got != 200 {
		t.Fatalf("a failed transfer broke the old credential: %d", got)
	}
	if after, _ := n.stored("#worker@h"); after.CredentialPair != before.CredentialPair {
		t.Fatalf("a failed transfer rewrote the stored pair: %+v", after.CredentialPair)
	}
	n.bus.Persistence(n.st)
	n = n.restart()
	if got := n.code(tok); got != 200 {
		t.Fatalf("after a restart the old credential answers %d", got)
	}
}

type refusingStore struct {
	ports.Store
	err error
}

func (s *refusingStore) Commit(ports.Change) error { return s.err }

// Removing an agent removes its credential in the removal's own commit, and a
// name registered again under another User is answered for by nothing the old
// holder kept — before and after a restart.
func TestARecreatedAgentNameIsNotAnsweredForByTheOldCredential(t *testing.T) {
	n := newNode(t)
	if _, err := n.bus.Register(protocol.Record{Name: "#worker@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"bob@h"}}); err != nil {
		t.Fatal(err)
	}
	old, err := n.issue("alice@h", "#worker@h")
	if err != nil {
		t.Fatal(err)
	}
	if err := n.bus.Unregister("#worker@h", "alice@h"); err != nil {
		t.Fatal(err)
	}
	if _, held := n.stored("#worker@h"); held {
		t.Fatal("the removal's commit kept the credential row")
	}
	if _, err := n.bus.Register(protocol.Record{Name: "#worker@h", Kind: protocol.KindAgent, Owner: "bob@h"}); err != nil {
		t.Fatal(err)
	}
	if got := n.code(old); got != 401 {
		t.Fatalf("the old holder's credential answers %d for the recreated name", got)
	}
	fresh, err := n.issue("bob@h", "#worker@h")
	if err != nil || fresh == old || n.code(fresh) != 200 {
		t.Fatalf("the new holder's credential: %v, same as old %v", err, fresh == old)
	}
	n = n.restart()
	if got := n.code(old); got != 401 {
		t.Fatalf("after a restart the old holder's credential answers %d", got)
	}
	if got := n.code(fresh); got != 200 {
		t.Fatalf("after a restart the new holder's credential answers %d", got)
	}
}

// Removing a record takes every reference to it in the same write: ACL,
// Maintainer, Group member and Deliver-To.
func TestRemovingARecordTakesEveryReferenceToIt(t *testing.T) {
	n := newNode(t)
	for _, r := range []protocol.Record{
		{Name: "#gone@h", Kind: protocol.KindAgent, Owner: "alice@h"},
		{Name: "#peer@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"#gone@h", "bob@h"}, Maintainers: protocol.MaintainerList{"#gone@h"}},
		{Name: "news@h", Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"*"}},
	} {
		if _, err := n.bus.Register(r); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := n.bus.Manage("alice@h", core.Management{Name: "news@h", Subs: &[]string{"#gone@h", "#peer@h"}}); err != nil {
		t.Fatal(err)
	}
	if err := n.bus.SetGroup("admin@h", "@crew", []string{"#gone@h", "bob@h"}); err != nil {
		t.Fatal(err)
	}
	if err := n.bus.Unregister("#gone@h", "alice@h"); err != nil {
		t.Fatal(err)
	}
	check := func(n *node, when string) {
		peer, _ := n.bus.Lookup("alice@h", "#peer@h")
		news, _ := n.bus.Lookup("alice@h", "news@h")
		crew := n.bus.Groups("admin@h")["@crew"]
		for what, list := range map[string][]string{"allow": peer.Allow, "maintainers": peer.Maintainers, "deliver_to": news.Subs, "@crew": crew} {
			for _, x := range list {
				if x == "#gone@h" {
					t.Errorf("%s: the removed agent is still in %s: %v", when, what, list)
				}
			}
		}
		if len(peer.Allow) != 1 || len(news.Subs) != 1 || len(crew) != 1 {
			t.Errorf("%s: the removal took more than the one reference: allow %v subs %v crew %v", when, peer.Allow, news.Subs, crew)
		}
	}
	check(n, "before a restart")
	check(n.restart(), "after a restart")
}

// A stored pair that does not match ownership can only be left by a failed
// write: it is ignored, authenticates nothing, and is reported as an alert
// naming the User, the Agent and the current Owner — never the credential.
func TestAMismatchedStoredCredentialIsIgnoredAndReported(t *testing.T) {
	n := newNode(t)
	if _, err := n.bus.Register(protocol.Record{Name: "#worker@h", Kind: protocol.KindAgent, Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	tok, err := n.issue("alice@h", "#worker@h")
	if err != nil {
		t.Fatal(err)
	}
	c, _ := n.stored("#worker@h")
	bob, _ := n.bus.PairFor("bob@h")
	// The damage a lost write would leave: the row names bob, the record alice.
	c.UserID = bob.UserID
	if err := n.st.Tokens().Put(c); err != nil {
		t.Fatal(err)
	}
	n = n.restart()
	if got := n.code(tok); got != 401 {
		t.Fatalf("a mismatched credential answers %d", got)
	}
	log := n.log.all()
	if !n.log.has("alert: stored credential for #worker@h is ignored") || !strings.Contains(log, "bob@h") || !strings.Contains(log, "owned by alice@h") {
		t.Fatalf("the mismatch was not reported with User, Agent and Owner: %s", log)
	}
	if strings.Contains(log, tok) {
		t.Fatal("the report disclosed the credential")
	}
	// Ignored, not repaired: the row is untouched for an operator.
	if kept, held := n.stored("#worker@h"); !held || kept.UserID != bob.UserID {
		t.Fatalf("the ignored row was changed: %+v held %v", kept.CredentialPair, held)
	}
}

// A stored credential for a kind that holds none is ignored the same way.
func TestAStoredQueueCredentialIsIgnoredAndReported(t *testing.T) {
	n := newNode(t)
	if _, err := n.bus.Register(protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	if err := n.st.Tokens().Put(ports.Credential{Name: "jobs@h", Current: "0123456789abcdef", CredentialPair: ports.CredentialPair{UserID: 9}}); err != nil {
		t.Fatal(err)
	}
	n = n.restart()
	if got := n.code("0123456789abcdef"); got != 401 {
		t.Fatalf("a queue's stored credential answers %d", got)
	}
	if !n.log.has("alert: stored credential for jobs@h is ignored") {
		t.Fatalf("the queue credential was not reported: %s", n.log.all())
	}
}

// Last use is durable, written on the flush's cadence rather than per call.
func TestLastUseSurvivesARestart(t *testing.T) {
	n := newNode(t)
	tok, err := n.issue("alice@h", "alice@h")
	if err != nil {
		t.Fatal(err)
	}
	if got := n.code(tok); got != 200 {
		t.Fatal(got)
	}
	if c, _ := n.stored("alice@h"); !c.Used.IsZero() {
		t.Fatal("an authenticated call wrote to the store on its own")
	}
	if err := n.tokens.FlushUsed(); err != nil {
		t.Fatal(err)
	}
	c, _ := n.stored("alice@h")
	if c.Used.IsZero() {
		t.Fatal("the flush did not record the last use")
	}
	n = n.restart()
	held := n.tokens.Holds([]string{"alice@h"})
	if len(held) != 1 || !held[0].Used.Equal(c.Used) {
		t.Fatalf("the last use after a restart is %+v, want %v", held, c.Used)
	}
}

// Every call checks the pair against who the name is now, not only a start:
// a credential whose pair went stale in memory answers for nothing.
func TestEveryCallChecksThePairAgainstTheRegistry(t *testing.T) {
	n := newNode(t)
	if _, err := n.bus.Register(protocol.Record{Name: "#worker@h", Kind: protocol.KindAgent, Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	tok, err := n.issue("alice@h", "#worker@h")
	if err != nil || n.code(tok) != 200 {
		t.Fatalf("control: %v %d", err, n.code(tok))
	}
	bob, _ := n.bus.PairFor("bob@h")
	good, _ := n.stored("#worker@h")
	n.tokens.Rebind("#worker@h", ports.CredentialPair{UserID: bob.UserID, AgentID: good.AgentID})
	if got := n.code(tok); got != 401 {
		t.Fatalf("a credential naming the wrong User answers %d", got)
	}
}

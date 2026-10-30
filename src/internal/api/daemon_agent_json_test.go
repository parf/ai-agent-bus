package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// jsonFixture is a node with every record kind, one inactive owner's record,
// two callers who see different records, and readers that are not agents.
func jsonFixture(t *testing.T) (*core.Bus, *Server, func(string) string) {
	t.Helper()
	b := core.New()
	s, token := serverFor(t, b, "admin@h")
	for _, who := range []string{"alice@h", "bob@h", "gone@h"} {
		if _, err := b.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.EstablishDaemonAgent("node.example"); err != nil {
		t.Fatal(err)
	}
	for _, r := range []protocol.Record{
		{Name: "#worker@h", Kind: protocol.KindAgent, Owner: "alice@h", Tags: []string{"team"}, Allow: []string{"*"}, Descr: "does work"},
		{Name: "#hidden@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"alice@h"}},
		{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "alice@h", Allow: []string{"*"}},
		{Name: "news@h", Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"*"}},
		{Name: "#suspended@h", Kind: protocol.KindAgent, Owner: "gone@h", Allow: []string{"*"}},
	} {
		if _, err := b.Register(r); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.SetUserState("admin@h", "gone@h", "inactive"); err != nil {
		t.Fatal(err)
	}
	return b, s, token
}

func busAnswer(t *testing.T, s *Server, from, body string) string {
	t.Helper()
	got, err := s.daemonAnswer(protocol.Envelope{From: from, Body: body})
	if err != nil {
		t.Fatalf("%s %q: %v", from, body, err)
	}
	return got
}

func httpJSON(t *testing.T, s *Server, token func(string) string, who, path string) []byte {
	t.Helper()
	r := httptest.NewRequest("GET", path, nil)
	r.Header.Set(HeaderToken, token(who))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("%s %s: %d %s", who, path, w.Code, w.Body.String())
	}
	return w.Body.Bytes()
}

// The JSON body is the daemon's own result and nothing else: it parses
// directly (no preamble, no fence) and says what /status says, node-wide, with
// the Owner-only detail absent, not zero, for anyone else.
func TestBusStatusJSONIsTheDaemonsStatusForTheCaller(t *testing.T) {
	b, s, token := jsonFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Three outstanding reads on two names: two on one agent, one of them filtered.
	for _, wait := range []struct{ name, topic string }{{"#worker@h", ""}, {"#worker@h", "x"}, {"jobs@h", ""}} {
		go b.ConsumeAs(ctx, "alice@h", wait.name, wait.topic, "", wait.topic != "", wait.topic == "")
	}
	for end := time.Now().Add(3 * time.Second); b.Status().Waiting < 3; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(end) {
			t.Fatalf("only %d readers waiting", b.Status().Waiting)
		}
	}
	var admin, alice map[string]any
	for who, into := range map[string]*map[string]any{"admin@h": &admin, "alice@h": &alice} {
		body := busAnswer(t, s, who, "status --json")
		if !strings.HasPrefix(body, "{") || !strings.HasSuffix(body, "}") || strings.Contains(body, "```") {
			t.Fatalf("not bare JSON: %q", body)
		}
		if err := json.Unmarshal([]byte(body), into); err != nil {
			t.Fatal(err)
		}
		var direct map[string]any
		if err := json.Unmarshal(httpJSON(t, s, token, who, "/status"), &direct); err != nil {
			t.Fatal(err)
		}
		for _, volatile := range []string{"up"} {
			delete(direct, volatile)
			delete(*into, volatile)
		}
		if !reflect.DeepEqual(direct, *into) {
			t.Fatalf("%s: #bus status --json differs from /status:\n%v\n%v", who, *into, direct)
		}
	}
	// Node totals, not the caller's view: live records of every kind (the
	// suspended owner's agent is out), and readers as outstanding polls.
	for _, got := range []map[string]any{admin, alice} {
		if got["waiting"] != float64(3) {
			t.Fatalf("readers are outstanding polls, filtered ones included: %v", got["waiting"])
		}
		kinds := got["kinds"].(map[string]any)
		if kinds[protocol.KindAgent] != float64(2+1) || kinds[protocol.KindQueue] != float64(1) || kinds[protocol.KindPubSub] != float64(1) {
			t.Fatalf("records by kind: %v", kinds) // #worker, #hidden and the built-in #bus; not #suspended
		}
		total := 0.0
		for _, n := range kinds {
			total += n.(float64)
		}
		if got["records"] != total || got["records"] != float64(b.Status().Records) {
			t.Fatalf("records are the live records of every kind: %v, kinds add to %v, node says %d", got["records"], total, b.Status().Records)
		}
		if got["you"] == "" {
			t.Fatal("no caller named")
		}
	}
	if admin["records"] != alice["records"] {
		t.Fatalf("callers with different views must see the same node totals: %v vs %v", admin["records"], alice["records"])
	}
	if _, has := admin["owner_inactive"]; !has {
		t.Fatalf("the daemon Owner sees the owner-inactive detail: %v", admin)
	}
	if _, has := alice["owner_inactive"]; has {
		t.Fatalf("an ordinary caller must not see it, not even as zero: %v", alice["owner_inactive"])
	}
	if text := busAnswer(t, s, "alice@h", "status"); !strings.HasPrefix(text, "ok: ") || strings.HasPrefix(text, "{") {
		t.Fatalf("the default stays the RT23 text line: %q", text)
	}
}

func TestBusVersionJSONIsTheNodeIdentity(t *testing.T) {
	_, s, token := jsonFixture(t)
	var got, direct map[string]any
	if err := json.Unmarshal([]byte(busAnswer(t, s, "alice@h", "version --json")), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(httpJSON(t, s, token, "alice@h", "/identity"), &direct); err != nil {
		t.Fatal(err)
	}
	delete(got, "up")
	delete(direct, "up")
	if !reflect.DeepEqual(got, direct) || got["version"] == nil || got["build_info"] == nil {
		t.Fatalf("version --json differs from /identity:\n%v\n%v", got, direct)
	}
}

// ls --json is the list the HTTP call gives that caller, with [] when empty,
// and one record's lookup for a name; what the caller may not see is not there.
func TestBusLsJSONKeepsTheCallersVisibility(t *testing.T) {
	_, s, token := jsonFixture(t)
	names := func(raw []byte) []string {
		var rs []protocol.Record
		if err := json.Unmarshal(raw, &rs); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		out := []string{}
		for _, r := range rs {
			out = append(out, r.Name)
		}
		slices.Sort(out) // the order follows token use, which asking over HTTP changes
		return out
	}
	for _, tc := range []struct{ who, body, query string }{
		{"alice@h", "ls --all --json", "?"}, {"bob@h", "ls --all --json", "?"},
		{"bob@h", "ls --json --kind agent", "?kind=agent"}, {"alice@h", "ls --tag=team --json", "?tag=team"},
	} {
		got := names([]byte(busAnswer(t, s, tc.who, tc.body)))
		want := names(httpJSON(t, s, token, tc.who, "/ls"+tc.query))
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s %q: %v, want what /ls gives them: %v", tc.who, tc.body, got, want)
		}
	}
	alice, bob := names([]byte(busAnswer(t, s, "alice@h", "ls --all --json"))), names([]byte(busAnswer(t, s, "bob@h", "ls --all --json")))
	if !strings.Contains(strings.Join(alice, " "), "#hidden@h") || strings.Contains(strings.Join(bob, " "), "#hidden@h") {
		t.Fatalf("visibility differs per caller: alice %v, bob %v", alice, bob)
	}
	if got := busAnswer(t, s, "bob@h", "ls --tag=nothing-has-this --json"); got != "[]" {
		t.Fatalf("an empty list is [], got %q", got)
	}
	if got := busAnswer(t, s, "bob@h", "ls --kind service --json"); got != "[]" { // a kind with no record: no list at all, still []
		t.Fatalf("an empty list is [], got %q", got)
	}
	if got := busAnswer(t, s, "bob@h", "ls --tag=nothing-has-this"); got != "No matching records." {
		t.Fatalf("the text default stays: %q", got)
	}
	var one, direct protocol.Record
	if err := json.Unmarshal([]byte(busAnswer(t, s, "alice@h", "ls #worker@h --json")), &one); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(httpJSON(t, s, token, "alice@h", "/lookup?name=%23worker%40h"), &direct); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(one, direct) || one.Name != "#worker@h" {
		t.Fatalf("a name gives that record's lookup: %+v vs %+v", one, direct)
	}
	if _, err := s.daemonAnswer(protocol.Envelope{From: "bob@h", Body: "ls #hidden@h --json"}); err == nil {
		t.Fatal("a record the caller may not see was looked up")
	}
}

func TestBusJSONOptionIsOnlyOnTheReadMethods(t *testing.T) {
	_, s, _ := jsonFixture(t)
	for _, body := range []string{"help --json", "status --json extra", "version --yaml", "status --json --json"} {
		if got, err := s.daemonAnswer(protocol.Envelope{From: "alice@h", Body: body}); err == nil {
			t.Fatalf("%q was accepted: %s", body, got)
		}
	}
	var meta struct {
		Methods map[string]struct {
			Args []struct{ Name string } `json:"args"`
		} `json:"methods"`
		Formats []string `json:"body_formats"`
	}
	if err := json.Unmarshal([]byte(busAnswer2(t, s)), &meta); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"status", "version"} {
		if len(meta.Methods[method].Args) != 1 || meta.Methods[method].Args[0].Name != "json" {
			t.Fatalf("help-json does not declare --json on %s: %+v", method, meta.Methods[method])
		}
	}
	if !reflect.DeepEqual(meta.Formats, []string{"text"}) { // accepted request bodies, which --json does not add to
		t.Fatalf("body formats: %v", meta.Formats)
	}
	if help := busAnswer(t, s, "alice@h", "help"); !strings.Contains(help, "status [--json]") || !strings.Contains(help, "[--json]") {
		t.Fatalf("help leaves the option out: %s", help)
	}
}

func busAnswer2(t *testing.T, s *Server) string {
	t.Helper()
	got, err := s.daemonAnswer(protocol.Envelope{From: "alice@h", Ask: protocol.AskHelpJSON})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// Every record kind is counted, a busy handler is not a reader, and queued is
// the depth now while dropped and expired are totals since the start: draining
// a queue lowers the first and never the others.
func TestBusStatusJSONCountsKindsReadersAndCurrentVersusCumulative(t *testing.T) {
	b, s, _ := jsonFixture(t)
	for _, r := range []protocol.Record{
		{Name: "#tool@h", Kind: protocol.KindAgent, Owner: "alice@h", Tags: []string{"mcp"}, Allow: []string{"*"}},
		{Name: "feed@h", Kind: protocol.KindService, Owner: "alice@h", Allow: []string{"*"}, Addr: "127.0.0.1:9", Proto: "http"},
		{Name: "doc@h", Kind: protocol.KindResource, Owner: "alice@h", Allow: []string{"*"}, Resource: &protocol.Resource{URI: "https://example.com/a.md"}},
		{Name: "#ring@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Bound: 1, Full: protocol.OverflowRing},
		{Name: "#busy@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}},
	} {
		if _, err := b.Register(r); err != nil {
			t.Fatalf("%s: %v", r.Name, err)
		}
	}
	status := func() map[string]any {
		var got map[string]any
		if err := json.Unmarshal([]byte(busAnswer(t, s, "bob@h", "status --json")), &got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	kinds := func(got map[string]any) map[string]float64 {
		out := map[string]float64{}
		for k, v := range got["kinds"].(map[string]any) {
			out[k] = v.(float64)
		}
		return out
	}
	k := kinds(status())
	for kind, atLeast := range map[string]float64{protocol.KindAgent: 5, protocol.KindQueue: 1, protocol.KindPubSub: 1, protocol.KindService: 1, protocol.KindResource: 1, protocol.KindGroup: 1, protocol.KindUser: 1} {
		if k[kind] < atLeast {
			t.Fatalf("kind %s counted %v, want at least %v: %v", kind, k[kind], atLeast, k)
		}
	}
	if k[protocol.KindAgent] != 6 { // #worker, #hidden, #tool, #ring, #busy and the built-in #bus; #suspended's owner is inactive
		t.Fatalf("agents: %v", k)
	}
	// A busy handler: its read returned with a message and has not polled again, so it is not a reader.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got := make(chan error, 1)
	go func() { _, err := b.ConsumeAs(ctx, "alice@h", "#busy@h", "", "", false, false); got <- err }()
	for end := time.Now().Add(3 * time.Second); status()["waiting"] != float64(1); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(end) {
			t.Fatalf("the poll never showed: %v", status()["waiting"])
		}
	}
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "#busy@h", Body: "work"}); err != nil {
		t.Fatal(err)
	}
	if err := <-got; err != nil {
		t.Fatal(err)
	}
	if w := status()["waiting"]; w != float64(0) {
		t.Fatalf("a busy handler is not an outstanding poll: %v", w)
	}
	// Queued now, dropped and expired since the start.
	before := status()
	for _, body := range []string{"one", "two"} { // the ring holds one: the second drops the first
		if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "#ring@h", Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "jobs@h", Body: "late", TTL: "1ns"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "jobs@h", Body: "fresh"}); err != nil {
		t.Fatal(err)
	}
	gone, stop := context.WithCancel(context.Background())
	stop()
	b.ConsumeAs(gone, "alice@h", "jobs@h", "zz", "", true, false) // a read that matches nothing still purges the expired one
	mid := status()
	if mid["dropped"] != before["dropped"].(float64)+1 || mid["expired"] != before["expired"].(float64)+1 {
		t.Fatalf("dropped and expired since start: %v/%v, want one more each than %v/%v", mid["dropped"], mid["expired"], before["dropped"], before["expired"])
	}
	if mid["queued"] != before["queued"].(float64)+2 { // the ring's survivor and the fresh job
		t.Fatalf("queued is the depth now: %v, was %v", mid["queued"], before["queued"])
	}
	for _, name := range []string{"#ring@h", "jobs@h"} {
		c, done := context.WithTimeout(context.Background(), time.Second)
		if _, err := b.ConsumeAs(c, "alice@h", name, "", "", false, false); err != nil {
			t.Fatal(err)
		}
		done()
	}
	after := status()
	if after["queued"] != before["queued"] || after["dropped"] != mid["dropped"] || after["expired"] != mid["expired"] {
		t.Fatalf("draining lowers queued only: before %v, after %v", mid, after)
	}
}

// Plain `ls --json` is what plain `ls` is: the agents that have a reader now.
func TestBusLsJSONWithoutOptionsListsOnlyAgentsWithReaders(t *testing.T) {
	b, s, _ := jsonFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.ConsumeAs(ctx, "alice@h", "#worker@h", "", "", false, false)
	for end := time.Now().Add(3 * time.Second); b.Status().Waiting < 1; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(end) {
			t.Fatal("the reader never waited")
		}
	}
	var live []protocol.Record
	if err := json.Unmarshal([]byte(busAnswer(t, s, "alice@h", "ls --json")), &live); err != nil {
		t.Fatal(err)
	}
	if len(live) != 1 || live[0].Name != "#worker@h" {
		t.Fatalf("plain ls lists agents with a reader, and #hidden@h has none: %+v", live)
	}
	var all []protocol.Record
	if err := json.Unmarshal([]byte(busAnswer(t, s, "alice@h", "ls --all --json")), &all); err != nil || len(all) <= 1 {
		t.Fatalf("--all lifts the filter: %d %v", len(all), err)
	}
}

// An active Administrator who is not the daemon Owner sees the owner-inactive
// detail over #bus exactly as over HTTP; an ordinary caller does not.
func TestBusStatusJSONForAnAdministratorWhoIsNotTheOwner(t *testing.T) {
	b, s, token := jsonFixture(t)
	if _, err := b.SetUser("admin@h", protocol.User{Name: "carol@h"}, true); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("admin@h", core.AdministratorsGroup, []string{"admin@h", "carol@h"}); err != nil {
		t.Fatal(err)
	}
	var viaBus, direct map[string]any
	if err := json.Unmarshal([]byte(busAnswer(t, s, "carol@h", "status --json")), &viaBus); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(httpJSON(t, s, token, "carol@h", "/status"), &direct); err != nil {
		t.Fatal(err)
	}
	delete(viaBus, "up")
	delete(direct, "up")
	if !reflect.DeepEqual(viaBus, direct) {
		t.Fatalf("an Administrator over #bus differs from /status:\n%v\n%v", viaBus, direct)
	}
	if viaBus["owner_inactive"] == nil || viaBus["administrator"] != true || viaBus["daemon_owner"] != nil {
		t.Fatalf("an Administrator sees the detail and is not the Owner: %v", viaBus)
	}
}

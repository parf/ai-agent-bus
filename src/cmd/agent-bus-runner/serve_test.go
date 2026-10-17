package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// A later list wins a name an earlier one set, in place: the agent's PATH
// beats the runner's, and the runner's stamp of the sender beats an env file.
func TestTheChildsEnvironmentLetsTheLaterLayerWin(t *testing.T) {
	got := mergeEnv(
		[]string{"PATH=/usr/bin", "HOME=/h"},
		[]string{"PATH=/opt/agent/bin", "AGENT_BUS_FROM=spoof"},
		[]string{"AGENT_BUS_FROM=alice@h"},
	)
	if strings.Join(got, " ") != "PATH=/opt/agent/bin HOME=/h AGENT_BUS_FROM=alice@h" {
		t.Fatalf("merged %q", got)
	}
}

// Two installed agents that come to one name are both refused, by name,
// rather than served as one with the other's command.
func TestTwoAgentsWithOneNameAreBothRefused(t *testing.T) {
	// A daemon that refuses everything: a start that got past the check
	// would be refused for another reason, and the assertion below says so.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "fixture: no daemon here", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := &client{http: srv.Client(), base: srv.URL}
	r := &runner{realm: "h", served: map[string]*served{}, say: io.Discard, bus: c}
	r.installed = []installed{
		{Key: "a", Dir: "a", Agent: agentFile{Command: "x"}, Host: hostEntry{Name: "#same@h"}},
		{Key: "b", Dir: "b", Agent: agentFile{Command: "y"}, Host: hostEntry{Name: "#same@h"}},
	}
	r.serve(context.Background())
	for _, in := range r.installed {
		if !strings.Contains(in.Refused, "#same@h is also the name of") {
			t.Errorf("%s: refused %q", in.Key, in.Refused)
		}
	}
	if len(r.served) != 0 {
		t.Fatalf("served %d", len(r.served))
	}
}

// The runner takes an agent's token only from the register that created it,
// keeps it to itself, and is answered as before by the kept one after a
// restart; a name that exists with no kept token is refused by name, with how
// to hand it over (runner M.10).
func TestTheRunnerClaimsOnlyWhatItCreated(t *testing.T) {
	registered := map[string]bool{"#theirs@h": true}
	owners := map[string]string{"#moved@h": "bob@h"} // alice@h's otherwise
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		tok := req.Header.Get("X-Agent-Bus-Token")
		switch req.URL.Path {
		case "/lookup":
			owner := owners[req.URL.Query().Get("name")]
			if owner == "" {
				owner = "alice@h"
			}
			io.WriteString(w, `{"owner":"`+owner+`"}`)
		case "/status":
			io.WriteString(w, `{"you":"`+strings.TrimPrefix(tok, "tok-")+`"}`)
		case "/manage":
			if !strings.HasPrefix(tok, "tok-#") {
				t.Error("tag refresh must act as the served agent")
			}
			io.WriteString(w, `{}`)
		case "/register":
			var rec struct{ Name string }
			json.NewDecoder(req.Body).Decode(&rec)
			if req.Header.Get("If-None-Match") != "*" {
				t.Errorf("register of %s is not create-only", rec.Name)
			}
			if registered[rec.Name] {
				http.Error(w, `{"error":"that name is already registered"}`, http.StatusPreconditionFailed)
				return
			}
			registered[rec.Name] = true
			io.WriteString(w, `{"name":"`+rec.Name+`","token":"tok-`+rec.Name+`"}`)
		default:
			http.Error(w, "fixture", http.StatusNotFound)
		}
	}))
	defer srv.Close()
	r := &runner{realm: "h", host: t.TempDir(), say: io.Discard, person: "alice@h",
		bus: &client{http: srv.Client(), base: srv.URL, token: "tok-#runner/alice@h"}}

	tok, err := r.claim(installed{Key: "mine"}, "#mine@h")
	if err != nil || tok != "tok-#mine@h" {
		t.Fatalf("creating #mine@h: %q %v", tok, err)
	}
	st, err := os.Stat(r.tokenFile("#mine@h"))
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("the kept token: %v %v", st, err)
	}
	// Restarted: the record exists now, and the kept token is what serves it.
	if again, err := r.claim(installed{Key: "mine"}, "#mine@h"); err != nil || again != tok {
		t.Fatalf("after a restart #mine@h: %q %v", again, err)
	}
	// Somebody's agent the runner never created: refused, never taken.
	if got, err := r.claim(installed{Key: "theirs"}, "#theirs@h"); err == nil || got != "" ||
		!strings.Contains(err.Error(), "#theirs@h exists and its token is not kept here") ||
		!strings.Contains(err.Error(), "agent-bus-token '#theirs@h' into "+r.tokenFile("#theirs@h")) {
		t.Fatalf("an agent it did not create: %q %v", got, err)
	}
	// Handed over, it is served; a token that is somebody else's is not.
	if err := keep(r.tokenFile("#theirs@h"), "tok-#theirs@h"); err != nil {
		t.Fatal(err)
	}
	if got, err := r.claim(installed{Key: "theirs"}, "#theirs@h"); err != nil || got != "tok-#theirs@h" {
		t.Fatalf("handed over: %q %v", got, err)
	}
	if err := keep(r.tokenFile("#theirs@h"), "tok-#other@h"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.claim(installed{Key: "theirs"}, "#theirs@h"); err == nil || !strings.Contains(err.Error(), "is not its any more") {
		t.Fatalf("a kept token that is another's: %v", err)
	}
	// A kept token that still answers, for an agent now somebody else's:
	// not this runner's to serve (review ri-M10 P2-1).
	if err := keep(r.tokenFile("#moved@h"), "tok-#moved@h"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.claim(installed{Key: "moved"}, "#moved@h"); err == nil || !strings.Contains(err.Error(), "#moved@h is bob@h's now, not alice@h's") {
		t.Fatalf("a transferred agent's kept token: %v", err)
	}
}

// A runner started with its person's own token refuses to run: it is the
// person's agent, never the person (runner M.10).
func TestTheRunnerIsNeverItsPerson(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		io.WriteString(w, `{"you":"alice@h"}`)
	}))
	defer srv.Close()
	r := &runner{say: io.Discard, bus: &client{http: srv.Client(), base: srv.URL, token: "alice"}}
	if err := r.become(); err == nil || !strings.Contains(err.Error(), "alice@h is a person, not a runner") {
		t.Fatalf("started as alice@h: %v", err)
	}
}

// An agent's limits are the host's over the built ones; a malformed one, or a
// need above them, refuses the agent by name rather than running it unbounded.
func TestAnAgentsLimitsAreTheHostsAndBoundItsNeeds(t *testing.T) {
	al, why := limitsFor(installed{})
	if why != "" || al.cgroup.Memory != 512<<20 || al.cgroup.Processes != 64 || al.files != 1024 || al.cgroup.CPU != 0 || al.disk != 1<<30 {
		t.Fatalf("built limits: %+v %q", al, why)
	}
	host := installed{Host: hostEntry{Limits: &limits{Memory: "1G", CPU: "150%", Processes: 8, Files: 64, IOWeight: 20, Disk: "10M"}}}
	al, why = limitsFor(host)
	if why != "" || al.cgroup.Memory != 1<<30 || al.cgroup.CPU != 150 || al.cgroup.Processes != 8 || al.cgroup.IOWeight != 20 || al.files != 64 || al.disk != 10<<20 {
		t.Fatalf("host limits: %+v %q", al, why)
	}
	for _, tc := range []struct {
		in   installed
		want string
	}{
		{installed{Host: hostEntry{Limits: &limits{Memory: "lots"}}}, "limits.memory"},
		{installed{Host: hostEntry{Limits: &limits{CPU: "2"}}}, "limits.cpu"},
		{installed{Host: hostEntry{Limits: &limits{IOWeight: 20000}}}, "limits.io_weight"},
		{installed{Host: hostEntry{Limits: &limits{Disk: "a lot"}}}, "limits.disk"},
		{installed{Agent: agentFile{Needs: &needs{Memory: "2G"}}}, "it needs 2G of memory"},
		{installed{Agent: agentFile{Needs: &needs{Memory: "2G"}}, Host: hostEntry{Limits: &limits{Memory: "1G"}}}, "it needs 2G of memory"},
	} {
		if _, why := limitsFor(tc.in); !strings.Contains(why, tc.want) {
			t.Errorf("%+v: refused %q, want %q", tc.in.Host.Limits, why, tc.want)
		}
	}
	if _, why := limitsFor(installed{Agent: agentFile{Needs: &needs{Memory: "2G"}}, Host: hostEntry{Limits: &limits{Memory: "4G"}}}); why != "" {
		t.Errorf("a need within its limit was refused: %q", why)
	}
}

// A work directory over its soft limit is said by name, its size and limit
// in the reason; one under it, or with no limit, is not.
func TestAWorkDirectoryOverItsDiskLimitIsSaid(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/big", make([]byte, 3000), 0o600); err != nil {
		t.Fatal(err)
	}
	if why := overDisk(dir, 2000); !strings.Contains(why, "holds 3000 bytes, over its disk limit of 2000") {
		t.Errorf("over: %q", why)
	}
	if why := overDisk(dir, 4000); why != "" {
		t.Errorf("under: %q", why)
	}
	if why := overDisk(dir, 0); why != "" {
		t.Errorf("no limit: %q", why)
	}
}

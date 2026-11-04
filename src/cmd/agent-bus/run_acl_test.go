package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestRunnerSharingIsExplicit(t *testing.T) {
	for _, tc := range []struct {
		args     []string
		entries  int
		personal bool
	}{
		{[]string{"#echo@h", "echo ok"}, 0, false},
		{[]string{"#echo@h", "echo ok", "--allow", "peer@h", "--tags", "personal"}, 1, true},
	} {
		ag, err := describe(tc.args)
		if err != nil {
			t.Fatal(err)
		}
		if len(ag.Allow) != tc.entries || protocol.HasTag(ag.Tags, "personal") != tc.personal {
			t.Fatalf("sharing settings: %+v", ag)
		}
		if tc.entries != 0 && ag.Allow[0] != "peer@h" {
			t.Fatalf("wrong recipient: %v", ag.Allow)
		}
	}
}

func TestRemovedMasterOptionsAreRejected(t *testing.T) {
	if _, err := describe([]string{"#echo@h", "echo ok", "--no-master"}); err == nil || !strings.Contains(err.Error(), "unknown option --no-master") {
		t.Fatalf("removed runner flag was accepted: %v", err)
	}
	if err := register([]string{"svc@h", "--kind", "agent", "--no-master"}); err == nil || !strings.Contains(err.Error(), "unknown option --no-master") {
		t.Fatalf("removed register flag was accepted: %v", err)
	}

	path := filepath.Join(t.TempDir(), "agent.json")
	if err := os.WriteFile(path, []byte(`{"name":"#echo@h","script":"echo ok","no_master":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	input, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	savedStdin := os.Stdin
	os.Stdin = input
	defer func() { os.Stdin = savedStdin }()
	if _, err := describe([]string{"-"}); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("removed runner JSON field was accepted: %v", err)
	}
}

// Stop at registration so this drives the real start path without a child.
func TestRunnerRegistrationCarriesExplicitSharing(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("AGENT_BUS_TOKEN", "fixture-token")
	savedTransport := transport
	defer func() { transport = savedTransport }()
	for _, jsonInput := range []bool{false, true} {
		t.Run(map[bool]string{false: "flags", true: "json"}[jsonInput], func(t *testing.T) {
			var got protocol.Record
			seen := 0
			server := protocolTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen++
				if r.URL.Path != "/register" || r.Method != "POST" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Error(err)
				}
				http.Error(w, "fixture stops after registration", http.StatusServiceUnavailable)
			}))
			defer server.Close()
			transport = func() (*http.Client, string) { return server.Client(), server.URL }
			args := []string{"#echo@h", "echo ok", "--allow", "peer@h", "--tags", "Build-Notes,claude,personal"}
			if jsonInput {
				path := filepath.Join(t.TempDir(), "agent.json")
				if err := os.WriteFile(path, []byte(`{"name":"#echo@h","script":"echo ok","allow":["peer@h"],"tags":["Build-Notes,claude,personal"]}`), 0600); err != nil {
					t.Fatal(err)
				}
				input, err := os.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				defer input.Close()
				savedStdin := os.Stdin
				os.Stdin = input
				defer func() { os.Stdin = savedStdin }()
				args = []string{"-"}
			}
			err := serveScript(args)
			if err == nil || !strings.Contains(err.Error(), "fixture stops after registration") || seen != 1 {
				t.Fatalf("run did not reach exactly one registration: calls=%d err=%v", seen, err)
			}
			if got.Name != "#echo@h" || len(got.Allow) != 1 || got.Allow[0] != "peer@h" || !got.IsPersonal() {
				t.Fatalf("registration lost explicit sharing: %+v", got)
			}
			// A script the runner serves is a name on this bus, so it
			// registers as an agent rather than taking the external default.
			if !slices.Equal(got.Tags, []string{"build-notes", "claude", "personal"}) {
				t.Fatalf("registration tags: %v", got.Tags)
			}
			if got.Kind != protocol.KindAgent {
				t.Fatalf("runner registered kind %q, want %s", got.Kind, protocol.KindAgent)
			}
		})
	}
}

func TestRunRequiresExplicitAgentName(t *testing.T) {
	for _, name := range []string{"hi", "hi@demo"} {
		if _, err := describe([]string{name, "echo ok"}); err == nil {
			t.Fatalf("inferred agent prefix for %s", name)
		}
	}
	ag, err := describe([]string{"#hi@demo", "echo ok", "-4"})
	if err != nil || ag.Name != "#hi@demo" || ag.Instances != 4 {
		t.Fatalf("canonical name/concurrency: %+v %v", ag, err)
	}
}

func TestRegisterCarriesPersonal(t *testing.T) {
	t.Setenv("AGENT_BUS_TOKEN", "fixture-token")
	savedTransport := transport
	defer func() { transport = savedTransport }()
	var got protocol.Record
	server := protocolTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/register" || r.Method != "POST" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	transport = func() (*http.Client, string) { return server.Client(), server.URL }
	if err := register([]string{"svc@h", "--kind", "agent", "--allow", "peer@h", "--tags", "personal"}); err != nil {
		t.Fatal(err)
	}
	if got.Name != "svc@h" || !got.IsPersonal() || len(got.Allow) != 1 || got.Allow[0] != "peer@h" {
		t.Fatalf("register lost Personal classification: %+v", got)
	}
	// A non-hash name needs an explicit Agent kind.
	if got.Kind != protocol.KindAgent {
		t.Fatalf("--personal registered kind %q, want %s", got.Kind, protocol.KindAgent)
	}
}

func TestPersonalBooleanFlagRefused(t *testing.T) {
	if _, err := describe([]string{"#example", "echo", "--personal"}); err == nil {
		t.Fatal("run accepted removed --personal")
	}
	if err := register([]string{"#example", "--personal"}); err == nil {
		t.Fatal("register accepted removed --personal")
	}
}

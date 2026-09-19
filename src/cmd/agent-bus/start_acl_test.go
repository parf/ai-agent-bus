package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
		{[]string{"svc@h", "echo ok"}, 0, false},
		{[]string{"svc@h", "echo ok", "--allow", "peer@h", "--personal"}, 1, true},
	} {
		svc, err := describe(tc.args)
		if err != nil {
			t.Fatal(err)
		}
		if len(svc.Allow) != tc.entries || svc.Personal != tc.personal {
			t.Fatalf("sharing settings: %+v", svc)
		}
		if tc.entries != 0 && svc.Allow[0] != "peer@h" {
			t.Fatalf("wrong recipient: %v", svc.Allow)
		}
	}
}

func TestRemovedMasterOptionsAreRejected(t *testing.T) {
	if _, err := describe([]string{"svc@h", "echo ok", "--no-master"}); err == nil || !strings.Contains(err.Error(), "unknown option --no-master") {
		t.Fatalf("removed runner flag was accepted: %v", err)
	}
	if err := register([]string{"svc@h", "--no-master"}); err == nil || !strings.Contains(err.Error(), "unknown option --no-master") {
		t.Fatalf("removed register flag was accepted: %v", err)
	}

	path := filepath.Join(t.TempDir(), "service.json")
	if err := os.WriteFile(path, []byte(`{"name":"svc@h","script":"echo ok","no_master":true}`), 0600); err != nil {
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
	if _, err := describe(nil); err == nil || !strings.Contains(err.Error(), "unknown field") {
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
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			args := []string{"svc@h", "echo ok", "--allow", "peer@h", "--personal"}
			if jsonInput {
				path := filepath.Join(t.TempDir(), "service.json")
				if err := os.WriteFile(path, []byte(`{"name":"svc@h","script":"echo ok","allow":["peer@h"],"personal":true}`), 0600); err != nil {
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
				args = nil
			}
			err := start(args)
			if err == nil || !strings.Contains(err.Error(), "fixture stops after registration") || seen != 1 {
				t.Fatalf("start did not reach exactly one registration: calls=%d err=%v", seen, err)
			}
			if got.Name != "svc@h" || len(got.Allow) != 1 || got.Allow[0] != "peer@h" || !got.Personal {
				t.Fatalf("registration lost explicit sharing: %+v", got)
			}
			// A script the runner serves is a name on this bus, so it
			// registers as an agent rather than taking the external default.
			if got.Kind != protocol.KindAgent {
				t.Fatalf("runner registered kind %q, want %s", got.Kind, protocol.KindAgent)
			}
		})
	}
}

func TestRegisterCarriesPersonal(t *testing.T) {
	t.Setenv("AGENT_BUS_TOKEN", "fixture-token")
	savedTransport := transport
	defer func() { transport = savedTransport }()
	var got protocol.Record
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	if err := register([]string{"svc@h", "--allow", "peer@h", "--personal"}); err != nil {
		t.Fatal(err)
	}
	if got.Name != "svc@h" || !got.Personal || len(got.Allow) != 1 || got.Allow[0] != "peer@h" {
		t.Fatalf("register lost Personal classification: %+v", got)
	}
	// Personal is an agent-only classification, so --personal alone states the
	// kind. Without this the CLI sends the external default and the daemon
	// refuses the registration for want of an address.
	if got.Kind != protocol.KindAgent {
		t.Fatalf("--personal registered kind %q, want %s", got.Kind, protocol.KindAgent)
	}
}

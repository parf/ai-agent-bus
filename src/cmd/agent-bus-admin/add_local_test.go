package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os/user"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// add-local creates a keyless User and maps a local account's socket to it,
// the managed runner's identity; an existing User is not a failure, and an
// account this host does not have is refused before the daemon is asked.
func TestAddLocalCreatesAKeylessUserAndMapsItsAccount(t *testing.T) {
	var requests []string
	userStatus := http.StatusOK
	mux := http.NewServeMux()
	mux.HandleFunc("POST /user", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, "POST /user "+in["name"].(string))
		w.WriteHeader(userStatus)
		w.Write([]byte("{}"))
	})
	mux.HandleFunc("POST /account", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, "POST /account "+in["account"].(string)+" "+in["principal"].(string))
		json.NewEncoder(w).Encode(protocol.AccountMappings{RestartRequired: true})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("AGENT_BUS_ADDR", srv.URL)
	known := func(name string) (*user.User, error) {
		if name == "agent-bus-runner" {
			return &user.User{Username: name}, nil
		}
		return nil, errors.New("unknown user " + name)
	}

	for _, existing := range []bool{false, true} {
		requests = nil
		userStatus = http.StatusOK
		if existing {
			userStatus = http.StatusPreconditionFailed
		}
		out, err := captureOutput(t, func() error {
			return userAddLocal([]string{"agent-bus-runner", "agent-bus-runner"}, known)
		})
		if err != nil {
			t.Fatalf("existing=%v: %v", existing, err)
		}
		want := "POST /user agent-bus-runner|POST /account agent-bus-runner agent-bus-runner"
		if got := strings.Join(requests, "|"); got != want {
			t.Fatalf("existing=%v: requests %q, want %q", existing, got, want)
		}
		if !strings.Contains(out, "restart agent-busd") {
			t.Fatalf("existing=%v: the socket change was not said: %q", existing, out)
		}
	}

	requests = nil
	if err := userAddLocal([]string{"ghost", "ghost"}, known); err == nil || !strings.Contains(err.Error(), "local account ghost") {
		t.Fatalf("an account this host lacks was not refused: %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("the daemon was asked for an account this host lacks: %v", requests)
	}
}

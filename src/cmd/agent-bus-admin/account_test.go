package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func captureOutput(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = fn()
	os.Stdout = old
	if closeErr := w.Close(); err == nil {
		err = closeErr
	}
	out, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	return string(out), err
}

func TestAccountVerbListsAndChangesTheDaemonMap(t *testing.T) {
	requests := []string{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /accounts", func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, "GET /accounts")
		json.NewEncoder(w).Encode(protocol.AccountMappings{
			Mappings:        []protocol.AccountMapping{{Account: "local", Principal: "alice@h"}},
			RestartRequired: true,
		})
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

	// Use the real dispatcher for the first request: parsing an SSH command is
	// not enough if the program then forgets to route the parsed account verb.
	out, err := captureOutput(t, func() error { return runAdmin([]string{"account", "list"}, "owner@h") })
	if err != nil || !strings.Contains(out, "local\talice@h") || !strings.Contains(out, "restart required") {
		t.Fatalf("list output=%q err=%v", out, err)
	}
	out, err = captureOutput(t, func() error { return accountVerb([]string{"set", "local", "bob@h"}) })
	if err != nil || !strings.Contains(out, "restart agent-busd") {
		t.Fatalf("set output=%q err=%v", out, err)
	}
	if got := strings.Join(requests, "\n"); got != "GET /accounts\nPOST /account local bob@h" {
		t.Fatalf("requests:\n%s", got)
	}
}

package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A node that asks for another namespace must not get the key's signature:
// enrol signs what the bus asks for, in the bus's namespace, or not at all.
func TestEnrolDoesNotSignForAnotherNamespace(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/identity" {
			io.WriteString(w, `{"protocol":1,"protocol_min":1}`)
			return
		}
		b, _ := io.ReadAll(r.Body)
		calls++
		var in map[string]any
		_ = json.Unmarshal(b, &in)
		if in["signature"] != nil {
			t.Errorf("a signature was sent to the node: %s", b)
		}
		json.NewEncoder(w).Encode(map[string]string{"name": "alice@test", "nonce": "0123456789abcdef0123456789abcdef", "namespace": "file"})
	}))
	defer srv.Close()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	script := "#!/bin/sh\necho \"$@\" >> '" + log + "'\ncat > /dev/null\necho SIGNATURE\n"
	if err := os.WriteFile(filepath.Join(dir, "ssh-keygen"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(dir, "id")
	if err := os.WriteFile(key, []byte("k"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("AGENT_BUS_ADDR", srv.URL)
	t.Setenv("AGENT_BUS_TOKEN", "fixture")
	err := enrol([]string{"alice@test", "--key=" + key})
	if err == nil || !strings.Contains(err.Error(), "namespace") {
		t.Fatalf("enrol = %v, want a refusal naming the namespace", err)
	}
	if _, statErr := os.Stat(log); statErr == nil {
		t.Fatal("ssh-keygen was run for another namespace")
	}
	if calls != 1 {
		t.Fatalf("the node saw %d calls, want only the challenge", calls)
	}
}

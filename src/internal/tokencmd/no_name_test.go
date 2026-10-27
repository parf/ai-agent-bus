package tokencmd

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestNoNameUsesLoginSocketPrincipalNotUSER(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "user-fixture.sock")
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	var mu sync.Mutex
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/status" {
			io.WriteString(w, `{"you":"socket-principal@realm"}`)
			return
		}
		if r.URL.Path != "/token" {
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		var req struct {
			Name   string
			Rotate bool
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		mu.Lock()
		names = append(names, req.Name)
		mu.Unlock()
		if req.Name != "socket-principal@realm" {
			t.Error("minted for a different principal than the socket")
		}
		io.WriteString(w, `{"token":"fixture-secret-token"}`)
	})}
	go server.Serve(l)
	defer server.Close()
	t.Setenv("AGENT_BUS_ADDR", path)
	t.Setenv("AGENT_BUS_TOKEN", "")
	t.Setenv("SSH_ORIGINAL_COMMAND", "")
	t.Setenv("USER", "wrong-principal@realm")
	args := os.Args
	out := os.Stdout
	defer func() { os.Args = args; os.Stdout = out }()
	f, err := os.Create(filepath.Join(dir, "token.out"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	os.Stdout = f
	for _, flags := range [][]string{nil, {"--rotate"}} {
		os.Args = append([]string{"agent-bus-token"}, flags...)
		if err := issue(os.Args[1:]); err != nil {
			t.Fatalf("no-name issue failed: %v", err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(names) != 2 {
		t.Fatalf("mint requests=%d", len(names))
	}
}
func TestNoNameWithoutLoginSocketGivesSafeInstruction(t *testing.T) {
	t.Setenv("AGENT_BUS_ADDR", filepath.Join(t.TempDir(), "user-missing.sock"))
	t.Setenv("AGENT_BUS_TOKEN", "fixture-secret-token")
	t.Setenv("SSH_ORIGINAL_COMMAND", "")
	args := os.Args
	defer func() { os.Args = args }()
	os.Args = []string{"agent-bus-token"}
	err := issue(os.Args[1:])
	if err == nil || !strings.Contains(err.Error(), "no name given and no login socket: pass <name>") || strings.Contains(err.Error(), "fixture-secret-token") {
		t.Fatal("missing socket must give a safe explicit-name instruction")
	}
}

func TestMalformedTokenAnswerNeverPrintsItsContents(t *testing.T) {
	_, err := tokenIn([]byte(`{"token":"fixture-private-value"}garbage`))
	if err == nil || strings.Contains(err.Error(), "fixture-private-value") {
		t.Fatal("malformed answer exposed credential bytes")
	}
}

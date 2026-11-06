package main

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// A test run from a launcher session's shell would otherwise hand every CLI it
// starts that session's identity and bus, whatever address the test set.
func init() {
	if os.Getenv("AB_SESSION_FILE_FIXTURE") != "1" {
		os.Unsetenv("AGENT_BUS_SESSION_FILE")
	}
}

// The real CLI entry point, run in a child so os.Exit and the environment are its own.
func TestSessionFileMainFixture(t *testing.T) {
	if os.Getenv("AB_SESSION_FILE_FIXTURE") != "1" {
		return
	}
	os.Args = []string{"agent-bus", "status"}
	main()
}

type seenTokens struct {
	sync.Mutex
	tokens []string
}

// sessionFileRun writes file (when given) with $SERVER as the fixture's address,
// then runs `agent-bus status` with env.
func sessionFileRun(t *testing.T, path string, file map[string]string, env map[string]string) (*seenTokens, string, error) {
	t.Helper()
	seen := &seenTokens{}
	server := protocolTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.Lock()
		seen.tokens = append(seen.tokens, r.Header.Get("X-Agent-Bus-Token"))
		seen.Unlock()
		w.Write([]byte(`{"you":"whoever"}`))
	}))
	t.Cleanup(server.Close)
	if file != nil {
		for k, v := range file {
			file[k] = strings.ReplaceAll(v, "$SERVER", server.URL)
		}
		data, _ := json.Marshal(file)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestSessionFileMainFixture$")
	// Never this shell's own bus identity: only what the case sets.
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "AGENT_BUS_") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	cmd.Env = append(cmd.Env, "AB_SESSION_FILE_FIXTURE=1", "AGENT_BUS_ADDR="+server.URL)
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.CombinedOutput()
	return seen, string(out), err
}

// J1: a launcher rewrites its session file on a rename; the environment its
// runtime started with still names the old identity.
func TestCLITakesItsIdentityFromTheSessionFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "bus-env.json")
	seen, out, err := sessionFileRun(t, file,
		map[string]string{"AGENT_BUS_NAME": "#claude/renamed@h", "AGENT_BUS_TOKEN": "token-after-rename", "AGENT_BUS_ADDR": "$SERVER"},
		map[string]string{"AGENT_BUS_SESSION_FILE": file, "AGENT_BUS_TOKEN": "token-before-rename", "AGENT_BUS_NAME": "#claude/old@h"})
	if err != nil {
		t.Fatalf("status failed: %v\n%s", err, out)
	}
	seen.Lock()
	defer seen.Unlock()
	if len(seen.tokens) == 0 {
		t.Fatalf("no request reached the server:\n%s", out)
	}
	for _, token := range seen.tokens {
		if token != "token-after-rename" {
			t.Fatalf("the CLI sent %q, not the session file's token; all: %q", token, seen.tokens)
		}
	}
}

func TestASetSessionFileThatCannotBeReadRefusesLoudly(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gone.json")
	seen, out, err := sessionFileRun(t, missing, nil, map[string]string{"AGENT_BUS_SESSION_FILE": missing, "AGENT_BUS_TOKEN": "token-before-rename"})
	if err == nil {
		t.Fatalf("a missing session file was not refused:\n%s", out)
	}
	if !strings.Contains(out, "AGENT_BUS_SESSION_FILE") || !strings.Contains(out, missing) {
		t.Fatalf("the refusal does not name the variable and the file:\n%s", out)
	}
	seen.Lock()
	defer seen.Unlock()
	if len(seen.tokens) != 0 {
		t.Fatalf("the CLI still called the bus with the environment's identity: %q", seen.tokens)
	}
}

// A key the file lacks is the environment's stale value too: never used. With
// only a token in the file, the CLI finds its bus as if no address were set
// (here a socket under XDG_RUNTIME_DIR), not at the env's old AGENT_BUS_ADDR.
func TestASessionFileWithoutAnAddressDropsTheEnvironmentsAddressAndPin(t *testing.T) {
	runtime := t.TempDir()
	if err := os.MkdirAll(filepath.Join(runtime, "agent-bus"), 0o700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", filepath.Join(runtime, "agent-bus", "bus.sock"))
	if err != nil {
		t.Fatal(err)
	}
	found := &seenTokens{}
	local := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/identity" {
			io.WriteString(w, `{"protocol":1,"protocol_min":1}`)
			return
		}
		found.Lock()
		found.tokens = append(found.tokens, r.Header.Get("X-Agent-Bus-Token"))
		found.Unlock()
		w.Write([]byte(`{"you":"whoever"}`))
	}))
	local.Listener = listener
	local.Start()
	defer local.Close()
	file := filepath.Join(t.TempDir(), "bus-env.json")
	stale, out, err := sessionFileRun(t, file,
		map[string]string{"AGENT_BUS_TOKEN": "token-after-rename"},
		map[string]string{"AGENT_BUS_SESSION_FILE": file, "XDG_RUNTIME_DIR": runtime,
			"AGENT_BUS_TLS_FINGERPRINT": "sha256:stale", "AGENT_BUS_NAME": "#claude/old@h"})
	if err != nil {
		t.Fatalf("status failed: %v\n%s", err, out)
	}
	stale.Lock()
	defer stale.Unlock()
	if len(stale.tokens) != 0 {
		t.Fatalf("the CLI used the environment's stale AGENT_BUS_ADDR: %q", stale.tokens)
	}
	found.Lock()
	defer found.Unlock()
	if len(found.tokens) != 1 || found.tokens[0] != "token-after-rename" {
		t.Fatalf("the discovered bus saw %q, want the file's token once", found.tokens)
	}
}

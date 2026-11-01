package tokencmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const nonce = "0123456789abcdef0123456789abcdef"

type seen struct{ path, token, body string }

// daemon answers the key-proof exchange and records what it was asked.
func daemon(t *testing.T, namespace string) (log func() []seen) {
	// namespace is the raw JSON of the member; empty leaves it out.
	t.Helper()
	var mu sync.Mutex
	var calls []seen
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		calls = append(calls, seen{r.URL.Path, r.Header.Get("X-Agent-Bus-Token"), string(b)})
		mu.Unlock()
		var in map[string]any
		_ = json.Unmarshal(b, &in)
		switch {
		case r.URL.Path == "/enrol" && in["signature"] == nil:
			answer := map[string]json.RawMessage{"name": json.RawMessage(strconv.Quote(in["name"].(string))), "nonce": json.RawMessage(strconv.Quote(nonce))}
			if namespace != "" {
				answer["namespace"] = json.RawMessage(namespace)
			}
			json.NewEncoder(w).Encode(answer)
		case r.URL.Path == "/enrol":
			json.NewEncoder(w).Encode(map[string]string{"token": "PROOF-TOKEN"})
		case r.URL.Path == "/token":
			json.NewEncoder(w).Encode(map[string]string{"token": "ROTATED-TOKEN"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("AGENT_BUS_ADDR", srv.URL)
	t.Setenv("AGENT_BUS_TOKEN", "")
	return func() []seen { mu.Lock(); defer mu.Unlock(); return append([]seen(nil), calls...) }
}

// keygen puts an ssh-keygen on PATH that records its calls, and returns a key file.
func keygen(t *testing.T) (key, log string) {
	t.Helper()
	dir := t.TempDir()
	log = filepath.Join(dir, "calls")
	script := "#!/bin/sh\necho \"$@\" >> '" + log + "'\ncat > /dev/null\necho SIGNATURE\n"
	if err := os.WriteFile(filepath.Join(dir, "ssh-keygen"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	key = filepath.Join(dir, "id")
	if err := os.WriteFile(key, []byte("k"), 0o600); err != nil {
		t.Fatal(err)
	}
	return key, log
}

func stdout(t *testing.T, f func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := f()
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	return strings.TrimSpace(string(out)), err
}

func TestAHostileNodeCannotChooseWhatTheKeySigns(t *testing.T) {
	log := daemon(t, `"git"`)
	key, keygenLog := keygen(t)
	_, err := prove("alice@test", key)
	if err == nil || !strings.Contains(err.Error(), "namespace") {
		t.Fatalf("prove = %v, want a refusal naming the namespace", err)
	}
	if _, statErr := os.Stat(keygenLog); statErr == nil {
		t.Fatal("ssh-keygen was run for a node that asked for another namespace")
	}
	if n := len(log()); n != 1 {
		t.Fatalf("the daemon saw %d calls, want only the challenge", n)
	}
}

func TestTheProofSignsTheNonceInTheBusNamespace(t *testing.T) {
	log := daemon(t, `"agent-bus"`)
	key, keygenLog := keygen(t)
	tok, err := prove("alice@test", key)
	if err != nil || tok != "PROOF-TOKEN" {
		t.Fatalf("prove = %q, %v", tok, err)
	}
	argv, _ := os.ReadFile(keygenLog)
	if !strings.Contains(string(argv), "-n agent-bus ") {
		t.Fatalf("ssh-keygen was called with %q", argv)
	}
	calls := log()
	if len(calls) != 2 || !strings.Contains(calls[1].body, `"signature"`) || !strings.Contains(calls[1].body, nonce) {
		t.Fatalf("the daemon saw %+v", calls)
	}
}

// Present and wrong is refused, whatever its shape; only an absent member is the older answer.
func TestAPresentButEmptyOrNullNamespaceIsRefusedBeforeTheKeyIsUsed(t *testing.T) {
	for _, ns := range []string{`""`, `null`, `7`, `"Agent-Bus"`} {
		log := daemon(t, ns)
		key, keygenLog := keygen(t)
		_, err := prove("alice@test", key)
		if err == nil || !strings.Contains(err.Error(), "namespace") {
			t.Errorf("namespace %s: prove = %v, want a refusal naming the namespace", ns, err)
		}
		if _, statErr := os.Stat(keygenLog); statErr == nil {
			t.Errorf("namespace %s: ssh-keygen was run", ns)
		}
		if n := len(log()); n != 1 {
			t.Errorf("namespace %s: the daemon saw %d calls", ns, n)
		}
	}
	// An answer without the member is the older daemon's and is signed in agent-bus.
	daemon(t, "")
	key, keygenLog := keygen(t)
	if _, err := prove("alice@test", key); err != nil {
		t.Fatalf("absent namespace: %v", err)
	}
	if argv, _ := os.ReadFile(keygenLog); !strings.Contains(string(argv), "-n agent-bus ") {
		t.Fatalf("ssh-keygen was called with %q", argv)
	}
}

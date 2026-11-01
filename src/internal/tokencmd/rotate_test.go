package tokencmd

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotateWithAKeyRotatesAfterTheProof(t *testing.T) {
	log := daemon(t, `"agent-bus"`)
	key, _ := keygen(t)
	out, err := stdout(t, func() error { return issue([]string{"alice@test", "--key=" + key, "--rotate"}) })
	if err != nil || out != "ROTATED-TOKEN" {
		t.Fatalf("issue printed %q, %v; want the rotated token", out, err)
	}
	calls := log()
	if len(calls) != 3 || calls[2].path != "/token" {
		t.Fatalf("the daemon saw %+v, want the proof then POST /token", calls)
	}
	if calls[2].token != "PROOF-TOKEN" || !strings.Contains(calls[2].body, `"rotate":true`) {
		t.Fatalf("the rotation was asked as %+v", calls[2])
	}
	// Without --rotate the proof's own token is the answer and nothing is rotated.
	log2 := daemon(t, `"agent-bus"`)
	out, err = stdout(t, func() error { return issue([]string{"alice@test", "--key=" + key}) })
	if err != nil || out != "PROOF-TOKEN" || len(log2()) != 2 {
		t.Fatalf("without --rotate: %q, %v, %d calls", out, err, len(log2()))
	}
}

// On the default transport the login socket speaks for its own account and
// ignores a presented token, so a rotation after a proof for another identity
// must go to the shared listener, where the token is the credential.
func TestRotateAfterAProofForAnotherIdentityUsesTheSharedListener(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Skip("no current account")
	}
	run := t.TempDir()
	dir := filepath.Join(run, "agent-bus")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	serve := func(path string, token http.HandlerFunc) {
		ln, err := net.Listen("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		mux := http.NewServeMux()
		mux.HandleFunc("/enrol", func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			if strings.Contains(string(b), `"signature"`) {
				io.WriteString(w, `{"token":"PROOF-TOKEN"}`)
				return
			}
			io.WriteString(w, `{"name":"bob@test","nonce":"`+nonce+`","namespace":"agent-bus"}`)
		})
		mux.HandleFunc("/token", token)
		srv := &httptest.Server{Listener: ln, Config: &http.Server{Handler: mux}}
		srv.Start()
		t.Cleanup(srv.Close)
	}
	var userSocketTokenCalls int
	serve(filepath.Join(dir, "user-"+u.Username+".sock"), func(w http.ResponseWriter, r *http.Request) {
		userSocketTokenCalls++
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, `{"error":"login principal does not own bob@test"}`)
	})
	serve(filepath.Join(dir, "bus.sock"), func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Agent-Bus-Token") != "PROOF-TOKEN" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		io.WriteString(w, `{"token":"ROTATED-TOKEN"}`)
	})
	t.Setenv("XDG_RUNTIME_DIR", run)
	t.Setenv("AGENT_BUS_ADDR", "")
	t.Setenv("AGENT_BUS_TOKEN", "")
	key, _ := keygen(t)
	out, err := stdout(t, func() error { return issue([]string{"bob@test", "--key=" + key, "--rotate"}) })
	if err != nil || out != "ROTATED-TOKEN" {
		t.Fatalf("issue printed %q, %v; want the rotated token from the shared listener", out, err)
	}
	if userSocketTokenCalls != 0 {
		t.Fatalf("the rotation was sent to the login socket %d times", userSocketTokenCalls)
	}
}

package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestEnrolKeyFileRequiresEqualsBeforeRequestingAProof(t *testing.T) {
	key := filepath.Join(t.TempDir(), "id")
	if err := os.WriteFile(key, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/identity" {
			io.WriteString(w, `{"protocol":1,"protocol_min":1}`)
			return
		}
		calls.Add(1)
		// A bad nonce ends the positive control before ssh-keygen can run.
		io.WriteString(w, `{"name":"alice@test","nonce":"invalid"}`)
	}))
	defer server.Close()
	savedTransport := transport
	transport = func() (*http.Client, string) { return server.Client(), server.URL }
	t.Cleanup(func() { transport = savedTransport })
	t.Setenv("AGENT_BUS_ADDR", server.URL)
	t.Setenv("AGENT_BUS_TOKEN", "fixture")
	for _, args := range [][]string{{"alice@test", "--key", key}, {"--key", key, "alice@test"}, {"--key", key}} {
		err := enrol(args)
		if err == nil || err.Error() != "--key takes its file as --key=<file>" {
			t.Errorf("enrol %q = %v; want the token command's spaced-file refusal", args, err)
		}
	}
	if err := enrol([]string{"alice@test", "--key="}); err == nil || err.Error() != "--key= needs a file; a bare --key uses the default key" {
		t.Errorf("empty --key= = %v; want the token command's empty-file refusal", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("refused key spelling asked the daemon for %d proofs", calls.Load())
	}
	err := enrol([]string{"alice@test", "--key=" + key})
	if err == nil || err.Error() != "the node sent a nonce that is not 32 hex digits" || calls.Load() != 1 {
		t.Fatalf("equals spelling did not request exactly one proof: error=%v calls=%d", err, calls.Load())
	}
}

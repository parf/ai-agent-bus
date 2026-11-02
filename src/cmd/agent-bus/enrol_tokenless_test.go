package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/parf/ai-agent-bus/internal/api"
)

// A newcomer has no token and is not on a login socket: `enrol` must still reach
// the daemon's /enrol and report what the daemon said, while any other verb stays
// refused before a connection is made.
func TestEnrolWithoutATokenReachesTheDaemonAndOtherVerbsStayRefused(t *testing.T) {
	const answer = "the realm answered: no such directory entry for this fixture"
	var mu sync.Mutex
	var paths, tokens []string
	var probes int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/identity" {
			mu.Lock()
			probes++
			mu.Unlock()
			io.WriteString(w, `{"protocol":1,"protocol_min":1}`)
			return
		}
		mu.Lock()
		paths = append(paths, r.Method+" "+r.URL.Path)
		tokens = append(tokens, r.Header.Get(api.HeaderToken))
		mu.Unlock()
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, answer)
	}))
	defer server.Close()
	savedTransport := transport
	transport = func() (*http.Client, string) { return server.Client(), server.URL }
	t.Cleanup(func() { transport = savedTransport })
	t.Setenv("AGENT_BUS_ADDR", server.URL)
	t.Setenv("AGENT_BUS_TOKEN", "")
	seen := func() ([]string, []string) {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), paths...), append([]string(nil), tokens...)
	}

	// No token: the daemon's own answer comes back, not the local refusal.
	err := enrol([]string{"alice@test", "--key=/nonexistent/never-read"})
	if err == nil || err.Error() != answer {
		t.Fatalf("tokenless enrol = %v; want the daemon's answer %q", err, answer)
	}
	got, sentTokens := seen()
	if len(got) != 1 || got[0] != "POST /enrol" || sentTokens[0] != "" {
		t.Fatalf("tokenless enrol sent %v with tokens %q; want one tokenless POST /enrol", got, sentTokens)
	}

	// Any other verb without a token is still refused here, before a connection:
	// not even the protocol probe goes out for it (the cached probe of the enrolment above is dropped first).
	protocolChecks.Lock()
	clear(protocolChecks.entries)
	protocolChecks.Unlock()
	mu.Lock()
	probesBefore := probes
	mu.Unlock()
	_, _, err = call("GET", "/status", nil, nil)
	if err == nil || err.Error() != "set AGENT_BUS_TOKEN" {
		t.Fatalf("a tokenless /status = %v; want the local refusal", err)
	}
	_, _, err = call("POST", "/token", nil, map[string]any{"name": "alice@test"})
	if err == nil || err.Error() != "set AGENT_BUS_TOKEN" {
		t.Fatalf("a tokenless POST /token = %v; want the local refusal", err)
	}
	_, _, err = call("GET", "/enrol", nil, nil)
	if err == nil || err.Error() != "set AGENT_BUS_TOKEN" {
		t.Fatalf("a tokenless GET /enrol = %v; want the local refusal (only POST is exempt)", err)
	}
	if got, _ := seen(); len(got) != 1 {
		t.Fatalf("the refused verbs reached the daemon: %v", got)
	}
	mu.Lock()
	probesAfter := probes
	mu.Unlock()
	if probesAfter != probesBefore {
		t.Fatalf("a refused tokenless verb sent %d protocol probes; it must not dial at all", probesAfter-probesBefore)
	}

	// With a token the call is what it was: the token travels.
	t.Setenv("AGENT_BUS_TOKEN", "fixture")
	err = enrol([]string{"alice@test", "--key=/nonexistent/never-read"})
	if err == nil || err.Error() != answer {
		t.Fatalf("enrol with a token = %v; want the daemon's answer", err)
	}
	if got, sentTokens := seen(); len(got) != 2 || sentTokens[1] != "fixture" {
		t.Fatalf("enrol with a token sent %v with tokens %q; want it to carry the token", got, sentTokens)
	}
}

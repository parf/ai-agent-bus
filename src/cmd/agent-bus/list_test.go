package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestReaderCountDistinguishesUnavailableFromMeasuredZero(t *testing.T) {
	zero := 0
	two := 2
	if got := readerCount(nil); got != "unavailable" {
		t.Fatalf("absent reader count = %q", got)
	}
	if got := readerCount(&zero); got != "0" {
		t.Fatalf("measured zero reader count = %q", got)
	}
	if got := readerCount(&two); got != "2" {
		t.Fatalf("measured nonzero reader count = %q", got)
	}
}

func captureOutput(t *testing.T, run func() error) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	err = run()
	w.Close()
	os.Stdout = old
	out := <-done
	r.Close()
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestHumanListLabelsEntitiesWithoutChangingJSONKinds(t *testing.T) {
	t.Setenv("AGENT_BUS_TOKEN", "fixture-token")
	// "generic" is no longer a kind, so it stands in for one the daemon never
	// stated: an unknown kind is printed as it came and never guessed at.
	answer := `[{"name":"svc@h","kind":"service","owner":"owner@h","maintainers":["alice@h","@ops"],"readers":0},{"name":"bot@h","kind":"agent","owner":"owner@h","readers":0},{"name":"jobs@h","kind":"queue","owner":"owner@h","readers":0},{"name":"news@h","kind":"pubsub","owner":"owner@h","readers":0},{"name":"alice@h","kind":"user","owner":"owner@h","readers":0},{"name":"old@h","kind":"generic","owner":"owner@h","readers":0}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, answer)
	}))
	defer srv.Close()
	oldAddress, oldTransport := cliAddress, transport
	cliAddress = srv.URL
	transport = sync.OnceValues(connect)
	t.Cleanup(func() { cliAddress, transport = oldAddress, oldTransport })

	human := captureOutput(t, func() error { return ls([]string{"-h", "--all"}) })
	for _, want := range []string{"📡 Service", "👾 Agent", "📮 Queue", "📣 PubSub", "👤 User", "old@h", "generic"} {
		if !strings.Contains(human, want) {
			t.Errorf("human listing lacks %q:\n%s", want, human)
		}
	}
	if strings.Contains(human, "\tservice\t") || strings.Contains(human, `"kind"`) {
		t.Errorf("human listing retained machine kinds: %s", human)
	}

	raw := captureOutput(t, func() error { return ls([]string{"--all"}) })
	if !strings.Contains(raw, `"kind":"service"`) || !strings.Contains(raw, `"kind":"agent"`) || !strings.Contains(raw, `"maintainers":["alice@h","@ops"]`) || strings.Contains(raw, "📡") || strings.Contains(raw, "👾") {
		t.Errorf("JSON listing changed its machine vocabulary: %s", raw)
	}
}

// With no name, kind or --all, ls asks for agents and keeps those being read,
// each record exactly as the daemon sent it; --kind asks for that kind and
// keeps every record of it (docs/user/cli.md#ls).
func TestDefaultListIsTheAgentsBeingRead(t *testing.T) {
	t.Setenv("AGENT_BUS_TOKEN", "fixture-token")
	var asked string
	answer := `[{"name":"#on@h","kind":"agent","owner":"o@h","readers":1,"descr":"kept as sent"},{"name":"#idle@h","kind":"agent","owner":"o@h","readers":0},{"name":"#old@h","kind":"agent","owner":"o@h"}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.Query().Get("kind")
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, answer)
	}))
	defer srv.Close()
	oldAddress, oldTransport := cliAddress, transport
	cliAddress = srv.URL
	transport = sync.OnceValues(connect)
	t.Cleanup(func() { cliAddress, transport = oldAddress, oldTransport })

	raw := captureOutput(t, func() error { return ls(nil) })
	if asked != "agent" {
		t.Fatalf("the default listing asked for kind %q, not agent", asked)
	}
	if !strings.Contains(raw, `"name":"#on@h"`) || !strings.Contains(raw, `"descr":"kept as sent"`) {
		t.Fatalf("the agent being read is missing or changed: %s", raw)
	}
	if strings.Contains(raw, "#idle@h") || strings.Contains(raw, "#old@h") {
		t.Fatalf("an agent nobody reads was listed: %s", raw)
	}
	human := captureOutput(t, func() error { return ls([]string{"-h"}) })
	if !strings.Contains(human, "#on@h") || strings.Contains(human, "#idle@h") {
		t.Fatalf("the table does not follow the same rule: %s", human)
	}
	raw = captureOutput(t, func() error { return ls([]string{"--kind", "agent"}) })
	if asked != "agent" || !strings.Contains(raw, "#idle@h") || !strings.Contains(raw, "#old@h") {
		t.Fatalf("--kind agent did not list every agent: %s", raw)
	}
}

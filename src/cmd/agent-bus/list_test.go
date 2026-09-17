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
	answer := `[{"name":"svc@h","kind":"generic","owner":"owner@h","readers":0},{"name":"bot@h","kind":"agent","owner":"owner@h","readers":0},{"name":"jobs@h","kind":"topic","owner":"owner@h","readers":0}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, answer)
	}))
	defer srv.Close()
	oldAddress, oldTransport := cliAddress, transport
	cliAddress = srv.URL
	transport = sync.OnceValues(connect)
	t.Cleanup(func() { cliAddress, transport = oldAddress, oldTransport })

	human := captureOutput(t, func() error { return ls([]string{"-h"}) })
	for _, want := range []string{"⚙️ Service", "👾️ Agent", "jobs@h  topic"} {
		if !strings.Contains(human, want) {
			t.Errorf("human listing lacks %q:\n%s", want, human)
		}
	}
	if strings.Contains(human, "\tgeneric\t") || strings.Contains(human, `"kind"`) {
		t.Errorf("human listing retained machine kinds: %s", human)
	}

	raw := captureOutput(t, func() error { return ls(nil) })
	if !strings.Contains(raw, `"kind":"generic"`) || !strings.Contains(raw, `"kind":"agent"`) || strings.Contains(raw, "⚙️") || strings.Contains(raw, "👾") {
		t.Errorf("JSON listing changed its machine vocabulary: %s", raw)
	}
}

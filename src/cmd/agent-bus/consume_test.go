package main

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestConsumeCarriesIndependentInboxAndFilters(t *testing.T) {
	t.Setenv("AGENT_BUS_TOKEN", "fixture-token")
	var got *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	oldAddress, oldTransport := cliAddress, transport
	cliAddress = srv.URL
	transport = sync.OnceValues(connect)
	t.Cleanup(func() { cliAddress, transport = oldAddress, oldTransport })

	if err := consume([]string{"--inbox", "jobs@h", "--topic", "MyTopic", "--tag", "result", "--wait", "0s"}); err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("consume made no request")
	}
	q := got.URL.Query()
	if q.Get("inbox") != "jobs@h" || q.Get("topic") != "MyTopic" || q.Get("tag") != "result" || q.Get("wait") != "0s" {
		t.Fatalf("consume query: %v", q)
	}
}

func TestConsumeAddressAndFiltersRequireValues(t *testing.T) {
	t.Setenv("AGENT_BUS_TOKEN", "fixture-token")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	oldAddress, oldTransport := cliAddress, transport
	cliAddress = srv.URL
	transport = sync.OnceValues(connect)
	t.Cleanup(func() { cliAddress, transport = oldAddress, oldTransport })

	for _, option := range []string{"inbox", "topic", "tag"} {
		if err := consume([]string{"--" + option}); err == nil || err.Error() != "--"+option+" wants a value" {
			t.Errorf("--%s: %v", option, err)
		}
	}
}

package main

import (
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
)

type secretTransport func(*http.Request) (*http.Response, error)

func (f secretTransport) RoundTrip(q *http.Request) (*http.Response, error) { return f(q) }

func TestEnvironmentUsesItsAdmittedAgentNameForSecret(t *testing.T) {
	for _, code := range []int{http.StatusOK, http.StatusForbidden} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			const name = "#chosen/instance@realm"
			calls := 0
			as := &client{base: "http://fixture", token: "fixture-token", http: &http.Client{Transport: secretTransport(func(q *http.Request) (*http.Response, error) {
				calls++
				if q.URL.Path != "/secret" {
					t.Errorf("unexpected identity recheck or request: %s", q.URL.Path)
					return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader("unexpected identity recheck")), Header: make(http.Header)}, nil
				}
				if got := q.URL.Query().Get("name"); got != name {
					t.Errorf("secret name %q, want admitted name %q", got, name)
				}
				if q.Method != "GET" {
					t.Errorf("secret method %s", q.Method)
				}
				body := "FIXTURE_MARKER=known"
				if code != http.StatusOK {
					body = "fixture-denied"
				}
				return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}}
			r := &runner{agents: t.TempDir(), host: t.TempDir()}
			env, why := r.environment(installed{Key: "different-key", Dir: "fixture", Host: hostEntry{Name: name}}, as)
			if code == http.StatusOK {
				if why != "" || !slices.Contains(env, "FIXTURE_MARKER=known") {
					t.Fatalf("secret delivery %v, %q", env, why)
				}
			} else if !strings.Contains(why, "fixture-denied") {
				t.Fatalf("secret authorization refusal was lost: %q", why)
			}
			if calls != 1 {
				t.Fatalf("secret delivery used %d requests, want exactly one", calls)
			}
		})
	}
}

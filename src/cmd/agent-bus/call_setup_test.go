package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type setupTransport func(*http.Request) (*http.Response, error)

func (f setupTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestTaggedSetupSharesDeadline(t *testing.T) {
	old := transport
	defer func() { transport = old }()
	t.Setenv("AGENT_BUS_TOKEN", "fixture")
	for _, blocked := range []string{"/status", "/lookup", "/register"} {
		t.Run(blocked, func(t *testing.T) {
			seen := false
			var deadline time.Time
			client := &http.Client{Transport: setupTransport(func(r *http.Request) (*http.Response, error) {
				d, ok := r.Context().Deadline()
				if !ok {
					t.Errorf("%s setup request has no shared deadline", r.URL.Path)
				}
				if deadline.IsZero() {
					deadline = d
				} else if !deadline.Equal(d) {
					t.Error("setup reset the shared deadline")
				}
				if r.URL.Path == blocked {
					seen = true
					return nil, context.DeadlineExceeded
				}
				status, body := 200, `{}`
				switch r.URL.Path {
				case "/status":
					body = `{"you":"caller@h"}`
				case "/lookup":
					status = 404
				case "/ls":
					body = `[]`
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			transport = func() (*http.Client, string) { return client, "http://fixture" }
			err := callTagged([]string{"status"}, map[string]string{"tagged": "fleet"}, 20*time.Millisecond)
			if !seen || err == nil {
				t.Fatalf("setup cancellation was not returned: seen=%v err=%v", seen, err)
			}
		})
	}
}

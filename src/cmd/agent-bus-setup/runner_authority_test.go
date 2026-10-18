package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type setupLookupTransport func(*http.Request) (*http.Response, error)

func (f setupLookupTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func setupReply(code int, body string) (*http.Response, error) {
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
}

func TestRunnerUsesOnlyItsPersonsAgent(t *testing.T) {
	p, err := runnerFor("alice", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	// No installed credential or real systemd command is part of this fixture.
	dir := t.TempDir()
	p.private = filepath.Join(dir, "private")
	if err := os.WriteFile(filepath.Join(dir, "systemctl"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	for _, tc := range []struct{ kind, owner string }{
		{"agent", "bob"}, {"service", "alice"}, {"agent", ""}, {"agent", "alice"},
	} {
		t.Run(tc.kind+"/"+tc.owner, func(t *testing.T) {
			client := &ownerClient{http: http.Client{Transport: setupLookupTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Query().Get("name") == p.person {
					return setupReply(200, `{"name":"alice","kind":"user"}`)
				}
				return setupReply(200, fmt.Sprintf(`{"name":%q,"kind":%q,"owner":%q}`, p.name, tc.kind, tc.owner))
			})}}
			state, err := observeRunner(p, runnerOps{bus: client})
			if err != nil {
				t.Fatal(err)
			}
			// Ensure enable would otherwise have a real installed subject.
			state.account, state.envValid, state.unit = true, true, "fixture unit"
			valid := tc.kind == "agent" && tc.owner == p.person
			for _, verb := range []string{"install", "enable"} {
				plan, err := runnerPlan(verb, p, state, "fixture unit", false, runnerOps{})
				if valid {
					if err != nil || len(plan) == 0 {
						t.Fatalf("%s refused the intended agent: %v", verb, err)
					}
				} else if err == nil || len(plan) != 0 {
					t.Fatalf("%s accepted kind=%q owner=%q: %d actions, %v", verb, tc.kind, tc.owner, len(plan), err)
				}
			}
		})
	}
}

func TestLookupDistinguishesAbsenceFromFailure(t *testing.T) {
	down := errors.New("fixture connection refused")
	for _, tc := range []struct {
		name  string
		code  int
		body  string
		err   error
		found bool
		bad   bool
	}{
		{"absent", 404, "", nil, false, false},
		{"record", 200, `{"name":"fixture","kind":"agent"}`, nil, true, false},
		{"transport", 0, "", down, false, true},
		{"forbidden", 403, "not for this caller", nil, false, true},
		{"daemon down", 503, "do not echo this upstream text", nil, false, true},
		{"malformed", 200, "{", nil, false, true},
		{"null", 200, "null", nil, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &ownerClient{http: http.Client{Transport: setupLookupTransport(func(*http.Request) (*http.Response, error) {
				if tc.err != nil {
					return nil, tc.err
				}
				return setupReply(tc.code, tc.body)
			})}}
			_, found, err := client.lookup("fixture")
			if found != tc.found || (err != nil) != tc.bad {
				t.Fatalf("lookup: found=%v err=%v, want found=%v error=%v", found, err, tc.found, tc.bad)
			}
			if tc.err != nil && !errors.Is(err, down) {
				t.Fatalf("transport cause lost: %v", err)
			}
		})
	}
}

func TestRunnerObservationAbortsOnLookupFailure(t *testing.T) {
	p, _ := runnerFor("alice", "fixture")
	p.private = filepath.Join(t.TempDir(), "private")
	for _, failName := range []string{p.person, p.name} {
		t.Run(failName, func(t *testing.T) {
			client := &ownerClient{http: http.Client{Transport: setupLookupTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Query().Get("name") == failName {
					return setupReply(503, "fixture unavailable")
				}
				return setupReply(200, `{"name":"alice","kind":"user"}`)
			})}}
			if _, err := observeRunner(p, runnerOps{bus: client}); err == nil {
				t.Fatal("failed lookup became a usable uninstall observation")
			}
		})
	}
}

func TestRunnerUnitLeavesDrainToTheRunnerWithinAHostBound(t *testing.T) {
	unit := runnerTemplate("/fixture/agent-bus-runner")
	for _, setting := range []string{"KillMode=mixed\n", "TimeoutStopSec=30s\n"} {
		if !strings.Contains(unit, setting) {
			t.Fatalf("runner unit lacks %q", setting)
		}
	}
}

func TestRunnerVerbStopsBeforePlanningWhenLookupFails(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "bus.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fixture unavailable", http.StatusServiceUnavailable)
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	t.Setenv("AGENT_BUS_ADDR", socket)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old; _ = r.Close(); _ = w.Close() }()
	err = runnerVerb("uninstall", "fixturealice", false, true)
	_ = w.Close()
	output, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if err == nil || strings.Contains(string(output), "would ") {
		t.Fatalf("unavailable daemon still produced a plan: error=%v output=%q", err, output)
	}
}

package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHumanResourceListDoesNotInventAQueue(t *testing.T) {
	t.Setenv("AGENT_BUS_TOKEN", "fixture-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`[{"name":"manual@h","kind":"resource","owner":"owner@h"},{"name":"jobs@h","kind":"queue","owner":"owner@h","readers":0,"queued":0}]`))
	}))
	defer server.Close()
	saved := transport
	transport = func() (*http.Client, string) { return server.Client(), server.URL }
	defer func() { transport = saved }()
	out := captureOutput(t, func() error { return ls([]string{"-h", "--all"}) })
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "manual@h" {
			if len(fields) < 6 || fields[4] != "-" || fields[5] != "-" {
				t.Fatalf("resource queue cells must be absent: %s", line)
			}
		}
		if len(fields) > 0 && fields[0] == "jobs@h" {
			if len(fields) < 6 || fields[4] != "0" || fields[5] != "0" {
				t.Fatalf("real queue observations changed: %s", line)
			}
		}
	}
	if !strings.Contains(out, "manual@h") || !strings.Contains(out, "jobs@h") {
		t.Fatalf("fixture rows missing: %s", out)
	}
	legend := strings.SplitN(out, "READERS  outstanding", 2)
	if len(legend) != 2 || !strings.Contains(legend[1], "📚") {
		t.Fatalf("legend does not explain the resource's absent queue: %s", out)
	}
}

package api

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
)

// These are the same refusal cases every client reads, produced by the real
// router/error mapper rather than the vectors' fake-daemon response bodies.
func TestDaemonMissingDataAndRoutesMatchSharedRefusalVectors(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/client-conformance/refusals.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		ID     string `json:"id"`
		Expect struct {
			Status int    `json:"status"`
			Kind   string `json:"kind"`
		} `json:"expect"`
	}
	if err = json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	requests := map[string][2]string{
		"kind-not_found-404":           {"GET", "/lookup?name=missing@fixture"},
		"kind-no_route-404":            {"GET", "/missing-route"},
		"kind-no_route-405-with-allow": {"POST", "/status"},
	}
	b := core.New()
	s, tok := serverFor(t, b, "owner@fixture")
	checked := 0
	for _, row := range rows {
		request, ok := requests[row.ID]
		if !ok {
			continue
		}
		checked++
		t.Run(row.ID, func(t *testing.T) {
			code, body := send(s, tok("owner@fixture"), request[0], request[1], "", "")
			var got struct {
				Kind string `json:"kind"`
			}
			if err := json.Unmarshal([]byte(body), &got); err != nil {
				t.Fatal(err)
			}
			if code != row.Expect.Status || got.Kind != row.Expect.Kind {
				t.Fatalf("daemon = %d kind %q, shared vector wants %d kind %q", code, got.Kind, row.Expect.Status, row.Expect.Kind)
			}
		})
	}
	if checked != len(requests) {
		t.Fatalf("matched %d shared refusal vectors, want %d", checked, len(requests))
	}
}

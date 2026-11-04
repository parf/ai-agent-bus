package core

import (
	"github.com/parf/ai-agent-bus/internal/protocol"
	"testing"
)

func TestStoredEmptyTagSentinelIsRefusedWithoutRepair(t *testing.T) {
	b := personalFixture(t)
	snap := b.Snapshot()
	for i := range snap.Records {
		if snap.Records[i].Name == "jobs@h" {
			snap.Records[i].Tags = []string{"ops", "no-tags"}
		}
	}
	restored := New()
	j := &reports{}
	restored.Journal(j)
	restored.Restore(snap)
	if _, ok := restored.records["jobs@h"]; ok {
		t.Fatal("stored sentinel was accepted or repaired")
	}
	if !j.has("no-tags") {
		t.Fatalf("missing refusal report: %v", j.lines)
	}
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", AddToSet: &ListDelta{Tags: []string{"no-tags"}}}); err == nil {
		t.Fatal("sentinel accepted on write")
	}
	if labels, none, err := protocol.TagFilter("no-tags"); err != nil || !none || len(labels) != 0 {
		t.Fatalf("valid empty filter refused: %v %v %v", labels, none, err)
	}
}

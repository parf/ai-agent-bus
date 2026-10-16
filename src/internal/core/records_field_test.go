package core

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// The node's count of records and a User's owned records are named "records";
// "services", their name before 0.8.108, carries the same value one release.
func TestRecordsAreCalledRecordsWithTheOldNameKeptBeside(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	if _, err := b.SetUser("admin@h", protocol.User{Name: "alice@h"}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Kind: protocol.KindQueue, Name: "jobs@h", Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	st := b.Status()
	var m map[string]any
	raw, _ := json.Marshal(st)
	_ = json.Unmarshal(raw, &m)
	if m["records"] == nil || m["records"] != m["services"] || st.Records == 0 {
		t.Fatalf("status names its count wrong: %s", raw)
	}
	var u protocol.User
	for _, x := range b.Users("admin@h", nil) {
		if x.Name == "alice@h" {
			u = x
		}
	}
	if !slices.Contains(u.Records, "jobs@h") || !slices.Equal(u.Records, u.Services) {
		t.Fatalf("a User's owned records: records %v, services %v", u.Records, u.Services)
	}
}

package core

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Node counts and owned-record lists use only their canonical field.
func TestRecordsNoLongerCarryTheOneReleaseServicesAlias(t *testing.T) {
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
	if m["records"] == nil || m["services"] != nil || st.Records == 0 {
		t.Fatalf("status names its count wrong: %s", raw)
	}
	var u protocol.User
	for _, x := range b.Users("admin@h", nil) {
		if x.Name == "alice@h" {
			u = x
		}
	}
	if !slices.Contains(u.Records, "jobs@h") {
		t.Fatalf("a User's owned records: records %v", u.Records)
	}
	raw, _ = json.Marshal(u)
	m = nil
	_ = json.Unmarshal(raw, &m)
	if _, present := m["services"]; present {
		t.Fatalf("User kept services alias: %s", raw)
	}
}

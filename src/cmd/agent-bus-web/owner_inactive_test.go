package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

const ownerInactiveHeading = `<h3>Records inactive because their owner is</h3>`

// The Overview's owner-inactive item is one node-wide item built from the
// daemon's count, shown only when the count was answered and is non-zero
// (docs/05-discovery.md#overview-and-diagnostics).
func TestOwnerInactiveItemComesFromTheDaemonCountOnly(t *testing.T) {
	// A daemon that answered no count — older, or a caller not answered it —
	// observed nothing, so nothing is claimed. Decoded from the wire, because
	// that is where an absent field has to stay absent.
	var old nodeStatus
	if err := json.Unmarshal([]byte(`{"up":"1s","kinds":{},"you":"admin@h"}`), &old); err != nil {
		t.Fatal(err)
	}
	for label, st := range map[string]core.Status{
		"absent":         old.Status,
		"observed none":  {OwnerInactive: &core.OwnerInactive{}},
		"messages alone": {OwnerInactive: &core.OwnerInactive{Messages: 3}},
	} {
		for _, item := range attentionItems(st, nil) {
			if item.Kind == "owner-inactive" {
				t.Errorf("%s: an item was claimed: %+v", label, item)
			}
		}
	}
	items := attentionItems(core.Status{OwnerInactive: &core.OwnerInactive{Records: 2, Messages: 5}}, nil)
	if len(items) != 1 || items[0].Kind != "owner-inactive" || items[0].Count != 2 || items[0].Queued != 5 || items[0].Level != "orange" || items[0].Href != "/users?state=inactive" {
		t.Fatalf("a counted pair gives %+v, want one orange item of 2 records and 5 messages", items)
	}
}

func TestOverviewShowsOwnerInactiveItemToAdministratorsOnly(t *testing.T) {
	m := meaningFixture(t)
	for _, name := range []string{"alice@h", "bob@h"} {
		if _, err := m.bus.SetUser("admin@h", protocol.User{Name: name}, true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.bus.Register(protocol.Record{Name: "#svc@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "#svc@h", Body: "held"}); err != nil {
			t.Fatal(err)
		}
	}
	if page := m.get("/"); strings.Contains(page, ownerInactiveHeading) {
		t.Fatal("the item appears while every owner is active")
	}
	if _, err := m.bus.SetUserState("admin@h", "alice@h", protocol.StatusInactive); err != nil {
		t.Fatal(err)
	}
	page := m.get("/")
	if !strings.Contains(page, ownerInactiveHeading) {
		t.Fatal("the daemon Owner's Overview lacks the owner-inactive item")
	}
	item := section(t, page, ownerInactiveHeading, "</article>")
	for _, want := range []string{"1 record · 2 messages held", `href="/users?state=inactive"`} {
		if !strings.Contains(item, want) {
			t.Errorf("the item lacks %q: %s", want, item)
		}
	}
	if strings.Count(page, ownerInactiveHeading) != 1 {
		t.Error("the node-wide item appears more than once")
	}
	if page := m.as("bob@h").get("/"); strings.Contains(page, ownerInactiveHeading) {
		t.Error("an ordinary User's Overview carries the node-wide item")
	}
}

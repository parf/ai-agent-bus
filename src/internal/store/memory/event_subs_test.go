package memory

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func memSub(id, creator string) []byte {
	raw, _ := json.Marshal(map[string]any{"id": id, "created_by": creator, "sent": 0, "dropped": 0})
	return raw
}

// The memory store keeps rows per source with ids unique across sources, copies
// what it is given, writes the counters alone, and a failed store changes nothing.
func TestMemorySubscriptionsAreSeparateCopiesWithUniqueIDs(t *testing.T) {
	s := NewState()
	in := memSub("000000000001", "alice@h")
	if err := s.AddSubscription(3, "000000000001", in); err != nil {
		t.Fatal(err)
	}
	in[2] = 'X'
	if err := s.AddSubscription(4, "000000000001", memSub("000000000001", "bob@h")); !errors.Is(err, ports.ErrSubscriptionExists) {
		t.Fatalf("a second source took the id: %v", err)
	}
	rows, _ := s.ListSubscriptions(3)
	if string(rows["000000000001"]) != string(memSub("000000000001", "alice@h")) {
		t.Fatalf("the stored row aliases the caller's: %s", rows["000000000001"])
	}
	if err := s.SetSubscriptionCounts(3, "000000000001", 4, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSubscriptionCounts(4, "000000000001", 4, 1); err == nil {
		t.Fatal("a counter write reached another source")
	}
	rows, _ = s.ListSubscriptions(3)
	var got map[string]any
	_ = json.Unmarshal(rows["000000000001"], &got)
	if got["sent"] != float64(4) || got["dropped"] != float64(1) || got["created_by"] != "alice@h" {
		t.Fatalf("counters %v", got)
	}
	if sources, _ := s.SubscriptionSources("alice@h"); !reflect.DeepEqual(sources, []uint32{3}) {
		t.Fatalf("sources %v", sources)
	}
	s.Err = errors.New("failed store")
	if err := s.AddSubscription(3, "000000000002", memSub("000000000002", "alice@h")); err == nil {
		t.Fatal("a failed store accepted a row")
	}
	if _, err := s.ListSubscriptions(3); err == nil {
		t.Fatal("a failed store answered a read")
	}
	s.Err = nil
	if rows, _ := s.ListSubscriptions(3); len(rows) != 1 {
		t.Fatalf("a failed write changed the rows: %d", len(rows))
	}
}

// A record's subscriptions, and those it or a removed User authored, go.
func TestMemorySubscriptionsGoWithTheirSourceAndAuthor(t *testing.T) {
	s := NewState()
	rec := func(name string, id uint32) *protocol.Record {
		return &protocol.Record{Name: name, ID: id, Kind: protocol.KindAgent}
	}
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"#a@h": rec("#a@h", 3), "#b@h": rec("#b@h", 4)}}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		source  uint32
		id, who string
	}{{3, "000000000001", "alice@h"}, {4, "000000000003", "alice@h"}, {4, "000000000004", "#a@h"}} {
		if err := s.AddSubscription(c.source, c.id, memSub(c.id, c.who)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"#a@h": rec("#a@h", 9)}}); err != nil {
		t.Fatal(err)
	}
	if rows, _ := s.ListSubscriptions(3); len(rows) != 0 {
		t.Fatalf("a replaced source kept %d rows", len(rows))
	}
	if rows, _ := s.ListSubscriptions(4); len(rows) != 1 {
		t.Fatalf("source 4 holds %d rows, want alice's one", len(rows))
	}
	if err := s.Commit(ports.Change{Users: map[string]*protocol.User{"alice@h": nil}}); err != nil {
		t.Fatal(err)
	}
	if rows, _ := s.ListSubscriptions(4); len(rows) != 0 {
		t.Fatalf("a removed User's rows remain: %d", len(rows))
	}
	if err := s.AddSubscription(7, "000000000007", memSub("000000000007", "x@h")); err != nil {
		t.Fatal(err)
	}
	if err := s.DropEventState(7); err != nil {
		t.Fatal(err)
	}
	if rows, _ := s.ListSubscriptions(7); len(rows) != 0 {
		t.Fatalf("the retirement prune left %d rows", len(rows))
	}
}

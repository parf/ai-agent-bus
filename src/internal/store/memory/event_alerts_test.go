package memory

import (
	"errors"
	"reflect"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func memAlerts(ids ...uint64) []ports.EventEntry {
	var out []ports.EventEntry
	for _, id := range ids {
		out = append(out, ports.EventEntry{ID: id, Level: "alert", Text: "x", At: "a", LastAt: "b", Count: 1})
	}
	return out
}

// The memory adapter keeps what it was given, separately per record and apart
// from the caller's slices, and a refused write changes nothing.
func TestMemoryEventAlertsAreSeparateCopiesAndAFailedWriteChangesNothing(t *testing.T) {
	s := NewState()
	in := memAlerts(1, 2)
	if err := s.ReplaceEventAlerts(1, in, true); err != nil {
		t.Fatal(err)
	}
	in[0].Text = "changed after the call"
	got, active, err := s.LoadEventAlerts(1)
	if err != nil || !active || !reflect.DeepEqual(got, memAlerts(1, 2)) {
		t.Fatalf("stored %+v active=%v %v", got, active, err)
	}
	got[0].Text = "changed after the load"
	if again, _, _ := s.LoadEventAlerts(1); again[0].Text != "x" {
		t.Fatal("a loaded slice aliases the stored one")
	}
	if other, active, _ := s.LoadEventAlerts(2); len(other) != 0 || active {
		t.Fatalf("record 2 inherited %+v active=%v", other, active)
	}
	s.Err = errors.New("failed store")
	if err := s.ReplaceEventAlerts(1, memAlerts(9), false); err == nil {
		t.Fatal("a failed store accepted a write")
	}
	if _, _, err := s.LoadEventAlerts(1); err == nil {
		t.Fatal("a failed store answered a read")
	}
	s.Err = nil
	if got, active, _ := s.LoadEventAlerts(1); !reflect.DeepEqual(got, memAlerts(1, 2)) || !active {
		t.Fatalf("a failed write changed the stored snapshot: %+v active=%v", got, active)
	}
	notice := memAlerts(5)
	notice[0].Level = "notice"
	for name, bad := range map[string][]ports.EventEntry{"descending": memAlerts(4, 3), "zero id": memAlerts(0), "a notice": notice, "id above the ceiling": memAlerts(ports.EventMaxID + 1), "count above the ceiling": overCount()} {
		if err := s.ReplaceEventAlerts(1, bad, true); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

// A record's alerts go with it.
func TestMemoryEventAlertsGoWithTheirRecord(t *testing.T) {
	s := NewState()
	rec := func(id uint32) *protocol.Record {
		return &protocol.Record{Name: "a@h", ID: id, Kind: protocol.KindQueue}
	}
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"a@h": rec(3)}}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceEventAlerts(3, memAlerts(1), true); err != nil {
		t.Fatal(err)
	}
	if err := s.ReserveEventIDs(3, 64); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"a@h": rec(9)}}); err != nil {
		t.Fatal(err)
	}
	if got, active, _ := s.LoadEventAlerts(3); len(got) != 0 || active {
		t.Fatalf("a replaced record's alerts survived: %+v active=%v", got, active)
	}
	if n, _ := s.EventHighWater(3); n != 0 {
		t.Fatalf("a replaced record's id counter survived: %d", n)
	}
}

func overCount() []ports.EventEntry {
	a := memAlerts(1)
	a[0].Count = ports.EventMaxID + 1
	return a
}

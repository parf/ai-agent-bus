package memory

import (
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// The memory store keeps schedules as the database does: due in order, counted,
// outcomes trimmed by age, and gone with their record's ID.
func TestMemorySchedulesFollowTheDatabaseContract(t *testing.T) {
	s := NewState()
	rec := func(name string, id uint32) *protocol.Record {
		return &protocol.Record{Name: name, ID: id, Owner: "admin@h", Kind: protocol.KindQueue}
	}
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"jobs@h": rec("jobs@h", 3)}}); err != nil {
		t.Fatal(err)
	}
	put := func(id string, v ports.Schedule, o *ports.ScheduleOutcome) {
		t.Helper()
		if err := s.ScheduleEdit(id, 3, func(ports.Schedule, bool, ports.ScheduleCount) (ports.ScheduleChange, error) {
			return ports.ScheduleChange{Write: true, Value: v, Outcome: o}, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	put("a", ports.Schedule{Enabled: true, Next: 200}, &ports.ScheduleOutcome{At: 10, Outcome: "old"})
	put("b", ports.Schedule{Once: true, Enabled: true, Next: 100}, &ports.ScheduleOutcome{At: 10 + ports.ScheduleOutcomeAge + 1, Outcome: "new"})
	if due, _ := s.ScheduleDue(300, 10); len(due) != 2 || due[0].ID != "b" {
		t.Fatalf("due: %+v", due)
	}
	if c, _ := s.ScheduleCounts(); c[3] != (ports.ScheduleCount{Recurring: 1, Once: 1}) {
		t.Fatalf("counts: %+v", c)
	}
	if out, _ := s.ScheduleOutcomes(3, ""); len(out) != 1 || out[0].Outcome != "new" {
		t.Fatalf("outcomes: %+v", out)
	}
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"jobs@h": rec("jobs@h", 9)}}); err != nil {
		t.Fatal(err)
	}
	if all, _ := s.ScheduleCounts(); len(all) != 0 {
		t.Fatalf("schedules outlived their record's ID: %+v", all)
	}
}

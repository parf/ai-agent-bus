package sqlite

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func putSchedule(t *testing.T, s *Store, id string, v ports.Schedule, outcome *ports.ScheduleOutcome) {
	t.Helper()
	if err := s.ScheduleEdit(id, v.Record, func(ports.Schedule, bool, ports.ScheduleCount) (ports.ScheduleChange, error) {
		return ports.ScheduleChange{Write: true, Value: v, Outcome: outcome}, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func scheduleStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bus.db")
	s, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"jobs@h": record("jobs@h", 3), "other@h": record("other@h", 4)}}); err != nil {
		t.Fatal(err)
	}
	return s, path
}

// A schedule survives a reopen; due work comes earliest first and only when
// enabled; counts and the next slot follow.
func TestSchedulesSurviveAReopenAndComeDueInOrder(t *testing.T) {
	s, path := scheduleStore(t)
	putSchedule(t, s, "a", ports.Schedule{Record: 3, Expr: "* * * * *", Target: "q@h", Body: "x", Enabled: true, Next: 200}, nil)
	putSchedule(t, s, "b", ports.Schedule{Record: 3, Once: true, At: 100, Target: "q@h", Body: "y", Enabled: true, Next: 100}, nil)
	putSchedule(t, s, "c", ports.Schedule{Record: 4, Once: true, At: 50, Target: "q@h", Body: "z", Next: 50}, nil)
	s = reopen(t, s, path)
	defer s.Close()
	if v, ok, err := s.ScheduleGet("a"); err != nil || !ok || v.Body != "x" || v.Record != 3 || v.ID != "a" {
		t.Fatalf("after a reopen: %+v %v %v", v, ok, err)
	}
	due, err := s.ScheduleDue(150, 10)
	if err != nil || len(due) != 1 || due[0].ID != "b" {
		t.Fatalf("due at 150: %+v %v (c is disabled, a is later)", due, err)
	}
	if due, _ := s.ScheduleDue(300, 1); len(due) != 1 || due[0].ID != "b" {
		t.Fatalf("a limit of one takes the earliest: %+v", due)
	}
	if next, ok, err := s.ScheduleNext(); err != nil || !ok || next != 100 {
		t.Fatalf("next: %d %v %v", next, ok, err)
	}
	counts, err := s.ScheduleCounts()
	if err != nil || counts[3] != (ports.ScheduleCount{Recurring: 1, Once: 1}) || counts[4] != (ports.ScheduleCount{Once: 1}) {
		t.Fatalf("counts: %+v %v", counts, err)
	}
	if err := s.ScheduleEdit("a", 99, func(ports.Schedule, bool, ports.ScheduleCount) (ports.ScheduleChange, error) {
		return ports.ScheduleChange{Write: true}, nil
	}); err != ports.ErrKVNoRecord {
		t.Fatalf("a schedule for a record not held: %v", err)
	}
}

// Schedules and outcomes go with their record; a name stored again under a
// new ID starts empty; another record's stay.
func TestSchedulesGoWithTheirRecord(t *testing.T) {
	s, _ := scheduleStore(t)
	defer s.Close()
	putSchedule(t, s, "a", ports.Schedule{Record: 3, Target: "q@h", Body: "x", Enabled: true, Next: 10}, &ports.ScheduleOutcome{At: 5, Outcome: "sent"})
	putSchedule(t, s, "b", ports.Schedule{Record: 4, Target: "q@h", Body: "y", Enabled: true, Next: 10}, nil)
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"jobs@h": record("jobs@h", 7)}}); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.ScheduleGet("a"); ok {
		t.Fatal("a schedule outlived its record's ID")
	}
	if out, _ := s.ScheduleOutcomes(3, ""); len(out) != 0 {
		t.Fatalf("outcomes outlived their record: %+v", out)
	}
	if _, ok, _ := s.ScheduleGet("b"); !ok {
		t.Fatal("another record's schedule went")
	}
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"other@h": nil}}); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.ScheduleGet("b"); ok {
		t.Fatal("a removed record's schedule stayed")
	}
}

// Outcomes are kept newest first, at most 1000 per record and none older than
// seven days.
func TestScheduleOutcomesAreKeptByCountAndAge(t *testing.T) {
	s, _ := scheduleStore(t)
	defer s.Close()
	for i := 0; i < ports.ScheduleOutcomesKept+5; i++ {
		putSchedule(t, s, "a", ports.Schedule{Record: 3, Target: "q@h", Body: "x"}, &ports.ScheduleOutcome{At: int64(1000 + i), Outcome: fmt.Sprint(i)})
	}
	out, err := s.ScheduleOutcomes(3, "a")
	if err != nil || len(out) != ports.ScheduleOutcomesKept || out[0].Outcome != fmt.Sprint(ports.ScheduleOutcomesKept+4) {
		t.Fatalf("kept %d, newest %+v, %v", len(out), out[0], err)
	}
	putSchedule(t, s, "a", ports.Schedule{Record: 3, Target: "q@h", Body: "x"}, &ports.ScheduleOutcome{At: 1000 + ports.ScheduleOutcomeAge + 2000, Outcome: "late"})
	if out, _ := s.ScheduleOutcomes(3, ""); len(out) != 1 || out[0].Outcome != "late" {
		t.Fatalf("outcomes older than seven days stayed: %d", len(out))
	}
}

// A row that does not decode is set aside: named, never due or listed.
func TestABrokenScheduleIsSetAsideAndNeverDue(t *testing.T) {
	s, _ := scheduleStore(t)
	defer s.Close()
	putSchedule(t, s, "good", ports.Schedule{Record: 3, Target: "q@h", Body: "x", Enabled: true, Next: 10}, nil)
	if _, err := s.db.Exec(`INSERT INTO schedules (id, record_id, once, enabled, next, in_flight, body) VALUES ('bad', 3, 0, 1, 5, 0, '{not json')`); err != nil {
		t.Fatal(err)
	}
	if due, err := s.ScheduleDue(100, 10); err != nil || len(due) != 1 || due[0].ID != "good" {
		t.Fatalf("due with a broken row: %+v %v", due, err)
	}
	if broken, err := s.ScheduleBroken(); err != nil || len(broken) != 1 || broken[0] != "bad" {
		t.Fatalf("broken: %v %v", broken, err)
	}
}

// A 1.0 database (schema 11) gains the schedule tables when opened, keeping
// what it holds.
func TestASchema11DatabaseGainsSchedules(t *testing.T) {
	s, path := scheduleStore(t)
	for _, q := range []string{`DROP TABLE schedules`, `DROP TABLE schedule_outcomes`, `PRAGMA user_version = 11`} {
		if _, err := s.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	s = reopen(t, s, path)
	defer s.Close()
	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 12 {
		t.Fatalf("version %d %v", version, err)
	}
	putSchedule(t, s, "a", ports.Schedule{Record: 3, Target: "q@h", Body: "x"}, nil)
	var name string
	if err := s.db.QueryRow(`SELECT name FROM records WHERE id = 3`).Scan(&name); err != nil || name != "jobs@h" {
		t.Fatalf("the upgrade lost a record: %q %v", name, err)
	}
	if _, err := s.db.Exec(`PRAGMA user_version = 10`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if _, err := Open(path, false); err == nil {
		t.Fatal("an older schema than 11 opened")
	}
	_ = sql.ErrNoRows
}

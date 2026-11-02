package sqlite

import (
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func alerts(ids ...uint64) []ports.EventEntry {
	var out []ports.EventEntry
	for _, id := range ids {
		out = append(out, ports.EventEntry{ID: id, Level: "alert", Text: "disk full", At: "2026-11-02T10:00:00.000Z", LastAt: "2026-11-02T10:00:01.000Z", Count: id})
	}
	return out
}

func openEvents(t *testing.T, path string) *Store {
	t.Helper()
	s, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func reopen(t *testing.T, s *Store, path string) *Store {
	t.Helper()
	s.Close()
	s, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func loadAlerts(t *testing.T, s *Store, id uint32) ([]ports.EventEntry, bool) {
	t.Helper()
	got, active, err := s.LoadEventAlerts(id)
	if err != nil {
		t.Fatalf("LoadEventAlerts(%d): %v", id, err)
	}
	return got, active
}

// The snapshot and the flag survive a reopen, a removal is a removal, and a
// log that is active but empty differs from one that is not active.
func TestEventAlertsSurviveReopenWithRemovalsAndTheActiveFlag(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.db")
	s := openEvents(t, path)
	if got, active := loadAlerts(t, s, 5); len(got) != 0 || active {
		t.Fatalf("a record never written is %v active=%v, want empty and inactive", got, active)
	}
	if err := s.ReplaceEventAlerts(5, alerts(3, 4, 9), true); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceEventAlerts(6, alerts(1), true); err != nil {
		t.Fatal(err)
	}
	s = reopen(t, s, path)
	if got, active := loadAlerts(t, s, 5); !reflect.DeepEqual(got, alerts(3, 4, 9)) || !active {
		t.Fatalf("after reopen: %+v active=%v", got, active)
	}
	// A pull or an expiry removes entries: the log stays active, empty.
	if err := s.ReplaceEventAlerts(5, alerts(9), true); err != nil {
		t.Fatal(err)
	}
	s = reopen(t, s, path)
	if got, active := loadAlerts(t, s, 5); !reflect.DeepEqual(got, alerts(9)) || !active {
		t.Fatalf("after a removal: %+v active=%v, want only id 9", got, active)
	}
	if err := s.ReplaceEventAlerts(5, nil, true); err != nil {
		t.Fatal(err)
	}
	s = reopen(t, s, path)
	if got, active := loadAlerts(t, s, 5); len(got) != 0 || !active {
		t.Fatalf("active and empty came back as %+v active=%v", got, active)
	}
	// A clean of the whole log: not active at all.
	if err := s.ReplaceEventAlerts(5, nil, false); err != nil {
		t.Fatal(err)
	}
	s = reopen(t, s, path)
	defer s.Close()
	if got, active := loadAlerts(t, s, 5); len(got) != 0 || active {
		t.Fatalf("a cleaned log came back as %+v active=%v", got, active)
	}
	if got, active := loadAlerts(t, s, 6); !reflect.DeepEqual(got, alerts(1)) || !active {
		t.Fatalf("another record's log changed: %+v active=%v", got, active)
	}
}

// A snapshot is written whole or not at all: a failure partway leaves the
// previous snapshot and flag exactly as they were.
func TestEventAlertsReplaceIsAtomic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.db")
	s := openEvents(t, path)
	defer s.Close()
	if err := s.ReplaceEventAlerts(5, alerts(1, 2), true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER refuse_three BEFORE INSERT ON event_alerts WHEN NEW.id = 3 BEGIN SELECT RAISE(ABORT, 'disk'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceEventAlerts(5, alerts(1, 2, 3), false); err == nil {
		t.Fatal("a write that failed partway reported success")
	}
	if got, active := loadAlerts(t, s, 5); !reflect.DeepEqual(got, alerts(1, 2)) || !active {
		t.Fatalf("a failed replace left %+v active=%v, want the previous snapshot", got, active)
	}
}

// A snapshot the store cannot hold is refused before anything is written.
func TestEventAlertsRefuseASnapshotTheModelCannotHave(t *testing.T) {
	s := openEvents(t, filepath.Join(t.TempDir(), "events.db"))
	defer s.Close()
	if err := s.ReplaceEventAlerts(5, alerts(2), true); err != nil {
		t.Fatal(err)
	}
	too := make([]ports.EventEntry, 501)
	for i := range too {
		too[i] = alerts(uint64(i + 1))[0]
	}
	zeroCount := alerts(1)
	zeroCount[0].Count = 0
	for name, bad := range map[string][]ports.EventEntry{
		"more than 500": too, "descending ids": alerts(4, 3), "repeated id": alerts(3, 3), "zero id": alerts(0), "zero count": zeroCount,
	} {
		if err := s.ReplaceEventAlerts(5, bad, true); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if err := s.ReplaceEventAlerts(0, alerts(1), true); err == nil {
		t.Error("record id 0 accepted")
	}
	if got, _ := loadAlerts(t, s, 5); !reflect.DeepEqual(got, alerts(2)) {
		t.Fatalf("a refused snapshot changed the stored one: %+v", got)
	}
}

// One record's damaged rows fail that record's log only: another record's log
// and the daemon's own load are untouched.
func TestADamagedAlertRowFailsOnlyThatLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.db")
	s := openEvents(t, path)
	defer s.Close()
	for _, id := range []uint32{7, 8} {
		if err := s.ReplaceEventAlerts(id, alerts(1, 2), true); err != nil {
			t.Fatal(err)
		}
	}
	for _, damage := range []struct{ name, stmt string }{
		{"zero count", `UPDATE event_alerts SET count = 0 WHERE record_id = 7 AND id = 2`},
		{"negative id", `UPDATE event_alerts SET id = -4 WHERE record_id = 7 AND id = 1`},
		{"count of the wrong type", `UPDATE event_alerts SET count = 'many' WHERE record_id = 7 AND id = 1`},
		{"active flag 5", `UPDATE event_logs SET active = 5 WHERE record_id = 7`},
		{"rows without a log state", `DELETE FROM event_logs WHERE record_id = 7`},
	} {
		t.Run(damage.name, func(t *testing.T) {
			s.db.Exec(`DELETE FROM event_alerts WHERE record_id = 7`)
			s.db.Exec(`DELETE FROM event_logs WHERE record_id = 7`)
			if err := s.ReplaceEventAlerts(7, alerts(1, 2), true); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.Exec(damage.stmt); err != nil {
				t.Fatal(err)
			}
			if _, _, err := s.LoadEventAlerts(7); !errors.Is(err, ports.ErrEventStateInvalid) {
				t.Fatalf("a damaged log: %v, want the invalid-state error the counters use", err)
			}
			if got, active := loadAlerts(t, s, 8); !reflect.DeepEqual(got, alerts(1, 2)) || !active {
				t.Fatalf("another record's log was affected: %+v active=%v", got, active)
			}
			if _, err := s.Load(); err != nil {
				t.Fatalf("the daemon's own load stopped for one record's rows: %v", err)
			}
		})
	}
}

// A record's alerts go with it, and a name registered again starts empty.
func TestEventAlertsGoWithTheirRecord(t *testing.T) {
	s := openEvents(t, filepath.Join(t.TempDir(), "events.db"))
	defer s.Close()
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"a@h": record("a@h", 3), "b@h": record("b@h", 4)}}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint32{3, 4} {
		if err := s.ReplaceEventAlerts(id, alerts(1), true); err != nil {
			t.Fatal(err)
		}
	}
	// a@h is registered again under a new ID: the old ID's private log is nobody's.
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"a@h": record("a@h", 9)}}); err != nil {
		t.Fatal(err)
	}
	if got, active := loadAlerts(t, s, 3); len(got) != 0 || active {
		t.Fatalf("a replaced record's alerts survived: %+v active=%v", got, active)
	}
	if got, _ := loadAlerts(t, s, 9); len(got) != 0 {
		t.Fatalf("a new registration inherited alerts: %+v", got)
	}
	// b@h is deleted.
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"b@h": nil}}); err != nil {
		t.Fatal(err)
	}
	if got, active := loadAlerts(t, s, 4); len(got) != 0 || active {
		t.Fatalf("a deleted record's alerts survived: %+v active=%v", got, active)
	}
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM event_alerts WHERE record_id IN (3, 4)`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("%d alert rows of removed records remain (%v)", n, err)
	}
}

// A schema-9 database opens as schema 10 with no alerts and keeps its data.
func TestSchemaNineGainsTheEventAlertTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bus.db")
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range tables {
		if strings.HasPrefix(stmt, "CREATE TABLE event_") {
			continue
		}
		if _, err := old.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	for _, stmt := range []string{`INSERT INTO kv (record_id, name, value) VALUES (3, 'kept', 'value')`, `PRAGMA user_version = 9`} {
		if _, err := old.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	old.Close()
	s, err := Open(path, false)
	if err != nil {
		t.Fatalf("schema 9 did not open: %v", err)
	}
	defer s.Close()
	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 10 {
		t.Fatalf("version %d (%v), want 10", version, err)
	}
	if got, active := loadAlerts(t, s, 3); len(got) != 0 || active {
		t.Fatalf("a migrated database holds alerts: %+v", got)
	}
	if err := s.ReplaceEventAlerts(3, alerts(1), true); err != nil {
		t.Fatalf("the migrated tables refuse a write: %v", err)
	}
	var kept string
	if err := s.db.QueryRow(`SELECT value FROM kv WHERE record_id = 3 AND name = 'kept'`).Scan(&kept); err != nil || kept != "value" {
		t.Fatalf("the migration lost data: %q %v", kept, err)
	}
}

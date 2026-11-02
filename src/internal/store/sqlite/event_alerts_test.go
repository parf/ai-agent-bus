package sqlite

import (
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/eventlog"
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
	// Replaced by an entirely different set: only the new one loads.
	if err := s.ReplaceEventAlerts(5, alerts(10), true); err != nil {
		t.Fatal(err)
	}
	if got, _ := loadAlerts(t, s, 5); !reflect.DeepEqual(got, alerts(10)) {
		t.Fatalf("after replacing {9} by {10}: %+v", got)
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

// A replacement by a disjoint set leaves only the new set: nothing of the old
// one is kept, and no key conflict stands in for that check.
func TestEventAlertsReplaceByADisjointSetKeepsOnlyTheNewOnes(t *testing.T) {
	s := openEvents(t, filepath.Join(t.TempDir(), "events.db"))
	defer s.Close()
	if err := s.ReplaceEventAlerts(11, alerts(1, 2), true); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceEventAlerts(11, alerts(3), true); err != nil {
		t.Fatal(err)
	}
	if got, _ := loadAlerts(t, s, 11); !reflect.DeepEqual(got, alerts(3)) {
		t.Fatalf("after replacing {1,2} by {3}: %+v", got)
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
	notAlert := alerts(1)
	notAlert[0].Level = "notice"
	if err := s.ReplaceEventAlerts(5, notAlert, true); err == nil {
		t.Error("a notice was accepted as an alert")
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
		if err := s.ReserveEventIDs(id, 64); err != nil {
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
	if err := s.db.QueryRow(`SELECT count(*) FROM meta WHERE key IN ('event-high:3', 'event-high:4')`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("%d id counters of removed records remain (%v)", n, err)
	}
	if got, err := s.EventHighWater(9); err != nil || got != 0 {
		t.Fatalf("the new record inherited a counter: %d %v", got, err)
	}
}

// A schema-9 database, with a record, its data and a reserved-id counter,
// opens as schema 10: its log reads inactive and empty, the next write goes
// above the reserved ids, and everything else is intact.
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
	for _, stmt := range []string{
		`INSERT INTO users (name, id, body) VALUES ('alice@h', 7, '{"name":"alice@h"}')`,
		`INSERT INTO records (name, id, kind, body, owner_id) VALUES ('jobs@h', 3, 'agent', '{"name":"jobs@h","kind":"agent","owner":"alice@h"}', 7)`,
		`INSERT INTO kv (record_id, name, value) VALUES (3, 'kept', 'value')`,
		`INSERT INTO meta (key, value) VALUES ('event-high:3', '64')`,
		`PRAGMA user_version = 9`,
	} {
		if _, err := old.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
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
		t.Fatalf("a migrated database holds alerts: %+v active=%v", got, active)
	}
	if n, err := s.EventHighWater(3); err != nil || n != 64 {
		t.Fatalf("the reserved-id counter changed: %d %v", n, err)
	}
	e := eventlog.New(s, s, func() time.Time { return time.Unix(1793318400, 0) })
	w, err := e.Write(3, "alert", "first after the upgrade", eventlog.Defaults())
	if err != nil || w.ID <= 64 {
		t.Fatalf("first write after the upgrade got id %d (%v), want above 64", w.ID, err)
	}
	snap, err := s.Load()
	if err != nil || len(snap.Records) != 1 || snap.Records[0].Name != "jobs@h" || snap.Records[0].OwnerID != 7 {
		t.Fatalf("the migration changed the registry: %+v %v", snap.Records, err)
	}
	var kept string
	if err := s.db.QueryRow(`SELECT value FROM kv WHERE record_id = 3 AND name = 'kept'`).Scan(&kept); err != nil || kept != "value" {
		t.Fatalf("the migration lost data: %q %v", kept, err)
	}
}

// A late write that finished on a retired registration after its deletion
// committed re-creates its rows; the drop removes all of them and nothing else.
func TestDroppingARetiredRecordsEventStateRemovesALateWrite(t *testing.T) {
	s := openEvents(t, filepath.Join(t.TempDir(), "events.db"))
	defer s.Close()
	for _, id := range []uint32{9, 10} {
		if err := s.ReplaceEventAlerts(id, alerts(1, 2), true); err != nil {
			t.Fatal(err)
		}
		if err := s.ReserveEventIDs(id, 64); err != nil {
			t.Fatal(err)
		}
	}
	var pruner ports.EventStatePruner = s
	if err := pruner.DropEventState(9); err != nil {
		t.Fatal(err)
	}
	if got, active := loadAlerts(t, s, 9); len(got) != 0 || active {
		t.Fatalf("a dropped record kept %+v active=%v", got, active)
	}
	if n, err := s.EventHighWater(9); err != nil || n != 0 {
		t.Fatalf("a dropped record kept its counter: %d %v", n, err)
	}
	if got, active := loadAlerts(t, s, 10); !reflect.DeepEqual(got, alerts(1, 2)) || !active {
		t.Fatalf("another record's alerts were touched: %+v active=%v", got, active)
	}
	if n, err := s.EventHighWater(10); err != nil || n != 64 {
		t.Fatalf("another record's counter was touched: %d %v", n, err)
	}
	if err := pruner.DropEventState(9); err != nil {
		t.Fatalf("dropping an id with nothing stored: %v", err)
	}
	if err := pruner.DropEventState(0); err == nil {
		t.Fatal("record id 0 accepted")
	}
}

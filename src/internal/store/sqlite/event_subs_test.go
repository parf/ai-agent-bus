package sqlite

import (
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func subRow(id, creator string) []byte {
	raw, _ := json.Marshal(map[string]any{"id": id, "target": "t@h", "selector": map[string]string{}, "filter": "", "created_by": creator,
		"created_at": "2026-11-02T00:00:00.000Z", "sent": 0, "dropped": 0})
	return raw
}

// Rows survive a reopen, ids are unique across sources, and a counter write
// changes the two counters and nothing else.
func TestSubscriptionsSurviveAReopenAndKeepOneIDPerSource(t *testing.T) {
	s, path := open(t)
	if err := s.AddSubscription(3, "000000000001", subRow("000000000001", "alice@h")); err != nil {
		t.Fatal(err)
	}
	if err := s.AddSubscription(4, "000000000002", subRow("000000000002", "bob@h")); err != nil {
		t.Fatal(err)
	}
	if err := s.AddSubscription(4, "000000000001", subRow("000000000001", "bob@h")); !errors.Is(err, ports.ErrSubscriptionExists) {
		t.Fatalf("an id taken by another source was accepted: %v", err)
	}
	if err := s.SetSubscriptionCounts(3, "000000000001", 5, 2); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSubscriptionCounts(4, "000000000001", 9, 9); err == nil {
		t.Fatal("a counter write reached another source's row")
	}
	s.Close()
	s, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows, err := s.ListSubscriptions(3)
	if err != nil || len(rows) != 1 {
		t.Fatalf("source 3: %v %v", rows, err)
	}
	var got map[string]any
	_ = json.Unmarshal(rows["000000000001"], &got)
	if got["sent"] != float64(5) || got["dropped"] != float64(2) || got["created_by"] != "alice@h" || got["target"] != "t@h" {
		t.Fatalf("the stored row after a reopen: %v", got)
	}
	if other, _ := s.ListSubscriptions(4); len(other) != 1 {
		t.Fatalf("source 4 holds %d rows, want its own one", len(other))
	}
	if src, found, err := s.FindSubscription("000000000002"); err != nil || !found || src != 4 {
		t.Fatalf("find: %d %v %v", src, found, err)
	}
	if sources, err := s.SubscriptionSources("alice@h"); err != nil || !reflect.DeepEqual(sources, []uint32{3}) {
		t.Fatalf("sources of alice: %v %v", sources, err)
	}
	if deleted, err := s.DeleteSubscription(4, "000000000001"); err != nil || deleted {
		t.Fatalf("a delete reached another source's row: %v %v", deleted, err)
	}
	if deleted, err := s.DeleteSubscription(3, "000000000001"); err != nil || !deleted {
		t.Fatalf("delete: %v %v", deleted, err)
	}
}

// A record's subscriptions go with it, and so do those it authored; a User's
// go when the User is removed. A name registered again inherits none.
func TestSubscriptionsGoWithTheirSourceAndTheirAuthor(t *testing.T) {
	s, _ := open(t)
	defer s.Close()
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"#a@h": record("#a@h", 3), "#b@h": record("#b@h", 4)}}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		source  uint32
		id, who string
	}{{3, "000000000001", "alice@h"}, {3, "000000000002", "#b@h"}, {4, "000000000003", "alice@h"}, {4, "000000000004", "#a@h"}} {
		if err := s.AddSubscription(c.source, c.id, subRow(c.id, c.who)); err != nil {
			t.Fatal(err)
		}
	}
	// #a@h is registered again under a new id: its log's subscriptions and the
	// ones it authored go, the others stay.
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"#a@h": record("#a@h", 9)}}); err != nil {
		t.Fatal(err)
	}
	left := func(source uint32) []string {
		rows, _ := s.ListSubscriptions(source)
		var ids []string
		for id := range rows {
			ids = append(ids, id)
		}
		return ids
	}
	if got := left(3); len(got) != 0 {
		t.Fatalf("a replaced source kept %v", got)
	}
	if got := left(4); !reflect.DeepEqual(got, []string{"000000000003"}) {
		t.Fatalf("source 4 holds %v, want only alice's (the replaced #a@h's was dropped)", got)
	}
	// alice is removed: hers go.
	if err := s.Commit(ports.Change{Users: map[string]*protocol.User{"alice@h": nil}}); err != nil {
		t.Fatal(err)
	}
	if got := left(4); len(got) != 0 {
		t.Fatalf("a removed User's subscriptions remain: %v", got)
	}
}

// The retirement prune removes a late subscription of the retired source, and
// only that source's.
func TestDroppingARetiredSourceRemovesLateSubscriptions(t *testing.T) {
	s, _ := open(t)
	defer s.Close()
	for _, source := range []uint32{7, 8} {
		id := map[uint32]string{7: "000000000007", 8: "000000000008"}[source]
		if err := s.AddSubscription(source, id, subRow(id, "alice@h")); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DropEventState(7); err != nil {
		t.Fatal(err)
	}
	if rows, _ := s.ListSubscriptions(7); len(rows) != 0 {
		t.Fatalf("a retired source kept %d rows", len(rows))
	}
	if rows, _ := s.ListSubscriptions(8); len(rows) != 1 {
		t.Fatalf("another source's row was dropped")
	}
}

// A row whose stored JSON is damaged does not stop a commit or the lookups.
func TestADamagedSubscriptionRowDoesNotBlockCommitsOrLookups(t *testing.T) {
	s, _ := open(t)
	defer s.Close()
	if _, err := s.db.Exec(`INSERT INTO event_subs (id, source_id, body) VALUES ('000000000009', 3, 'not json')`); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(ports.Change{Users: map[string]*protocol.User{"alice@h": nil}, Records: map[string]*protocol.Record{"#a@h": record("#a@h", 3)}}); err != nil {
		t.Fatalf("a damaged subscription row stopped a commit: %v", err)
	}
	if _, err := s.SubscriptionSources("alice@h"); err != nil {
		t.Fatalf("a damaged row stopped a lookup: %v", err)
	}
	if rows, _ := s.ListSubscriptions(3); len(rows) != 1 {
		t.Fatalf("the damaged row is not listed for its source's managers to repair")
	}
}

// A schema-9 database, with a record and its data, opens as the current schema
// (through 10 to 11) with no subscriptions and keeps everything.
func TestSchemaNineGainsTheSubscriptionTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bus.db")
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range tables {
		if strings.HasPrefix(stmt, "CREATE TABLE event_") || strings.HasPrefix(stmt, "CREATE INDEX event_") {
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
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 11 {
		t.Fatalf("version %d (%v), want 11", version, err)
	}
	if rows, err := s.ListSubscriptions(3); err != nil || len(rows) != 0 {
		t.Fatalf("a migrated database holds subscriptions: %v %v", rows, err)
	}
	if err := s.AddSubscription(3, "000000000001", subRow("000000000001", "alice@h")); err != nil {
		t.Fatalf("the migrated table refuses a write: %v", err)
	}
	snap, err := s.Load()
	if err != nil || len(snap.Records) != 1 || snap.Records[0].Name != "jobs@h" {
		t.Fatalf("the migration changed the registry: %+v %v", snap.Records, err)
	}
	var kept string
	if err := s.db.QueryRow(`SELECT value FROM kv WHERE record_id = 3 AND name = 'kept'`).Scan(&kept); err != nil || kept != "value" {
		t.Fatalf("the migration lost data: %q %v", kept, err)
	}
}

// A counter write to a stored value that is not an object is refused, not applied.
func TestACounterWriteToANonObjectRowIsRefused(t *testing.T) {
	s, _ := open(t)
	defer s.Close()
	for _, body := range []string{"null", "[]", "3", "not json"} {
		if _, err := s.db.Exec(`DELETE FROM event_subs`); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.Exec(`INSERT INTO event_subs (id, source_id, body) VALUES ('000000000001', 3, ?)`, body); err != nil {
			t.Fatal(err)
		}
		if err := s.SetSubscriptionCounts(3, "000000000001", 1, 1); !errors.Is(err, ports.ErrSubscriptionInvalid) {
			t.Errorf("a counter write to %q gave %v, want ErrSubscriptionInvalid", body, err)
		}
		var got string
		if err := s.db.QueryRow(`SELECT body FROM event_subs WHERE id = '000000000001'`).Scan(&got); err != nil || got != body {
			t.Errorf("the stored %q was changed to %q", body, got)
		}
	}
}

package sqlite

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func put(t *testing.T, s *Store, kind ports.KVKind, id uint32, name string, v ports.KVValue) {
	t.Helper()
	if err := s.KVEdit(kind, id, name, func(ports.KVValue, bool, int) (ports.KVEdit, error) {
		return ports.KVEdit{Write: true, Value: v}, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func record(name string, id uint32) *protocol.Record {
	return &protocol.Record{Name: name, ID: id, Owner: "admin@h", Kind: protocol.KindQueue}
}

// Every kind round-trips, survives a reopen, and is its own namespace.
func TestKVRoundTripsAndSurvivesAReopen(t *testing.T) {
	s, path := open(t)
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"jobs@h": record("jobs@h", 3)}}); err != nil {
		t.Fatal(err)
	}
	put(t, s, ports.KVString, 3, "k", ports.KVValue{Bytes: []byte{0, 255, 'x'}})
	put(t, s, ports.KVInt, 3, "k", ports.KVValue{Int: -42})
	put(t, s, ports.KVJSON, 3, "k", ports.KVValue{Bytes: []byte(`{"a":1}`)})
	s.Close()
	s, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for kind, want := range map[ports.KVKind]string{ports.KVString: "\x00\xffx", ports.KVJSON: `{"a":1}`} {
		v, found, err := s.KVGet(kind, 3, "k")
		if err != nil || !found || string(v.Bytes) != want {
			t.Errorf("%s after a reopen: %q %v %v", kind, v.Bytes, found, err)
		}
	}
	if v, found, err := s.KVGet(ports.KVInt, 3, "k"); err != nil || !found || v.Int != -42 {
		t.Errorf("int after a reopen: %d %v %v", v.Int, found, err)
	}
	if _, found, _ := s.KVGet(ports.KVString, 3, "other"); found {
		t.Error("an absent name was found")
	}
}

// A store's values go with its record, and a name stored again under a new
// ID starts empty; another record's values stay.
func TestKVGoesWithItsRecord(t *testing.T) {
	s, _ := open(t)
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"a@h": record("a@h", 1), "b@h": record("b@h", 2)}}); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []ports.KVKind{ports.KVString, ports.KVInt, ports.KVJSON} {
		put(t, s, kind, 1, "k", ports.KVValue{Bytes: []byte(`{}`), Int: 1})
		put(t, s, kind, 2, "k", ports.KVValue{Bytes: []byte(`{}`), Int: 1})
	}
	// a@h stored again under ID 5: its old values are nobody's.
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"a@h": record("a@h", 5)}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"b@h": nil}}); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := s.db.QueryRow(`SELECT (SELECT count(*) FROM kv) + (SELECT count(*) FROM kv_int) + (SELECT count(*) FROM kv_json)`).Scan(&left); err != nil || left != 0 {
		t.Fatalf("%d values outlived their record: %v", left, err)
	}
	// Rewriting a record under the same ID keeps its store.
	put(t, s, ports.KVInt, 5, "k", ports.KVValue{Int: 9})
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"a@h": record("a@h", 5)}}); err != nil {
		t.Fatal(err)
	}
	if v, found, _ := s.KVGet(ports.KVInt, 5, "k"); !found || v.Int != 9 {
		t.Fatal("an edit of the record dropped its store")
	}
}

// A value is never written for a record the store does not hold, and one left
// by a record that is gone is reported as an orphan.
func TestKVWritesOnlyForAStoredRecord(t *testing.T) {
	s, _ := open(t)
	err := s.KVEdit(ports.KVString, 7, "k", func(ports.KVValue, bool, int) (ports.KVEdit, error) {
		return ports.KVEdit{Write: true, Value: ports.KVValue{Bytes: []byte("x")}}, nil
	})
	if !errors.Is(err, ports.ErrKVNoRecord) {
		t.Fatalf("a value for no record: %v", err)
	}
	if _, err := s.db.Exec(`INSERT INTO kv_int (record_id, name, value) VALUES (8, 'k', 1)`); err != nil {
		t.Fatal(err)
	}
	if ids, err := s.KVOrphans(); err != nil || len(ids) != 1 || ids[0] != 8 {
		t.Fatalf("orphans: %v %v", ids, err)
	}
}

// An edit sees the value it replaces and the record's count, and an error
// from it writes nothing.
func TestKVEditIsOneTransaction(t *testing.T) {
	s, _ := open(t)
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"a@h": record("a@h", 1)}}); err != nil {
		t.Fatal(err)
	}
	put(t, s, ports.KVInt, 1, "x", ports.KVValue{Int: 1})
	boom := errors.New("boom")
	err := s.KVEdit(ports.KVInt, 1, "x", func(old ports.KVValue, found bool, names int) (ports.KVEdit, error) {
		if !found || old.Int != 1 || names != 1 {
			t.Errorf("edit saw %v %v %d", old, found, names)
		}
		return ports.KVEdit{}, boom
	})
	if !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if v, _, _ := s.KVGet(ports.KVInt, 1, "x"); v.Int != 1 {
		t.Fatal("a refused edit wrote")
	}
	if err := s.KVEdit(ports.KVInt, 1, "x", func(ports.KVValue, bool, int) (ports.KVEdit, error) { return ports.KVEdit{Delete: true}, nil }); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := s.KVGet(ports.KVInt, 1, "x"); found {
		t.Fatal("a delete left the value")
	}
}

// A schema-6 database migrates to 7 and keeps values at once.
func TestSchemaSixMigratesToKeepValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bus.db")
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range tables[:len(tables)-3] {
		if _, err := old.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if _, err := old.Exec(`PRAGMA user_version = 6`); err != nil {
		t.Fatal(err)
	}
	old.Close()
	s, err := Open(path, false)
	if err != nil {
		t.Fatalf("schema 6 did not open: %v", err)
	}
	defer s.Close()
	if err := s.Commit(ports.Change{Records: map[string]*protocol.Record{"a@h": record("a@h", 1)}}); err != nil {
		t.Fatal(err)
	}
	put(t, s, ports.KVJSON, 1, "k", ports.KVValue{Bytes: []byte(`{}`)})
}

package sqlite

import (
	"bytes"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"testing"
)

func TestRecordImportSQLFailureRollsBackRecordsKVAndCredentials(t *testing.T) {
	s, _ := open(t)
	if _, e := s.db.Exec(`CREATE TRIGGER refuse_import_kv BEFORE INSERT ON kv BEGIN SELECT RAISE(ABORT,'fixture'); END`); e != nil {
		t.Fatal(e)
	}
	rec := protocol.Record{ID: 42, Name: "#imported@h", Kind: protocol.KindAgent, Owner: "owner@h"}
	credential := ports.Credential{Name: rec.Name, Current: "private-fixture-token"}
	e := s.Commit(ports.Change{Records: map[string]*protocol.Record{rec.Name: &rec}, ImportKV: map[uint32][]ports.StoredKV{42: {{RecordID: 42, Kind: ports.KVString, Name: "x", Value: ports.KVValue{Bytes: []byte("private")}}}}, ImportCredentials: map[string]ports.Credential{rec.Name: credential}})
	if e == nil {
		t.Fatal("SQL failure was swallowed")
	}
	snap, e := s.Load()
	if e != nil {
		t.Fatal(e)
	}
	if len(snap.Records) != 0 {
		t.Fatal("record committed before failed KV write")
	}
	state, e := s.ExportState([]uint32{42}, []string{rec.Name})
	if e != nil {
		t.Fatal(e)
	}
	if len(state.KV) != 0 || len(state.Credentials) != 0 {
		t.Fatal("partial import survived failed transaction")
	}
}

// Import must use the same SQLite representation as an ordinary KV write.
func TestImportedKVMatchesOrdinaryWrites(t *testing.T) {
	cases := []struct {
		name    string
		kind    ports.KVKind
		value   ports.KVValue
		storage string
	}{
		{"empty-string", ports.KVString, ports.KVValue{}, "blob"},
		{"empty-bytes", ports.KVString, ports.KVValue{Bytes: []byte{}}, "blob"},
		{"bytes", ports.KVString, ports.KVValue{Bytes: []byte{0, 255, 42}}, "blob"},
		{"json", ports.KVJSON, ports.KVValue{Bytes: []byte(`{"x":1}`)}, "text"},
		{"json-empty-string", ports.KVJSON, ports.KVValue{Bytes: []byte(`""`)}, "text"},
		{"json-null", ports.KVJSON, ports.KVValue{Bytes: []byte(`null`)}, "text"},
		{"zero", ports.KVInt, ports.KVValue{}, "integer"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source, _ := open(t)
			if err := source.Commit(ports.Change{Records: map[string]*protocol.Record{"normal@h": record("normal@h", 1)}}); err != nil {
				t.Fatal(err)
			}
			put(t, source, tc.kind, 1, "kept", tc.value)
			exported, err := source.ExportState([]uint32{1}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(exported.KV) != 1 {
				t.Fatalf("exported %d values, want 1", len(exported.KV))
			}
			target, _ := open(t)
			if err = target.Commit(ports.Change{Records: map[string]*protocol.Record{"normal@h": record("normal@h", 1)}, ImportKV: map[uint32][]ports.StoredKV{1: exported.KV}}); err != nil {
				t.Fatalf("import exported value: %v", err)
			}
			table, err := kvTable(tc.kind)
			if err != nil {
				t.Fatal(err)
			}
			for label, s := range map[string]*Store{"ordinary": source, "imported": target} {
				var storage string
				if err = s.db.QueryRow("SELECT typeof(value) FROM " + table + " WHERE record_id=1 AND name='kept'").Scan(&storage); err != nil {
					t.Fatal(err)
				}
				if storage != tc.storage {
					t.Errorf("%s SQLite type = %s, want %s", label, storage, tc.storage)
				}
				got, found, err := s.KVGet(tc.kind, 1, "kept")
				if err != nil || !found || got.Int != tc.value.Int || !bytes.Equal(got.Bytes, tc.value.Bytes) {
					t.Errorf("%s read = %+v, found=%v, err=%v; want %+v", label, got, found, err, tc.value)
				}
			}
		})
	}
}

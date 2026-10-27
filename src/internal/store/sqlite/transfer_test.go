package sqlite

import (
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

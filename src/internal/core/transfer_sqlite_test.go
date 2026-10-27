package core

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/sqlite"
)

func openTransferSQLite(t *testing.T, path string, create bool) (*Bus, *auth.Tokens, *sqlite.Store) {
	t.Helper()
	store, e := sqlite.Open(path, create)
	if e != nil {
		t.Fatal(e)
	}
	snapshot, e := store.Load()
	if e != nil {
		t.Fatal(e)
	}
	b := New()
	b.Restore(snapshot)
	b.Persistence(store)
	if e = b.EstablishDaemonOwner("owner@source"); e != nil {
		t.Fatal(e)
	}
	tokens, e := auth.Load(store.Tokens(), "owner@source")
	if e != nil {
		t.Fatal(e)
	}
	b.BindCredentials(tokens)
	return b, tokens, store
}
func TestRecordSQLiteRebuildRetainsRunnerAndStandaloneTokens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.db")
	source, tokens, store := openTransferSQLite(t, path, true)
	held := map[string]string{}
	for _, name := range []string{"#runner@source", "#standalone@source"} {
		if _, e := source.Register(protocol.Record{Name: name, Kind: protocol.KindAgent, Owner: "owner@source", Allow: []string{"@owner"}}); e != nil {
			t.Fatal(e)
		}
		token, e := tokens.Issue(name)
		if e != nil {
			t.Fatal(e)
		}
		held[name] = token
		if _, e = source.Configure(name, "owner@source", json.RawMessage(`{"mode":"kept"}`)); e != nil {
			t.Fatal(e)
		}
		if _, e = source.SetSecret(name, "owner@source", "API_KEY=private-fixture"); e != nil {
			t.Fatal(e)
		}
		if e = source.KVSet("owner@source", name, ports.KVString, "binary", ports.KVValue{Bytes: []byte{0, 255, 42}}, ""); e != nil {
			t.Fatal(e)
		}
	}
	data, _, e := source.ExportRecords("owner@source", RecordSelection{})
	if e != nil {
		t.Fatal(e)
	}
	if e = store.Close(); e != nil {
		t.Fatal(e)
	}
	// Fresh SQLite identity allocation differs, so old pair IDs cannot be reused.
	destPath := filepath.Join(t.TempDir(), "rebuilt.db")
	dest, destTokens, destStore := openTransferSQLite(t, destPath, true)
	if _, e = dest.SetUser("owner@source", protocol.User{Name: "unrelated@source"}, true); e != nil {
		t.Fatal(e)
	}
	if _, e = dest.ImportRecords("owner@source", data, ImportChoices{KeepExisting: []string{"all"}}); e != nil {
		t.Fatal(e)
	}
	for name, token := range held {
		who, pair, ok := destTokens.Credential(token)
		if !ok || who != name || dest.CheckCredential(who, pair) != nil {
			t.Fatal("old agent token refused immediately after import")
		}
	}
	if e = destStore.Close(); e != nil {
		t.Fatal(e)
	}
	dest, destTokens, destStore = openTransferSQLite(t, destPath, false)
	defer destStore.Close()
	for name, token := range held {
		who, pair, ok := destTokens.Credential(token)
		if !ok || who != name || dest.CheckCredential(who, pair) != nil {
			t.Fatal("old agent token refused after restart")
		}
		cfg, e := dest.Config(name, "owner@source")
		if e != nil || string(cfg) != `{"mode":"kept"}` {
			t.Fatal("configuration did not survive rebuild")
		}
		v, e := dest.KVGet("owner@source", name, ports.KVString, "binary")
		if e != nil || !reflect.DeepEqual(v.Bytes, []byte{0, 255, 42}) {
			t.Fatal("KV did not survive rebuild")
		}
	}
}

// Validation failure must remain absent after reopening SQLite.
func TestRecordSQLiteImportFailureRollsBackEveryTable(t *testing.T) {
	_, _, data, _, token := transferSource(t)
	path := filepath.Join(t.TempDir(), "target.db")
	b, tokens, store := openTransferSQLite(t, path, true)
	// Duplicate an imported KV key. Core validation must refuse before commit.
	data.KV = append(data.KV, data.KV[0])
	if _, e := b.ImportRecords("owner@source", data, ImportChoices{KeepExisting: []string{"all"}}); e == nil {
		t.Fatal("duplicate KV accepted")
	}
	if _, _, ok := tokens.Credential(token); ok {
		t.Fatal("refused import published token")
	}
	if e := store.Close(); e != nil {
		t.Fatal(e)
	}
	b, _, store = openTransferSQLite(t, path, false)
	defer store.Close()
	if _, ok := b.Lookup("owner@source", "#runner@source"); ok {
		t.Fatal("refused import persisted record")
	}

}

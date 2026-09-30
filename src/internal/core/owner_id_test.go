package core

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/sqlite"
	_ "modernc.org/sqlite"
)

// startedStore is the store of the start in progress; a start closes the previous
// one first, as a daemon stopping does.
var startedStore *sqlite.Store

// start is a daemon start over the database at path: load, restore,
// establish, as agent-busd does.
func start(t *testing.T, path string) (*Bus, *reports) {
	t.Helper()
	stop(t)
	st, err := sqlite.Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	startedStore = st
	t.Cleanup(func() { stop(t) })
	snap, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	b, rep := New(), &reports{}
	b.Journal(rep)
	b.Restore(snap)
	b.Persistence(st)
	if err := b.EstablishDaemonOwner("owner@h"); err != nil {
		t.Fatal(err)
	}
	return b, rep
}

func stop(t *testing.T) {
	if startedStore != nil {
		startedStore.Close()
		startedStore = nil
	}
}

// Ownership follows user_id, not the name (K.24): a User recreated under a
// vanished User's name owns none of that User's records after a restart.
func TestAUserRecreatedUnderAVanishedNameOwnsNothingOfItsRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bus.db")
	b, _ := start(t, path)
	if _, err := b.SetUser("owner@h", protocol.User{Name: "alice@h"}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	// Falsifiable: across an ordinary restart alice still owns it.
	b, _ = start(t, path)
	if r, ok := b.records["jobs@h"]; !ok || r.Owner != "alice@h" {
		t.Fatalf("an ordinary restart lost alice's record: %+v %v", r, ok)
	}
	// alice vanishes: her profile and her own record are gone from the store.
	stop(t)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM users WHERE name = 'alice@h'; DELETE FROM records WHERE name = 'alice@h'`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	b, rep := start(t, path)
	if _, ok := b.records["jobs@h"]; ok || !rep.has("stored record jobs@h is ignored") {
		t.Fatalf("a record whose owner vanished was loaded: %v", rep.lines)
	}
	// A new alice is made under the name, and the node restarts.
	if _, err := b.SetUser("owner@h", protocol.User{Name: "alice@h"}, true); err != nil {
		t.Fatal(err)
	}
	b, rep = start(t, path)
	if r, ok := b.records["jobs@h"]; ok {
		t.Fatalf("the new alice owns the vanished one's record: %+v", r)
	}
	if !rep.has("stored record jobs@h is ignored: its owner alice@h is user") {
		t.Fatalf("not reported as another user's: %v", rep.lines)
	}
	if _, found := b.Lookup("alice@h", "jobs@h"); found {
		t.Fatal("the new alice reaches jobs@h")
	}
}

// A record snapshot carrying another user_id than its owner's is ignored.
func TestARecordOwnedByAnotherUserIDIsIgnored(t *testing.T) {
	b, rep := New(), &reports{}
	b.Journal(rep)
	b.Restore(ports.Snapshot{
		Users:   []protocol.User{{Name: "alice@h", ID: 9, Status: "active"}},
		Records: []protocol.Record{{Name: "alice@h", Owner: "alice@h", OwnerID: 9, Kind: protocol.KindUser, Personal: true}, {Name: "jobs@h", Owner: "alice@h", OwnerID: 5, Kind: protocol.KindQueue}},
		NextUserID: 10, NextRecordID: 10,
	})
	if err := b.EstablishDaemonOwner("alice@h"); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.records["jobs@h"]; ok || !rep.has("stored record jobs@h is ignored") {
		t.Fatalf("a record of user 5 was given to user 9: %v", rep.lines)
	}
}

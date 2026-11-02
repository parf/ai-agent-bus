package core

import (
	"encoding/json"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/store/memory"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/eventlog"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func eventFixture(t *testing.T) *Bus {
	t.Helper()
	b := ownerFixture(t)
	known(t, b, "alice@h", "bob@h", "maint@h")
	for _, name := range []string{"#a@h", "#other@h", "#never@h"} {
		if _, err := b.Register(protocol.Record{Name: name, Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Maintainers: protocol.MaintainerList{"maint@h"}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.EstablishDaemonAgent("node.example"); err != nil {
		t.Fatal(err)
	}
	return b
}
func TestEventAuthorityIsPrivateAndPrecedesExistence(t *testing.T) {
	b := eventFixture(t)
	if _, err := b.EventWrite("#a@h", "#a@h", "notice", "private"); err != nil {
		t.Fatal(err)
	}
	for _, who := range []string{"bob@h", "#a@h", "#other@h"} {
		for _, target := range []string{"#a@h", "#never@h"} {
			if _, err := b.EventRead(who, target, ""); eventlog.Code(err) != "forbidden" {
				t.Fatalf("%s read %s: %v", who, target, err)
			}
		}
	}
	for _, who := range []string{"alice@h", "maint@h", "owner@h"} {
		if _, err := b.EventRead(who, "#a@h", ""); err != nil {
			t.Fatalf("manager %s refused: %v", who, err)
		}
	}
	if _, err := b.EventRead("alice@h", "#never@h", ""); eventlog.Code(err) != "not_found" {
		t.Fatalf("authorized never-written %v", err)
	}
	for _, who := range []string{"alice@h", "#other@h", "maint@h"} {
		if _, err := b.EventWrite(who, "#a@h", "notice", "forged"); eventlog.Code(err) != "forbidden" {
			t.Fatalf("%s wrote another log: %v", who, err)
		}
	}
	if _, err := b.EventWrite("owner@h", "#a@h", "alert", "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "#a@h", Maintainers: ptr(protocol.MaintainerList{})}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.EventRead("maint@h", "#a@h", ""); eventlog.Code(err) != "forbidden" {
		t.Fatalf("revoked read: %v", err)
	}
	list, err := b.EventList("bob@h")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(list["logs"])
	if string(raw) != "[]" {
		t.Fatalf("list leaked private counts %s", raw)
	}
	if _, err := b.EventPull("bob@h", "#a@h", "", 1); eventlog.Code(err) != "forbidden" {
		t.Fatalf("private pull %v", err)
	}
	if _, err := b.EventClean("bob@h", "#a@h", "", 0); eventlog.Code(err) != "forbidden" {
		t.Fatalf("private clean %v", err)
	}
}
func TestEventReadEmptySelectedRingsAndFullClean(t *testing.T) {
	b := eventFixture(t)
	b.EventWrite("#a@h", "#a@h", "notice", "one")
	v, err := b.EventRead("alice@h", "#a@h", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range eventlog.Levels {
		if v.Rings[s] == nil {
			t.Fatalf("empty level %s missing or null", s)
		}
	}
	if _, err = b.EventPull("alice@h", "#a@h", "", 1); err != nil {
		t.Fatal(err)
	}
	v, err = b.EventRead("alice@h", "#a@h", "")
	if err != nil || v.Counts["notice"] != 0 {
		t.Fatalf("active empty: %+v %v", v, err)
	}
	if _, err = b.EventClean("alice@h", "#a@h", "", 0); err != nil {
		t.Fatal(err)
	}
	if _, err = b.EventRead("alice@h", "#a@h", ""); eventlog.Code(err) != "not_found" {
		t.Fatalf("fully cleaned %v", err)
	}
}
func TestBusEventCapacityCapDoesNotPublishAnInvalidSetup(t *testing.T) {
	b := eventFixture(t)
	before := b.Realm()
	if _, err := b.Configure(DaemonAgent, "owner@h", []byte(`{"realm":"wrong","notice_entries":251}`)); err == nil {
		t.Fatal("501 entries accepted")
	}
	if b.Realm() != before {
		t.Fatal("invalid settings published")
	}
	for _, cfg := range []string{`{"realm":"node.example"}`, `{"realm":"node.example","notice_entries":250}`} {
		if _, err := b.Configure(DaemonAgent, "owner@h", []byte(cfg)); err != nil {
			t.Fatalf("valid cap boundary refused %v", err)
		}
	}
}

func TestDeletingARecordReleasesItsLoadedEventLog(t *testing.T) {
	b := eventFixture(t)
	if _, err := b.EventWrite("#a@h", "#a@h", "notice", "old"); err != nil {
		t.Fatal(err)
	}
	if err := b.Unregister("#a@h", "alice@h"); err != nil {
		t.Fatal(err)
	}
	if n := b.events.Loaded(); n != 0 {
		t.Fatalf("deleted log retained in catalog: %d", n)
	}
	if _, err := b.Register(protocol.Record{Name: "#a@h", Kind: protocol.KindAgent, Owner: "bob@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.EventRead("bob@h", "#a@h", ""); eventlog.Code(err) != "not_found" {
		t.Fatalf("replacement inherited old data %v", err)
	}
}

type blockedEventStore struct {
	*memory.State
	entered, release chan struct{}
}

func (s *blockedEventStore) ReserveEventIDs(id uint32, n uint64) error {
	close(s.entered)
	<-s.release
	return s.State.ReserveEventIDs(id, n)
}
func TestEventStoreIOLeavesOrdinaryKVWritesUnblocked(t *testing.T) {
	st := &blockedEventStore{State: memory.NewState(), entered: make(chan struct{}), release: make(chan struct{})}
	b := New()
	b.Persistence(st)
	b.SetDaemonOwner("owner@h")
	if _, err := b.Register(protocol.Record{Name: "#a@h", Kind: protocol.KindAgent, Owner: "owner@h"}); err != nil {
		t.Fatal(err)
	}
	write := make(chan error, 1)
	go func() { _, err := b.EventWrite("#a@h", "#a@h", "notice", "slow"); write <- err }()
	select {
	case <-st.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("reservation did not enter barrier")
	}
	kv := make(chan error, 1)
	go func() {
		kv <- b.KVSet("owner@h", "#a@h", ports.KVString, "ordinary", ports.KVValue{Bytes: []byte("ok")}, KVSet)
	}()
	var problem string
	select {
	case err := <-kv:
		if err != nil {
			problem = err.Error()
		}
	case <-time.After(time.Second):
		problem = "ordinary KV write blocked behind event I/O"
	}
	close(st.release)
	select {
	case err := <-write:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("event write failed to finish after release")
	}
	if problem != "" {
		t.Fatal(problem)
	}
}

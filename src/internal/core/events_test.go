package core

import (
	"encoding/json"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/store/memory"
	"strings"
	"sync"
	"sync/atomic"
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

type eventLoadBarrier struct {
	saveBlock                atomic.Bool
	saveEntered, saveRelease chan struct{}
	*memory.State
	block            atomic.Bool
	entered, release chan struct{}
	mu               sync.Mutex
	alerts           map[uint32][]ports.EventEntry
	active           map[uint32]bool
}

func (s *eventLoadBarrier) EventHighWater(id uint32) (uint64, error) {
	n, err := s.State.EventHighWater(id)
	if s.block.CompareAndSwap(true, false) {
		close(s.entered)
		<-s.release
	}
	return n, err
}
func (s *eventLoadBarrier) LoadEventAlerts(id uint32) ([]ports.EventEntry, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]ports.EventEntry{}, s.alerts[id]...), s.active[id], nil
}
func (s *eventLoadBarrier) ReplaceEventAlerts(id uint32, v []ports.EventEntry, active bool) error {
	if s.saveBlock.CompareAndSwap(true, false) {
		close(s.saveEntered)
		<-s.saveRelease
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alerts[id] = append([]ports.EventEntry{}, v...)
	s.active[id] = active
	return nil
}
func (s *eventLoadBarrier) DropEventState(id uint32) error {
	s.mu.Lock()
	delete(s.alerts, id)
	delete(s.active, id)
	s.mu.Unlock()
	return s.State.DropEventState(id)
}
func TestEventAuthorityRecheckedAfterLoadBeforeCommit(t *testing.T) {
	for _, method := range []string{"read", "pull", "clean", "write"} {
		for _, change := range []string{"revoke", "delete"} {
			t.Run(method+"/"+change, func(t *testing.T) {
				b := eventFixture(t)
				st := &eventLoadBarrier{State: memory.NewState(), alerts: map[uint32][]ports.EventEntry{}, active: map[uint32]bool{}, entered: make(chan struct{}), release: make(chan struct{})}
				b.Persistence(st)
				if _, err := b.EventWrite("#a@h", "#a@h", "alert", "kept"); err != nil {
					t.Fatal(err)
				}
				old := b.records["#a@h"].ID
				b.events = nil
				st.block.Store(true)
				done := make(chan struct {
					value any
					err   error
				}, 1)
				go func() {
					var value any
					var err error
					switch method {
					case "read":
						value, err = b.EventRead("maint@h", "#a@h", "")
					case "pull":
						value, err = b.EventPull("maint@h", "#a@h", "", 1)
					case "clean":
						value, err = b.EventClean("maint@h", "#a@h", "", 0)
					case "write":
						value, err = b.EventWrite("#a@h", "#a@h", "alert", "new")
					}
					done <- struct {
						value any
						err   error
					}{value, err}
				}()
				select {
				case <-st.entered:
				case <-time.After(2 * time.Second):
					t.Fatal("load never entered barrier")
				}
				want := "forbidden"
				if change == "delete" {
					if err := b.Unregister("#a@h", "alice@h"); err != nil {
						t.Fatal(err)
					}
					want = "forbidden"
				} else if method == "write" {
					if _, err := b.SetUserState("owner@h", "alice@h", "inactive"); err != nil {
						t.Fatal(err)
					}
				} else {
					if _, err := b.Manage("alice@h", Management{Name: "#a@h", Maintainers: ptr(protocol.MaintainerList{})}); err != nil {
						t.Fatal(err)
					}
				}
				close(st.release)
				select {
				case result := <-done:
					if eventlog.Code(result.err) != want {
						t.Fatalf("%s after %s: %+v %v", method, change, result.value, result.err)
					}
					raw, _ := json.Marshal(result.value)
					if strings.Contains(string(raw), "kept") || strings.Contains(string(raw), "new") {
						t.Fatalf("refusal returned private entry: %s", raw)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("operation did not finish after release")
				}
				if change == "revoke" {
					if method == "write" {
						b.SetUserState("owner@h", "alice@h", "active")
					} else {
						b.Manage("alice@h", Management{Name: "#a@h", Maintainers: ptr(protocol.MaintainerList{"maint@h"})})
					}
					v, err := b.EventRead("alice@h", "#a@h", "")
					if err != nil || v.Counts["alert"] != 1 || v.Rings["alert"][0].Text != "kept" {
						t.Fatalf("revoked mutation changed entries: %+v %v", v, err)
					}
				} else {
					n, _ := st.State.EventHighWater(old)
					if n != 0 || len(st.alerts[old]) != 0 {
						t.Fatalf("retired state resurrected: high %d alerts %v", n, st.alerts[old])
					}
				}
			})
		}
	}
}
func TestEventUnknownAndNonAgentTargetsDoNotRevealExistence(t *testing.T) {
	b := eventFixture(t)
	for _, name := range []string{"#missing@h", "news@h"} {
		if _, err := b.EventRead("bob@h", name, ""); eventlog.Code(err) != "forbidden" {
			t.Fatalf("nonreader learned %s: %v", name, err)
		}
	}
	b.mu.Lock()
	r := b.records[DaemonAgent]
	r.Maintainers = protocol.MaintainerList{"maint@h"}
	b.records[DaemonAgent] = r
	b.unlock()
	if _, err := b.EventRead("maint@h", DaemonAgent, ""); eventlog.Code(err) != "forbidden" {
		t.Fatalf("bus Maintainer saw daemon log: %v", err)
	}
}
func TestReplacingARecordIDDropsItsLoadedLog(t *testing.T) {
	for _, how := range []string{"commit", "restore"} {
		t.Run(how, func(t *testing.T) {
			b := eventFixture(t)
			b.EventWrite("#a@h", "#a@h", "alert", "old")
			if how == "commit" {
				b.mu.Lock()
				r := b.records["#a@h"]
				b.dropRecord(r.Name)
				b.setRecord(r.Name, r)
				err := b.commit()
				b.unlock()
				if err != nil {
					t.Fatal(err)
				}
			} else {
				snap := b.Snapshot()
				for i := range snap.Records {
					if snap.Records[i].Name == "#a@h" {
						snap.Records[i].ID = snap.NextRecordID
						snap.NextRecordID++
					}
				}
				b.Restore(snap)
			}
			if n := b.events.Loaded(); n != 0 {
				t.Fatalf("old-ID log retained after %s: %d", how, n)
			}
		})
	}
}

func TestEventReadFinalRecheckHidesDataRevokedDuringExpiryCommit(t *testing.T) {
	b := eventFixture(t)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	b.Clock(func() time.Time { return now })
	st := &eventLoadBarrier{State: memory.NewState(), alerts: map[uint32][]ports.EventEntry{}, active: map[uint32]bool{}, saveEntered: make(chan struct{}), saveRelease: make(chan struct{})}
	b.Persistence(st)
	b.EventWrite("#a@h", "#a@h", "alert", "expired")
	now = now.Add(24 * time.Hour)
	b.EventWrite("#a@h", "#a@h", "alert", "kept")
	now = now.Add(29*24*time.Hour + 12*time.Hour)
	st.saveBlock.Store(true)
	done := make(chan struct {
		v   eventlog.ReadResult
		err error
	}, 1)
	go func() {
		v, err := b.EventRead("maint@h", "#a@h", "")
		done <- struct {
			v   eventlog.ReadResult
			err error
		}{v, err}
	}()
	select {
	case <-st.saveEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("expiry snapshot did not reach commit barrier")
	}
	if _, err := b.Manage("alice@h", Management{Name: "#a@h", Maintainers: ptr(protocol.MaintainerList{})}); err != nil {
		t.Fatal(err)
	}
	close(st.saveRelease)
	select {
	case result := <-done:
		if eventlog.Code(result.err) != "forbidden" || len(result.v.Rings) != 0 {
			t.Fatalf("private data returned after revoke during commit: %+v %v", result.v, result.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("read did not finish after commit release")
	}
}

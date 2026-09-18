package core

import (
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

type durableMemory struct {
	mu               sync.Mutex
	saved            ports.Snapshot
	err              error
	entered, release chan struct{}
	first            atomic.Bool
}

func (d *durableMemory) Save(s ports.Snapshot) error {
	if d.entered != nil && d.first.CompareAndSwap(false, true) {
		close(d.entered)
		<-d.release
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.err != nil {
		return d.err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	d.saved = ports.Snapshot{}
	return json.Unmarshal(data, &d.saved)
}
func (d *durableMemory) Load() (ports.Snapshot, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.saved, true, nil
}

func durabilityFixture(t *testing.T) *Bus {
	t.Helper()
	b := New()
	b.SetDaemonOwner("admin@h")
	for _, who := range []string{"alice@h", "bob@h", "friend@h"} {
		if _, err := b.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.SetGroup("admin@h", "@readers", []string{"bob@h", "friend@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.EstablishAccounts(map[string]string{"local-user": "alice@h"}); err != nil {
		t.Fatal(err)
	}
	for _, r := range []protocol.Record{
		{Name: "svc@h", Owner: "alice@h", Allow: []string{"@readers"}},
		{Name: "topic@h", Owner: "alice@h", Kind: protocol.KindTopic, Mode: protocol.ModePubSub, Allow: []string{"*"}},
	} {
		if _, err := b.Register(r); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.Subscribe("bob@h", "topic@h", true); err != nil {
		t.Fatal(err)
	}
	if _, visible := b.Lookup("bob@h", "svc@h"); !visible {
		t.Fatal("fixture does not grant Bob access")
	}
	return b
}

type durableChange struct {
	name  string
	apply func(*Bus) error
	check func(*testing.T, *Bus)
}

func administrativeChanges() []durableChange {
	banned := func(t *testing.T, b *Bus) {
		if !errors.Is(b.Authenticate("bob@h"), ErrInactive) {
			t.Fatal("acknowledged ban was not recovered")
		}
	}
	hidden := func(t *testing.T, b *Bus) {
		if _, ok := b.Lookup("bob@h", "svc@h"); ok {
			t.Fatal("acknowledged restriction was not recovered")
		}
	}
	unsubscribed := func(t *testing.T, b *Bus) {
		r, ok := b.Lookup("alice@h", "topic@h")
		if !ok || slices.Contains(r.Subs, "bob@h") {
			t.Fatal("acknowledged unsubscribe was not recovered")
		}
	}
	return []durableChange{
		{"ban", func(b *Bus) error { _, e := b.SetUserState("admin@h", "bob@h", "banned"); return e }, banned},
		{"self-email", func(b *Bus) error { _, e := b.EditOwnEmail("bob@h", "new@example.com"); return e }, func(t *testing.T, b *Bus) {
			users := b.Users("bob@h", nil)
			if len(users) != 1 || users[0].Email != "new@example.com" {
				t.Fatalf("acknowledged self email was not recovered: %+v", users)
			}
		}},
		{"profile-ban", func(b *Bus) error {
			_, e := b.SetUser("admin@h", protocol.User{Name: "bob@h", State: "banned"}, false)
			return e
		}, banned},
		{"group-removal", func(b *Bus) error { return b.SetGroup("admin@h", "@readers", []string{"friend@h"}) }, hidden},
		{"acl", func(b *Bus) error {
			a := []string{"friend@h"}
			_, e := b.Manage("alice@h", Management{Name: "svc@h", Allow: &a})
			return e
		}, hidden},
		{"refresh-acl", func(b *Bus) error {
			_, e := b.Register(protocol.Record{Name: "svc@h", Owner: "alice@h", Allow: []string{"friend@h"}})
			return e
		}, hidden},
		{"unregister", func(b *Bus) error { return b.Unregister("svc@h", "alice@h") }, func(t *testing.T, b *Bus) {
			if _, ok := b.Lookup("alice@h", "svc@h"); ok {
				t.Fatal("acknowledged removal was not recovered")
			}
		}},
		{"unsubscribe", func(b *Bus) error { _, e := b.Subscribe("bob@h", "topic@h", false); return e }, unsubscribed},
		{"remove-subscriber", func(b *Bus) error { _, e := b.RemoveSubscriber("alice@h", "topic@h", "bob@h"); return e }, unsubscribed},
		{"configure", func(b *Bus) error { _, e := b.Configure("svc@h", "alice@h", json.RawMessage(`{"new":true}`)); return e }, func(t *testing.T, b *Bus) {
			v, e := b.Config("svc@h", "svc@h")
			if e != nil || string(v) != `{"new":true}` {
				t.Fatal("acknowledged configuration was not recovered")
			}
		}},
		{"daemon-owner", func(b *Bus) error { _, e := b.TransferDaemonOwner("admin@h", "bob@h"); return e }, func(t *testing.T, b *Bus) {
			if b.DaemonOwner() != "bob@h" || !b.IsAdministrator("admin@h") || !b.IsAdministrator("bob@h") {
				t.Fatalf("acknowledged owner transfer was not recovered: owner=%s", b.DaemonOwner())
			}
		}},
		{"account-map", func(b *Bus) error {
			_, e := b.SetAccount("admin@h", "local-user", "bob@h", false)
			return e
		}, func(t *testing.T, b *Bus) {
			view, err := b.Accounts("admin@h")
			if err != nil || len(view.Mappings) != 1 || view.Mappings[0].Principal != "bob@h" {
				t.Fatalf("acknowledged account map was not recovered: %+v, %v", view, err)
			}
		}},
	}
}

func TestAdministrativeSuccessHasAlreadyPersisted(t *testing.T) {
	for _, c := range administrativeChanges() {
		t.Run(c.name, func(t *testing.T) {
			b := durabilityFixture(t)
			d := &durableMemory{}
			b.Persistence(d)
			if err := b.Checkpoint(false); err != nil {
				t.Fatal(err)
			}
			if err := c.apply(b); err != nil {
				t.Fatal(err)
			}
			// No checkpoint after the returned success. Recover only what Save received.
			saved, _, _ := d.Load()
			recovered := New()
			recovered.Restore(saved)
			if err := recovered.EstablishDaemonOwner("admin@h"); err != nil {
				t.Fatal(err)
			}
			c.check(t, recovered)
			if err := recovered.Authenticate("friend@h"); err != nil {
				t.Fatalf("unrelated user lost standing: %v", err)
			}
		})
	}
}

func TestAdministrativeWriteFailuresCannotReturnSuccess(t *testing.T) {
	boom := errors.New("disk rejected the snapshot")
	for _, c := range administrativeChanges() {
		t.Run(c.name, func(t *testing.T) {
			b := durabilityFixture(t)
			b.Persistence(&durableMemory{err: boom})
			if err := c.apply(b); !errors.Is(err, boom) {
				t.Fatalf("failed persistence reported %v instead of failure", err)
			}
		})
	}
}

func TestOlderCheckpointCannotOverwriteAcknowledgedBan(t *testing.T) {
	b := durabilityFixture(t)
	d := &durableMemory{entered: make(chan struct{}), release: make(chan struct{})}
	b.Persistence(d)
	old := make(chan error, 1)
	go func() { old <- b.Checkpoint(false) }()
	<-d.entered
	newer := make(chan error, 1)
	go func() { _, err := b.SetUserState("admin@h", "bob@h", "banned"); newer <- err }()
	select {
	case err := <-newer:
		close(d.release)
		<-old
		t.Fatalf("administration returned before older checkpoint completed: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(d.release)
	if err := <-old; err != nil {
		t.Fatal(err)
	}
	if err := <-newer; err != nil {
		t.Fatal(err)
	}
	saved, _, _ := d.Load()
	recovered := New()
	recovered.Restore(saved)
	if err := recovered.EstablishDaemonOwner("admin@h"); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(recovered.Authenticate("bob@h"), ErrInactive) {
		t.Fatal("older checkpoint erased acknowledged ban")
	}
}

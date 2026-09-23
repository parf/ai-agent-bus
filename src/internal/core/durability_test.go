package core

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func durabilityFixture(t *testing.T, st ports.Store) *Bus {
	t.Helper()
	b := New()
	if st != nil {
		b.Persistence(st)
	}
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
		{Kind: protocol.KindAgent, Name: "svc@h", Owner: "alice@h", Allow: []string{"@readers"}},
		{Name: "topic@h", Owner: "alice@h", Kind: protocol.KindPubSub, Allow: []string{"*"}},
	} {
		if _, err := b.Register(r); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.Manage("alice@h", Management{Name: "topic@h", Subs: ptr([]string{"bob@h"})}); err != nil {
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
			_, e := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "svc@h", Owner: "alice@h", Allow: []string{"friend@h"}})
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

// recoverFrom is a fresh daemon started on what the store holds.
func recoverFrom(t *testing.T, st ports.Store) *Bus {
	t.Helper()
	saved, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	recovered := New()
	recovered.Restore(saved)
	if err := recovered.EstablishDaemonOwner("admin@h"); err != nil {
		t.Fatal(err)
	}
	return recovered
}

func TestAdministrativeSuccessHasAlreadyPersisted(t *testing.T) {
	for _, c := range administrativeChanges() {
		t.Run(c.name, func(t *testing.T) {
			d := memory.NewState()
			b := durabilityFixture(t, d)
			if err := c.apply(b); err != nil {
				t.Fatal(err)
			}
			// No flush after the returned success. Recover only what was committed.
			saved, _ := d.Load()
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
	boom := errors.New("disk rejected the commit")
	for _, c := range administrativeChanges() {
		t.Run(c.name, func(t *testing.T) {
			d := memory.NewState()
			b := durabilityFixture(t, d)
			d.Err = boom
			if err := c.apply(b); !errors.Is(err, boom) {
				t.Fatalf("failed persistence reported %v instead of failure", err)
			}
		})
	}
}

// A write whose commit fails publishes nothing: the daemon answers exactly
// as it did before the call (docs/constitution.md#persistence-and-loading).
func TestFailedCommitPublishesNothing(t *testing.T) {
	boom := errors.New("disk rejected the commit")
	d := memory.NewState()
	b := durabilityFixture(t, d)
	d.Err = boom
	if _, err := b.SetUserState("admin@h", "bob@h", "banned"); !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	if err := b.Authenticate("bob@h"); err != nil {
		t.Fatalf("a ban whose commit failed took effect: %v", err)
	}
	a := []string{"friend@h"}
	if _, err := b.Manage("alice@h", Management{Name: "svc@h", Allow: &a}); !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	if _, ok := b.Lookup("bob@h", "svc@h"); !ok {
		t.Fatal("an ACL edit whose commit failed took effect")
	}
	if err := b.Unregister("svc@h", "alice@h"); !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	if _, ok := b.Lookup("alice@h", "svc@h"); !ok {
		t.Fatal("a removal whose commit failed took effect")
	}
	if err := b.SetGroup("admin@h", "@readers", []string{"friend@h"}); !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	if _, ok := b.Lookup("bob@h", "svc@h"); !ok {
		t.Fatal("a group edit whose commit failed took effect")
	}
	// And a restart reads what was committed before the failures.
	d.Err = nil
	recovered := recoverFrom(t, d)
	if _, ok := recovered.Lookup("bob@h", "svc@h"); !ok {
		t.Fatal("the committed state was not what the store held")
	}
}

// An edit of one record commits that record and nothing else.
func TestOneEditCommitsOneRecord(t *testing.T) {
	d := &countingState{State: memory.NewState()}
	b := durabilityFixture(t, d)
	d.last = ports.Change{}
	descr := "edited"
	if _, err := b.Manage("alice@h", Management{Name: "svc@h", Descr: &descr}); err != nil {
		t.Fatal(err)
	}
	if len(d.last.Records) != 1 || d.last.Records["svc@h"] == nil || len(d.last.Users) != 0 || len(d.last.Groups) != 0 {
		t.Fatalf("one edit committed %+v", d.last)
	}
}

type countingState struct {
	*memory.State
	last ports.Change
}

func (c *countingState) Commit(ch ports.Change) error {
	c.last = ch
	return c.State.Commit(ch)
}

// A write held open in the store holds the node: a second write waits for it
// rather than committing around it.
func TestHeldCommitSerializesTheNextWrite(t *testing.T) {
	d := memory.NewState()
	b := durabilityFixture(t, d)
	d.Enter, d.Release = make(chan struct{}), make(chan struct{})
	old := make(chan error, 1)
	go func() { old <- b.SetGroup("admin@h", "@readers", []string{"bob@h", "friend@h", "alice@h"}) }()
	<-d.Enter
	newer := make(chan error, 1)
	go func() { _, err := b.SetUserState("admin@h", "bob@h", "banned"); newer <- err }()
	select {
	case err := <-newer:
		close(d.Release)
		<-old
		t.Fatalf("administration returned before the held commit completed: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(d.Release)
	if err := <-old; err != nil {
		t.Fatal(err)
	}
	if err := <-newer; err != nil {
		t.Fatal(err)
	}
	recovered := recoverFrom(t, d)
	if !errors.Is(recovered.Authenticate("bob@h"), ErrInactive) {
		t.Fatal("the held write erased the acknowledged ban")
	}
}

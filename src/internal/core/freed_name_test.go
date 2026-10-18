package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// An ignored record stays in the database for an operator, and so does its
// queue. Its name is free on the running bus, and a record registered under it
// is somebody else's: it must not find the old messages on the next start.
// See docs/constitution.md#persistence-and-loading.
func TestANameFreedByAnIgnoredRecordInheritsNoStoredQueue(t *testing.T) {
	st := memory.NewState()
	if err := st.Commit(ports.Change{Records: map[string]*protocol.Record{
		"#lost@h": {Name: "#lost@h", ID: 90, Owner: "absent@h", Kind: protocol.KindAgent, Full: protocol.OverflowStrict},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveQueues([]ports.Queue{{Name: "#lost@h", In: 1, Messages: []protocol.Envelope{{To: "#lost@h", Body: "for the old owner"}}}}, nil, true); err != nil {
		t.Fatal(err)
	}
	b := New()
	b.Journal(&reports{})
	b.Persistence(st)
	saved, _ := st.Load()
	b.Restore(saved)
	b.SetDaemonOwner("owner@h")
	if _, err := b.SetUser("owner@h", protocol.User{Name: "keeper@h"}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "#lost@h", Owner: "keeper@h", Kind: protocol.KindAgent}); err != nil {
		t.Fatal(err)
	}
	after, _ := st.Load()
	for _, q := range after.Queues {
		if q.Name == "#lost@h" {
			t.Fatalf("the new #lost@h inherits the ignored record's stored queue: %+v", q)
		}
	}
	recovered := New()
	rep := &reports{}
	recovered.Journal(rep)
	recovered.Restore(after)
	if r, ok := recovered.Lookup("keeper@h", "#lost@h"); !ok || r.Owner != "keeper@h" || r.Queued != 0 {
		t.Fatalf("the restarted #lost@h is %+v (found %v), want keeper@h's with nothing queued; reports %v", r, ok, rep.lines)
	}
}

// Every record is owned by a User. An Agent that creates one registers it for
// its own Owner and gains no authority over it by doing so; only an explicit
// Maintainer entry grants that. See docs/constitution.md#-registry-record.
func TestAnAgentThatCreatesARecordOwnsNothingAndManagesNothing(t *testing.T) {
	b := New()
	provision(t, b, []string{"alice@h"}, protocol.Record{Name: "#maker@h", Kind: protocol.KindAgent, Owner: "alice@h"})
	r, err := b.Register(protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "#maker@h"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Owner != "alice@h" {
		t.Fatalf("a record an agent created is owned by %q, want its User alice@h", r.Owner)
	}
	descr := "changed"
	if _, err := b.Manage("#maker@h", Management{Name: "jobs@h", Descr: &descr}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("the creating agent managed the record: %v", err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", Maintainers: &protocol.MaintainerList{"#maker@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("#maker@h", Management{Name: "jobs@h", Descr: &descr}); err != nil {
		t.Fatalf("an explicit Maintainer entry did not grant management: %v", err)
	}
}

// A write the store refuses is reported to the error log, as well as refused.
func TestAFailedCommitIsReported(t *testing.T) {
	b := New()
	rep := &reports{}
	b.Journal(rep)
	st := memory.NewState()
	b.Persistence(st)
	b.SetDaemonOwner("owner@h")
	st.Err = errors.New("database or disk is full")
	if _, err := b.SetUser("owner@h", protocol.User{Name: "new@h"}, true); err == nil {
		t.Fatal("a refused commit was reported as done")
	}
	if !rep.has("error: a management write was not committed and nothing changed: database or disk is full") {
		t.Fatalf("the failed commit was not reported: %v", rep.lines)
	}
}

// A fresh database has had no run to lose, so it is not an unclean stop; one
// written by a run that never stopped gracefully is.
func TestAFreshDatabaseIsNotAnUncleanStop(t *testing.T) {
	b := New()
	b.Restore(ports.Snapshot{})
	if b.Status().Unclean {
		t.Fatal("a fresh database reads as an unclean stop")
	}
	c := New()
	c.Restore(ports.Snapshot{At: time.Now()})
	if !c.Status().Unclean {
		t.Fatal("a run that did not stop gracefully reads as clean")
	}
}

// A queue flush skips a queue whose counters match the last saved mark. A name
// removed and registered again starts its counters from zero, so it can reach
// the old name's mark with different messages; the removal forgets the mark,
// or the new messages are never saved and are lost at the next start.
func TestARegisteredAgainNameIsFlushedAtTheOldMark(t *testing.T) {
	st := memory.NewState()
	b := New()
	b.Persistence(st)
	b.SetDaemonOwner("owner@h")
	if _, err := b.SetUser("owner@h", protocol.User{Name: "keeper@h"}, true); err != nil {
		t.Fatal(err)
	}
	box := protocol.Record{Name: "#box@h", Owner: "keeper@h", Kind: protocol.KindAgent, Allow: []string{"*"}}
	if _, err := b.Register(box); err != nil {
		t.Fatal(err)
	}
	send := func(body string) {
		t.Helper()
		if _, err := b.Send(protocol.Envelope{From: "keeper@h", To: "#box@h", Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	// The mark is one in, none out.
	send("old")
	if err := b.FlushQueues(false); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Consume(context.Background(), "#box@h", "", "", false, false); err != nil {
		t.Fatal(err)
	}
	if err := b.Unregister("#box@h", "keeper@h"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(box); err != nil {
		t.Fatal(err)
	}
	// One in, none out again: the same counters, a different message.
	send("new")
	if err := b.FlushQueues(false); err != nil {
		t.Fatal(err)
	}
	saved, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	back := New()
	back.Restore(saved)
	// Bounded: a restore that lost the message leaves nothing to take, and an
	// unbounded read would wait for ever instead of failing.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	e, err := back.Consume(ctx, "#box@h", "", "", false, false)
	if err != nil {
		t.Fatalf("the restored #box@h holds nothing to take: %v", err)
	}
	if e.Body != "new" {
		t.Fatalf("the restored #box@h holds %q, want the message sent after it was registered again", e.Body)
	}
}

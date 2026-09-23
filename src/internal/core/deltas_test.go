package core

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func deltaFixture(t *testing.T) *Bus {
	t.Helper()
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "bob@h", "carol@h", "maint@h")
	provision(t, b, nil,
		protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "alice@h", Allow: []string{"bob@h"}, Maintainers: protocol.MaintainerList{"maint@h"}, Full: protocol.OverflowStrict},
		protocol.Record{Name: "#box@h", Kind: protocol.KindAgent, Owner: "alice@h", Full: protocol.OverflowStrict},
		protocol.Record{Name: "#other@h", Kind: protocol.KindAgent, Owner: "alice@h", Full: protocol.OverflowStrict},
	)
	return b
}

func allowOf(b *Bus, name string) []string {
	r := b.records[name]
	out := append([]string{}, r.Allow...)
	sort.Strings(out)
	return out
}

// add, add_to_set and remove, each on the list as the write finds it.
func TestListDeltas(t *testing.T) {
	b := deltaFixture(t)
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", Add: &ListDelta{Allow: []string{"carol@h"}}}); err != nil {
		t.Fatal(err)
	}
	if got := allowOf(b, "jobs@h"); fmt.Sprint(got) != "[bob@h carol@h]" {
		t.Fatalf("add left %v", got)
	}
	// add refuses a term already there, and the whole delta changes nothing.
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", Add: &ListDelta{Allow: []string{"maint@h", "bob@h"}}}); !errors.Is(err, ErrBusy) {
		t.Fatalf("add of a present term: %v", err)
	}
	if got := allowOf(b, "jobs@h"); fmt.Sprint(got) != "[bob@h carol@h]" {
		t.Fatalf("a refused mixed delta changed the list to %v", got)
	}
	// add_to_set adds what is absent and passes over what is present.
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", AddToSet: &ListDelta{Allow: []string{" BOB@h ", "maint@h"}}}); err != nil {
		t.Fatalf("add_to_set: %v", err)
	}
	if got := allowOf(b, "jobs@h"); fmt.Sprint(got) != "[bob@h carol@h maint@h]" {
		t.Fatalf("add_to_set left %v", got)
	}
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", AddToSet: &ListDelta{Allow: []string{"bob@h"}}}); err != nil {
		t.Fatalf("add_to_set of a present term: %v", err)
	}
	// remove takes what is there and is a no-op for what is not.
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", Remove: &ListDelta{Allow: []string{"carol@h", "nobody@h"}}}); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if got := allowOf(b, "jobs@h"); fmt.Sprint(got) != "[bob@h maint@h]" {
		t.Fatalf("remove left %v", got)
	}
	// A list written whole and by a delta at once is refused.
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", Allow: &[]string{"bob@h"}, Add: &ListDelta{Allow: []string{"carol@h"}}}); !errors.Is(err, ErrBadName) {
		t.Fatalf("a whole write and a delta together: %v", err)
	}
	// An invalid term refuses the lot, as a whole write does.
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", Add: &ListDelta{Allow: []string{"carol@h", "@nosuch"}}}); !errors.Is(err, ErrUnknown) {
		t.Fatalf("a delta with an unknown group: %v", err)
	}
	if got := allowOf(b, "jobs@h"); fmt.Sprint(got) != "[bob@h maint@h]" {
		t.Fatalf("an invalid delta changed the list to %v", got)
	}
}

// A delta is checked with the authority a whole write needs.
func TestADeltaNeedsTheAuthorityAWholeWriteNeeds(t *testing.T) {
	b := deltaFixture(t)
	if _, err := b.Manage("maint@h", Management{Name: "jobs@h", AddToSet: &ListDelta{Allow: []string{"carol@h"}}}); err != nil {
		t.Fatalf("a Maintainer could not add to the ACL: %v", err)
	}
	if _, err := b.Manage("maint@h", Management{Name: "jobs@h", Add: &ListDelta{Maintainers: []string{"carol@h"}}}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("a Maintainer changed the Maintainers by a delta: %v", err)
	}
	if _, err := b.Manage("carol@h", Management{Name: "jobs@h", AddToSet: &ListDelta{Allow: []string{"carol@h"}}}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("a stranger admitted herself by a delta: %v", err)
	}
	if _, err := b.Manage("admin@h", Management{Name: AdministratorsGroup, AddToSet: &ListDelta{Allow: []string{"carol@h"}}}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("the administrators took a membership delta: %v", err)
	}
	// And Personal still holds against the list the delta leaves.
	provision(t, b, nil, protocol.Record{Name: "#mine@h", Kind: protocol.KindAgent, Owner: "alice@h", Personal: true, Full: protocol.OverflowStrict})
	if _, err := b.Manage("alice@h", Management{Name: "#mine@h", AddToSet: &ListDelta{Allow: []string{"*"}}}); !errors.Is(err, ErrPersonal) {
		t.Fatalf("a delta opened a Personal record to everyone: %v", err)
	}
}

// One slot: add only when empty, add_to_set of the occupant is a no-op,
// remove clears it, and replacement is a whole-field write.
func TestTheOneSlotDeliverTo(t *testing.T) {
	b := deltaFixture(t)
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", Add: &ListDelta{Subs: []string{"#box@h"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", Add: &ListDelta{Subs: []string{"#other@h"}}}); !errors.Is(err, ErrBusy) {
		t.Fatalf("an occupied slot took a second destination: %v", err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", AddToSet: &ListDelta{Subs: []string{"#box@h"}}}); err != nil {
		t.Fatalf("add_to_set of the occupant: %v", err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", AddToSet: &ListDelta{Subs: []string{"#other@h"}}}); !errors.Is(err, ErrBusy) {
		t.Fatalf("add_to_set replaced the occupant: %v", err)
	}
	if got := b.records["jobs@h"].Subs; len(got) != 1 || got[0] != "#box@h" {
		t.Fatalf("the slot holds %v", got)
	}
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", Subs: &[]string{"#other@h"}}); err != nil {
		t.Fatalf("a whole-field replacement: %v", err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", Remove: &ListDelta{Subs: []string{"#other@h"}}}); err != nil {
		t.Fatal(err)
	}
	if got := b.records["jobs@h"].Subs; len(got) != 0 {
		t.Fatalf("remove left %v in the slot", got)
	}
}

// Concurrent adds cannot overwrite one another.
func TestConcurrentAddsLoseNothing(t *testing.T) {
	b := deltaFixture(t)
	var names []string
	for i := 0; i < 20; i++ {
		n := fmt.Sprintf("u%d@h", i)
		names = append(names, n)
	}
	known(t, b, names...)
	var wg sync.WaitGroup
	errs := make(chan error, len(names))
	for _, n := range names {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			_, err := b.Manage("alice@h", Management{Name: "jobs@h", Add: &ListDelta{Allow: []string{n}}})
			errs <- err
		}(n)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := allowOf(b, "jobs@h"); len(got) != 21 {
		t.Fatalf("concurrent adds left %d terms, want 21: %v", len(got), got)
	}
}

// A 📣 list delivers into a queue's inbox and through a further topic, and
// two topics listing each other end with an error rather than looping.
func TestAPubSubDeliversToQueuesAndThroughTopics(t *testing.T) {
	b := deltaFixture(t)
	provision(t, b, nil,
		protocol.Record{Name: "outer@h", Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"*"}},
		protocol.Record{Name: "inner@h", Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"*"}},
	)
	if _, err := b.Manage("alice@h", Management{Name: "outer@h", Subs: &[]string{"jobs@h", "inner@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "inner@h", Subs: &[]string{"#box@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "outer@h", Body: "news"}); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"jobs@h", "#box@h"} {
		if in := b.inboxes[n]; in == nil || len(in.queue) != 1 || in.queue[0].Body != "news" {
			t.Fatalf("%s did not get the copy", n)
		}
	}
	// A loop: inner lists outer as well, and the box is taken off.
	if _, err := b.Manage("alice@h", Management{Name: "inner@h", Subs: &[]string{"outer@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "outer@h", Subs: &[]string{"inner@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "outer@h", Body: "loop"}); !errors.Is(err, ErrForwards) {
		t.Fatalf("two topics listing each other: %v, want the forward limit", err)
	}
}

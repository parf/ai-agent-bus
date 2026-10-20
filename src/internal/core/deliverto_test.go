package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// maintainedBy makes manager a Maintainer of each named record or group, which
// is what lets it put them on a Deliver-To list (docs/04-messaging.md#deliver-to).
func maintainedBy(t *testing.T, b *Bus, manager string, names ...string) {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, n := range names {
		r, ok := b.records[n]
		if !ok {
			t.Fatalf("no record %s to maintain", n)
		}
		r.Maintainers = append(append(protocol.MaintainerList(nil), r.Maintainers...), manager)
		b.records[n] = r
	}
}

// A group on the list stands for its members at the publish, so somebody
// added to the group afterwards receives without the list being touched —
// which is the whole reason the group is stored rather than flattened.
// See docs/04-messaging.md#deliver-to.
func TestAGroupOnDeliverToIsExpandedWhenThePublishHappens(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "a@h", "pub@h", "#early@h", "#late@h")
	if err := b.SetGroup("admin@h", "@team", []string{"#early@h"}); err != nil {
		t.Fatal(err)
	}
	mustRegister(t, b, protocol.Record{Name: "news@h", Kind: protocol.KindPubSub, Owner: "a@h", Allow: []string{"*"}})
	maintainedBy(t, b, "a@h", "@team", "#early@h", "#late@h")
	ownedBy(t, b, "a@h", "#early@h", "#late@h")
	delivers(t, b, "a@h", "news@h", "@team")

	if _, err := b.Send(protocol.Envelope{From: "pub@h", To: "news@h", Body: "first"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	// Added to the group, not to the list, and between two publications.
	if err := b.SetGroup("admin@h", "@team", []string{"#early@h", "#late@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "pub@h", To: "news@h", Body: "second"}); err != nil {
		t.Fatalf("second publish: %v", err)
	}
	for _, c := range []struct {
		name   string
		queued int
	}{{"#early@h", 2}, {"#late@h", 1}} {
		r, ok := b.Lookup("admin@h", c.name)
		if !ok {
			t.Fatalf("%s is gone", c.name)
		}
		if r.Queued != c.queued {
			t.Fatalf("%s holds %d copies, want %d", c.name, r.Queued, c.queued)
		}
	}
	// The stored list is still the group: expansion is not a rewrite.
	if r, _ := b.Lookup("a@h", "news@h"); len(r.Subs) != 1 || r.Subs[0] != "@team" {
		t.Fatalf("the stored list became %v", r.Subs)
	}
}

// The same inbox reached through two groups and its own name is one copy, not
// three: a name that appears twice on the expanded list would be delivered to
// twice, and the publisher is told nothing either way.
func TestANameOnDeliverToTwiceOverGetsOneCopy(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "a@h", "pub@h", "#both@h")
	for _, g := range []string{"@left", "@right"} {
		if err := b.SetGroup("admin@h", g, []string{"#both@h"}); err != nil {
			t.Fatal(err)
		}
	}
	mustRegister(t, b, protocol.Record{Name: "news@h", Kind: protocol.KindPubSub, Owner: "a@h", Allow: []string{"*"}})
	maintainedBy(t, b, "a@h", "@left", "#both@h", "@right")
	ownedBy(t, b, "a@h", "#both@h")
	delivers(t, b, "a@h", "news@h", "@left", "#both@h", "@right")
	if _, err := b.Send(protocol.Envelope{From: "pub@h", To: "news@h", Body: "x"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if r, _ := b.Lookup("admin@h", "#both@h"); r.Queued != 1 {
		t.Fatalf("one publication landed %d times", r.Queued)
	}
}

// Nothing puts itself on a Deliver-To list. Delivery stopped consulting the
// topic's ACL, so a name adding itself would be answering to nobody at all.
func TestANameCannotPutItselfOnADeliverToList(t *testing.T) {
	b := New()
	known(t, b, "a@h", "#eager@h")
	mustRegister(t, b, protocol.Record{Name: "news@h", Kind: protocol.KindPubSub, Owner: "a@h", Allow: []string{"*"}})

	if _, err := b.Subscribe("#eager@h", "news@h", true); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("joining: err = %v, want ErrNotOwner", err)
	}
	if _, err := b.Manage("#eager@h", Management{Name: "news@h", Subs: &[]string{"#eager@h"}}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("writing the list: err = %v, want ErrNotOwner", err)
	}
	if r, _ := b.Lookup("a@h", "news@h"); len(r.Subs) != 0 {
		t.Fatalf("the list is %v", r.Subs)
	}
	// And the publication proves it rather than the field: the ACL admits
	// everyone, which is exactly what no longer decides delivery.
	// The list is empty, so the publication is refused: nothing takes it.
	if _, err := b.Send(protocol.Envelope{From: "a@h", To: "news@h", Body: "x"}); !errors.Is(err, ErrUnknown) {
		t.Fatalf("publish to an empty list: %v", err)
	}
	if r, _ := b.Lookup("a@h", "#eager@h"); r.Queued != 0 {
		t.Fatalf("a name the ACL admits was delivered to %d times", r.Queued)
	}
}

// Leaving is yours, because it is your inbox that fills — including when the
// topic's ACL never admitted you, which would otherwise hide the channel
// from the one name that most needs to stop it.
func TestARecipientTheACLDoesNotAdmitCanStillTakeItselfOff(t *testing.T) {
	b := New()
	known(t, b, "a@h", "#outsider@h")
	mustRegister(t, b, protocol.Record{Name: "news@h", Kind: protocol.KindPubSub, Owner: "a@h"})
	maintainedBy(t, b, "a@h", "#outsider@h")
	delivers(t, b, "a@h", "news@h", "#outsider@h")
	if _, ok := b.Lookup("#outsider@h", "news@h"); ok {
		t.Fatal("the fixture admits the outsider, so this proves nothing")
	}
	if _, err := b.Subscribe("#outsider@h", "news@h", false); err != nil {
		t.Fatalf("leaving: %v", err)
	}
	if r, _ := b.Lookup("a@h", "news@h"); len(r.Subs) != 0 {
		t.Fatalf("the list is still %v", r.Subs)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	// The list is empty, so the publication is refused: nothing takes it.
	if _, err := b.Send(protocol.Envelope{From: "a@h", To: "news@h", Body: "x"}); !errors.Is(err, ErrUnknown) {
		t.Fatalf("publish to an empty list: %v", err)
	}
	if _, err := b.Consume(ctx, "#outsider@h", "", "", false, false); err == nil {
		t.Fatal("a copy arrived after leaving")
	}
}

// Removing a name that receives through a group takes nothing out of the
// list, so reporting a removal would be a lie: the copies would keep coming.
func TestLeavingIsRefusedWhenTheDeliveryComesThroughAGroup(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "a@h", "#member@h")
	if err := b.SetGroup("admin@h", "@team", []string{"#member@h"}); err != nil {
		t.Fatal(err)
	}
	mustRegister(t, b, protocol.Record{Name: "news@h", Kind: protocol.KindPubSub, Owner: "a@h", Allow: []string{"*"}})
	maintainedBy(t, b, "a@h", "@team", "#member@h")
	ownedBy(t, b, "a@h", "#member@h")
	delivers(t, b, "a@h", "news@h", "@team")
	_, err := b.Subscribe("#member@h", "news@h", false)
	if err == nil {
		t.Fatal("leaving reported success and removed nothing")
	}
	if !strings.Contains(err.Error(), "@team") {
		t.Fatalf("the refusal does not name the group: %v", err)
	}
	// Published by one holding no roles toward news@h: its Owner's copy would
	// not cross to an agent a@h does not answer for.
	if _, err := b.Send(protocol.Envelope{From: "#member@h", To: "news@h", Body: "x"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if r, _ := b.Lookup("admin@h", "#member@h"); r.Queued != 1 {
		t.Fatalf("the copy the refusal promised did not arrive: %d held", r.Queued)
	}
}

// A Deliver-To list is checked whole before any of it is stored, the way a
// Maintainer list is: a refusal that had already written half the names would
// leave a topic delivering somewhere its owner never finished asking for.
func TestADeliverToListIsRefusedWholeAndStoresNothing(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "a@h", "#good@h")
	mustRegister(t, b, protocol.Record{Name: "news@h", Kind: protocol.KindPubSub, Owner: "a@h"})
	mustRegister(t, b, protocol.Record{Name: "db@h", Kind: protocol.KindService, Owner: "a@h", Addr: "db:5432", Proto: "postgresql"})
	mustRegister(t, b, protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "a@h"})
	maintainedBy(t, b, "a@h", "#good@h")
	delivers(t, b, "a@h", "news@h", "#good@h")

	for _, c := range []struct {
		why  string
		list []string
		want error
	}{
		{"a service has no inbox", []string{"#good@h", "db@h"}, ErrBadName},
		{"a user takes no published copy", []string{"#good@h", "a@h"}, ErrBadName},
		{"nobody registered it", []string{"#good@h", "ghost@h"}, ErrUnknown},
		{"no such group", []string{"#good@h", "@nobody"}, ErrUnknown},
		{"@owner is an ACL term about publishing", []string{"#good@h", OwnerGroup}, ErrBadName},
		{"the same inbox twice", []string{"#good@h", "#good@h"}, ErrBadName},
	} {
		list := c.list
		if _, err := b.Manage("a@h", Management{Name: "news@h", Subs: &list}); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.why, err, c.want)
		}
		if r, _ := b.Lookup("a@h", "news@h"); len(r.Subs) != 1 || r.Subs[0] != "#good@h" {
			t.Fatalf("%s: a refused list left %v behind", c.why, r.Subs)
		}
	}
}

// deliver_to is a list on a 📣, one slot on an 👾 or 📮, and nothing on any
// other kind; each term is refused for its kind, never for permission
// (docs/constitution.md#common-record-fields).
func TestDeliverToFollowsTheKind(t *testing.T) {
	b := New()
	known(t, b, "a@h", "#reader@h", "#other@h")
	mustRegister(t, b, protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "a@h"})
	mustRegister(t, b, protocol.Record{Name: "news@h", Kind: protocol.KindPubSub, Owner: "a@h"})
	mustRegister(t, b, protocol.Record{Name: "db@h", Kind: protocol.KindService, Owner: "a@h", Addr: "db:1", Proto: "pg"})
	mustRegister(t, b, protocol.Record{Name: "#mine@h", Kind: protocol.KindAgent, Owner: "a@h"})
	if err := b.SetGroup("a@h", "@crew", []string{"#reader@h"}); err != nil {
		t.Fatal(err)
	}
	// A 📣 list takes an agent, a group, a queue and a pubsub.
	maintainedBy(t, b, "a@h", "#reader@h", "@crew")
	if _, err := b.Manage("a@h", Management{Name: "news@h", Subs: &[]string{"#reader@h", "@crew", "jobs@h"}}); err != nil {
		t.Fatalf("a pubsub list refused a valid set: %v", err)
	}
	// One slot on an agent or a queue: an agent, a queue or a pubsub, whose
	// allow names the source — "*" admits no queue.
	if _, err := b.Manage("a@h", Management{Name: "jobs@h", Subs: &[]string{"#reader@h"}}); !errors.Is(err, ErrNotAllow) {
		t.Fatalf("a route to an agent open only to \"*\" was stored: %v", err)
	}
	if _, err := b.Manage(fixtureOwner, Management{Name: "#reader@h", Allow: &[]string{"*", "jobs@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("a@h", Management{Name: "jobs@h", Subs: &[]string{"#reader@h"}}); err != nil {
		t.Fatalf("a queue's one slot refused an agent: %v", err)
	}
	for why, c := range map[string]struct {
		name string
		list []string
		want error
	}{
		"two destinations": {"jobs@h", []string{"#reader@h", "#other@h"}, ErrBadName},
		"a group is many":  {"jobs@h", []string{"@crew"}, ErrBadName},
		"a user":           {"#mine@h", []string{"a@h"}, ErrBadName},
		"a service":        {"#mine@h", []string{"db@h"}, ErrBadName},
		"no record":        {"#mine@h", []string{"ghost@h"}, ErrUnknown},
		"a user record":    {"a@h", []string{"#reader@h"}, ErrKind},
		"a service's slot": {"db@h", []string{"#reader@h"}, ErrKind},
	} {
		list := c.list
		if _, err := b.Manage("a@h", Management{Name: c.name, Subs: &list}); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", why, err, c.want)
		}
	}
	if r, _ := b.Lookup("a@h", "jobs@h"); len(r.Subs) != 1 || r.Subs[0] != "#reader@h" {
		t.Fatalf("a refused write changed the queue's slot: %v", r.Subs)
	}
}

func TestDeliverToGroupRequiresCurrentChannelManagerConsent(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "eve@h", "pub@h", "#allowed@h", "#foreign@h", "#direct@h")
	if err := b.SetGroup("admin@h", "@nested", []string{"#foreign@h", "#allowed@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("admin@h", "@team", []string{"@nested", "#foreign@h"}); err != nil {
		t.Fatal(err)
	}
	mustRegister(t, b, protocol.Record{Name: "news@h", Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"*"}})
	maintainedBy(t, b, "alice@h", "@team", "#allowed@h", "#direct@h")
	ownedBy(t, b, "alice@h", "#allowed@h")
	delivers(t, b, "alice@h", "news@h", "@team", "#direct@h")
	b.mu.Lock()
	direct := b.records["#direct@h"]
	direct.Maintainers = nil
	b.records[direct.Name] = direct
	b.mu.Unlock()
	if _, err := b.Send(protocol.Envelope{From: "pub@h", To: "news@h", Body: "first"}); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]int{"#allowed@h": 1, "#foreign@h": 0, "#direct@h": 1} {
		r, _ := b.Lookup("admin@h", name)
		if r.Queued != want {
			t.Fatalf("%s queued %d copies, want %d", name, r.Queued, want)
		}
	}
	b.mu.Lock()
	topic := b.records["news@h"]
	topic.Owner = "eve@h"
	topic.Maintainers = protocol.MaintainerList{"alice@h"}
	b.records[topic.Name] = topic
	b.mu.Unlock()
	if _, err := b.Send(protocol.Envelope{From: "pub@h", To: "news@h", Body: "after transfer"}); err != nil {
		t.Fatal(err)
	}
	r, _ := b.Lookup("admin@h", "#allowed@h")
	if r.Queued != 2 {
		t.Fatalf("current Maintainer did not grant group copies: %d", r.Queued)
	}
	b.mu.Lock()
	topic = b.records["news@h"]
	topic.Maintainers = nil
	b.records[topic.Name] = topic
	b.mu.Unlock()
	if _, err := b.Send(protocol.Envelope{From: "pub@h", To: "news@h", Body: "former owner"}); err != nil {
		t.Fatal(err)
	}
	r, _ = b.Lookup("admin@h", "#allowed@h")
	if r.Queued != 2 {
		t.Fatalf("former Owner still grants group copies: %d", r.Queued)
	}
}

func TestRemovedChannelMaintainerDoesNotGrantGroupCopies(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "eve@h", "pub@h", "#member@h", "#direct@h")
	if err := b.SetGroup("admin@h", "@crew", []string{"alice@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("admin@h", "@team", []string{"#member@h"}); err != nil {
		t.Fatal(err)
	}
	mustRegister(t, b, protocol.Record{Name: "news@h", Kind: protocol.KindPubSub, Owner: "eve@h", Maintainers: protocol.MaintainerList{"@crew"}, Allow: []string{"*"}})
	maintainedBy(t, b, "alice@h", "@team", "#member@h", "#direct@h")
	ownedBy(t, b, "alice@h", "#member@h")
	delivers(t, b, "alice@h", "news@h", "@team", "#direct@h")
	b.mu.Lock()
	g := b.records["@team"]
	g.Allow = append(g.Allow, "#absent@h")
	b.records[g.Name] = g
	b.mu.Unlock()
	rep := &reports{}
	b.Journal(rep)
	if _, err := b.Send(protocol.Envelope{From: "pub@h", To: "news@h", Body: "allowed"}); err != nil {
		t.Fatal(err)
	}
	if !rep.has("skipped unknown deliver-to group member #absent@h") {
		t.Fatalf("unknown member was not reported: %v", rep.lines)
	}
	if err := b.SetGroup("admin@h", "@crew", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "pub@h", To: "news@h", Body: "removed"}); err != nil {
		t.Fatal(err)
	}
	if !rep.has("skipped deliver-to group member #member@h: owner alice@h does not manage the channel") {
		t.Errorf("missing consent warning: %v", rep.lines)
	}
	r, _ := b.Lookup("admin@h", "#member@h")
	if r.Queued != 1 {
		t.Fatalf("removed Maintainer still grants group copies: %d", r.Queued)
	}
}

func ownedBy(t *testing.T, b *Bus, owner string, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, err := b.Manage("admin@h", Management{Name: name, Owner: &owner}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDaemonOwnerDoesNotConsentToGroupCopies(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "pub@h", "#root@h", "#allowed@h")
	if err := b.SetGroup("admin@h", "@team", []string{"#root@h", "#allowed@h"}); err != nil {
		t.Fatal(err)
	}
	mustRegister(t, b, protocol.Record{Name: "news@h", Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"*"}})
	maintainedBy(t, b, "alice@h", "@team")
	ownedBy(t, b, "admin@h", "#root@h")
	ownedBy(t, b, "alice@h", "#allowed@h")
	delivers(t, b, "alice@h", "news@h", "@team")
	rep := &reports{}
	b.Journal(rep)
	if _, err := b.Send(protocol.Envelope{From: "pub@h", To: "news@h", Body: "unconsented"}); err != nil {
		t.Fatal(err)
	}
	r, _ := b.Lookup("admin@h", "#root@h")
	if r.Queued != 0 {
		t.Errorf("daemon Owner override consented: %d copies", r.Queued)
	}
	if !rep.has("skipped deliver-to group member #root@h: owner admin@h does not manage the channel") {
		t.Errorf("missing consent warning: %v", rep.lines)
	}
}

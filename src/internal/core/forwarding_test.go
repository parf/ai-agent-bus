package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// sender → A → B (docs/constitution.md#-channels).
func forwardFixture(t *testing.T) *Bus {
	t.Helper()
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "bob@h", "sender@h")
	provision(t, b, nil,
		// A admits the sender; B lists A and neither the sender nor A's Owner.
		protocol.Record{Name: "#a@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"sender@h"}, Full: protocol.OverflowStrict},
		protocol.Record{Name: "#b@h", Kind: protocol.KindAgent, Owner: "bob@h", Allow: []string{"#a@h", "jobs@h"}, Full: protocol.OverflowStrict},
		protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "alice@h", Allow: []string{"sender@h"}, Full: protocol.OverflowStrict},
	)
	return b
}

func counts(b *Bus, name string) (queued, in, out, dropped int) {
	if x := b.inboxes[name]; x != nil {
		return len(x.queue), x.in, x.out, x.dropped
	}
	return 0, 0, 0, 0
}

func TestAMessageIsForwardedThroughTheOneSlot(t *testing.T) {
	for _, source := range []string{"#a@h", "jobs@h"} {
		t.Run(source, func(t *testing.T) {
			b := forwardFixture(t)
			if _, err := b.Manage("alice@h", Management{Name: source, Subs: &[]string{"#b@h"}}); err != nil {
				t.Fatalf("configuring a route B allows: %v", err)
			}
			// A caller-supplied provenance grants nothing and is replaced.
			if _, err := b.Send(protocol.Envelope{From: "sender@h", To: source, Body: "work", OriginalTo: "forged@h", Forwards: 9}); err != nil {
				t.Fatalf("forwarding: %v", err)
			}
			if q, in, out, _ := counts(b, source); q != 0 || in != 0 || out != 0 {
				t.Fatalf("the source kept or counted it: queued %d in %d out %d", q, in, out)
			}
			q, in, _, _ := counts(b, "#b@h")
			if q != 1 || in != 1 {
				t.Fatalf("the destination holds %d, in %d; want 1 and 1", q, in)
			}
			e := b.inboxes["#b@h"].queue[0]
			if e.From != "sender@h" || e.OriginalTo != source || e.Forwards != 1 || e.To != "#b@h" {
				t.Fatalf("the forwarded envelope is %+v", e)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if _, err := b.ConsumeAs(ctx, "#b@h", "#b@h", "", "", false, false); err != nil {
				t.Fatal(err)
			}
			if _, _, out, _ := counts(b, "#b@h"); out != 1 {
				t.Fatalf("the destination's out after a read is %d", out)
			}
		})
	}
}

// B must list A, before the route is stored and again at delivery; neither the
// sender's nor A's Owner's access stands in, and a revocation keeps the route.
func TestTheDestinationMustListTheForwardingRecord(t *testing.T) {
	b := forwardFixture(t)
	provision(t, b, nil, protocol.Record{Name: "#c@h", Kind: protocol.KindAgent, Owner: "bob@h", Allow: []string{"sender@h", "alice@h"}, Full: protocol.OverflowStrict})
	if _, err := b.Manage("alice@h", Management{Name: "#a@h", Subs: &[]string{"#c@h"}}); !errors.Is(err, ErrNotAllow) {
		t.Fatalf("a route to a destination that lists only the sender and A's owner was stored: %v", err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "#a@h", Subs: &[]string{"#b@h"}}); err != nil {
		t.Fatal(err)
	}
	// Revoked after it was stored: delivery is refused, the route kept.
	if _, err := b.Manage("bob@h", Management{Name: "#b@h", Allow: &[]string{"sender@h", "alice@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "sender@h", To: "#a@h", Body: "x"}); !errors.Is(err, ErrNotAllow) {
		t.Fatalf("delivery through a revoked route: %v", err)
	}
	if q, in, _, _ := counts(b, "#b@h"); q != 0 || in != 0 {
		t.Fatal("a refused forward stored or counted something")
	}
	if got := b.records["#a@h"].Subs; len(got) != 1 || got[0] != "#b@h" {
		t.Fatalf("the revocation cleared the route: %v", got)
	}
	// Granted again, it resumes without an edit.
	if _, err := b.Manage("bob@h", Management{Name: "#b@h", Allow: &[]string{"#a@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "sender@h", To: "#a@h", Body: "y"}); err != nil {
		t.Fatalf("the re-granted route did not resume: %v", err)
	}
	// And A still judges the sender.
	if _, err := b.Send(protocol.Envelope{From: "bob@h", To: "#a@h", Body: "z"}); !errors.Is(err, ErrNotAllow) {
		t.Fatalf("a sender A denies was forwarded: %v", err)
	}
}

// The destination's rules apply as for a direct send, and an inactive or
// absent destination refuses the sender with nothing stored or counted.
func TestTheDestinationsRulesDecideAForward(t *testing.T) {
	b := forwardFixture(t)
	if _, err := b.Manage("alice@h", Management{Name: "#a@h", Subs: &[]string{"#b@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("bob@h", Management{Name: "#b@h", Bound: ptr(1)}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "sender@h", To: "#a@h", Body: "1"}); err != nil {
		t.Fatal(err)
	}
	_, in, _, dropped := counts(b, "#b@h")
	if _, err := b.Send(protocol.Envelope{From: "sender@h", To: "#a@h", Body: "2"}); !errors.Is(err, ErrFull) {
		t.Fatalf("strict overflow at the destination: %v", err)
	}
	if _, in2, _, d2 := counts(b, "#b@h"); in2 != in || d2 != dropped {
		t.Fatal("a strict refusal changed a counter")
	}
	if _, err := b.Manage("bob@h", Management{Name: "#b@h", Full: ptr(protocol.OverflowRing)}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "sender@h", To: "#a@h", Body: "3"}); err != nil {
		t.Fatal(err)
	}
	if q, _, _, d := counts(b, "#b@h"); q != 1 || d != dropped+1 || b.inboxes["#b@h"].queue[0].Body != "3" {
		t.Fatalf("ring overflow: queued %d dropped %d", q, d)
	}
	if _, _, _, d := counts(b, "#a@h"); d != 0 {
		t.Fatal("the forwarding record counted an overflow")
	}
	if _, err := b.Manage("bob@h", Management{Name: "#b@h", Status: ptr(protocol.StatusInactive)}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "sender@h", To: "#a@h", Body: "4"}); !errors.Is(err, ErrUnknown) {
		t.Fatalf("a forward to an inactive destination: %v", err)
	}
}

// Ten forwarding steps and no more.
func TestTheEleventhStepIsRefused(t *testing.T) {
	for _, hops := range []int{maxForwards, maxForwards + 1} {
		t.Run(fmt.Sprint(hops), func(t *testing.T) {
			b := New()
			known(t, b, "alice@h")
			// n0 → n1 → … → n{hops}: hops forwarding steps.
			for i := hops; i >= 0; i-- {
				r := protocol.Record{Name: fmt.Sprintf("#n%d@h", i), Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict}
				if i < hops {
					r.Subs = []string{fmt.Sprintf("#n%d@h", i+1)}
				}
				provision(t, b, nil, r)
			}
			_, err := b.Send(protocol.Envelope{From: "alice@h", To: "#n0@h", Body: "x"})
			last := fmt.Sprintf("#n%d@h", hops)
			q, _, _, _ := counts(b, last)
			if hops == maxForwards && (err != nil || q != 1) {
				t.Fatalf("ten steps were refused: %v", err)
			}
			if hops > maxForwards && (!errors.Is(err, ErrForwards) || q != 0) {
				t.Fatalf("an eleventh step was taken: %v, %d stored", err, q)
			}
		})
	}
}

// A queue routing back into the topic that feeds it ends with an error.
func TestAQueueRoutingBackIntoItsTopicEnds(t *testing.T) {
	b := New()
	known(t, b, "alice@h")
	provision(t, b, nil,
		protocol.Record{Name: "feed@h", Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"*"}, Subs: []string{"loopq@h"}},
		protocol.Record{Name: "loopq@h", Kind: protocol.KindQueue, Owner: "alice@h", Allow: []string{"*"}, Subs: []string{"feed@h"}, Full: protocol.OverflowStrict},
	)
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "feed@h", Body: "x"}); !errors.Is(err, ErrForwards) {
		t.Fatalf("a queue looping into its topic: %v", err)
	}
}

// A topic's in and out count routing: accepted publications and accepted
// copies, never reads (docs/constitution.md#pubsub-routing).
func TestPubSubRoutingCounters(t *testing.T) {
	cases := []struct {
		name            string
		inactive, full  int
		wantIn, wantOut int
		wantErr         bool
	}{
		{"three accepted", 0, 0, 1, 3, false},
		{"two and one failed", 1, 0, 1, 2, false},
		{"all failed", 3, 0, 0, 0, true},
		{"one full, two inactive", 2, 1, 0, 0, true},
	}
	for _, c := range cases {
		for _, order := range [][]int{{0, 1, 2}, {2, 1, 0}, {1, 2, 0}} {
			t.Run(fmt.Sprint(c.name, order), func(t *testing.T) {
				b := New()
				rep := &reports{}
				b.Journal(rep)
				known(t, b, "alice@h")
				var subs []string
				for _, i := range order {
					n := fmt.Sprintf("#r%d@h", i)
					r := protocol.Record{Name: n, Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict}
					if i < c.inactive {
						r.Status = protocol.StatusInactive
					} else if i < c.inactive+c.full {
						r.Bound = 1
					}
					provision(t, b, nil, r)
					subs = append(subs, n)
				}
				provision(t, b, nil, protocol.Record{Name: "news@h", Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"*"}, Subs: subs})
				for i := 0; i < 3; i++ {
					if i >= c.inactive && i < c.inactive+c.full {
						n := fmt.Sprintf("#r%d@h", i)
						if err := b.route(b.records[n], protocol.Envelope{To: n, Body: "fill"}); err != nil {
							t.Fatal(err)
						}
					}
				}
				_, err := b.Send(protocol.Envelope{From: "alice@h", To: "news@h", Body: "secret body"})
				if (err != nil) != c.wantErr {
					t.Fatalf("publish: %v, want error %v", err, c.wantErr)
				}
				_, in, out, _ := counts(b, "news@h")
				if in != c.wantIn || out != c.wantOut {
					t.Fatalf("in %d out %d, want %d and %d", in, out, c.wantIn, c.wantOut)
				}
				if c.inactive > 0 && !c.wantErr && !rep.has("was not delivered to") {
					t.Fatalf("a partial failure was not warned: %v", rep.lines)
				}
				for _, l := range rep.lines {
					if strings.Contains(l, "secret body") {
						t.Fatalf("a warning carried the body: %s", l)
					}
				}
			})
		}
	}
	// No recipients: refused, and nothing counted.
	b := New()
	known(t, b, "alice@h")
	provision(t, b, nil, protocol.Record{Name: "empty@h", Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"*"}})
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "empty@h", Body: "x"}); err == nil {
		t.Fatal("a publication to no recipients was accepted")
	}
	if _, in, out, _ := counts(b, "empty@h"); in != 0 || out != 0 {
		t.Fatalf("an empty publication counted in %d out %d", in, out)
	}
	// Reading an accepted copy moves the recipient's out, never the topic's.
	provision(t, b, nil, protocol.Record{Name: "#r@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict})
	if _, err := b.Manage("alice@h", Management{Name: "empty@h", Subs: &[]string{"#r@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "empty@h", Body: "x"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := b.ConsumeAs(ctx, "#r@h", "#r@h", "", "", false, false); err != nil {
		t.Fatal(err)
	}
	if _, in, out, _ := counts(b, "empty@h"); in != 1 || out != 1 {
		t.Fatalf("after a read the topic counts in %d out %d, want 1 and 1", in, out)
	}
}

// A face can tell a configured route from an allowed one.
func TestAListingSaysWhetherARouteIsAllowed(t *testing.T) {
	b := forwardFixture(t)
	if _, err := b.Manage("alice@h", Management{Name: "#a@h", Subs: &[]string{"#b@h"}}); err != nil {
		t.Fatal(err)
	}
	allowed := func() *bool { r, _ := b.Lookup("alice@h", "#a@h"); return r.RouteAllowed }
	if a := allowed(); a == nil || !*a {
		t.Fatalf("an allowed route reads %v", a)
	}
	if _, err := b.Manage("bob@h", Management{Name: "#b@h", Allow: &[]string{"sender@h"}}); err != nil {
		t.Fatal(err)
	}
	if a := allowed(); a == nil || *a {
		t.Fatalf("a revoked route reads %v", a)
	}
	if r, _ := b.Lookup("alice@h", "jobs@h"); r.RouteAllowed != nil {
		t.Fatal("a record with no route claims one")
	}
}

// A route that stops working is written to the error log; a direct send to a
// name that is not there is an ordinary refusal and is not (Q108).
func TestABrokenRouteIsLoggedAndAnUnknownNameIsNot(t *testing.T) {
	b := forwardFixture(t)
	rep := &reports{}
	b.Journal(rep)
	if _, err := b.Manage("alice@h", Management{Name: "#a@h", Subs: &[]string{"#b@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("bob@h", Management{Name: "#b@h", Allow: &[]string{"sender@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "sender@h", To: "#a@h", Body: "x"}); !errors.Is(err, ErrNotAllow) {
		t.Fatal(err)
	}
	if !rep.has("warning: the route from #a@h to #b@h is broken: #b@h no longer allows #a@h") {
		t.Fatalf("a revoked route was not logged: %v", rep.lines)
	}
	if _, err := b.Manage("bob@h", Management{Name: "#b@h", Allow: &[]string{"#a@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("bob@h", Management{Name: "#b@h", Status: ptr(protocol.StatusInactive)}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "sender@h", To: "#a@h", Body: "y"}); !errors.Is(err, ErrUnknown) {
		t.Fatal(err)
	}
	if !rep.has("the route from #a@h to #b@h is broken: #b@h is absent or inactive") {
		t.Fatalf("a route to an inactive destination was not logged: %v", rep.lines)
	}
	n := len(rep.lines)
	if _, err := b.Send(protocol.Envelope{From: "sender@h", To: "nobody@h", Body: "z"}); !errors.Is(err, ErrUnknown) {
		t.Fatal(err)
	}
	if len(rep.lines) != n {
		t.Fatalf("a send to an unknown name was logged: %v", rep.lines[n:])
	}
}

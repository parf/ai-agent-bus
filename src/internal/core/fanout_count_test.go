package core

import (
	"errors"
	"fmt"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// 📣 channels that all list each other and one sink: a Deliver-To loop. The
// send fails at the eleventh 📣 it passes through, where every channel used
// to route it again on every branch, 29,524 copies for three of them.
func TestPubSubsListingEachOtherFailTheSend(t *testing.T) {
	b := New()
	known(t, b, "alice@h")
	const k = 3
	recs := []protocol.Record{{Name: "#sink@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict}}
	for i := 0; i < k; i++ {
		subs := []string{"#sink@h"}
		for j := 0; j < k; j++ {
			if j != i {
				subs = append(subs, fmt.Sprintf("p%d@h", j))
			}
		}
		recs = append(recs, protocol.Record{Name: fmt.Sprintf("p%d@h", i), Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"*"}, Subs: subs})
	}
	provision(t, b, nil, recs...)
	rep := &reports{}
	b.Journal(rep)
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "p0@h", Body: "x"}); !errors.Is(err, ErrForwards) {
		t.Fatalf("a deliver-to loop: %v, want the send refused", err)
	}
	if q, _, _, _ := counts(b, "#sink@h"); q > maxForwards {
		t.Fatalf("the sink holds %d copies of one send", q)
	}
	if !rep.has("deliver-to loop") {
		t.Fatal("the loop was not reported")
	}
	// The count is the one send's: the next one through a 📣 still goes.
	provision(t, b, nil, protocol.Record{Name: "solo@h", Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"*"}, Subs: []string{"#sink@h"}})
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "solo@h", Body: "y"}); err != nil {
		t.Fatalf("the next send: %v", err)
	}
}

// Channels side by side under one count too: ten in all go, eleven fail the
// send although no chain is longer than two.
func TestTheCountIsEveryPubSubOfTheSend(t *testing.T) {
	for _, wide := range []int{maxForwards - 1, maxForwards} {
		t.Run(fmt.Sprint(wide), func(t *testing.T) {
			b := New()
			known(t, b, "alice@h")
			top := protocol.Record{Name: "top@h", Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"*"}}
			recs := []protocol.Record{{Name: "#sink@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict}}
			for i := 0; i < wide; i++ {
				n := fmt.Sprintf("p%d@h", i)
				top.Subs = append(top.Subs, n)
				recs = append(recs, protocol.Record{Name: n, Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"*"}, Subs: []string{"#sink@h"}})
			}
			provision(t, b, nil, append(recs, top)...)
			_, err := b.Send(protocol.Envelope{From: "alice@h", To: "top@h", Body: "x"})
			if wide+1 <= maxForwards && err != nil {
				t.Fatalf("%d channels in all: %v", wide+1, err)
			}
			if wide+1 > maxForwards && !errors.Is(err, ErrForwards) {
				t.Fatalf("%d channels in all: %v, want the send refused", wide+1, err)
			}
		})
	}
}

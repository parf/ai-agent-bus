package core

import (
	"fmt"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// 📣 channels that all list each other and one sink: a send reaches the
// sink once, where every channel used to route it again on every branch,
// 29,524 copies for three of them.
func TestPubSubsListingEachOtherDeliverOnce(t *testing.T) {
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
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "p0@h", Body: "x"}); err != nil {
		t.Fatal(err)
	}
	if q, in, _, d := counts(b, "#sink@h"); q != 1 || in != 1 || d != 0 {
		t.Fatalf("the sink holds %d, took %d, dropped %d; want one copy", q, in, d)
	}
}

// A name reached by two paths takes one copy, and the second path is
// neither a failure nor a drop: through a further 📣 and through a queue's
// route alike.
func TestASecondPathToARecipientIsNoFailure(t *testing.T) {
	b := New()
	known(t, b, "alice@h")
	provision(t, b, nil,
		protocol.Record{Name: "#a@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*", "q@h"}, Full: protocol.OverflowStrict},
		protocol.Record{Name: "inner@h", Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"*"}, Subs: []string{"#a@h"}},
		protocol.Record{Name: "q@h", Kind: protocol.KindQueue, Owner: "alice@h", Allow: []string{"*"}, Subs: []string{"#a@h"}, Full: protocol.OverflowStrict},
		protocol.Record{Name: "outer@h", Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"*"}, Subs: []string{"#a@h", "inner@h", "q@h"}},
	)
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "outer@h", Body: "x"}); err != nil {
		t.Fatal(err)
	}
	if q, _, _, _ := counts(b, "#a@h"); q != 1 {
		t.Fatalf("#a holds %d copies, want one", q)
	}
	for _, n := range []string{"#a@h", "inner@h", "q@h", "outer@h"} {
		if _, _, _, d := counts(b, n); d != 0 {
			t.Fatalf("%s counted %d drops for a name that already had the copy", n, d)
		}
	}
	// The reached set is the one send's: the next send delivers again.
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "outer@h", Body: "y"}); err != nil {
		t.Fatal(err)
	}
	if q, _, _, _ := counts(b, "#a@h"); q != 2 {
		t.Fatalf("#a holds %d after a second send, want two", q)
	}
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "#a@h", Body: "z"}); err != nil {
		t.Fatalf("a direct send after publications: %v", err)
	}
}

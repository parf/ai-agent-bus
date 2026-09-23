package core

import (
	"context"
	"fmt"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func benchBus(tb testing.TB, backlog int) *Bus {
	b := New()
	if _, err := b.Register(protocol.Record{Name: "#sink@h", Kind: protocol.KindAgent, Owner: "#sink@h"}); err != nil {
		tb.Fatal(err)
	}
	for i := 0; i < backlog; i++ {
		if _, err := b.Send(protocol.Envelope{From: "src@h", To: "#sink@h", Body: fmt.Sprint(i)}); err != nil {
			tb.Fatal(err)
		}
	}
	return b
}

// A backlogged inbox is where taking from the head is felt: every pop shifts
// everything behind it.
func BenchmarkSendConsume(b *testing.B) {
	for _, backlog := range []int{0, 10, 50, 100, 300, 999} {
		b.Run(fmt.Sprint("backlog", backlog), func(b *testing.B) {
			bus := benchBus(b, backlog)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := bus.Send(protocol.Envelope{From: "src@h", To: "#sink@h", Body: "x"}); err != nil {
					b.Fatal(err)
				}
				if _, err := bus.Consume(context.Background(), "#sink@h", "", "", false, false); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// A full ring drops the oldest on every send, which is the same shift.
func BenchmarkSendIntoAFullRing(b *testing.B) {
	bus := New()
	if _, err := bus.Register(protocol.Record{Name: "#ringy@h", Kind: protocol.KindAgent, Owner: "#ringy@h", Full: protocol.OverflowRing}); err != nil {
		b.Fatal(err)
	}
	for i := 0; i < maxQueue; i++ {
		if _, err := bus.Send(protocol.Envelope{From: "src@h", To: "#ringy@h", Body: fmt.Sprint(i)}); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := bus.Send(protocol.Envelope{From: "src@h", To: "#ringy@h", Body: "x"}); err != nil {
			b.Fatal(err)
		}
	}
}

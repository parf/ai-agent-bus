package baseline_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func BenchmarkRoles(b *testing.B) {
	for _, mode := range []string{"plain", "direct", "nonmatching", "groups", "long_union", "fanout"} {
		b.Run(mode, func(b *testing.B) {
			bus := node(b, 1000, 0)
			target := agent(999)
			allow := []string{team}
			switch mode {
			case "direct":
				allow = []string{reader + "(deploy,read_only)", "*(guest)"}
			case "nonmatching":
				allow = []string{team, owner + "(deploy)"}
			case "groups":
				allow = []string{team + "(deploy,read_only)", "*(guest)"}
			case "fanout":
				target = "roles-news@h"
				register(b, bus, protocol.Record{Name: target, Kind: protocol.KindPubSub, Owner: owner, Allow: []string{team + "(deploy)"}, Subs: []string{agent(999)}})
			case "long_union":
				allow = nil
				for i := 0; i < 100; i++ {
					allow = append(allow, reader+"("+strings.Repeat("x", 100)+fmt.Sprint(i)+")")
				}
			}
			if mode != "fanout" {
				if _, e := bus.Manage(owner, core.Management{Name: target, Allow: &allow}); e != nil {
					b.Fatal(e)
				}
			}
			b.Run("send", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if _, e := bus.Send(protocol.Envelope{From: reader, To: target, Body: "work"}); e != nil {
						b.Fatal(e)
					}
					if _, e := bus.Consume(context.Background(), agent(999), "", "", false, false); e != nil {
						b.Fatal(e)
					}
				}
			})
			b.Run("lookup", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if _, ok := bus.Lookup(reader, target); !ok {
						b.Fatal("lookup refused")
					}
				}
			})
		})
	}
}

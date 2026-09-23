// Package baseline_test is the 0.7 K.1 performance baseline: the same
// operations measured before the SQLite store replaces the JSON snapshot, and
// again after it, at increasing registry and backlog sizes.
// See Plans/MVP/0.7.0-TODO.md#verification.
//
// It sits outside core so that core keeps importing no adapter: the
// management benchmark writes through the real JSON snapshot adapter, because
// what 0.6 pays for a one-field edit is a whole-registry rewrite.
package baseline_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/dump/jsonfile"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

const (
	owner  = "owner@h"
	reader = "reader@h"
	team   = "@team"
)

// sizes are the registry sizes K.17 compares against.
var sizes = []int{1_000, 10_000, 100_000}

func agent(i int) string { return fmt.Sprintf("a%06d@h", i) }

// node builds a registry of n agents owned by one User, each admitting the
// reader through a group, so authorization walks the membership path a real
// shared record takes rather than the owner shortcut. backlog messages are
// spread over the first agents' inboxes.
func node(tb testing.TB, n, backlog int) *core.Bus {
	tb.Helper()
	b := core.New()
	b.SetDaemonOwner(owner)
	register(tb, b, protocol.Record{Name: owner, Kind: protocol.KindUser, Owner: owner})
	register(tb, b, protocol.Record{Name: reader, Kind: protocol.KindAgent, Owner: owner})
	if err := b.SetGroup(owner, team, []string{reader}); err != nil {
		tb.Fatal(err)
	}
	for i := 0; i < n; i++ {
		register(tb, b, protocol.Record{Name: agent(i), Kind: protocol.KindAgent, Owner: owner, Allow: []string{team}})
	}
	for i := 0; i < backlog; i++ {
		if _, err := b.Send(protocol.Envelope{From: reader, To: agent(i % n), Body: "queued"}); err != nil {
			tb.Fatal(err)
		}
	}
	return b
}

func register(tb testing.TB, b *core.Bus, r protocol.Record) {
	tb.Helper()
	if _, err := b.Register(r); err != nil {
		tb.Fatal(err)
	}
}

// BenchmarkLookup is one authorized read of one record.
func BenchmarkLookup(b *testing.B) {
	for _, n := range sizes {
		b.Run(fmt.Sprint("records", n), func(b *testing.B) {
			bus := node(b, n, 0)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, ok := bus.Lookup(reader, agent(i%n)); !ok {
					b.Fatal("lookup refused")
				}
			}
		})
	}
}

// BenchmarkList is the whole caller-visible listing, which every dashboard
// registry page starts from.
func BenchmarkList(b *testing.B) {
	for _, n := range sizes {
		b.Run(fmt.Sprint("records", n), func(b *testing.B) {
			bus := node(b, n, 0)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if got := len(bus.List(reader, "")); got < n {
					b.Fatalf("listed %d of %d", got, n)
				}
			}
		})
	}
}

// BenchmarkSendConsume is one authorized send and its read, with the rest of
// the node holding a backlog.
func BenchmarkSendConsume(b *testing.B) {
	for _, n := range sizes {
		for _, backlog := range []int{0, 10_000} {
			b.Run(fmt.Sprintf("records%d/backlog%d", n, backlog), func(b *testing.B) {
				bus := node(b, n, backlog)
				target := agent(n - 1) // outside the backlogged inboxes
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err := bus.Send(protocol.Envelope{From: reader, To: target, Body: "x"}); err != nil {
						b.Fatal(err)
					}
					if _, err := bus.Consume(context.Background(), target, "", "", false, false); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// BenchmarkConcurrentSendConsume is the same pair from every CPU at once, each
// goroutine on its own inbox, so the figure is the node lock's contention
// rather than one inbox's. Run with -mutexprofile to see where it waits.
func BenchmarkConcurrentSendConsume(b *testing.B) {
	for _, n := range sizes {
		b.Run(fmt.Sprint("records", n), func(b *testing.B) {
			bus := node(b, n, 0)
			var next atomic.Int64
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				target := agent(int(next.Add(1)) % n)
				for pb.Next() {
					if _, err := bus.Send(protocol.Envelope{From: reader, To: target, Body: "x"}); err != nil {
						b.Error(err)
						return
					}
					if _, err := bus.Consume(context.Background(), target, "", "", false, false); err != nil {
						b.Error(err)
						return
					}
				}
			})
		})
	}
}

// BenchmarkManage is a one-field edit acknowledged only once durable. Through
// 0.6 that is a full snapshot through the JSON adapter, so the write volume it
// reports is the whole registry and backlog, not the edited record: the figure
// K.17 must bring down to the record itself.
func BenchmarkManage(b *testing.B) {
	for _, n := range sizes {
		for _, backlog := range []int{0, 10_000} {
			b.Run(fmt.Sprintf("records%d/backlog%d", n, backlog), func(b *testing.B) {
				bus := node(b, n, backlog)
				path := filepath.Join(b.TempDir(), "snapshot.json")
				bus.Persistence(jsonfile.New(path))
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					descr := fmt.Sprint("edit ", i)
					if _, err := bus.Manage(owner, core.Management{Name: agent(i % n), Descr: &descr}); err != nil {
						b.Fatal(err)
					}
				}
				b.StopTimer()
				st, err := os.Stat(path)
				if err != nil {
					b.Fatal(err)
				}
				b.ReportMetric(float64(st.Size()), "written-B/op")
			})
		}
	}
}

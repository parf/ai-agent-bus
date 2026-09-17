package callstats

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestCallWindowsUseActualTimesAndExistingCounter(t *testing.T) {
	var counter atomic.Uint64
	counter.Store(1 << 63)
	h := New(&counter)
	start := time.Now()
	for _, w := range h.Snapshot(start).Windows {
		if w.Available {
			t.Fatal("invented pre-sample history")
		}
	}
	h.Sample(start)
	counter.Add(2)
	h.Sample(start.Add(time.Minute))
	counter.Add(3)
	h.Sample(start.Add(time.Hour))
	counter.Add(4)
	got := h.Snapshot(start.Add(61*time.Minute + 30*time.Second))
	if got.Total != (1<<63)+9 {
		t.Fatalf("lost existing uint64 counter: %d", got.Total)
	}
	for i, want := range []struct {
		label, span string
		count       uint64
	}{{"1m", "1m30s", 4}, {"1h", "1h0m30s", 7}} {
		w := got.Windows[i]
		if !w.Available || w.Window != want.label || w.Observed != want.span || w.Count != want.count {
			t.Fatalf("window %d: %+v, want %+v", i, w, want)
		}
	}
	// A new process has new atomic state and no inherited history.
	var fresh atomic.Uint64
	newRun := New(&fresh)
	if got := newRun.Snapshot(start); got.Total != 0 || got.Windows[1].Available {
		t.Fatal("new run inherited calls")
	}
	newRun.Sample(start)
	for _, w := range newRun.Snapshot(start.Add(5 * time.Second)).Windows {
		if !w.Available || w.Count != 0 || w.Observed != "5s" {
			t.Fatalf("real zero or partial history concealed: %+v", w)
		}
	}
	fresh.Add(8)
	newRun.Sample(start) // duplicate timestamp cannot replace the baseline
	for _, w := range newRun.Snapshot(start.Add(5 * time.Second)).Windows {
		if w.Count != 8 {
			t.Fatal("duplicate sample overwrote history")
		}
	}
}

func TestCallHistoryRetainsAnHourWithoutGrowing(t *testing.T) {
	var counter atomic.Uint64
	h := New(&counter)
	start := time.Now()
	for i := 0; i <= 1500; i++ {
		counter.Add(1)
		h.Sample(start.Add(time.Duration(i) * time.Minute))
	}
	if len(h.samples) != 61 {
		t.Fatalf("retained %d samples, want an hour plus baseline", len(h.samples))
	}
	got := h.Snapshot(start.Add(1500*time.Minute + 30*time.Second)).Windows[1]
	if !got.Available || got.Count != 60 || got.Observed != "1h0m30s" {
		t.Fatalf("hour was truncated or samples unbounded: %+v", got)
	}
}

package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/eventlog"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

type stuckAlertBatch struct {
	*memory.State
	calls   atomic.Int64
	entered chan bool
	release chan struct{}
}

func (s *stuckAlertBatch) ReplaceEventAlertBatch(ctx context.Context, v []ports.EventAlertSnapshot) error {
	if s.calls.Add(1) > 20 {
		_, bounded := ctx.Deadline()
		s.entered <- bounded
		// Deliberately ignore cancellation: shutdown's lock wait must be bounded
		// independently of whether a third-party adapter cooperates.
		<-s.release
	}
	return s.State.ReplaceEventAlertBatch(ctx, v)
}

type flushWarnings struct {
	ports.Silent
	mu    sync.Mutex
	lines []string
}

func (j *flushWarnings) Report(sev ports.Severity, line string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.lines = append(j.lines, fmt.Sprintf("%s: %s", sev, line))
}

func TestAlertShutdownDeadlineReportsTheUnsavedRingCount(t *testing.T) {
	s := &stuckAlertBatch{State: memory.NewState(), entered: make(chan bool, 2), release: make(chan struct{})}
	var release sync.Once
	defer release.Do(func() { close(s.release) })
	b := New()
	j := &flushWarnings{}
	b.Journal(j)
	b.events = eventlog.New(s, s, nil)
	for id := uint32(1); id <= 21; id++ {
		if _, err := b.events.Write(id, "alert", "retained", eventlog.Defaults()); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case bounded := <-s.entered:
		if !bounded {
			t.Error("timer batch has no deadline")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timer never entered blocked batch")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- b.FlushEvents(ctx) }()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("stop error %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("shutdown exceeded its own deadline while a timer batch held the gate")
		release.Do(func() { close(s.release) })
		<-done
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if len(j.lines) != 1 || !strings.Contains(j.lines[0], "warning: alert flush at stop: 1 alert rings not saved") {
		t.Errorf("stop warnings: %v", j.lines)
	}
}

type retirementWindow struct {
	*memory.State
	afterPrune func()
}

func (s *retirementWindow) Commit(c ports.Change) error {
	if err := s.State.Commit(c); err != nil {
		return err
	}
	if s.afterPrune != nil {
		s.afterPrune()
	}
	return nil
}

// The store hook runs after the actual registry prune, before core can forget
// the catalog. A flush attempted in that exact window must not recreate rows.
func TestRetirementExcludesAPendingFlushAcrossTheStorePrune(t *testing.T) {
	s := &retirementWindow{State: memory.NewState()}
	b := subFixture(t, s)
	if _, err := b.EventWrite("#a@h", "#a@h", "alert", "old"); err != nil {
		t.Fatal(err)
	}
	id := subSource(b).ID
	for other := uint32(1000); other < 1020; other++ {
		if _, err := b.events.Write(other, "alert", "drain burst", eventlog.Defaults()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.EventWrite("#a@h", "#a@h", "alert", "dirty"); err != nil {
		t.Fatal(err)
	}
	before, _, err := s.LoadEventAlerts(id)
	if err != nil || len(before) != 1 {
		t.Fatalf("fixture did not leave a dirty snapshot: %+v %v", before, err)
	}
	s.afterPrune = func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		_ = b.events.Close(ctx)
	}
	if err := b.Unregister("#a@h", "alice@h"); err != nil {
		t.Fatal(err)
	}
	after, active, err := s.LoadEventAlerts(id)
	if err != nil || active || len(after) != 0 {
		t.Fatalf("retired state resurrected: active %v rows %+v err %v", active, after, err)
	}
	high, err := s.EventHighWater(id)
	if err != nil || high != 0 {
		t.Fatalf("retired counter: %d %v", high, err)
	}
}

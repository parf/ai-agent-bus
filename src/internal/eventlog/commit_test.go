package eventlog

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

type batchBackend struct {
	*backend
	transactions, largest int
	batchErr              error
}

func (s *batchBackend) ReplaceEventAlertBatch(batch []ports.EventAlertSnapshot) error {
	s.transactions++
	s.largest = max(s.largest, len(batch))
	if s.batchErr != nil {
		return s.batchErr
	}
	for _, v := range batch {
		if err := s.ReplaceEventAlerts(v.RecordID, v.Alerts, v.Active); err != nil {
			return err
		}
	}
	return nil
}
func batchFixture(t *testing.T) (*Engine, *batchBackend, *time.Time) {
	t.Helper()
	s := &batchBackend{backend: newBackend()}
	now := time.Unix(1800000000, 0)
	e := New(s, s, func() time.Time { return now })
	e.persistence.now = func() time.Time { return now }
	e.persistence.at = now
	e.persistence.after = func(time.Duration, func()) *time.Timer { return nil }
	t.Cleanup(func() {
		if err := e.Close(); err != nil {
			t.Error(err)
		}
	})
	return e, s, &now
}
func stopTimer(e *Engine) {
	e.persistence.mu.Lock()
	if e.persistence.timer != nil {
		e.persistence.timer.Stop()
		e.persistence.timer = nil
	}
	e.persistence.mu.Unlock()
}
func writeLogs(t *testing.T, e *Engine, first, n uint32, text string) {
	t.Helper()
	for id := first; id < first+n; id++ {
		if _, err := e.Write(id, "alert", fmt.Sprintf("%s %d", text, id), Defaults()); err != nil {
			t.Fatal(err)
		}
	}
	stopTimer(e)
}
func TestAlertCommitsShareOneDaemonWideBucketAndBatch(t *testing.T) {
	e, s, now := batchFixture(t)
	// 100 different logs cannot multiply the daemon's burst allowance.
	writeLogs(t, e, 1, 100, "first")
	if s.transactions != 20 {
		t.Fatalf("burst: %d transactions, want 20", s.transactions)
	}
	if len(e.persistence.dirty) != 80 {
		t.Fatalf("dirty: %d, want 80", len(e.persistence.dirty))
	}
	for step := 1; step <= 10; step++ {
		*now = now.Add(100 * time.Millisecond)
		// Another 100 updates per tick, sharing a single transaction per token.
		writeLogs(t, e, 1, 100, fmt.Sprint(step))
		if s.transactions > 20+step {
			t.Fatalf("tick %d: %d transactions exceeds %d", step, s.transactions, 20+step)
		}
	}
	e.persistence.tick()
	stopTimer(e)
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	if s.largest < 80 {
		t.Fatalf("largest batch %d: dirty logs were not grouped", s.largest)
	}
	for id := uint32(1); id <= 100; id++ {
		if len(s.alerts[id]) != 11 {
			t.Fatalf("log %d: %d persisted alerts, want 11", id, len(s.alerts[id]))
		}
		if e.Dropped(id) != 0 {
			t.Fatalf("group commit changed dropped for %d", id)
		}
	}
}
func TestDirtyAlertsFlushOnTheNextToken(t *testing.T) {
	e, s, now := batchFixture(t)
	writeLogs(t, e, 1, 21, "dirty")
	if s.active[21] {
		t.Fatal("over-budget snapshot committed inline")
	}
	*now = now.Add(100 * time.Millisecond)
	e.persistence.tick()
	stopTimer(e)
	if !s.active[21] || len(s.alerts[21]) != 1 {
		t.Fatal("next token lost dirty alert")
	}
	if len(e.persistence.dirty) != 0 {
		t.Fatal("successful flush left dirty state")
	}
}
func TestDirtyAlertsCannotUndoCleanOrRetirement(t *testing.T) {
	for _, op := range []string{"clean", "pull", "forget", "expiry"} {
		t.Run(op, func(t *testing.T) {
			e, s, now := batchFixture(t)
			writeLogs(t, e, 1, 21, "dirty")
			switch op {
			case "clean":
				if _, err := e.Clean(21, "a", "", 0, Defaults()); err != nil {
					t.Fatal(err)
				}
			case "pull":
				if _, err := e.Pull(21, "a", "", 1, Defaults()); err != nil {
					t.Fatal(err)
				}
			case "expiry":
				*now = now.Add(31 * 24 * time.Hour)
				if _, err := e.Read(21, "a", "", Defaults()); err != nil {
					t.Fatal(err)
				}
			case "forget":
				e.Forget(21)
			}
			*now = now.Add(time.Second)
			e.persistence.tick()
			stopTimer(e)
			if err := e.Close(); err != nil {
				t.Fatal(err)
			}
			if len(s.alerts[21]) != 0 {
				t.Fatalf("%s resurrected pending alert", op)
			}
		})
	}
}
func TestFailedAlertBatchRemainsDirtyAndStopReportsFailure(t *testing.T) {
	e, s, _ := batchFixture(t)
	writeLogs(t, e, 1, 21, "dirty")
	s.batchErr = errors.New("disk unavailable")
	if err := e.Close(); err == nil {
		t.Fatal("failed drain was reported saved")
	}
	if len(e.persistence.dirty) != 1 {
		t.Fatal("failed drain discarded dirty state")
	}
	s.batchErr = nil
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	if len(s.alerts[21]) != 1 {
		t.Fatal("retry failed to retain accepted alert")
	}
}

// The real timer, rather than a test-driven tick, must persist a quiet ring.
type notifyingBatch struct {
	*memory.State
	committed chan []ports.EventAlertSnapshot
}

func (s *notifyingBatch) ReplaceEventAlertBatch(v []ports.EventAlertSnapshot) error {
	if err := s.State.ReplaceEventAlertBatch(v); err != nil {
		return err
	}
	s.committed <- v
	return nil
}
func TestAQuietDirtyRingIsFlushedByTheTimer(t *testing.T) {
	s := &notifyingBatch{State: memory.NewState(), committed: make(chan []ports.EventAlertSnapshot, 1)}
	e := New(s, s, nil)
	defer e.Close()
	e.persistence.tokens = 0
	if _, err := e.Write(1, "alert", "quiet", Defaults()); err != nil {
		t.Fatal(err)
	}
	select {
	case batch := <-s.committed:
		if len(batch) != 1 || batch[0].RecordID != 1 || len(batch[0].Alerts) != 1 || batch[0].Alerts[0].Text != "quiet" {
			t.Fatalf("timer batch: %+v", batch)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timer failed to flush dirty alert on the next token")
	}
}

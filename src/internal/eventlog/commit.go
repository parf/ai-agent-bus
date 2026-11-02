package eventlog

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
)

// One token buys one transaction across ALL dirty rings, not one per agent.
// At the measured 6.25 ms/snapshot, 10/s uses about 6.25% of connection time;
// burst 20 permits about 125 ms before coalescing. Shrinks remain synchronous.
const alertCommitRate = 10
const alertCommitBurst = 20
const alertCommitInterval = time.Second / alertCommitRate

// Never takes a log or registry lock. Callers may hold a log lock; the timer
// only sees immutable copies, so it cannot invert that order. The same mutex
// orders synchronous removals and retirement against older dirty snapshots.
type alertCommitter struct {
	mu       deadlineMutex
	scalar   ports.EventAlertStore
	batch    ports.EventAlertBatchStore
	dirty    map[uint32]ports.EventAlertSnapshot
	timer    *time.Timer
	now      func() time.Time
	after    func(time.Duration, func()) *time.Timer
	at       time.Time
	tokens   float64
	closed   bool
	err      error
	pending  atomic.Int64
	stopping atomic.Bool
}

func (c *alertCommitter) init(s ports.EventAlertStore) {
	c.scalar = s
	c.batch, _ = s.(ports.EventAlertBatchStore)
	c.dirty = map[uint32]ports.EventAlertSnapshot{}
	c.now = time.Now
	c.after = time.AfterFunc
	c.tokens = alertCommitBurst
	c.at = c.now()
}
func (c *alertCommitter) refill() {
	now := c.now()
	c.tokens = min(float64(alertCommitBurst), c.tokens+max(0, now.Sub(c.at).Seconds())*alertCommitRate)
	c.at = now
}
func (c *alertCommitter) schedule() {
	if c.timer == nil && !c.closed && !c.stopping.Load() {
		c.timer = c.after(alertCommitInterval, c.tick)
	}
}
func (c *alertCommitter) tick() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.timer = nil
	if c.closed {
		return
	}
	c.refill()
	if len(c.dirty) > 0 && c.tokens >= 1 {
		c.tokens--
		c.err = c.flush()
	}
	if len(c.dirty) > 0 {
		c.schedule()
	}
}
func (c *alertCommitter) flush() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return c.flushContext(ctx)
}
func (c *alertCommitter) flushContext(ctx context.Context) error {
	if len(c.dirty) == 0 {
		return nil
	}
	batch := make([]ports.EventAlertSnapshot, 0, len(c.dirty))
	for _, v := range c.dirty {
		batch = append(batch, v)
	}
	sort.Slice(batch, func(i, j int) bool { return batch[i].RecordID < batch[j].RecordID })
	if err := c.batch.ReplaceEventAlertBatch(ctx, batch); err != nil {
		return err
	}
	clear(c.dirty)
	c.pending.Store(0)
	return nil
}
func (c *alertCommitter) save(v ports.EventAlertSnapshot, deferred bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.stopping.Load() {
		return Error("storage_unknown")
	}
	// Existing optional/non-batch stores retain their synchronous behaviour.
	if c.batch == nil {
		return c.scalar.ReplaceEventAlerts(v.RecordID, v.Alerts, v.Active)
	}
	if !deferred {
		// A failed removal must leave its previous pending snapshot intact.
		if err := c.scalar.ReplaceEventAlerts(v.RecordID, v.Alerts, v.Active); err != nil {
			return err
		}
		delete(c.dirty, v.RecordID)
		c.pending.Store(int64(len(c.dirty)))
		return nil
	}
	if c.err != nil {
		return c.err
	}
	c.dirty[v.RecordID] = v
	c.pending.Store(int64(len(c.dirty)))
	c.refill()
	if c.tokens >= 1 {
		c.tokens--
		c.err = c.flush()
		if c.err != nil {
			c.schedule()
			return c.err
		}
	} else {
		c.schedule()
	}
	return nil
}
func (c *alertCommitter) forget(id uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.dirty, id)
	c.pending.Store(int64(len(c.dirty)))
}

// Close drains accepted writes without the rate bound at graceful shutdown.
// The daemon calls this only after joining the request handlers and #bus.
// A failed batch stays dirty and returns an error; it is never called saved.
func (e *Engine) Close(deadline ...context.Context) error {
	parent := context.Background()
	if len(deadline) > 0 {
		parent = deadline[0]
	}
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	c := &e.persistence
	c.stopping.Store(true)
	unsaved := func(err error) error {
		return fmt.Errorf("%d alert rings not saved at stop: %w", c.pending.Load(), err)
	}
	if err := c.mu.LockContext(ctx); err != nil {
		return unsaved(err)
	}
	defer c.mu.Unlock()
	c.closed = true
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	if c.batch == nil {
		return nil
	}
	if err := c.flushContext(ctx); err != nil {
		return unsaved(err)
	}
	return nil
}

// A context-aware gate bounds shutdown even when an adapter fails to honour
// cancellation. The in-flight transaction itself also receives a deadline.
type deadlineMutex struct {
	once sync.Once
	gate chan struct{}
}

func (m *deadlineMutex) init() {
	m.once.Do(func() { m.gate = make(chan struct{}, 1); m.gate <- struct{}{} })
}
func (m *deadlineMutex) Lock() { _ = m.LockContext(context.Background()) }
func (m *deadlineMutex) LockContext(ctx context.Context) error {
	m.init()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.gate:
	}
	if err := ctx.Err(); err != nil {
		m.Unlock()
		return err
	}
	return nil
}
func (m *deadlineMutex) Unlock() { m.gate <- struct{}{} }

// CommitRetirement excludes the flusher across a registry prune. A failed
// commit retains every pending snapshot; a successful one drops private state
// before another flusher can run. No log lock is taken while core holds b.mu.
func (e *Engine) CommitRetirement(ids []uint32, commit func() error) error {
	if len(ids) == 0 {
		return commit()
	}
	c := &e.persistence
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := commit(); err != nil {
		return err
	}
	for _, id := range ids {
		delete(c.dirty, id)
	}
	c.pending.Store(int64(len(c.dirty)))
	e.mu.Lock()
	for _, id := range ids {
		delete(e.logs, id)
	}
	e.mu.Unlock()
	return nil
}

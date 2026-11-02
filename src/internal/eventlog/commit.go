package eventlog

import (
	"sort"
	"sync"
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
	mu     sync.Mutex
	scalar ports.EventAlertStore
	batch  ports.EventAlertBatchStore
	dirty  map[uint32]ports.EventAlertSnapshot
	timer  *time.Timer
	now    func() time.Time
	after  func(time.Duration, func()) *time.Timer
	at     time.Time
	tokens float64
	closed bool
	err    error
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
	if c.timer == nil && !c.closed {
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
	if len(c.dirty) == 0 {
		return nil
	}
	batch := make([]ports.EventAlertSnapshot, 0, len(c.dirty))
	for _, v := range c.dirty {
		batch = append(batch, v)
	}
	sort.Slice(batch, func(i, j int) bool { return batch[i].RecordID < batch[j].RecordID })
	if err := c.batch.ReplaceEventAlertBatch(batch); err != nil {
		return err
	}
	clear(c.dirty)
	return nil
}
func (c *alertCommitter) save(v ports.EventAlertSnapshot, deferred bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
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
		return nil
	}
	if c.err != nil {
		return c.err
	}
	c.dirty[v.RecordID] = v
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
}

// Close drains accepted writes without the rate bound at graceful shutdown.
// The daemon calls this only after joining the request handlers and #bus.
// A failed batch stays dirty and returns an error; it is never called saved.
func (e *Engine) Close() error {
	c := &e.persistence
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	if c.batch == nil {
		return nil
	}
	return c.flush()
}

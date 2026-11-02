package api

import (
	"context"
	"sync"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// daemonDispatch has independent health, event-read and mutation lanes: store
// work never borrows a health worker. A target has at most one active mutation and takes one
// turn before rejoining the end of the ready queue.
type daemonDispatch struct {
	ctx     context.Context
	control chan protocol.Envelope
	health  chan protocol.Envelope
	mu      sync.Mutex
	changed *sync.Cond
	queues  map[string][]protocol.Envelope
	active  map[string]bool
	ready   []string
	waiting int
	run     func(protocol.Envelope)
	wg      sync.WaitGroup
}

func newDaemonDispatch(ctx context.Context, run func(protocol.Envelope)) *daemonDispatch {
	return newDaemonDispatchWorkers(ctx, run, 4)
}
func newDaemonDispatchWorkers(ctx context.Context, run func(protocol.Envelope), mutations int) *daemonDispatch {
	d := &daemonDispatch{ctx: ctx, health: make(chan protocol.Envelope, 64), control: make(chan protocol.Envelope, 64), queues: map[string][]protocol.Envelope{}, active: map[string]bool{}, run: run}
	d.changed = sync.NewCond(&d.mu)
	for _, lane := range []struct {
		queue   chan protocol.Envelope
		workers int
	}{{d.health, 1}, {d.control, 2}} {
		for range lane.workers {
			d.wg.Add(1)
			go func(queue chan protocol.Envelope) {
				defer d.wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					case e := <-queue:
						if ctx.Err() != nil {
							return
						}
						d.run(e)
					}
				}
			}(lane.queue)
		}
	}

	for range mutations {
		d.wg.Add(1)
		go d.mutationWorker()
	}
	context.AfterFunc(ctx, func() { d.mu.Lock(); d.changed.Broadcast(); d.mu.Unlock() })
	return d
}
func mutationTarget(e protocol.Envelope) (string, bool) {
	if e.Ask != "" {
		return "", false
	}
	method := eventMethod(e.Body)
	if !writeLooking(e.Body) && method != "pull" && method != "clean" {
		return "", false
	}
	c, _ := parseEventCall(e.Body)
	key := c.Log
	if key == "" {
		key = e.From
	}
	if n, err := protocol.ParseName(key); err == nil {
		key = n.String()
	}
	return key, true
}

// enqueue is the scheduler-level entry used by component checks. The server
// enters through dispatchDaemonTask, which checks input and authority first.
func (d *daemonDispatch) enqueue(e protocol.Envelope) bool {
	key, mutation := mutationTarget(e)
	return d.enqueueAt(e, key, mutation, e.Ask != "" || !isEventMethod(eventMethod(e.Body)) && !mutation)
}
func (d *daemonDispatch) enqueueAt(e protocol.Envelope, key string, mutation, health bool) bool {
	if !mutation {
		queue := d.control
		if health {
			queue = d.health
		}
		select {
		case queue <- e:
			return true
		default:
			return false
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.ctx.Err() != nil || d.waiting >= 64 || len(d.queues[key]) >= 16 {
		return false
	}
	if len(d.queues[key]) == 0 && !d.active[key] {
		d.ready = append(d.ready, key)
	}
	d.queues[key] = append(d.queues[key], e)
	d.waiting++
	d.changed.Signal()
	return true
}
func (d *daemonDispatch) mutationWorker() {
	defer d.wg.Done()
	for {
		d.mu.Lock()
		for len(d.ready) == 0 && d.ctx.Err() == nil {
			d.changed.Wait()
		}
		if d.ctx.Err() != nil {
			d.mu.Unlock()
			return
		}
		key := d.ready[0]
		d.ready = d.ready[1:]
		e := d.queues[key][0]
		d.queues[key] = d.queues[key][1:]
		d.waiting--
		d.active[key] = true
		d.mu.Unlock()
		d.run(e)
		d.mu.Lock()
		delete(d.active, key)
		if len(d.queues[key]) > 0 {
			d.ready = append(d.ready, key)
			d.changed.Signal()
		} else {
			delete(d.queues, key)
		}
		d.mu.Unlock()
	}
}

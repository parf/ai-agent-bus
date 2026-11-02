package api

import (
	"context"
	"sync"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// daemonDispatch has fixed, independent lanes: a blocked mutation never borrows
// a control worker. A target has at most one active mutation and takes one
// turn before rejoining the end of the ready queue.
type daemonDispatch struct {
	ctx     context.Context
	control chan protocol.Envelope
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
	d := &daemonDispatch{ctx: ctx, control: make(chan protocol.Envelope, 64), queues: map[string][]protocol.Envelope{}, active: map[string]bool{}, run: run}
	d.changed = sync.NewCond(&d.mu)
	for range 2 {
		d.wg.Add(1)
		go func() {
			defer d.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case e := <-d.control:
					if ctx.Err() != nil {
						return
					}
					d.run(e)
				}
			}
		}()
	}
	for range 4 {
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
func (d *daemonDispatch) enqueue(e protocol.Envelope) bool {
	key, mutation := mutationTarget(e)
	if !mutation {
		select {
		case d.control <- e:
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

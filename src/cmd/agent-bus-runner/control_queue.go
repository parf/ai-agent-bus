package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// controlQueueMax bounds the calls waiting for the control worker. Over it a
// call is refused as busy at once: the inbox loop never blocks on a control.
const controlQueueMax = 16

// A controlJob is one call that changes the runner's configuration. The
// control worker runs jobs one at a time, in the order they were accepted;
// it is the only path from a message to reloadMu, so the inbox loop, and the
// status it answers, never waits behind an apply. refuse answers a job that
// will not run.
type controlJob struct {
	run    func()
	refuse func(reason string)
}

// controlQueue is the control worker's FIFO and what it knows of the installs
// under way.
type controlQueue struct {
	once   sync.Once
	mu     sync.Mutex
	jobs   chan controlJob
	closed bool // the runner is stopping: every queued job is refused
}

// messageJob runs one call on the worker and answers it on its own route.
func (r *runner) messageJob(e protocol.Envelope, do func() string) controlJob {
	return controlJob{
		run:    func() { r.answer(e, do()) },
		refuse: func(reason string) { r.answer(e, "error: "+reason) },
	}
}

// enqueue hands a job to the control worker, or says why it cannot: the
// queue is full, or the runner is stopping. It never blocks.
func (r *runner) enqueue(j controlJob) error {
	q := &r.queue
	q.once.Do(func() {
		q.jobs = make(chan controlJob, controlQueueMax)
		ctx := r.ctx
		if ctx == nil {
			ctx = context.Background()
		}
		go r.controlWorker(ctx, q.jobs)
	})
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return fmt.Errorf("runner stopping")
	}
	select {
	case q.jobs <- j:
		return nil
	default:
		return fmt.Errorf("busy: %d runner controls are already waiting, the most it queues; send again once they are answered", controlQueueMax)
	}
}

func (r *runner) controlWorker(ctx context.Context, jobs <-chan controlJob) {
	for {
		select {
		case <-ctx.Done():
			return // refuseQueued answers what is left
		case j := <-jobs:
			if ctx.Err() != nil {
				j.refuse("runner stopping")
				continue
			}
			j.run()
		}
	}
}

// refuseQueued closes the queue and refuses every job still in it: a stopping
// runner answers each queued control, never drops one silently.
func (r *runner) refuseQueued() {
	q := &r.queue
	q.mu.Lock()
	q.closed = true
	jobs := q.jobs
	q.mu.Unlock()
	if jobs == nil {
		return
	}
	for {
		select {
		case j := <-jobs:
			j.refuse("runner stopping")
		default:
			return
		}
	}
}

// An install under way, from its acceptance to its publish. A stop or a
// remove of its package that the worker carries out meanwhile supersedes it:
// the later call wins, and the publish is refused with that reason.
type installWatch struct {
	superseded string // the verb that superseded it; empty while it stands
}

// beginInstall records an install of pkg as under way, or refuses a second.
func (r *runner) beginInstall(pkg string) (*installWatch, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.installing[pkg] != nil {
		return nil, fmt.Errorf("busy: an install of %s is already under way", pkg)
	}
	if r.installing == nil {
		r.installing = map[string]*installWatch{}
	}
	w := &installWatch{}
	r.installing[pkg] = w
	return w, nil
}

// endInstall forgets the install w of pkg, and never a later one.
func (r *runner) endInstall(pkg string, w *installWatch) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.installing[pkg] == w {
		delete(r.installing, pkg)
	}
}

// supersede marks an install of key's package as superseded by verb. The
// worker calls it after the verb took effect.
func (r *runner) supersede(key, verb string) {
	pkg, _, _ := strings.Cut(key, "/")
	r.mu.Lock()
	defer r.mu.Unlock()
	if w := r.installing[pkg]; w != nil {
		w.superseded = verb
	}
}

// supersededBy is the verb that superseded an install of pkg, if any.
func (r *runner) supersededBy(pkg string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if w := r.installing[pkg]; w != nil {
		return w.superseded
	}
	return ""
}

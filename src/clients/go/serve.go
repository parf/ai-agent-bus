package bus

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Done is a nil body; a nonnil pointer to an empty string is an answer.
var Done *string

type ServeError struct {
	Failed, Unfinished int
	Cause              error
}

func (e *ServeError) Error() string {
	return fmt.Sprintf("serve ended: %d failed, %d unfinished (%v)", e.Failed, e.Unfinished, e.Cause)
}
func (e *ServeError) Unwrap() error { return e.Cause }
func (b *Bus) dispatch() {
	turn := 0
	for {
		if b.ctx.Err() != nil {
			return
		}
		b.mu.Lock()
		ids := []string{}
		for id, p := range b.pending {
			if p.poll && p.ctx.Err() == nil {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		var s *serving
		var slot bool
		if b.serving != nil && b.serving.ctx.Err() == nil && (turn%2 == 1 || len(ids) == 0) {
			s = b.serving
			select {
			case s.slots <- struct{}{}:
				slot = true
				s.polls.Add(1)
				s.inflight.Add(1)
			default:
			}
		}
		var p *pending
		if !slot && len(ids) > 0 {
			p = b.pending[ids[(turn/2)%len(ids)]]
		}
		b.mu.Unlock()
		turn++
		if slot {
			m, e := b.consumeWire(s.pollCtx, settings{inbox: s.inbox, topic: s.topic, tag: s.tag, wait: 100 * time.Millisecond, share: s.share})
			release := func() { <-s.slots }
			finishPoll := func() { s.inflight.Add(-1); s.polls.Done() }
			if m == nil && e == nil {
				release()
				finishPoll()
				continue
			}
			if m != nil {
				b.mu.Lock()
				var match *pending
				for _, candidate := range b.pending {
					if candidate.inbox == m.To && candidate.topic == m.Topic && candidate.tag == m.Tag {
						match = candidate
						break
					}
				}
				b.mu.Unlock()
				if match != nil {
					release()
					select {
					case match.ch <- delivery{m, e}:
					case <-match.ctx.Done():
					case <-b.ctx.Done():
					}
					finishPoll()
					continue
				}
			}
			// The reserved slot guarantees buffer capacity, even on intake stop.
			s.ch <- work{delivery{m, e}, release}
			finishPoll()
		} else if p != nil {
			m, e := b.consumeWire(b.ctx, settings{inbox: p.inbox, topic: p.topic, tag: p.tag, wait: 100 * time.Millisecond})
			if e != nil {
				b.mu.Lock()
				p.poll = false
				b.mu.Unlock()
			}
			if m != nil || e != nil {
				select {
				case p.ch <- delivery{m, e}:
				case <-p.ctx.Done():
				case <-b.ctx.Done():
				}
			}
		} else {
			select {
			case <-b.ctx.Done():
				return
			case <-b.notify:
			case <-time.After(20 * time.Millisecond):
			}
		}
	}
}
func (b *Bus) Serve(ctx context.Context, handler func(context.Context, Envelope) (*string, error), opts ...Option) error {
	if e := b.ensure(ctx); e != nil {
		return e
	}
	o := config(opts)
	if o.workers <= 0 || o.drain < 0 || handler == nil {
		return errors.New("valid handler, workers and drain required")
	}
	intake, stop := context.WithCancel(ctx)
	defer stop()
	// Ending intake must not cancel a take already sent to the daemon.
	// Finish that poll during drain; its known work or uncertainty is reported.
	pollCtx, cancelPolls := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelPolls()
	messageCtx, cancelMessages := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelMessages()
	s := &serving{ctx: intake, pollCtx: pollCtx, inbox: o.inbox, topic: o.topic, tag: o.tag, share: o.share, slots: make(chan struct{}, o.workers), ch: make(chan work, o.workers)}
	b.mu.Lock()
	if b.raw || b.serving != nil {
		b.mu.Unlock()
		return ErrReaderInUse
	}
	if s.inbox == "" {
		s.inbox = b.you
	}
	b.serving = s
	b.mu.Unlock()
	b.wake()
	completed := make(chan error, o.workers)
	active, failed := 0, 0
	var cause error
	spawn := func(w work) {
		if w.err != nil {
			w.release()
			cause = w.err
			return
		}
		m := w.msg
		if m == nil || m.Receipt != "" || m.Re != "" || strings.HasPrefix(m.Tag, b.prefix) {
			w.release()
			return
		}
		var deadline time.Time
		if m.Deadline != "" {
			var e error
			deadline, e = time.Parse(time.RFC3339Nano, m.Deadline)
			if e != nil {
				w.release()
				failed++
				cause = e
				return
			}
			if !deadline.IsZero() && !time.Now().Before(deadline) {
				w.release()
				return
			}
		}
		active++
		go func() {
			defer w.release()
			var err error
			defer func() {
				if p := recover(); p != nil {
					err = fmt.Errorf("handler panic: %v", p)
				}
				completed <- err
			}()
			handlerCtx := messageCtx
			if _, err = b.Ack(handlerCtx, m); err != nil {
				return
			}
			if err = handlerCtx.Err(); err != nil {
				return
			}
			if !deadline.IsZero() && !time.Now().Before(deadline) {
				return
			}
			var body *string
			body, err = handler(handlerCtx, *m)
			if err != nil {
				return
			}
			if body == nil {
				_, err = b.Done(handlerCtx, m)
			} else {
				_, err = b.Reply(handlerCtx, m, *body)
			}
		}()
	}
	running := true
	for running {
		select {
		case <-ctx.Done():
			running = false
		case <-b.ctx.Done():
			running = false
			cause = ErrClosed
		case err := <-completed:
			active--
			if err != nil {
				failed++
			}
		case w := <-s.ch:
			spawn(w)
			if cause != nil {
				running = false
			}
		}
	}
	stop()
	b.mu.Lock()
	if b.serving == s {
		b.serving = nil
	}
	b.mu.Unlock()
	b.wake()
	// Removal under the reader lock prevents further poll additions. All known
	// successful in-flight takes are handed over before this channel closes.
	go func() { s.polls.Wait(); close(s.ch) }()
	timer := time.NewTimer(o.drain)
	defer timer.Stop()
	ch := s.ch
	for ch != nil || active > 0 {
		select {
		case w, ok := <-ch:
			if !ok {
				ch = nil
			} else {
				spawn(w)
			}
		case err := <-completed:
			active--
			if err != nil {
				failed++
			}
		case <-timer.C:
			cancelMessages()
			return &ServeError{failed, active + len(s.ch) + int(s.inflight.Load()), cause}
		}
	}
	if failed > 0 || cause != nil {
		return &ServeError{failed, 0, cause}
	}
	return nil
}

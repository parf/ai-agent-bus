package bus

import (
	"context"
	"errors"
	"fmt"
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

// follow is one pending call's own filtered read, held at the daemon for up to longWait and renewed until the call
// ends. Ending the call cancels it; the daemon hands a cancelled read nothing (docs/04-messaging.md#one-reader-per-inbox).
func (b *Bus) follow(ctx context.Context, p *pending) {
	for ctx.Err() == nil {
		wait := longWait
		if deadline, ok := ctx.Deadline(); ok {
			wait = min(wait, max(time.Until(deadline), time.Millisecond))
		}
		m, e := b.consumeWire(ctx, settings{inbox: p.inbox, topic: p.topic, tag: p.tag, wait: wait})
		if m == nil && (e == nil || ctx.Err() != nil) {
			continue
		}
		select {
		case p.ch <- delivery{m, e}:
		case <-ctx.Done():
			return
		}
		if e != nil {
			return
		}
	}
}

// claim finds the pending call a message taken by the serving read answers, or its own request taken back.
func (b *Bus) claim(m *Envelope) *pending {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, candidate := range b.pending {
		if candidate.topic == m.Topic && candidate.tag == m.Tag && (candidate.inbox == m.To || (m.From == b.you && m.Receipt == "" && m.Re == "")) {
			return candidate
		}
	}
	return nil
}

// serveLoop holds one read at the daemon while a worker is free and none while all are busy. Ending intake cancels
// that read, which takes nothing; a message it did take is handed over and drained like other taken work.
func (b *Bus) serveLoop(s *serving) {
	defer s.polls.Done()
	for s.ctx.Err() == nil && b.ctx.Err() == nil {
		// Wait for a free worker without holding it: a self request taken by a call's read may need it.
		select {
		case s.slots <- struct{}{}:
			<-s.slots
		case <-s.ctx.Done():
			return
		case <-b.ctx.Done():
			return
		}
		s.inflight.Add(1)
		m, e := b.consumeWire(s.ctx, settings{inbox: s.inbox, topic: s.topic, tag: s.tag, wait: longWait, share: s.share})
		if m == nil && (e == nil || s.ctx.Err() != nil || b.ctx.Err() != nil) {
			s.inflight.Add(-1)
			continue
		}
		if m != nil {
			if match := b.claim(m); match != nil {
				select {
				case match.ch <- delivery{m, nil}:
				case <-match.ctx.Done():
				case <-b.ctx.Done():
				}
				s.inflight.Add(-1)
				continue
			}
			if m.Receipt != "" {
				s.inflight.Add(-1)
				continue
			}
		}
		// What the read brought waits for a free worker; the slot guarantees buffer capacity, even on intake stop.
		select {
		case s.slots <- struct{}{}:
			s.ch <- work{delivery: delivery{m, e}, release: func() { <-s.slots }}
		case <-s.ended.Done():
		case <-b.ctx.Done():
		}
		s.inflight.Add(-1)
		if e != nil {
			return
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
	// Ending intake cancels the waiting read; the daemon hands a cancelled read nothing.
	intake, stop := context.WithCancel(ctx)
	defer stop()
	messageCtx, cancelMessages := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelMessages()
	s := &serving{ctx: intake, ended: messageCtx, inbox: o.inbox, topic: o.topic, tag: o.tag, share: o.share, slots: make(chan struct{}, o.workers), ch: make(chan work, o.workers)}
	b.mu.Lock()
	if b.raw || b.serving != nil {
		b.mu.Unlock()
		return ErrReaderInUse
	}
	if s.inbox == "" {
		s.inbox = b.you
	}
	b.serving = s
	s.polls.Add(1)
	b.mu.Unlock()
	go b.serveLoop(s)
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
		if m != nil && o.onReply != nil && m.Receipt == "" && m.Re != "" && !strings.HasPrefix(m.Tag, b.prefix) {
			// An answer to this Bus's own send: no ack, reply or done, exactly as when it is dropped.
			active++
			go func() {
				defer w.release()
				var err error
				defer func() {
					if p := recover(); p != nil {
						err = fmt.Errorf("reply handler panic: %v", p)
					}
					completed <- err
				}()
				err = o.onReply(messageCtx, *m)
			}()
			return
		}
		if m == nil || m.Receipt != "" || m.Re != "" || (strings.HasPrefix(m.Tag, b.prefix) && !w.selfRequest) {
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
	// The serving read ends with intake; any message it took and every self
	// request already taken are handed over before this channel closes.
	go func() { s.polls.Wait(); s.selfCalls.Wait(); close(s.ch) }()
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
			return &ServeError{failed, active + len(s.ch) + int(s.inflight.Load()) + int(s.selfWaiting.Load()), cause}
		}
	}
	if failed > 0 || cause != nil {
		return &ServeError{failed, 0, cause}
	}
	return nil
}

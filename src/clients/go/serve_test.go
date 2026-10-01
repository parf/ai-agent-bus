package bus

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type queueFixture struct {
	mu         sync.Mutex
	queue      []map[string]any
	sent       []map[string]any
	unfiltered chan struct{}
}

func (q *queueFixture) handler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/status":
		statusReply(w)
	case "/send":
		v := acceptance(w, r)
		q.mu.Lock()
		q.sent = append(q.sent, v)
		q.mu.Unlock()
	case "/consume":
		if r.URL.Query().Get("tag") == "" {
			select {
			case q.unfiltered <- struct{}{}:
			default:
			}
		}
		q.mu.Lock()
		for i, m := range q.queue {
			if tag := r.URL.Query().Get("tag"); tag != "" && tag != m["tag"] {
				continue
			}
			q.queue = append(q.queue[:i], q.queue[i+1:]...)
			q.mu.Unlock()
			json.NewEncoder(w).Encode(m)
			return
		}
		q.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
		w.WriteHeader(204)
	default:
		io.WriteString(w, `{}`)
	}
}
func (q *queueFixture) push(m map[string]any) {
	q.mu.Lock()
	q.queue = append(q.queue, m)
	q.mu.Unlock()
}
func TestServeEmptyAnswerDoneFailureAndRawReaderExclusion(t *testing.T) {
	q := &queueFixture{unfiltered: make(chan struct{}, 100)}
	b := fakeBus(t, q.handler)
	for _, body := range []string{"empty", "done", "fail"} {
		q.push(map[string]any{"message_id": body, "from": "#caller", "to": "#me", "body": body, "tag": body})
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	var handled atomic.Int32
	go func() {
		result <- b.Serve(ctx, func(_ context.Context, m Envelope) (*string, error) {
			handled.Add(1)
			switch m.Body {
			case "empty":
				v := ""
				return &v, nil
			case "fail":
				return nil, errors.New("failed")
			default:
				return Done, nil
			}
		}, WithWorkers(2))
	}()
	<-q.unfiltered
	if _, e := b.Consume(context.Background()); !errors.Is(e, ErrReaderInUse) {
		t.Fatal(e)
	}
	deadline := time.After(time.Second)
	for {
		q.mu.Lock()
		writes := len(q.sent)
		q.mu.Unlock()
		if writes >= 5 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("completion messages missing")
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	e := <-result
	var serve *ServeError
	if !errors.As(e, &serve) || serve.Failed != 1 || handled.Load() != 3 {
		t.Fatal(e, handled.Load())
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	answers, done, acks := 0, 0, 0
	for _, m := range q.sent {
		switch m["receipt"] {
		case "ack":
			acks++
		case "done":
			done++
		default:
			answers++
			if m["body"] != "" || m["re"] != "empty" {
				t.Fatalf("empty reply changed: %#v", m)
			}
		}
	}
	if answers != 1 || done != 1 || acks != 3 {
		t.Fatal(answers, done, acks)
	}
}
func TestServeStopsIntakeDrainsWithIndependentHandlerContext(t *testing.T) {
	q := &queueFixture{unfiltered: make(chan struct{}, 100)}
	b := fakeBus(t, q.handler)
	q.push(map[string]any{"message_id": "work", "from": "#caller", "to": "#me", "body": "x"})
	started := make(chan context.Context, 1)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- b.Serve(ctx, func(ctx context.Context, m Envelope) (*string, error) {
			started <- ctx
			<-ctx.Done()
			return nil, ctx.Err()
		}, WithDrainTimeout(40*time.Millisecond))
	}()
	handlerCtx := <-started
	cancel()
	select {
	case <-handlerCtx.Done():
		t.Fatal("handler canceled at intake stop")
	case <-time.After(10 * time.Millisecond):
	}
	select {
	case e := <-result:
		var s *ServeError
		if !errors.As(e, &s) || s.Unfinished != 1 {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("drain unbounded")
	}
	select {
	case <-handlerCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("handler context not canceled at drain deadline")
	}
}
func TestLegacyLateCallAnswersNeverBecomeWorkAndRolesAreCopy(t *testing.T) {
	q := &queueFixture{unfiltered: make(chan struct{}, 100)}
	b := fakeBus(t, q.handler)
	_, e := b.Call(context.Background(), "#other", "x", WithTimeout(30*time.Millisecond))
	var timeout *ErrCallTimeout
	if !errors.As(e, &timeout) {
		t.Fatal(e)
	}
	q.mu.Lock()
	tag := q.sent[0]["tag"]
	topic := q.sent[0]["topic"]
	q.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	var count atomic.Int32
	handled := make(chan struct{}, 1)
	go func() {
		result <- b.Serve(ctx, func(_ context.Context, m Envelope) (*string, error) {
			count.Add(1)
			if m.Body != "normal" {
				t.Errorf("late call answer ran as work: %s", m.Body)
			}
			if roles := m.Roles(); len(roles) > 0 {
				roles[0] = "fake"
				if m.Roles()[0] != "owner" {
					t.Error("roles mutated")
				}
			}
			if m.Body == "normal" {
				handled <- struct{}{}
			}
			return Done, nil
		})
	}()
	<-q.unfiltered
	for _, re := range []string{"", "old"} {
		q.push(map[string]any{"message_id": "late", "from": "#other", "to": "#me", "topic": topic, "tag": tag, "body": "late", "re": re, "forwards": 1})
	}
	q.push(map[string]any{"message_id": "work", "from": "#caller", "to": "#me", "body": "normal", "roles": []string{"owner"}})
	select {
	case <-handled:
	case <-time.After(time.Second):
		t.Fatal("normal work missing")
	}
	time.Sleep(30 * time.Millisecond)
	cancel()
	if e = <-result; e != nil {
		t.Fatal(e)
	}
	if count.Load() != 1 {
		t.Fatal("late reply became work", count.Load())
	}
}
func TestServingReservesCapacityBeforeConsuming(t *testing.T) {
	q := &queueFixture{unfiltered: make(chan struct{}, 100)}
	b := fakeBus(t, q.handler)
	for _, id := range []string{"one", "two", "three"} {
		q.push(map[string]any{"message_id": id, "from": "#caller", "to": "#me", "body": id})
	}
	started := make(chan struct{}, 3)
	release := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- b.Serve(ctx, func(_ context.Context, m Envelope) (*string, error) {
			started <- struct{}{}
			<-release
			return Done, nil
		}, WithWorkers(1))
	}()
	<-started
	time.Sleep(30 * time.Millisecond)
	q.mu.Lock()
	left := len(q.queue)
	q.mu.Unlock()
	cancel()
	close(release)
	if e := <-result; e != nil {
		t.Fatal(e)
	}
	if left != 2 {
		t.Fatalf("consumed beyond available capacity: remaining %d want 2", left)
	}
}

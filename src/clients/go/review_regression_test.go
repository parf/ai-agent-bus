package bus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNodeIdentityRemainsPublic(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/identity" {
			fmt.Fprint(w, `{"version":"fixture","protocol":1,"protocol_min":1}`)
		} else {
			w.WriteHeader(401)
			fmt.Fprint(w, `{"error":"bad credential"}`)
		}
	}))
	defer s.Close()
	b, _ := Dial(Options{Addr: s.URL})
	defer b.Close()
	_, e := b.Identity(context.Background())
	if e != nil {
		t.Fatalf("public node identity requires auth unexpectedly: %v", e)
	}
}
func TestServeTopicKeepsUnrelatedMessagesQueued(t *testing.T) {
	addr, tokens := realFixture(t)
	owner := fixtureBus(t, addr, tokens["owner@fixture"], "owner@fixture")
	worker := fixtureBus(t, addr, tokens["#worker@fixture"], "#worker@fixture")
	ctx, cancel := context.WithCancel(context.Background())
	var handled atomic.Int32
	end := make(chan error, 1)
	go func() {
		end <- worker.Serve(ctx, func(context.Context, Envelope) (*string, error) { handled.Add(1); return Done, nil }, WithTopic("wanted"))
	}()
	time.Sleep(20 * time.Millisecond)
	_, e := owner.Send(context.Background(), "#worker@fixture", "other", WithTopic("unrelated"))
	if e != nil {
		t.Fatal(e)
	}
	time.Sleep(150 * time.Millisecond)
	cancel()
	<-end
	if handled.Load() != 0 {
		t.Fatalf("serve topic wanted handled %d unrelated messages", handled.Load())
	}
}
func TestServeSkipsAlreadyExpiredDeadline(t *testing.T) {
	addr, tokens := realFixture(t)
	owner := fixtureBus(t, addr, tokens["owner@fixture"], "owner@fixture")
	worker := fixtureBus(t, addr, tokens["#worker@fixture"], "#worker@fixture")
	_, e := owner.Send(context.Background(), "#worker@fixture", "stale", WithWait(time.Millisecond))
	if e != nil {
		t.Fatal(e)
	}
	time.Sleep(30 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	var handled atomic.Int32
	end := make(chan error, 1)
	go func() {
		end <- worker.Serve(ctx, func(context.Context, Envelope) (*string, error) { handled.Add(1); return Done, nil })
	}()
	time.Sleep(150 * time.Millisecond)
	cancel()
	<-end
	if handled.Load() != 0 {
		t.Fatalf("expired deadline caused %d handler runs", handled.Load())
	}
}

type reviewRoundTripper func(*http.Request) (*http.Response, error)

func (f reviewRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type reviewBody struct {
	*strings.Reader
	cancel context.CancelFunc
}

func (r *reviewBody) Read(p []byte) (int, error) {
	n, e := r.Reader.Read(p)
	if e == io.EOF {
		r.cancel()
	}
	return n, e
}
func (r *reviewBody) Close() error { return nil }
func TestServeDrainsKnownTakenWorkOnStop(t *testing.T) {
	b, _ := Dial(Options{Addr: "http://fake"})
	defer b.Close()
	ctx, cancel := context.WithCancel(context.Background())
	var handled atomic.Int32
	var took atomic.Int32
	b.client.Transport = reviewRoundTripper(func(r *http.Request) (*http.Response, error) {
		var body io.ReadCloser = io.NopCloser(strings.NewReader(`{"you":"#me","protocol":1,"protocol_min":1}`))
		if r.URL.Path == "/consume" {
			took.Add(1)
			body = &reviewBody{strings.NewReader(`{"message_id":"taken","from":"#sender","to":"#me","body":"work"}`), cancel}
		}
		if r.URL.Path == "/send" {
			body = io.NopCloser(strings.NewReader(`{"message_id":"reply"}`))
		}
		return &http.Response{StatusCode: 200, Body: body, Header: make(http.Header)}, nil
	})
	e := b.Serve(ctx, func(context.Context, Envelope) (*string, error) { handled.Add(1); return Done, nil }, WithDrainTimeout(time.Second))
	time.Sleep(20 * time.Millisecond)
	if took.Load() != 1 || handled.Load() != 1 {
		t.Fatalf("stop discarded known successful consume: taken=%d handled=%d err=%v", took.Load(), handled.Load(), e)
	}
}
func TestCallIdentityDeadlineReturnsCallTimeout(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/identity" {
			fmt.Fprint(w, `{"protocol":1,"protocol_min":1}`)
			return
		}
		time.Sleep(60 * time.Millisecond)
		fmt.Fprint(w, `{"you":"#me"}`)
	}))
	defer s.Close()
	b, _ := Dial(Options{Addr: s.URL})
	defer b.Close()
	_, e := b.Call(context.Background(), "#other", "x", WithTimeout(10*time.Millisecond))
	var timeout *ErrCallTimeout
	if !errors.As(e, &timeout) {
		t.Fatalf("identity-phase deadline wasn't ErrCallTimeout: %T %v", e, e)
	}
}

// Stopping cancels the waiting serving read, which takes nothing: Serve ends at
// once instead of waiting out the read, and nothing it would have brought runs.
func TestStoppingCancelsTheWaitingServingReadWhichTakesNothing(t *testing.T) {
	started, cancelled := make(chan struct{}, 1), make(chan struct{})
	var takes, handled, sends atomic.Int32
	var wait atomic.Value
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/identity":
			fmt.Fprint(w, `{"protocol":1,"protocol_min":1}`)
		case "/status":
			fmt.Fprint(w, `{"you":"#me"}`)
		case "/consume":
			takes.Add(1)
			wait.Store(r.URL.Query().Get("wait"))
			started <- struct{}{}
			select {
			case <-r.Context().Done():
				close(cancelled)
			case <-time.After(2 * time.Second):
				// A read nobody cancelled ends with the message it waited for.
				fmt.Fprint(w, `{"message_id":"taken","from":"#sender","to":"#me","body":"work"}`)
			}
		default:
			sends.Add(1)
			fmt.Fprint(w, `{"message_id":"accepted"}`)
		}
	}))
	defer s.Close()
	b, err := Dial(Options{Addr: s.URL})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stopped time.Time
	go func() { <-started; stopped = time.Now(); cancel() }()
	err = b.Serve(ctx, func(context.Context, Envelope) (*string, error) {
		handled.Add(1)
		return Done, nil
	}, WithDrainTimeout(3*time.Second))
	took := time.Since(stopped)
	if err != nil {
		t.Fatalf("a cancelled serving read is not a failure: %v", err)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("the waiting serving read was not cancelled at the daemon")
	}
	if took > 500*time.Millisecond {
		t.Fatalf("Serve waited out the serving read after stop: %v", took)
	}
	if w, _ := wait.Load().(string); w != longWait.String() {
		t.Fatalf("serving read waits %q at the daemon, want %v", w, longWait)
	}
	if takes.Load() != 1 || handled.Load() != 0 || sends.Load() != 0 {
		t.Fatalf("stop: takes=%d handled=%d sends=%d, want 1 0 0", takes.Load(), handled.Load(), sends.Load())
	}
}

// A serving read whose connection is lost while intake is open is an uncertain take, never retried.
func TestServeReportsALostServingReadAsUncertain(t *testing.T) {
	var takes, handled atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/identity":
			fmt.Fprint(w, `{"protocol":1,"protocol_min":1}`)
		case "/status":
			fmt.Fprint(w, `{"you":"#me"}`)
		case "/consume":
			takes.Add(1)
			if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
				conn.Close()
			}
		default:
			fmt.Fprint(w, `{"message_id":"accepted"}`)
		}
	}))
	defer s.Close()
	b, err := Dial(Options{Addr: s.URL})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = b.Serve(ctx, func(context.Context, Envelope) (*string, error) { handled.Add(1); return Done, nil })
	var uncertain *UncertainOutcome
	if !errors.As(err, &uncertain) {
		t.Fatalf("lost serving read was not uncertain: %v", err)
	}
	if takes.Load() != 1 || handled.Load() != 0 {
		t.Fatalf("lost read: takes=%d handled=%d, want 1 0", takes.Load(), handled.Load())
	}
}

// A wait of nothing is no wait: the daemon refuses a zero one, so a send
// carrying WithWait(0) must still be delivered.
func TestSendWithAZeroWaitIsASendWithoutOne(t *testing.T) {
	addr, tokens := realFixture(t)
	owner := fixtureBus(t, addr, tokens["owner@fixture"], "owner@fixture")
	worker := fixtureBus(t, addr, tokens["#worker@fixture"], "#worker@fixture")
	id, e := owner.Send(context.Background(), "#worker@fixture", "zero", WithWait(0))
	if e != nil {
		t.Fatalf("a send with a zero wait was refused: %v", e)
	}
	m, e := worker.Consume(context.Background(), WithWait(time.Second))
	if e != nil || m == nil || m.ID != id.ID {
		t.Fatalf("the zero-wait message was not delivered: %v %v", m, e)
	}
}

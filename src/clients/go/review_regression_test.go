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
			fmt.Fprint(w, `{"version":"fixture"}`)
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
		var body io.ReadCloser = io.NopCloser(strings.NewReader(`{"you":"#me"}`))
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

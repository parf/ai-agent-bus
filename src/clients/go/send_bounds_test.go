package bus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// A zero wait or ttl means none and is left out; a negative one is not "none":
// it is sent as it was given, so the daemon (which refuses a wait or ttl that
// is not positive) refuses it, as the other clients refuse it themselves.
func TestSendLeavesOutAZeroWaitAndTTLAndNeverHidesANegativeOne(t *testing.T) {
	var mu sync.Mutex
	var bodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/identity":
			fmt.Fprint(w, `{"protocol":1,"protocol_min":1}`)
		case "/send":
			raw, _ := io.ReadAll(r.Body)
			var v map[string]any
			_ = json.Unmarshal(raw, &v)
			mu.Lock()
			bodies = append(bodies, v)
			mu.Unlock()
			fmt.Fprint(w, `{"message_id":"m1","from":"owner@fixture","to":"x@fixture","body":"b"}`)
		default:
			fmt.Fprint(w, `{"you":"owner@fixture"}`)
		}
	}))
	defer server.Close()
	b, err := Dial(Options{Addr: server.URL, Token: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	ctx := context.Background()
	for _, c := range []struct {
		name        string
		options     []Option
		wait, ttl   bool
		waitS, ttlS string
	}{
		{"zero means none", []Option{WithWait(0), WithTTL(0)}, false, false, "", ""},
		{"a negative wait and ttl are sent as given", []Option{WithWait(-time.Second), WithTTL(-2 * time.Second)}, true, true, "-1s", "-2s"},
		{"a positive one is sent", []Option{WithWait(time.Second), WithTTL(2 * time.Second)}, true, true, "1s", "2s"},
	} {
		if _, err := b.Send(ctx, "x@fixture", "b", c.options...); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		mu.Lock()
		got := bodies[len(bodies)-1]
		mu.Unlock()
		w, hasWait := got["wait"]
		l, hasTTL := got["ttl"]
		if hasWait != c.wait || hasTTL != c.ttl || (c.wait && (w != c.waitS || l != c.ttlS)) {
			t.Fatalf("%s: sent %v", c.name, got)
		}
	}
}

// A call whose context has already ended says so and sends nothing: its wait
// would be negative, and "no wait" is not what the caller asked for.
func TestCallWithAnExpiredContextReturnsItsErrorBeforeSending(t *testing.T) {
	var mu sync.Mutex
	sends := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/identity":
			fmt.Fprint(w, `{"protocol":1,"protocol_min":1}`)
		case "/send":
			mu.Lock()
			sends++
			mu.Unlock()
			fmt.Fprint(w, `{"message_id":"m1"}`)
		default:
			fmt.Fprint(w, `{"you":"owner@fixture"}`)
		}
	}))
	defer server.Close()
	b, err := Dial(Options{Addr: server.URL, Token: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if _, err := b.Status(context.Background()); err != nil { // the handshake is behind it
		t.Fatal(err)
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_, err = b.Call(ctx, "x@fixture", "b")
	var timeout *ErrCallTimeout
	if !errors.As(err, &timeout) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Call = %v, want a timeout wrapping the context's error", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if sends != 0 {
		t.Fatalf("an expired call sent %d message(s)", sends)
	}
}

package bus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func fakeBus(t *testing.T, h http.HandlerFunc) *Bus {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	return fixtureBus(t, s.URL, "fake-token", "#me")
}
func statusReply(w http.ResponseWriter) { io.WriteString(w, `{"you":"#me"}`) }
func acceptance(w http.ResponseWriter, r *http.Request) map[string]any {
	var v map[string]any
	json.NewDecoder(r.Body).Decode(&v)
	v["message_id"] = "sent"
	v["from"] = "#me"
	json.NewEncoder(w).Encode(v)
	return v
}
func TestExpectedNameCheckedBeforeWriteAndCanceledVerificationRetries(t *testing.T) {
	var writes atomic.Int32
	b := fakeBus(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			io.WriteString(w, `{"you":"#wrong"}`)
		} else {
			writes.Add(1)
			acceptance(w, r)
		}
	})
	_, e := b.Send(context.Background(), "#other", "x")
	var mismatch *IdentityMismatch
	if !errors.As(e, &mismatch) || writes.Load() != 0 {
		t.Fatal(e, writes.Load())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = b.Status(ctx); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestPrivateReadsUncachedAndClosedRefuses(t *testing.T) {
	var configs atomic.Int32
	b := fakeBus(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/status":
			statusReply(w)
		case "/config":
			fmt.Fprintf(w, `{"generation":%d}`, configs.Add(1))
		case "/secret":
			io.WriteString(w, "SECRET=hidden\n")
		}
	})
	r := b.Record("#target")
	for i := 1; i <= 2; i++ {
		v, e := r.Config(context.Background())
		if e != nil || !strings.Contains(string(v), fmt.Sprint(i)) {
			t.Fatal(string(v), e)
		}
	}
	sec, e := r.Secret(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	for _, format := range []string{"%s", "%v", "%#v", "%+v"} {
		if strings.Contains(fmt.Sprintf(format, sec), "hidden") {
			t.Fatal("secret logs leaked")
		}
	}
	b.Close()
	if _, e = r.Config(context.Background()); !errors.Is(e, ErrClosed) {
		t.Fatal(e)
	}
}
func TestLostWritesAndDestructiveConsumeNeverReplay(t *testing.T) {
	for _, path := range []string{"/send", "/consume"} {
		t.Run(path, func(t *testing.T) {
			var requests atomic.Int32
			b := fakeBus(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/status" {
					statusReply(w)
					return
				}
				io.Copy(io.Discard, r.Body)
				requests.Add(1)
				conn, _, e := w.(http.Hijacker).Hijack()
				if e != nil {
					t.Error(e)
					return
				}
				conn.Close()
			})
			var e error
			if path == "/send" {
				_, e = b.Send(context.Background(), "#other", "x")
			} else {
				_, e = b.Consume(context.Background())
			}
			var u *UncertainOutcome
			if !errors.As(e, &u) || requests.Load() != 1 {
				t.Fatalf("%v requests=%d", e, requests.Load())
			}
		})
	}
}
func TestCallDeadlineCoversSendAndPreservesAckOnAcceptedTimeout(t *testing.T) {
	var sent map[string]any
	var mu sync.Mutex
	var consumed atomic.Bool
	b := fakeBus(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/status":
			statusReply(w)
		case "/send":
			var v map[string]any
			json.NewDecoder(r.Body).Decode(&v)
			if v["body"] == "slow" {
				time.Sleep(150 * time.Millisecond)
			}
			v["message_id"] = "original"
			mu.Lock()
			sent = v
			mu.Unlock()
			json.NewEncoder(w).Encode(v)
		case "/consume":
			mu.Lock()
			v := sent
			mu.Unlock()
			if v != nil && !consumed.Swap(true) {
				json.NewEncoder(w).Encode(map[string]any{"message_id": "ack", "to": "#me", "topic": v["topic"], "tag": v["tag"], "receipt": "ack", "re": "original"})
			} else {
				time.Sleep(5 * time.Millisecond)
				w.WriteHeader(204)
			}
		}
	})
	start := time.Now()
	_, e := b.Call(context.Background(), "#other", "slow", WithTimeout(30*time.Millisecond))
	var u *UncertainOutcome
	if !errors.As(e, &u) || time.Since(start) > 100*time.Millisecond {
		t.Fatal(e, time.Since(start))
	}
	time.Sleep(160 * time.Millisecond)
	consumed.Store(false)
	out, e := b.Call(context.Background(), "#other", "ack", WithTimeout(60*time.Millisecond))
	var timeout *ErrCallTimeout
	if !errors.As(e, &timeout) || !out.Acked || !timeout.Outcome.Acked || !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(out, e)
	}
}
func TestReceiptsRequireOriginalIDAndReplyRouting(t *testing.T) {
	var mu sync.Mutex
	queue := []map[string]any{}
	b := fakeBus(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/status":
			statusReply(w)
		case "/send":
			v := acceptance(w, r)
			mu.Lock()
			base := map[string]any{"message_id": "r", "from": "#other", "to": "jobs", "topic": v["topic"], "tag": v["tag"]}
			for _, receipt := range []struct{ kind, re string }{{"done", ""}, {"ack", "wrong"}, {"ack", "sent"}, {"", "sent"}} {
				m := map[string]any{}
				for k, x := range base {
					m[k] = x
				}
				m["receipt"] = receipt.kind
				m["re"] = receipt.re
				m["body"] = "answer"
				queue = append(queue, m)
			}
			mu.Unlock()
		case "/consume":
			if r.URL.Query().Get("inbox") != "jobs" {
				t.Errorf("wrong inbox %s", r.URL.RawQuery)
			}
			mu.Lock()
			defer mu.Unlock()
			if len(queue) == 0 {
				w.WriteHeader(204)
				return
			}
			v := queue[0]
			queue = queue[1:]
			json.NewEncoder(w).Encode(v)
		}
	})
	out, e := b.Call(context.Background(), "#other", "x", WithInbox("jobs"), WithTimeout(time.Second))
	if e != nil || !out.Answered || out.Done || !out.Acked {
		t.Fatal(out, e)
	}
	r := Envelope{ID: "request", From: "#source", Topic: "old", Tag: "old", ReplyTo: &ReplyTo{Name: "jobs", Topic: "new", Tag: "new"}}
	m, e := b.Reply(context.Background(), &r, "")
	if e != nil || m.To != "jobs" || m.Topic != "new" || m.Tag != "new" || m.Re != "request" {
		t.Fatal(m, e)
	}
}
func TestCloseWakesCallAndCancellationReleasesReader(t *testing.T) {
	started := make(chan struct{}, 1)
	var mu sync.Mutex
	var sent map[string]any
	b := fakeBus(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/status":
			statusReply(w)
		case "/send":
			v := acceptance(w, r)
			mu.Lock()
			sent = v
			mu.Unlock()
		case "/consume":
			time.Sleep(5 * time.Millisecond)
			mu.Lock()
			v := sent
			mu.Unlock()
			if v == nil {
				w.WriteHeader(204)
			} else {
				json.NewEncoder(w).Encode(map[string]any{"message_id": "ack", "to": "#me", "topic": v["topic"], "tag": v["tag"], "receipt": "ack", "re": "sent"})
			}
		}
	})
	observer := WithAckObserver(func(Envelope) {
		select {
		case started <- struct{}{}:
		default:
		}
	})
	result := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _, e := b.Call(ctx, "#other", "x", observer); result <- e }()
	<-started
	cancel()
	select {
	case e := <-result:
		if !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("call cancellation blocked")
	}
	if _, e := b.Consume(context.Background()); e != nil {
		t.Fatal(e)
	}
	go func() { _, e := b.Call(context.Background(), "#other", "x", observer); result <- e }()
	<-started
	b.Close()
	select {
	case e := <-result:
		if !errors.Is(e, ErrClosed) {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("close did not wake call")
	}
}
func TestLeaseRetiresAfterUncertainMutation(t *testing.T) {
	for _, method := range []string{"release", "extend"} {
		t.Run(method, func(t *testing.T) {
			var mutations atomic.Int32
			b := fakeBus(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/status" {
					statusReply(w)
					return
				}
				if r.URL.Path == "/try-lock" {
					io.WriteString(w, `{"holder":"#me"}`)
					return
				}
				mutations.Add(1)
				c, _, _ := w.(http.Hijacker).Hijack()
				c.Close()
			})
			lease, e := b.Record("jobs").Locks.TryAcquire(context.Background(), "x", time.Second)
			if e != nil {
				t.Fatal(e)
			}
			if method == "release" {
				e = lease.Release(context.Background())
			} else {
				e = lease.Extend(context.Background(), time.Second)
			}
			var u *UncertainOutcome
			if !errors.As(e, &u) {
				t.Fatal(e)
			}
			if e = lease.Release(context.Background()); e == nil || mutations.Load() != 1 {
				t.Fatal(e, mutations.Load())
			}
		})
	}
}
func TestTLSFingerprintRejectsBeforeCredentialsAndUnixHTTP(t *testing.T) {
	var credentials atomic.Int32
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Agent-Bus-Token") != "" {
			credentials.Add(1)
		}
		statusReply(w)
	}))
	defer s.Close()
	sum := sha256.Sum256(s.Certificate().Raw)
	pin := hex.EncodeToString(sum[:])
	for _, correct := range []bool{false, true} {
		p := strings.Repeat("0", 64)
		if correct {
			p = pin
		}
		b, e := Dial(Options{Addr: s.URL, Token: "secret", ExpectedName: "#me", TLSFingerprint: p})
		if e != nil {
			t.Fatal(e)
		}
		_, e = b.Status(context.Background())
		b.Close()
		if correct && e != nil {
			t.Fatal(e)
		}
		if !correct && (e == nil || credentials.Load() != 0) {
			t.Fatal("credentials sent on wrong pin", e)
		}
	}
	dir := shortSocketDir(t)
	path := filepath.Join(dir, "b.sock")
	listener, e := listenTestUnix(t, path)
	if e != nil {
		t.Fatal(e)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { statusReply(w) })}
	go server.Serve(listener)
	defer server.Close()
	defer os.Remove(path)
	b, e := Dial(Options{Addr: path, ExpectedName: "#me"})
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	if _, e = b.Status(context.Background()); e != nil {
		t.Fatal(e)
	}
}

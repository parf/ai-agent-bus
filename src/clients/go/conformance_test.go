package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type conformanceCase struct {
	ID       string   `json:"id"`
	Request  Envelope `json:"request"`
	Answer   string   `json:"answer"`
	Expected Envelope `json:"expected"`
}

func sharedCases(t *testing.T, name string) []conformanceCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "client-conformance", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []conformanceCase
	if err = json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("shared case file must contain concrete examples: " + name)
	}
	return rows
}
func sharedEnvelope(t *testing.T, got *Envelope, want Envelope) {
	t.Helper()
	if got == nil {
		t.Fatal("no envelope")
	}
	if got.From != want.From || got.To != want.To || got.Body != want.Body || got.Topic != want.Topic || got.Ask != want.Ask {
		t.Fatalf("shared envelope: got %+v, want %+v", got, want)
	}
	if want.Tag != "" && got.Tag != want.Tag {
		t.Fatalf("shared tag: %q want %q", got.Tag, want.Tag)
	}
	if got.ID == "" {
		t.Fatal("missing message id")
	}
}
func sharedServe(t *testing.T, b *Bus, answer string) {
	sharedHandler(t, b, func(context.Context, Envelope) (*string, error) { return &answer, nil })
}
func sharedHandler(t *testing.T, b *Bus, handler func(context.Context, Envelope) (*string, error)) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- b.Serve(ctx, handler, WithWorkers(1)) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-result:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("serve did not drain")
		}
	})
	end := time.Now().Add(time.Second)
	for {
		b.mu.Lock()
		ready := b.serving != nil
		b.mu.Unlock()
		if ready {
			return cancel
		}
		if time.Now().After(end) {
			t.Fatal("serve did not start")
		}
		time.Sleep(time.Millisecond)
	}
}
func TestSharedConformanceSend(t *testing.T) {
	addr, tokens := realFixture(t)
	caller := fixtureBus(t, addr, tokens["#caller@fixture"], "#caller@fixture")
	worker := fixtureBus(t, addr, tokens["#worker@fixture"], "#worker@fixture")
	for _, row := range sharedCases(t, "send") {
		t.Run(row.ID, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			sent, err := caller.Send(ctx, row.Request.To, row.Request.Body, WithTopic(row.Request.Topic), WithTag(row.Request.Tag), WithAsk(row.Request.Ask))
			if err != nil {
				t.Fatal(err)
			}
			got, err := worker.Consume(ctx, WithWait(time.Second))
			if err != nil {
				t.Fatal(err)
			}
			sharedEnvelope(t, got, row.Expected)
			if got.ID != sent.ID {
				t.Fatal("acceptance id differs from received id")
			}
		})
	}
}
func TestSharedConformanceCall(t *testing.T) {
	for _, row := range sharedCases(t, "call") {
		t.Run(row.ID, func(t *testing.T) {
			addr, tokens := realFixture(t)
			caller := fixtureBus(t, addr, tokens["#caller@fixture"], "#caller@fixture")
			worker := fixtureBus(t, addr, tokens["#worker@fixture"], "#worker@fixture")
			sharedServe(t, worker, row.Answer)
			out, err := caller.Call(context.Background(), row.Request.To, row.Request.Body, WithTopic(row.Request.Topic), WithTimeout(2*time.Second))
			if err != nil {
				t.Fatal(err)
			}
			sharedEnvelope(t, out.Message, row.Expected)
			if !out.Answered || !out.Acked || out.Message.Re == "" || out.Message.Re == out.Message.ID || out.Message.Tag == "" {
				t.Fatalf("invalid correlated answer: %+v", out)
			}
		})
	}
}
func TestSharedConformanceSelfCall(t *testing.T) {
	for _, row := range sharedCases(t, "self-call") {
		t.Run(row.ID, func(t *testing.T) {
			addr, tokens := realFixture(t)
			caller := fixtureBus(t, addr, tokens["#caller@fixture"], "#caller@fixture")
			sharedServe(t, caller, row.Answer)
			out, err := caller.Call(context.Background(), row.Request.To, row.Request.Body, WithTopic(row.Request.Topic), WithTimeout(2*time.Second))
			if err != nil {
				t.Fatal(err)
			}
			sharedEnvelope(t, out.Message, row.Expected)
			if !out.Answered || !out.Acked || out.Message.Re == "" {
				t.Fatalf("self request mistaken for answer: %+v", out)
			}
		})
	}
}

func TestSelfCallAlternateReplyInbox(t *testing.T) {
	row := sharedCases(t, "self-call")[0]
	addr, tokens := realFixture(t)
	caller := fixtureBus(t, addr, tokens["#caller@fixture"], "#caller@fixture")
	sharedServe(t, caller, row.Answer)
	out, err := caller.Call(context.Background(), row.Request.To, row.Request.Body, WithTopic(row.Request.Topic), WithInbox("jobs@fixture"), WithTimeout(2*time.Second))
	if err != nil {
		t.Fatalf("self call did not get handler answer via alternate inbox: %v", err)
	}
	want := row.Expected
	want.To = "jobs@fixture"
	sharedEnvelope(t, out.Message, want)
	if !out.Answered || !out.Acked || out.Message.Re == "" {
		t.Fatalf("invalid alternate self reply: %+v", out)
	}
}
func TestSelfCallWaitsForCapacityAndDrainsKnownWork(t *testing.T) {
	row := sharedCases(t, "self-call")[0]
	addr, tokens := realFixture(t)
	caller := fixtureBus(t, addr, tokens["#caller@fixture"], "#caller@fixture")
	owner := fixtureBus(t, addr, tokens["owner@fixture"], "owner@fixture")
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	var handled atomic.Int32
	stop := sharedHandler(t, caller, func(_ context.Context, m Envelope) (*string, error) {
		handled.Add(1)
		if m.Body == "hold slot" {
			close(entered)
			<-release
		}
		return &row.Answer, nil
	})
	caller.mu.Lock()
	server := caller.serving
	caller.mu.Unlock()
	if _, err := owner.Send(context.Background(), caller.you, "hold slot"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first handler did not start")
	}
	type result struct {
		out CallOutcome
		err error
	}
	done := make(chan result, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				done <- result{err: fmt.Errorf("self queue panic: %v", p)}
			}
		}()
		out, err := caller.Call(context.Background(), row.Request.To, row.Request.Body, WithTopic(row.Request.Topic), WithTimeout(2*time.Second))
		done <- result{out, err}
	}()
	until := time.Now().Add(time.Second)
	for server.selfWaiting.Load() == 0 && handled.Load() == 1 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if handled.Load() != 1 {
		t.Fatalf("self work bypassed capacity: %d handlers", handled.Load())
	}
	if server.selfWaiting.Load() != 1 {
		t.Fatal("self request was not retained while capacity was full")
	}
	stop()
	unblock()
	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("known self work was not drained: %v", got.err)
		}
		sharedEnvelope(t, got.out.Message, row.Expected)
	case <-time.After(3 * time.Second):
		t.Fatal("known self work did not finish")
	}
	if handled.Load() != 2 {
		t.Fatalf("self request handled %d times, want one after the first handler", handled.Load()-1)
	}
}

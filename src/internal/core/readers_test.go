package core

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestReadersCountsEveryOutstandingReadAndRemovesEndedWaits(t *testing.T) {
	b := newBusWith(t, "svc@h", "sender@h")
	live := func() protocol.Record {
		b.mu.Lock()
		defer b.mu.Unlock()
		return b.withLiveness("svc@h", b.records["svc@h"])
	}
	wait := func(readers int) protocol.Record {
		t.Helper()
		for deadline := time.Now().Add(2 * time.Second); ; {
			got := live()
			if got.Readers != nil && *got.Readers == readers {
				return got
			}
			if time.Now().After(deadline) {
				t.Fatalf("readers=%v, want %d", got.Readers, readers)
			}
			time.Sleep(time.Millisecond)
		}
	}
	run := func(ctx context.Context, topic string, filtered, share bool) <-chan error {
		done := make(chan error, 1)
		go func() {
			_, err := b.Consume(ctx, "svc@h", topic, "", filtered, share)
			done <- err
		}()
		return done
	}

	if got := live(); got.Readers == nil || *got.Readers != 0 || got.Reading {
		t.Fatalf("idle inbox: %+v", got)
	}
	filteredCtx, stopFiltered := context.WithCancel(context.Background())
	defer stopFiltered()
	filteredDone := run(filteredCtx, "wanted", true, false)
	if got := wait(1); got.Reading {
		t.Fatal("a filtered-only wait set the legacy unfiltered-reading bit")
	}

	firstCtx, stopFirst := context.WithCancel(context.Background())
	defer stopFirst()
	firstDone := run(firstCtx, "", false, true)
	if got := wait(2); !got.Reading {
		t.Fatal("an unfiltered wait did not set the compatibility bit")
	}
	secondCtx, stopSecond := context.WithCancel(context.Background())
	defer stopSecond()
	secondDone := run(secondCtx, "", false, true)
	wait(3)

	if _, err := b.Send(protocol.Envelope{From: "sender@h", To: "svc@h", Topic: "wanted", Body: "matched"}); err != nil {
		t.Fatal(err)
	}
	if err := <-filteredDone; err != nil {
		t.Fatalf("filtered completion: %v", err)
	}
	wait(2)

	stopFirst()
	if err := <-firstDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled read: %v", err)
	}
	wait(1)
	stopSecond()
	if err := <-secondDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("second canceled read: %v", err)
	}
	if got := wait(0); got.Reading {
		t.Fatal("the last ended wait left the compatibility bit set")
	}

	timeoutCtx, stopTimeout := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer stopTimeout()
	timeoutDone := run(timeoutCtx, "never", true, false)
	wait(1)
	if err := <-timeoutDone; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timed read: %v", err)
	}
	wait(0)
}

func TestReadersIsNeverRestoredOrPersisted(t *testing.T) {
	b := New()
	b.Restore(ports.Snapshot{Clean: true, Records: []protocol.Record{{Kind: protocol.KindAgent, 
		Name: "svc@h", Owner: "svc@h", Readers: ptr(99), Reading: true,
	}}})
	if got, ok := b.Lookup("svc@h", "svc@h"); !ok || got.Readers == nil || *got.Readers != 0 || got.Reading {
		t.Fatalf("restored live state: ok=%v record=%+v", ok, got)
	}
	raw, err := json.Marshal(b.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	if containsRecordJSONField(t, raw, "readers") || containsRecordJSONField(t, raw, "reading") {
		t.Fatalf("snapshot persisted live reader state: %s", raw)
	}
}

func TestReadersRemovesATimedOutRead(t *testing.T) {
	b := newBusWith(t, "svc@h")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := b.Consume(ctx, "svc@h", "never", "", true, false)
		done <- err
	}()
	waitForWaiters(t, b, "svc@h", 1)
	if err := <-done; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timed read: %v", err)
	}
	b.mu.Lock()
	got := b.withLiveness("svc@h", b.records["svc@h"])
	b.mu.Unlock()
	if got.Readers == nil || *got.Readers != 0 {
		t.Fatalf("timed-out read remained visible: %v", got.Readers)
	}
}

func containsRecordJSONField(t *testing.T, raw []byte, name string) bool {
	t.Helper()
	var snapshot map[string]any
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	records, ok := snapshot["Records"].([]any)
	if !ok || len(records) != 1 {
		t.Fatalf("snapshot records have unexpected shape: %s", raw)
	}
	for _, item := range records {
		record, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("snapshot record has unexpected shape: %s", raw)
		}
		if _, present := record[name]; present {
			return true
		}
	}
	return false
}

package core

import (
	"context"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Each filter selects on its own field: `--topic X` takes a message on topic
// X whatever its tag, and `--tag g` one tagged g on any topic
// (docs/04-messaging.md#inbox-selection-and-filters). Both the queued and
// the waiting path are checked, because each matches separately.
func TestAFilterSelectsOnlyTheFieldItNames(t *testing.T) {
	b := newBusWith(t, "#svc@h", "#peer@h")
	send := func(topic, tag, body string) {
		t.Helper()
		if _, err := b.Send(protocol.Envelope{From: "#peer@h", To: "#svc@h", Topic: topic, Tag: tag, Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	take := func(topic, tag string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		e, err := b.Consume(ctx, "#svc@h", topic, tag, true, false)
		if err != nil {
			return ""
		}
		return e.Body
	}

	send("other", "g", "wrong topic")
	send("T", "g", "tagged")
	if got := take("T", ""); got != "tagged" {
		t.Fatalf("queued: topic filter alone took %q, want the tagged message on its topic", got)
	}
	send("T", "h", "on T")
	if got := take("", "g"); got != "wrong topic" {
		t.Fatalf("queued: tag filter alone took %q, want the message tagged g", got)
	}
	if got := take("T", "g"); got != "" {
		t.Fatalf("queued: both filters took %q, want nothing: none left is T and g", got)
	}

	// Waiting: the reader blocks before the message exists.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got := make(chan string, 1)
	go func() {
		e, _ := b.Consume(ctx, "#svc@h", "W", "", true, false)
		got <- e.Body
	}()
	for deadline := time.Now().Add(time.Second); ; {
		b.mu.Lock()
		n := len(b.inboxes["#svc@h"].waiters)
		b.mu.Unlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("reader never waited")
		}
		time.Sleep(time.Millisecond)
	}
	send("W", "x", "handed over")
	if body := <-got; body != "handed over" {
		t.Fatalf("waiting: topic filter alone got %q, want the tagged message handed over", body)
	}
}

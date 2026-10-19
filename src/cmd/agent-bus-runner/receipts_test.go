package main

import (
	"context"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"testing"
	"time"
)

func TestRunnerReceiptsOnlyForSlowScripts(t *testing.T) {
	for _, tc := range []struct {
		name, delay string
		receipts    int
	}{{"fast", "0.1", 0}, {"slow", "3", 1}} {
		t.Run(tc.name, func(t *testing.T) {
			r, b, _ := configurationFixture(t, "", "sleep "+tc.delay+"; printf answer")
			r.mu.Lock()
			s := r.served["#fixture@h"]
			s.in.Agent.Deadline = "10s"
			r.mu.Unlock()
			start := time.Now()
			r.runOne(s, protocol.Envelope{ID: "0123456789abcdef", From: "owner@h", To: s.name, Body: "go"})
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			var messages []protocol.Envelope
			for i := 0; i < tc.receipts+1; i++ {
				e, err := b.Consume(ctx, "owner@h", "", "", false, false)
				if err != nil {
					t.Fatal(err)
				}
				messages = append(messages, e)
			}
			if tc.receipts == 1 && (messages[0].Receipt != protocol.ReceiptAck || messages[0].At.Sub(start) < 1900*time.Millisecond) {
				t.Fatalf("slow script needs a delayed ack: %+v", messages)
			}
			last := messages[len(messages)-1]
			if last.Receipt != "" || last.Body != "answer" {
				t.Fatalf("want exactly %d messages ending in answer, got %+v", tc.receipts+1, messages)
			}
			// runOne has returned, so no receipt sender may remain pending.
			end, stop := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer stop()
			if e, err := b.Consume(end, "owner@h", "", "", false, false); err == nil {
				t.Fatalf("extra message after answer: %+v", e)
			}
		})
	}
}

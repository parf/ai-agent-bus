package core

import (
	"context"
	"errors"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A message handed to a poll whose transport says its reader is gone, while its
// context is still live, is returned to the inbox and the poll gets an error:
// a success with an empty message would be requeued again as a phantom.
func TestHandoffToAGoneReaderWithALiveContextIsAnError(t *testing.T) {
	b := newBusWith(t, "#svc@h", "#peer@h")
	if _, err := b.Send(protocol.Envelope{From: "#peer@h", To: "#svc@h", Body: "keep"}); err != nil {
		t.Fatal(err)
	}
	e := readOne(t, b, "#svc@h")
	ctx := WithAlive(context.Background(), func() bool { return false })
	w := &waiter{ctx: ctx, alive: func() bool { return false }}
	got, err := b.handed("#svc@h", w, e, ctx)
	if err == nil || !errors.Is(err, context.Canceled) || got.ID != "" {
		t.Fatalf("handed %+v %v, want an error and no message", got, err)
	}
	if again := readOne(t, b, "#svc@h"); again.Body != "keep" {
		t.Fatalf("read %q, want the message back in the inbox once", again.Body)
	}
	b.mu.Lock()
	queued := len(b.inboxes["#svc@h"].queue)
	b.mu.Unlock()
	if queued != 0 {
		t.Fatalf("%d messages left queued, want none", queued)
	}
}

package core

import (
	"context"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// slowSave is a store whose queue writes wait until released.
type slowSave struct {
	*memory.State
	entered, release chan struct{}
}

func (s *slowSave) SaveQueues(qs []ports.Queue, node []byte, clean bool) error {
	select {
	case s.entered <- struct{}{}:
		<-s.release
	default:
	}
	return s.State.SaveQueues(qs, node, clean)
}

// A flush writes without the bus lock: traffic goes on while the database
// is busy, and a queue unregistered meanwhile is not written back by the
// older copy the flush took (issues/done/2026-10/2026-10-17.md#A7).
func TestAFlushDoesNotHoldTheBus(t *testing.T) {
	for _, again := range []bool{false, true} {
		t.Run(map[bool]string{false: "unregistered", true: "registered again"}[again], func(t *testing.T) {
			flushWhileUnregistering(t, again)
		})
	}
}

func flushWhileUnregistering(t *testing.T, again bool) {
	st := &slowSave{State: memory.NewState(), entered: make(chan struct{}), release: make(chan struct{})}
	b := New()
	b.Persistence(st)
	known(t, b, "alice@h")
	provision(t, b, nil,
		protocol.Record{Name: "#a@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict},
		protocol.Record{Name: "#b@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict},
	)
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "#a@h", Body: "for the old #a"}); err != nil {
		t.Fatal(err)
	}
	flushed := make(chan error, 1)
	go func() { flushed <- b.FlushQueues(false) }()
	<-st.entered
	released := false
	release := func() {
		if !released {
			released = true
			close(st.release)
		}
	}
	t.Cleanup(release)

	moved := make(chan error, 1)
	go func() {
		if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "#b@h", Body: "during"}); err != nil {
			moved <- err
			return
		}
		_, err := b.Consume(context.Background(), "#b@h", "", "", false, false)
		if err == nil {
			_, err = b.Consume(context.Background(), "#a@h", "", "", false, false)
		}
		if err == nil {
			err = b.Unregister("#a@h", "alice@h")
		}
		if err == nil && again {
			_, err = b.Register(protocol.Record{Name: "#a@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict})
		}
		moved <- err
	}()
	select {
	case err := <-moved:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a send, a consume and an unregister waited for the queue flush")
	}
	release()
	if err := <-flushed; err != nil {
		t.Fatal(err)
	}
	after, _ := st.Load()
	for _, q := range after.Queues {
		if q.Name == "#a@h" && len(q.Messages) > 0 {
			t.Fatalf("the flush wrote back the queue unregistered while it ran: %+v", q)
		}
	}
}

package api

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func TestDaemonDispatchIsBoundedFairAndKeepsControlIndependent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	release := make(chan struct{})
	entered := make(chan struct{}, 32)
	other := make(chan struct{}, 1)
	control := make(chan struct{}, 1)
	d := newDaemonDispatch(ctx, func(e protocol.Envelope) {
		switch {
		case strings.Contains(e.Body, "#a@h"):
			entered <- struct{}{}
			<-release
		case e.Body == "status":
			control <- struct{}{}
		default:
			select {
			case other <- struct{}{}:
			default:
			}
		}
	})
	defer func() { close(release); cancel(); d.wg.Wait() }()
	a := protocol.Envelope{From: "#a@h", Body: "write #a@h notice blocked"}
	if !d.enqueue(a) {
		t.Fatal("first refused")
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("mutation did not start")
	}
	for range 16 {
		if !d.enqueue(a) {
			t.Fatal("per-agent queue refused early")
		}
	}
	if d.enqueue(a) {
		t.Fatal("per-agent queue exceeded 16")
	}
	if !d.enqueue(protocol.Envelope{From: "#b@h", Body: "write #b@h notice fair"}) {
		t.Fatal("other agent refused")
	}
	select {
	case <-other:
	case <-time.After(time.Second):
		t.Fatal("one agent occupied every mutation worker")
	}
	if !d.enqueue(protocol.Envelope{Body: "status"}) {
		t.Fatal("control refused")
	}
	select {
	case <-control:
	case <-time.After(time.Second):
		t.Fatal("mutation borrowed control lane")
	}
}

type dispatchBarrierStore struct {
	*memory.State
	block            uint32
	entered, release chan struct{}
}

func (s *dispatchBarrierStore) ReserveEventIDs(id uint32, n uint64) error {
	if id == s.block {
		close(s.entered)
		<-s.release
	}
	return s.State.ReserveEventIDs(id, n)
}
func TestDaemonControlAndOtherReadsAnswerWhileAWriteIsBlocked(t *testing.T) {
	st := &dispatchBarrierStore{State: memory.NewState(), entered: make(chan struct{}), release: make(chan struct{})}
	b := core.New()
	b.Persistence(st)
	s, _ := serverFor(t, b, "admin@h")
	if err := b.EstablishDaemonAgent("node.example"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"#a@h", "#b@h"} {
		if _, err := b.Register(protocol.Record{Name: name, Kind: protocol.KindAgent, Owner: "admin@h", Allow: []string{"*"}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.EventWrite("#b@h", "#b@h", "notice", "ready"); err != nil {
		t.Fatal(err)
	}
	a, _ := b.Lookup("admin@h", "#a@h")
	st.block = a.ID
	stop := s.ServeDaemonAgent(context.Background())
	var release sync.Once
	defer func() { release.Do(func() { close(st.release) }); stop() }()
	accepted, err := b.Send(protocol.Envelope{From: "#a@h", To: core.DaemonAgent, Body: "write #a@h notice blocked", Wait: "10s"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-st.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("write did not enter reservation barrier")
	}
	for _, tc := range []struct{ body, tag string }{{"read #b@h", "read"}, {"status", "status"}} {
		sent, err := b.Send(protocol.Envelope{From: "admin@h", To: core.DaemonAgent, Body: tc.body, Topic: "control", Tag: tc.tag, Wait: "2s"})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		answer, err := b.Consume(ctx, "admin@h", "control", tc.tag, true, false)
		cancel()
		if err != nil || answer.Re != sent.ID {
			t.Fatalf("%s blocked by another log: %+v %v", tc.body, answer, err)
		}
		if tc.tag == "read" && !strings.Contains(answer.Body, "ready") || tc.tag == "status" && !strings.HasPrefix(answer.Body, "ok: ") {
			t.Fatalf("wrong control answer: %s", answer.Body)
		}
	}
	release.Do(func() { close(st.release) })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	answer, err := b.Consume(ctx, "#a@h", "", "", false, false)
	if err != nil || answer.Re != accepted.ID {
		t.Fatalf("write did not finish after release %+v %v", answer, err)
	}
}

func TestDaemonDispatchGlobalCapacity(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	release := make(chan struct{})
	entered := make(chan struct{}, 64)
	d := newDaemonDispatch(ctx, func(e protocol.Envelope) { entered <- struct{}{}; <-release })
	defer func() { close(release); cancel(); d.wg.Wait() }()
	env := func(i int) protocol.Envelope {
		return protocol.Envelope{Body: fmt.Sprintf("write #agent%d@h notice queued", i)}
	}
	for i := 0; i < 4; i++ {
		if !d.enqueue(env(i)) {
			t.Fatal("active mutation refused")
		}
	}
	for range 4 {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("mutation workers not ready")
		}
	}
	for i := 4; i < 68; i++ {
		if !d.enqueue(env(i)) {
			t.Fatalf("global queue refused entry %d early", i-4)
		}
	}
	if d.enqueue(env(68)) {
		t.Fatal("global queue exceeded 64")
	}
}

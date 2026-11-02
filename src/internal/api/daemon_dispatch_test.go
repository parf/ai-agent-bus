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

func TestDaemonForbiddenFloodCannotReserveVictimOrGlobalCapacity(t *testing.T) {
	b := core.New()
	s, _ := serverFor(t, b, "admin@h")
	b.EstablishDaemonAgent("node.example")
	for _, name := range []string{"#victim@h", "#b@h", "#c@h", "#d@h", "#attacker@h"} {
		if _, err := b.Register(protocol.Record{Name: name, Kind: protocol.KindAgent, Owner: "admin@h", Allow: []string{"*"}}); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	release := make(chan struct{})
	entered := make(chan struct{}, 128)
	d := newDaemonDispatch(ctx, func(e protocol.Envelope) { entered <- struct{}{}; <-release })
	defer func() { close(release); cancel(); d.wg.Wait() }()
	for _, name := range []string{"#victim@h", "#b@h", "#c@h", "#d@h"} {
		if !s.dispatchDaemonTask(d, protocol.Envelope{From: name, Body: "write " + name + " alert held"}) {
			t.Fatal("legitimate first write refused")
		}
	}
	for range 4 {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("mutation workers did not enter barriers")
		}
	}
	for i := 0; i < 80; i++ {
		name := "#victim@h"
		if i >= 16 {
			name = fmt.Sprintf("#missing%d@h", i)
		}
		s.dispatchDaemonTask(d, protocol.Envelope{From: "#attacker@h", Body: "pull " + name})
	}
	accepted := s.dispatchDaemonTask(d, protocol.Envelope{From: "#victim@h", Body: "write #victim@h alert important"})
	d.mu.Lock()
	queued := d.waiting
	d.mu.Unlock()
	if !accepted || queued != 1 {
		t.Fatalf("forbidden flood reserved capacity: victim accepted=%v queued=%d", accepted, queued)
	}
}
func TestDaemonOneActiveMutationPerAgent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan struct{})
	firstRelease := make(chan struct{})
	firstDone := make(chan struct{})
	others := make(chan struct{}, 3)
	othersRelease := make(chan struct{})
	second := make(chan bool, 1)
	d := newDaemonDispatch(ctx, func(e protocol.Envelope) {
		switch e.ID {
		case "first":
			close(first)
			<-firstRelease
			close(firstDone)
		case "second":
			select {
			case <-firstDone:
				second <- false
			default:
				second <- true
			}
		default:
			others <- struct{}{}
			<-othersRelease
		}
	})
	var a, b sync.Once
	defer func() {
		a.Do(func() { close(firstRelease) })
		b.Do(func() { close(othersRelease) })
		cancel()
		d.wg.Wait()
	}()
	env := func(id, name string) protocol.Envelope {
		return protocol.Envelope{ID: id, Body: "write " + name + " notice x"}
	}
	d.enqueue(env("first", "#a@h"))
	select {
	case <-first:
	case <-time.After(time.Second):
		t.Fatal("first mutation did not start")
	}
	d.enqueue(env("second", "#a@h"))
	for _, name := range []string{"#b@h", "#c@h", "#d@h"} {
		d.enqueue(env(name, name))
	}
	for range 3 {
		select {
		case <-others:
		case <-second:
			t.Fatal("second mutation started before first returned")
		case <-time.After(time.Second):
			t.Fatal("free workers did not start other agents")
		}
	}
	select {
	case <-second:
		t.Fatal("two mutations of one agent overlapped")
	default:
	}
	a.Do(func() { close(firstRelease) })
	select {
	case overlap := <-second:
		if overlap {
			t.Fatal("second mutation overlapped first")
		}
	case <-time.After(time.Second):
		t.Fatal("second mutation did not run after first")
	}
}
func TestDaemonMutationReadyQueueIsRoundRobin(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan struct{})
	release := make(chan struct{})
	order := make(chan string, 16)
	d := newDaemonDispatchWorkers(ctx, func(e protocol.Envelope) {
		if e.ID == "A1" {
			close(first)
			<-release
		}
		order <- e.ID
	}, 1)
	var once sync.Once
	defer func() { once.Do(func() { close(release) }); cancel(); d.wg.Wait() }()
	env := func(id, name string) protocol.Envelope {
		return protocol.Envelope{ID: id, Body: "write " + name + " notice x"}
	}
	d.enqueue(env("A1", "#a@h"))
	select {
	case <-first:
	case <-time.After(time.Second):
		t.Fatal("A never started")
	}
	for _, id := range []string{"A2", "A3", "A4"} {
		d.enqueue(env(id, "#a@h"))
	}
	d.enqueue(env("B1", "#b@h"))
	d.enqueue(env("C1", "#c@h"))
	once.Do(func() { close(release) })
	for _, want := range []string{"A1", "B1", "C1", "A2"} {
		select {
		case got := <-order:
			if got != want {
				t.Fatalf("round-robin order got %s want %s", got, want)
			}
		case <-time.After(time.Second):
			t.Fatal("scheduled mutation did not finish")
		}
	}
}
func TestDaemonWaitingWriteToFullLaneGetsBusyAnswer(t *testing.T) {
	b := core.New()
	s, _ := serverFor(t, b, "admin@h")
	b.EstablishDaemonAgent("node.example")
	b.Register(protocol.Record{Name: "#a@h", Kind: protocol.KindAgent, Owner: "admin@h", Allow: []string{"*"}})
	ctx, cancel := context.WithCancel(context.Background())
	entered := make(chan struct{}, 32)
	release := make(chan struct{})
	d := newDaemonDispatch(ctx, func(e protocol.Envelope) { entered <- struct{}{}; <-release })
	defer func() { close(release); cancel(); d.wg.Wait() }()
	e := protocol.Envelope{From: "#a@h", Body: "write #a@h alert queued"}
	s.dispatchDaemonTask(d, e)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first never started")
	}
	for range 16 {
		if !s.dispatchDaemonTask(d, e) {
			t.Fatal("lane refused early")
		}
	}
	e.ID = "0123456789abcdef"
	e.Wait = "2s"
	if s.dispatchDaemonTask(d, e) {
		t.Fatal("full lane accepted waiting call")
	}
	readCtx, end := context.WithTimeout(context.Background(), time.Second)
	defer end()
	answer, err := b.Consume(readCtx, "#a@h", "", "", false, false)
	if err != nil || answer.Re != e.ID || answer.Body != "error: busy: Agent event dispatch is busy." {
		t.Fatalf("missing exact busy answer: %+v %v", answer, err)
	}
}

type twoHungEventReads struct {
	*memory.State
	entered chan uint32
	release chan struct{}
}

func (s *twoHungEventReads) EventHighWater(id uint32) (uint64, error) {
	s.entered <- id
	<-s.release
	return s.State.EventHighWater(id)
}
func TestDaemonHealthMethodsAnswerWithBothEventReadersHung(t *testing.T) {
	st := &twoHungEventReads{State: memory.NewState(), entered: make(chan uint32, 2), release: make(chan struct{})}
	b := core.New()
	b.Persistence(st)
	s, _ := serverFor(t, b, "admin@h")
	b.EstablishDaemonAgent("node.example")
	for _, name := range []string{"#a@h", "#b@h"} {
		b.Register(protocol.Record{Name: name, Kind: protocol.KindAgent, Owner: "admin@h"})
	}
	stop := s.ServeDaemonAgent(context.Background())
	defer func() { close(st.release); stop() }()
	for _, name := range []string{"#a@h", "#b@h"} {
		if _, err := b.Send(protocol.Envelope{From: "admin@h", To: core.DaemonAgent, Body: "read " + name, Topic: "hung"}); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		select {
		case <-st.entered:
		case <-time.After(time.Second):
			t.Fatal("both event readers did not enter store")
		}
	}
	for _, method := range []string{"status", "version", "ls", "help"} {
		sent, err := b.Send(protocol.Envelope{From: "admin@h", To: core.DaemonAgent, Body: method, Topic: "health", Tag: method, Wait: "2s"})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		answer, err := b.Consume(ctx, "admin@h", "health", method, true, false)
		cancel()
		if err != nil || answer.Re != sent.ID || answer.Body == "" || strings.HasPrefix(answer.Body, "error:") {
			t.Fatalf("health method %s depended on event store: %+v %v", method, answer, err)
		}
	}
}

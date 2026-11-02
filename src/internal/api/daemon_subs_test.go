package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// slowSubs is a store whose subscription reads can be held: the read numbered
// holdAt waits, standing in for a delivery that is in flight and held.
type slowSubs struct {
	*memory.State
	reads  atomic.Int64
	holdAt int64
	held   chan struct{}
	freed  chan struct{}
	once   sync.Once
}

// release frees a held read; safe to call twice, and run at cleanup so a failed
// assertion cannot leave the server's stop waiting on a delivery that never ends.
func (s *slowSubs) release() { s.once.Do(func() { close(s.freed) }) }

func (s *slowSubs) ListSubscriptions(source uint32) (map[string][]byte, error) {
	if n := s.reads.Add(1); s.holdAt > 0 && n == s.holdAt {
		close(s.held)
		<-s.freed
	}
	return s.State.ListSubscriptions(source)
}

type subAPI struct {
	t    *testing.T
	b    *core.Bus
	st   *slowSubs
	tags atomic.Int64
}

func newSubAPI(t *testing.T) *subAPI {
	t.Helper()
	st := &slowSubs{State: memory.NewState(), held: make(chan struct{}), freed: make(chan struct{})}
	b := core.New()
	b.Persistence(st)
	s, _ := serverFor(t, b, "admin@h")
	if err := b.EstablishDaemonAgent("node.example"); err != nil {
		t.Fatal(err)
	}
	for _, r := range []protocol.Record{
		{Name: "#a@h", Kind: protocol.KindAgent, Owner: "admin@h", Allow: []string{"*"}},
		{Name: "#other@h", Kind: protocol.KindAgent, Owner: "admin@h", Allow: []string{"*"}},
		{Name: "q@h", Kind: protocol.KindQueue, Owner: "admin@h", Allow: []string{"*"}},
	} {
		if _, err := b.Register(r); err != nil {
			t.Fatal(err)
		}
	}
	stop := s.ServeDaemonAgent(context.Background())
	t.Cleanup(stop)
	return &subAPI{t: t, b: b, st: st}
}

// reply makes the queue one call's answer comes back to: an inbox has one
// reader, and calls run at the same time.
func (a *subAPI) reply() string {
	name := fmt.Sprintf("ans%d@h", a.tags.Add(1))
	if _, err := a.b.Register(protocol.Record{Name: name, Kind: protocol.KindQueue, Owner: "admin@h", Allow: []string{"*"}}); err != nil {
		a.t.Fatal(err)
	}
	return name
}

// send puts a call to #bus and says where its answer will be.
func (a *subAPI) send(from, body, wait string) (queue, id string) {
	a.t.Helper()
	queue = a.reply()
	sent, err := a.b.Send(protocol.Envelope{From: from, To: core.DaemonAgent, Body: body, Wait: wait, Topic: "call", Tag: "x",
		ReplyTo: &protocol.ReplyTo{Name: queue, Topic: "call", Tag: "x"}})
	if err != nil {
		a.t.Fatal(err)
	}
	return queue, sent.ID
}

// ask sends a call to #bus and returns the answer, within the wait.
func (a *subAPI) ask(from, body, wait string) (string, bool) {
	a.t.Helper()
	queue, id := a.send(from, body, wait)
	return a.take(queue, id, 3*time.Second)
}

func (a *subAPI) take(queue, id string, within time.Duration) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), within)
	defer cancel()
	e, err := a.b.Consume(ctx, queue, "call", "x", false, false)
	if err != nil || e.Re != id {
		return "", false
	}
	return e.Body, true
}

// askBefore is ask with a bound on the whole call, the send included: a call
// that cannot even be accepted fails on its deadline, not as a hang.
func (a *subAPI) askBefore(from, body string, within time.Duration) (string, bool) {
	a.t.Helper()
	type result struct {
		out string
		ok  bool
	}
	done := make(chan result, 1)
	go func() {
		queue := a.reply()
		sent, err := a.b.Send(protocol.Envelope{From: from, To: core.DaemonAgent, Body: body, Wait: "5s", Topic: "call", Tag: "x",
			ReplyTo: &protocol.ReplyTo{Name: queue, Topic: "call", Tag: "x"}})
		if err != nil {
			done <- result{}
			return
		}
		out, ok := a.take(queue, sent.ID, within)
		done <- result{out, ok}
	}()
	select {
	case r := <-done:
		return r.out, r.ok
	case <-time.After(within):
		return "", false
	}
}

func (a *subAPI) mustAsk(from, body string) string {
	a.t.Helper()
	out, ok := a.ask(from, body, "5s")
	if !ok {
		a.t.Fatalf("no answer to %q", body)
	}
	return out
}

func (a *subAPI) copies(queue string) []protocol.Envelope {
	var out []protocol.Envelope
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		e, err := a.b.Consume(ctx, queue, "", "", false, false)
		cancel()
		if err != nil {
			return out
		}
		out = append(out, e)
	}
}

func errorCode(t *testing.T, answer string) string {
	t.Helper()
	var v struct{ Error struct{ Code string } }
	if json.Unmarshal([]byte(answer), &v) != nil {
		t.Fatalf("not an error answer: %s", answer)
	}
	return v.Error.Code
}

// sub add, list and delete through #bus, in text and structured forms.
func TestDaemonSubscriptionCallsAddListAndDelete(t *testing.T) {
	a := newSubAPI(t)
	var row map[string]any
	if err := json.Unmarshal([]byte(a.mustAsk("admin@h", "sub add q@h #a@h alert /boom in prod/i")), &row); err != nil {
		t.Fatal(err)
	}
	if row["target"] != "q@h" || row["filter"] != "/boom in prod/i" || row["created_by"] != "admin@h" || row["sent"] != float64(0) {
		t.Fatalf("the added row: %v", row)
	}
	if sel, _ := row["selector"].(map[string]any); sel["level"] != "alert" {
		t.Fatalf("the selector: %v", row["selector"])
	}
	id := row["id"].(string)
	var listed struct{ Subscriptions []map[string]any }
	if err := json.Unmarshal([]byte(a.mustAsk("admin@h", `{"method":"sub.list","log":"#a@h"}`)), &listed); err != nil || len(listed.Subscriptions) != 1 || listed.Subscriptions[0]["id"] != id || listed.Subscriptions[0]["source"] != "#a@h" {
		t.Fatalf("the structured listing: %+v %v", listed, err)
	}
	if err := json.Unmarshal([]byte(a.mustAsk("admin@h", "sub list")), &listed); err != nil || len(listed.Subscriptions) != 1 {
		t.Fatalf("the unscoped listing: %+v %v", listed, err)
	}
	// A structured add with a bare filter and no level.
	var second map[string]any
	if err := json.Unmarshal([]byte(a.mustAsk("admin@h", `{"method":"sub.add","target":"q@h","log":"#a@h","filter":"fatal"}`)), &second); err != nil || second["filter"] != "fatal" {
		t.Fatalf("the structured add: %v %v", second, err)
	}
	if got := a.mustAsk("admin@h", "sub delete "+id); got != `{"deleted":true}` {
		t.Fatalf("delete answered %s", got)
	}
	if code := errorCode(t, a.mustAsk("admin@h", "sub delete "+id)); code != "not_found" {
		t.Fatalf("a repeated delete: %s", code)
	}
	// Refusals: a non-manager, a bad filter, a bad id, a bad form.
	if code := errorCode(t, a.mustAsk("#other@h", "sub add q@h #a@h")); code != "forbidden" {
		t.Fatalf("a non-manager added: %s", code)
	}
	if code := errorCode(t, a.mustAsk("admin@h", "sub add q@h #a@h alert /(a+)+/")); code != "invalid_filter" {
		t.Fatalf("a filter outside the subset: %s", code)
	}
	for body, want := range map[string]string{"sub delete zz": "invalid_request", "sub add q@h": "invalid_request", `{"method":"sub.add","target":"q@h"}`: "invalid_request", `{"method":"sub.list","bogus":"x"}`: "invalid_request"} {
		if code := errorCode(t, a.mustAsk("admin@h", body)); code != want {
			t.Errorf("%s answered %s, want %s", body, code, want)
		}
	}
}

// Every token of a text call is split on the same white space, the method
// words included: a tab or a newline gives the row the same call with spaces
// gives, never a shifted word.
func TestASubscriptionTextCallSplitsEveryWordOnTheSameWhiteSpace(t *testing.T) {
	a := newSubAPI(t)
	for _, body := range []string{
		"sub\tadd q@h #a@h alert /x/",
		"sub add\tq@h #a@h alert /x/",
		"sub add q@h #a@h\talert /x/",
		"sub add q@h\n#a@h\nalert\n/x/",
		"\tsub \t add  q@h  #a@h  alert  /x/",
	} {
		var row map[string]any
		if err := json.Unmarshal([]byte(a.mustAsk("admin@h", body)), &row); err != nil {
			t.Fatalf("%q: %v", body, err)
		}
		sel, _ := row["selector"].(map[string]any)
		if row["target"] != "q@h" || sel["level"] != "alert" || row["filter"] != "/x/" {
			t.Errorf("%q made %v, want target q@h, level alert, filter /x/", body, row)
		}
	}
	for _, body := range []string{"sub\tlist\t#a@h", "sub list\n#a@h"} {
		var listed struct{ Subscriptions []map[string]any }
		if err := json.Unmarshal([]byte(a.mustAsk("admin@h", body)), &listed); err != nil || len(listed.Subscriptions) != 5 {
			t.Errorf("%q listed %d (%v), want the 5 made above", body, len(listed.Subscriptions), err)
		}
	}
	var row map[string]any
	json.Unmarshal([]byte(a.mustAsk("admin@h", "sub add q@h #a@h")), &row)
	if got := a.mustAsk("admin@h", "sub\tdelete\t"+row["id"].(string)); got != `{"deleted":true}` {
		t.Errorf("a delete with tabs answered %s", got)
	}
	if code := errorCode(t, a.mustAsk("admin@h", "sub\tdelete\t"+row["id"].(string)+"\textra")); code != "invalid_request" {
		t.Errorf("a delete with a stray word: %s", code)
	}
}

// A write is done when its fan-out is: the answer carries the real counts, a
// copy comes from #bus once per matching subscription, and a refusing target is
// counted dropped.
func TestAWriteAnswersWithItsRealDeliveries(t *testing.T) {
	a := newSubAPI(t)
	a.mustAsk("admin@h", "sub add q@h #a@h alert /boom/i")
	if _, err := a.b.Register(protocol.Record{Name: "shut@h", Kind: protocol.KindQueue, Owner: "admin@h", Allow: []string{"admin@h", "#bus"}}); err != nil {
		t.Fatal(err)
	}
	a.mustAsk("admin@h", "sub add shut@h #a@h alert")
	if _, err := a.b.Manage("admin@h", core.Management{Name: "shut@h", Allow: &[]string{"admin@h"}}); err != nil { // it stops admitting #bus after the subscription
		t.Fatal(err)
	}
	var out struct {
		Entry      map[string]any
		Deliveries map[string]int
	}
	// The write is the agent's own; its answer needs #bus admitted by #a@h (Allow *).
	if err := json.Unmarshal([]byte(a.mustAsk("#a@h", "write #a@h alert BOOM in prod")), &out); err != nil {
		t.Fatal(err)
	}
	if out.Entry["text"] != "BOOM in prod" || out.Deliveries["matched"] != 2 || out.Deliveries["sent"] != 1 || out.Deliveries["dropped"] != 1 {
		t.Fatalf("the write's answer: %+v, want matched 2, sent 1 (q@h), dropped 1 (shut@h)", out)
	}
	got := a.copies("q@h")
	if len(got) != 1 || got[0].From != "#bus" || got[0].Topic != "#a@h" || got[0].Tag != "alert" || !strings.Contains(got[0].Body, "BOOM in prod") {
		t.Fatalf("the copy in q@h: %+v", got)
	}
	if len(a.copies("shut@h")) != 0 {
		t.Fatal("a refused copy was delivered")
	}
	// A notice matches no subscription: matched 0, nothing sent.
	if err := json.Unmarshal([]byte(a.mustAsk("#a@h", "write #a@h notice quiet")), &out); err != nil || out.Deliveries["matched"] != 0 || len(a.copies("q@h")) != 0 {
		t.Fatalf("a notice: %+v %v", out, err)
	}
}

// A write that could feed a subscriber and has under three seconds left is
// refused before it is stored.
func TestAWriteWithoutTimeForItsSubscribersIsRefusedBeforeItIsStored(t *testing.T) {
	a := newSubAPI(t)
	if got, ok := a.ask("#a@h", "write #a@h alert early", "1s"); !ok || !strings.HasPrefix(got, "error: ") && !strings.Contains(got, "entry") {
		t.Fatalf("a write with no subscribers: %q %v", got, ok)
	}
	a.mustAsk("admin@h", "sub add q@h #a@h alert")
	got, ok := a.ask("#a@h", "write #a@h alert too late", "1s")
	if !ok || !strings.HasPrefix(got, "error: deadline:") {
		t.Fatalf("a write with 1 s left and a subscriber answered %q (%v), want error: deadline", got, ok)
	}
	v, err := a.b.EventRead("admin@h", "#a@h", "alert")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range v.Rings["alert"] {
		if e.Text == "too late" {
			t.Fatal("the refused write was stored")
		}
	}
}

// While a delivery is in flight and held: an unrelated read and status answer;
// a pull of the same log waits behind it and cannot take the entry before its
// copy is sent; after release the copy is exactly one and the pull returns the entry.
func TestAHeldDeliveryBlocksNeitherUnrelatedCallsNorTheCopyOfItsEntry(t *testing.T) {
	a := newSubAPI(t)
	t.Cleanup(a.st.release) // runs before the server's stop
	a.mustAsk("admin@h", "sub add q@h #a@h alert")
	a.st.holdAt = a.st.reads.Load() + 1 // the next read is the delivery's row read (the preflight asks only a write that is short of time)
	wrote := make(chan string, 1)
	go func() {
		out, _ := a.ask("#a@h", "write #a@h alert held entry", "20s")
		wrote <- out
	}()
	select {
	case <-a.st.held:
	case <-time.After(3 * time.Second):
		t.Fatal("the delivery never started")
	}
	// Unrelated calls answer while the delivery is held.
	if out, ok := a.askBefore("admin@h", "status", 2*time.Second); !ok || !strings.HasPrefix(out, "ok:") {
		t.Fatalf("status while a delivery is held: %q %v", out, ok)
	}
	if _, ok := a.askBefore("admin@h", "read #other@h", 2*time.Second); !ok {
		t.Fatal("an unrelated read waited for a held delivery")
	}
	// A pull of the same log is queued behind it and does not answer yet.
	pullQueue, pullID := a.send("admin@h", "pull #a@h", "20s")
	if out, ok := a.take(pullQueue, pullID, 300*time.Millisecond); ok {
		t.Fatalf("a pull took an entry whose copy was not sent yet: %s", out)
	}
	if len(a.copies("q@h")) != 0 {
		t.Fatal("the copy was sent while its delivery was held")
	}
	a.st.release()
	var done struct{ Deliveries map[string]int }
	select {
	case out := <-wrote:
		if err := json.Unmarshal([]byte(out), &done); err != nil || done.Deliveries["sent"] != 1 {
			t.Fatalf("the held write's answer: %s %v", out, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the write did not finish after its delivery was released")
	}
	pulled, ok := a.take(pullQueue, pullID, 5*time.Second)
	if !ok || !strings.Contains(pulled, "held entry") {
		t.Fatalf("the pull after the release: %q %v, want the entry", pulled, ok)
	}
	if n := len(a.copies("q@h")); n != 1 {
		t.Fatalf("the copy was delivered %d times, want exactly once", n)
	}
}

// The subscription calls are listed to those who manage a log and hidden from
// those who do not.
func TestTheSubscriptionCallsAreInHelpOnlyForManagers(t *testing.T) {
	a := newSubAPI(t)
	manager := a.mustAsk("admin@h", "help")
	for _, line := range []string{"sub add <target> <agent> [level] [/regex/]", "sub list [agent]", "sub delete <id>"} {
		if !strings.Contains(manager, line) {
			t.Errorf("a manager's help lacks %q:\n%s", line, manager)
		}
	}
	// #a@h is an agent that manages nothing: it may write its own log and is
	// offered no subscription call.
	agent, ok := a.ask("#a@h", "help", "5s")
	if !ok || !strings.Contains(agent, "write <agent>") {
		t.Fatalf("an agent's help: %q %v", agent, ok)
	}
	if strings.Contains(agent, "sub add") || strings.Contains(agent, "sub list") || strings.Contains(agent, "sub delete") {
		t.Fatalf("an agent that manages nothing is offered the subscription calls:\n%s", agent)
	}
}

// The machine-readable method list is filtered the same way: a manager's lists
// the subscription calls, an agent that manages nothing is not shown them.
func TestTheMethodListOffersTheSubscriptionCallsOnlyToManagers(t *testing.T) {
	a := newSubAPI(t)
	methods := func(from string) map[string]any {
		queue := a.reply()
		sent, err := a.b.Send(protocol.Envelope{From: from, To: core.DaemonAgent, Ask: protocol.AskHelpJSON, Wait: "5s", Topic: "call", Tag: "x",
			ReplyTo: &protocol.ReplyTo{Name: queue, Topic: "call", Tag: "x"}})
		if err != nil {
			t.Fatal(err)
		}
		body, ok := a.take(queue, sent.ID, 3*time.Second)
		var doc struct{ Methods map[string]any }
		if !ok || json.Unmarshal([]byte(body), &doc) != nil {
			t.Fatalf("no method list for %s: %q", from, body)
		}
		return doc.Methods
	}
	manager, agent := methods("admin@h"), methods("#a@h")
	for _, name := range []string{"sub add", "sub list", "sub delete"} {
		if manager[name] == nil {
			t.Errorf("a manager's method list lacks %q", name)
		}
		if agent[name] != nil {
			t.Errorf("an agent that manages nothing is offered %q", name)
		}
	}
	if d, _ := manager["sub delete"].(map[string]any); d["destructive"] != true {
		t.Errorf("sub delete is not marked destructive: %v", manager["sub delete"])
	}
}

// The refusal of a target that cannot take #bus's copies uses the existing invalid_target code on the
// wire; only its text says why.
func TestSubAddTargetThatCannotTakeBusCopiesIsInvalidTargetWithItsReason(t *testing.T) {
	a := newSubAPI(t)
	if _, err := a.b.Register(protocol.Record{Name: "#nobus@h", Kind: protocol.KindAgent, Owner: "admin@h", Allow: []string{"admin@h"}}); err != nil {
		t.Fatal(err)
	}
	for body, why := range map[string]string{"sub add #bus #a@h": "#bus cannot be a subscription target", "sub add #nobus@h #a@h": "does not admit #bus"} {
		var out struct{ Error struct{ Code, Text string } }
		if err := json.Unmarshal([]byte(a.mustAsk("admin@h", body)), &out); err != nil {
			t.Fatal(err)
		}
		if out.Error.Code != "invalid_target" || !strings.Contains(out.Error.Text, why) {
			t.Errorf("%s answered %+v, want invalid_target mentioning %q", body, out.Error, why)
		}
	}
}

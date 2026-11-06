package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/eventlog"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
	"github.com/parf/ai-agent-bus/internal/subscription"
)

// subFixture: #a@h is the source (Owner alice, Maintainer maint), q@h and
// q2@h are open queues, shut@h admits only alice (not #bus). With a store, the
// subscriptions persist.
func subFixture(t *testing.T, st ports.Store) *Bus {
	t.Helper()
	b := New()
	if st != nil {
		if err := b.Persistence(st); err != nil {
			t.Fatal(err)
		}
	}
	b.SetDaemonOwner("owner@h")
	for _, name := range []string{"alice@h", "bob@h", "maint@h"} {
		if _, err := b.SetUser("owner@h", protocol.User{Name: name}, true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.Register(protocol.Record{Name: "#a@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Maintainers: protocol.MaintainerList{"maint@h"}}); err != nil {
		t.Fatal(err)
	}
	for name, allow := range map[string][]string{"q@h": {"*"}, "q2@h": {"*"}, "shut@h": {"alice@h", "maint@h", DaemonAgent}} {
		if _, err := b.Register(protocol.Record{Name: name, Kind: protocol.KindQueue, Owner: "alice@h", Allow: allow}); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.EstablishDaemonAgent("node.example"); err != nil {
		t.Fatal(err)
	}
	return b
}

func subSource(b *Bus) protocol.Record {
	b.mu.Lock()
	defer b.unlock()
	return b.records["#a@h"]
}

func subEntry(id uint64, level, text string) ports.EventEntry {
	return ports.EventEntry{ID: id, Level: level, Text: text, At: "2026-11-02T00:00:00.000Z", LastAt: "2026-11-02T00:00:00.000Z", Count: 1}
}

func drain(t *testing.T, b *Bus, queue string) []protocol.Envelope {
	t.Helper()
	var out []protocol.Envelope
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		e, err := b.Consume(ctx, queue, "", "", false, false)
		cancel()
		if err != nil {
			return out
		}
		out = append(out, e)
	}
}

func subCounts(t *testing.T, b *Bus, who, id string) (sent, dropped float64) {
	t.Helper()
	rows, err := b.SubList(who, "#a@h")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		raw, _ := json.Marshal(r)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		if m["id"] == id {
			return m["sent"].(float64), m["dropped"].(float64)
		}
	}
	t.Fatalf("subscription %s not listed", id)
	return
}

// Only the source's managers subscribe and list its log's subscriptions; the
// author keeps its own after it stops managing; a manager removes any.
func TestSubscriptionAuthorityFollowsTheSourcesManagers(t *testing.T) {
	b := subFixture(t, nil)
	if _, err := b.SubAdd("bob@h", "#a@h", "q@h", "", ""); eventlog.Code(err) != "forbidden" {
		t.Fatalf("a non-manager subscribed: %v", err)
	}
	if _, err := b.SubAdd("bob@h", "#nothing@h", "q@h", "", ""); eventlog.Code(err) != "forbidden" {
		t.Fatalf("a non-manager learned a source does not exist: %v", err)
	}
	row, err := b.SubAdd("maint@h", "#a@h", "q@h", "alert", "/boom/i")
	if err != nil {
		t.Fatal(err)
	}
	if row.CreatedBy != "maint@h" || row.Target != "q@h" || row.Selector["level"] != "alert" || row.Sent != 0 {
		t.Fatalf("row %+v", row)
	}
	if _, err := b.SubList("bob@h", "#a@h"); eventlog.Code(err) != "forbidden" {
		t.Fatalf("a non-manager listed: %v", err)
	}
	if rows, err := b.SubList("alice@h", "#a@h"); err != nil || len(rows) != 1 {
		t.Fatalf("the Owner's listing: %v %v", rows, err)
	}
	// maint stops being a Maintainer: it may no longer list the source, but it
	// still lists and deletes its own subscription.
	if _, err := b.Manage("alice@h", Management{Name: "#a@h", Maintainers: ptr(protocol.MaintainerList{})}); err != nil {
		t.Fatal(err)
	}
	// Narrowed to the source, a former Maintainer sees its own row there, as
	// the bare listing shows it (A5).
	if rows, err := b.SubList("maint@h", "#a@h"); err != nil || len(rows) != 1 || rows[0].Listed.Row == nil || rows[0].Listed.Row.CreatedBy != "maint@h" {
		t.Fatalf("a former Maintainer's own row, narrowed to the source: %v %v", rows, err)
	}
	if rows, err := b.SubList("maint@h", ""); err != nil || len(rows) != 1 {
		t.Fatalf("a former Maintainer's own subscription is not listed: %v %v", rows, err)
	}
	if _, err := b.SubAdd("maint@h", "#a@h", "q@h", "", ""); eventlog.Code(err) != "forbidden" {
		t.Fatalf("a former Maintainer subscribed: %v", err)
	}
	other, err := b.SubAdd("alice@h", "#a@h", "q2@h", "", "")
	if err != nil {
		t.Fatal(err)
	}
	// Another author's row in that source stays hidden from the former
	// Maintainer, and a caller with no row there is refused as before.
	if rows, err := b.SubList("maint@h", "#a@h"); err != nil || len(rows) != 1 || rows[0].Listed.Row.ID != row.ID {
		t.Fatalf("a non-manager author saw more than its own row: %v %v", rows, err)
	}
	if rows, err := b.SubList("alice@h", "#a@h"); err != nil || len(rows) != 2 {
		t.Fatalf("the Owner's listing of both rows: %v %v", rows, err)
	}
	if _, err := b.SubList("bob@h", "#a@h"); eventlog.Code(err) != "forbidden" {
		t.Fatalf("a caller with no row in the source listed it: %v", err)
	}
	if err := b.SubDelete("maint@h", other.ID); eventlog.Code(err) != "not_author" {
		t.Fatalf("an author deleted another's subscription: %v", err)
	}
	if err := b.SubDelete("bob@h", row.ID); eventlog.Code(err) != "not_author" {
		t.Fatalf("a stranger deleted a subscription: %v", err)
	}
	if err := b.SubDelete("maint@h", row.ID); err != nil {
		t.Fatalf("an author deleting its own: %v", err)
	}
	if _, err := b.SubList("maint@h", "#a@h"); eventlog.Code(err) != "forbidden" {
		t.Fatalf("a non-manager with no row left listed the source: %v", err)
	}
	if err := b.SubDelete("alice@h", other.ID); err != nil {
		t.Fatalf("a manager deleting another's: %v", err)
	}
	if err := b.SubDelete("alice@h", other.ID); eventlog.Code(err) != "not_found" {
		t.Fatalf("a repeated delete: %v", err)
	}
}

// The author must be allowed to reach the target when it subscribes, and a
// target that does not exist or is not a queue or agent is refused.
func TestSubscribingChecksTheAuthorsReachToTheTarget(t *testing.T) {
	b := subFixture(t, nil)
	if _, err := b.SubAdd("maint@h", "#a@h", "shut@h", "", ""); err != nil {
		t.Fatalf("maint is admitted by shut@h: %v", err)
	}
	if _, err := b.SubAdd("alice@h", "#a@h", "nobody@h", "", ""); eventlog.Code(err) != "admission_refused" {
		t.Fatalf("an absent target: %v", err)
	}
	if _, err := b.SubAdd("alice@h", "#a@h", "q@h", "bogus", ""); eventlog.Code(err) != "invalid_selector" {
		t.Fatalf("a bad level: %v", err)
	}
	if _, err := b.SubAdd("alice@h", "#a@h", "q@h", "", "/(a+)+/"); eventlog.Code(err) != "invalid_filter" {
		t.Fatalf("a filter outside the subset: %v", err)
	}
	// bob manages nothing, but a queue he may not reach is refused first for a manager who may not.
	if _, err := b.Manage("alice@h", Management{Name: "shut@h", Allow: ptr([]string{"alice@h", DaemonAgent})}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SubAdd("maint@h", "#a@h", "shut@h", "", ""); eventlog.Code(err) != "admission_refused" {
		t.Fatalf("a target the author may not reach: %v", err)
	}
}

// Twenty per author, one hundred per source.
func TestSubscriptionLimits(t *testing.T) {
	b := subFixture(t, nil)
	for i := 0; i < 20; i++ {
		if _, err := b.SubAdd("alice@h", "#a@h", "q@h", "", ""); err != nil {
			t.Fatalf("subscription %d: %v", i, err)
		}
	}
	if _, err := b.SubAdd("alice@h", "#a@h", "q@h", "", ""); eventlog.Code(err) != "limit" {
		t.Fatalf("the 21st of one author: %v", err)
	}
}

// A copy is one message from #bus, the service, never the source agent: to the
// target, with the log as its topic and the level as its tag and the entry as
// its body; once per matching subscription, none for another level or a filter
// that does not match.
func TestACopyIsOneMessageFromBusToEachMatchingTarget(t *testing.T) {
	b := subFixture(t, nil)
	if _, err := b.SubAdd("alice@h", "#a@h", "q@h", "alert", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SubAdd("alice@h", "#a@h", "q2@h", "", "/boom/i"); err != nil {
		t.Fatal(err)
	}
	r := subSource(b)
	b.deliverEntry(r, subEntry(1, "alert", "BOOM in prod"), time.Now().Add(time.Minute))
	b.deliverEntry(r, subEntry(2, "notice", "quiet"), time.Now().Add(time.Minute))
	b.deliverEntry(r, subEntry(3, "alert", "all fine"), time.Now().Add(time.Minute))
	first := drain(t, b, "q@h")
	second := drain(t, b, "q2@h")
	if len(first) != 2 || len(second) != 1 {
		t.Fatalf("q@h got %d copies, q2@h %d; want 2 alerts and 1 boom", len(first), len(second))
	}
	e := first[0]
	if e.From != "#bus" || e.To != "q@h" || e.Topic != "#a@h" || e.Tag != "alert" {
		t.Fatalf("the copy is %+v, want from #bus to q@h, topic #a@h, tag alert", e)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(e.Body), &body); err != nil || body["text"] != "BOOM in prod" || body["log"] != "#a@h" || body["id"] != float64(1) || body["level"] != "alert" {
		t.Fatalf("the copy's body: %s (%v)", e.Body, err)
	}
	if second[0].Tag != "alert" || !strings.Contains(second[0].Body, "BOOM") {
		t.Fatalf("q2@h's copy %+v", second[0])
	}
}

// A refused copy is dropped and counted, the subscription stays, and the next
// entry is attempted.
func TestARefusedCopyIsDroppedCountedAndTheNextEntryIsAttempted(t *testing.T) {
	b := subFixture(t, nil)
	// shut@h admitted #bus when the subscription was made and stops admitting it: the copy is refused.
	row, err := b.SubAdd("alice@h", "#a@h", "shut@h", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "shut@h", Allow: ptr([]string{"alice@h", "maint@h"})}); err != nil {
		t.Fatal(err)
	}
	r := subSource(b)
	res := b.deliverEntry(r, subEntry(1, "alert", "x"), time.Now().Add(time.Minute))
	if res.Matched != 1 || res.Sent != 0 || res.Dropped != 1 {
		t.Fatalf("result %+v, want matched 1, dropped 1", res)
	}
	if n := len(drain(t, b, "shut@h")); n != 0 {
		t.Fatalf("a refused copy was delivered: %d", n)
	}
	// The target now admits #bus: the next entry goes.
	if _, err := b.Manage("alice@h", Management{Name: "shut@h", Allow: ptr([]string{"*"})}); err != nil {
		t.Fatal(err)
	}
	res = b.deliverEntry(r, subEntry(2, "alert", "y"), time.Now().Add(time.Minute))
	if res.Sent != 1 || res.Dropped != 0 {
		t.Fatalf("the next entry: %+v", res)
	}
	if sent, dropped := subCounts(t, b, "alice@h", row.ID); sent != 1 || dropped != 1 {
		t.Fatalf("counts sent %v dropped %v, want 1 and 1", sent, dropped)
	}
}

// The author's right to the source is checked when the subscription is
// created and not again (Q325): a Maintainer removed afterwards leaves the
// subscription delivering, while the author's reach to the target is checked at
// every send.
func TestTheSourceRightIsCheckedAtCreationAndTheTargetReachAtEverySend(t *testing.T) {
	b := subFixture(t, nil)
	byMaint, err := b.SubAdd("maint@h", "#a@h", "q@h", "", "")
	if err != nil {
		t.Fatal(err)
	}
	// bq@h is bob's queue: alice, who is not its Owner, reaches it only by its Allow.
	if _, err := b.Register(protocol.Record{Name: "bq@h", Kind: protocol.KindQueue, Owner: "bob@h", Allow: []string{"*"}}); err != nil {
		t.Fatal(err)
	}
	byAlice, err := b.SubAdd("alice@h", "#a@h", "bq@h", "", "")
	if err != nil {
		t.Fatal(err)
	}
	r := subSource(b)
	// maint stops being a Maintainer: its subscription still delivers.
	if _, err := b.Manage("alice@h", Management{Name: "#a@h", Maintainers: ptr(protocol.MaintainerList{})}); err != nil {
		t.Fatal(err)
	}
	if res := b.deliverEntry(r, subEntry(1, "alert", "x"), time.Now().Add(time.Minute)); res.Sent != 2 || res.Dropped != 0 {
		t.Fatalf("after the source right was revoked: %+v, want both delivered", res)
	}
	if len(drain(t, b, "q@h")) != 1 || len(drain(t, b, "bq@h")) != 1 {
		t.Fatal("a copy was not delivered after the source right was revoked")
	}
	// alice's reach to bq@h is revoked while #bus is still admitted: nothing
	// is sent for her, and the copy is counted dropped. maint's goes on.
	if _, err := b.Manage("bob@h", Management{Name: "bq@h", Allow: ptr([]string{"#bus"})}); err != nil {
		t.Fatal(err)
	}
	res := b.deliverEntry(r, subEntry(2, "alert", "y"), time.Now().Add(time.Minute))
	if res.Sent != 1 || res.Dropped != 1 {
		t.Fatalf("with the author's reach to the target revoked: %+v, want 1 sent and 1 dropped", res)
	}
	if len(drain(t, b, "bq@h")) != 0 || len(drain(t, b, "q@h")) != 1 {
		t.Fatal("a copy went to a target its author may no longer reach, or a healthy one was withheld")
	}
	if s, d := subCounts(t, b, "alice@h", byMaint.ID); s != 2 || d != 0 {
		t.Fatalf("maint's counts %v/%v", s, d)
	}
	if s, d := subCounts(t, b, "alice@h", byAlice.ID); s != 1 || d != 1 {
		t.Fatalf("alice's counts %v/%v", s, d)
	}
}

// The subscriptions survive a restart with their counters, and nothing is
// replayed: no copy is sent by starting again.
func TestSubscriptionsSurviveARestartAndNothingIsReplayed(t *testing.T) {
	st := memory.NewState()
	b := subFixture(t, st)
	row, err := b.SubAdd("alice@h", "#a@h", "q@h", "", "")
	if err != nil {
		t.Fatal(err)
	}
	b.deliverEntry(subSource(b), subEntry(1, "alert", "before"), time.Now().Add(time.Minute))
	if n := len(drain(t, b, "q@h")); n != 1 {
		t.Fatalf("copies before the restart: %d", n)
	}
	saved, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	again := New()
	again.Restore(saved)
	if err := again.Persistence(st); err != nil {
		t.Fatal(err)
	}
	if err := again.EstablishDaemonOwner("owner@h"); err != nil {
		t.Fatal(err)
	}
	if err := again.EstablishDaemonAgent("node.example"); err != nil {
		t.Fatal(err)
	}
	rows, err := again.SubList("alice@h", "#a@h")
	if err != nil || len(rows) != 1 {
		t.Fatalf("after a restart: %v %v", rows, err)
	}
	if sent, _ := subCounts(t, again, "alice@h", row.ID); sent != 1 {
		t.Fatalf("the counter after a restart: %v", sent)
	}
	if n := len(drain(t, again, "q@h")); n != 0 {
		t.Fatalf("a restart replayed %d copies", n)
	}
	again.deliverEntry(subSource(again), subEntry(2, "alert", "after"), time.Now().Add(time.Minute))
	if got := drain(t, again, "q@h"); len(got) != 1 || !strings.Contains(got[0].Body, "after") {
		t.Fatalf("the first entry after a restart: %v", got)
	}
}

// The store holds no credential: the stored row has the names, the selector and
// the filter, and nothing else.
func TestAStoredSubscriptionHoldsNoCredential(t *testing.T) {
	st := memory.NewState()
	b := subFixture(t, st)
	if _, err := b.SubAdd("alice@h", "#a@h", "q@h", "alert", "/x/"); err != nil {
		t.Fatal(err)
	}
	rows, _ := st.ListSubscriptions(subSource(b).ID)
	for _, raw := range rows {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		allowed := map[string]bool{"id": true, "target": true, "selector": true, "filter": true, "created_by": true, "created_at": true, "sent": true, "dropped": true}
		for k := range m {
			if !allowed[k] {
				t.Errorf("the stored subscription has a field %q", k)
			}
		}
	}
}

// A removed source takes its subscriptions with it; its name registered again
// starts with none.
func TestARemovedSourceTakesItsSubscriptionsWithIt(t *testing.T) {
	st := memory.NewState()
	b := subFixture(t, st)
	if _, err := b.SubAdd("alice@h", "#a@h", "q@h", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := b.Unregister("#a@h", "alice@h"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "#a@h", Kind: protocol.KindAgent, Owner: "bob@h"}); err != nil {
		t.Fatal(err)
	}
	rows, err := b.SubList("bob@h", "#a@h")
	if err != nil || len(rows) != 0 {
		t.Fatalf("a new registration inherited %v (%v)", rows, err)
	}
}

func TestSubErrorsUseTheLogsErrorForm(t *testing.T) {
	if got := eventlog.Code(subError(errors.New("x"))); got != "internal_error" {
		t.Fatalf("a foreign error became %q", got)
	}
	for _, code := range []string{"limit", "not_author", "invalid_filter", "admission_refused"} {
		if got := eventlog.Code(subError(fmt.Errorf("wrapped: %w", subscription.Error(code)))); got != code {
			t.Errorf("%s became %q", code, got)
		}
	}
}

// hookStore runs a hook inside a store read or write, as a slow disk lets
// standing change in the middle of a call.
type hookStore struct {
	*memory.State
	onList, afterAdd func()
}

func (h *hookStore) ListSubscriptions(source uint32) (map[string][]byte, error) {
	if h.onList != nil {
		h.onList()
	}
	return h.State.ListSubscriptions(source)
}

func (h *hookStore) AddSubscription(source uint32, id string, value []byte) error {
	err := h.State.AddSubscription(source, id, value)
	if h.afterAdd != nil {
		h.afterAdd()
	}
	return err
}

func revokeMaint(t *testing.T, b **Bus) func() {
	return func() {
		if _, err := (*b).Manage("alice@h", Management{Name: "#a@h", Maintainers: ptr(protocol.MaintainerList{})}); err != nil {
			t.Error(err)
		}
	}
}

// Standing is asked again after the store is read: a Maintainer revoked while
// the rows are being read gets nothing of the Owner's, and deletes nothing.
func TestAManagerRevokedDuringAListOrDeleteSeesAndRemovesNothing(t *testing.T) {
	var b *Bus
	hooks := &hookStore{State: memory.NewState()}
	b = subFixture(t, hooks)
	row, err := b.SubAdd("alice@h", "#a@h", "q@h", "", "")
	if err != nil {
		t.Fatal(err)
	}
	hooks.onList = revokeMaint(t, &b)
	// The hook fires inside the read: the Maintainer is revoked just before the rows come back.
	got, err := b.SubList("maint@h", "#a@h")
	if eventlog.Code(err) != "forbidden" || len(got) != 0 {
		t.Fatalf("a manager revoked during the read got %v, %v; want forbidden and no rows", got, err)
	}
	// Restore the role, and again revoke it in the middle of a delete.
	hooks.onList = nil
	if _, err := b.Manage("alice@h", Management{Name: "#a@h", Maintainers: ptr(protocol.MaintainerList{"maint@h"})}); err != nil {
		t.Fatal(err)
	}
	hooks.onList = revokeMaint(t, &b)
	if err := b.SubDelete("maint@h", row.ID); eventlog.Code(err) != "not_author" {
		t.Fatalf("a manager revoked during a delete: %v, want not_author", err)
	}
	hooks.onList = nil
	if rows, err := b.SubList("alice@h", "#a@h"); err != nil || len(rows) != 1 {
		t.Fatalf("the subscription was deleted by a revoked manager: %v %v", rows, err)
	}
}

// A Maintainer revoked while a subscription is being stored leaves none stored:
// before the write, and after it.
func TestAManagerRevokedDuringAnAddLeavesNothingStored(t *testing.T) {
	var b *Bus
	hooks := &hookStore{State: memory.NewState()}
	b = subFixture(t, hooks)
	count := func() int {
		rows, _ := hooks.State.ListSubscriptions(subSource(b).ID)
		return len(rows)
	}
	hooks.onList = revokeMaint(t, &b) // during the read that precedes the write
	if _, err := b.SubAdd("maint@h", "#a@h", "q@h", "", ""); eventlog.Code(err) != "forbidden" || count() != 0 {
		t.Fatalf("revoked before the write: %v, %d stored", err, count())
	}
	hooks.onList = nil
	if _, err := b.Manage("alice@h", Management{Name: "#a@h", Maintainers: ptr(protocol.MaintainerList{"maint@h"})}); err != nil {
		t.Fatal(err)
	}
	hooks.afterAdd = revokeMaint(t, &b) // right after the row is stored
	if _, err := b.SubAdd("maint@h", "#a@h", "q@h", "", ""); eventlog.Code(err) != "forbidden" || count() != 0 {
		t.Fatalf("revoked after the write: %v, %d stored", err, count())
	}
}

// A subscription whose target is its own source is delivered under the target's
// ordinary Allow, whatever happened to its author's role: the per-send question
// is never a management one.
func TestASubscriptionToItsOwnSourceIsOrdinaryTargetAdmission(t *testing.T) {
	b := subFixture(t, nil)
	row, err := b.SubAdd("maint@h", "#a@h", "#a@h", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "#a@h", Maintainers: ptr(protocol.MaintainerList{})}); err != nil {
		t.Fatal(err)
	}
	res := b.deliverEntry(subSource(b), subEntry(1, "alert", "loop"), time.Now().Add(time.Minute))
	if res.Sent != 1 || res.Dropped != 0 {
		t.Fatalf("a self-target copy after the author's role was revoked: %+v, want sent", res)
	}
	got := drain(t, b, "#a@h")
	if len(got) != 1 || got[0].From != "#bus" || !strings.Contains(got[0].Body, "loop") {
		t.Fatalf("the copy: %v", got)
	}
	if s, d := subCounts(t, b, "alice@h", row.ID); s != 1 || d != 0 {
		t.Fatalf("counts %v/%v", s, d)
	}
}

// A suspended author shows and removes nothing, not even its own, whatever it
// manages: standing as an active principal is asked after the store is read, for
// list, delete and add alike; and a suspended author's copies are not forwarded.
func TestASuspendedAuthorListsDeletesAddsAndForwardsNothing(t *testing.T) {
	var b *Bus
	hooks := &hookStore{State: memory.NewState()}
	b = subFixture(t, hooks)
	row, err := b.SubAdd("maint@h", "#a@h", "q@h", "", "")
	if err != nil {
		t.Fatal(err)
	}
	suspend := func() {
		if _, err := b.SetUserState("owner@h", "maint@h", "inactive"); err != nil {
			t.Error(err)
		}
	}
	hooks.onList = suspend
	if rows, err := b.SubList("maint@h", ""); err == nil && len(rows) != 0 || err != nil && eventlog.Code(err) != "forbidden" {
		t.Fatalf("a caller suspended during the read listed %v, %v", rows, err)
	}
	if _, err := b.SetUserState("owner@h", "maint@h", "active"); err != nil {
		t.Fatal(err)
	}
	if err := b.SubDelete("maint@h", row.ID); eventlog.Code(err) != "forbidden" {
		t.Fatalf("a caller suspended during a delete: %v, want forbidden", err)
	}
	hooks.onList = nil
	if rows, _ := b.SubList("alice@h", "#a@h"); len(rows) != 1 {
		t.Fatalf("the suspended author's subscription was deleted: %d rows", len(rows))
	}
	if _, err := b.SetUserState("owner@h", "maint@h", "active"); err != nil {
		t.Fatal(err)
	}
	hooks.onList = suspend
	before := subCount(hooks.State, subSource(b).ID)
	if _, err := b.SubAdd("maint@h", "#a@h", "q2@h", "", ""); eventlog.Code(err) != "forbidden" || subCount(hooks.State, subSource(b).ID) != before {
		t.Fatalf("an add by a caller suspended during the read: %v", err)
	}
	// Forwarding: the author is suspended now, the target still admits #bus.
	hooks.onList = nil
	res := b.deliverEntry(subSource(b), subEntry(1, "alert", "x"), time.Now().Add(time.Minute))
	if res.Sent != 0 || res.Dropped != 1 || len(drain(t, b, "q@h")) != 0 {
		t.Fatalf("a suspended author's copy: %+v, want dropped and not delivered", res)
	}
}

func subCount(st *memory.State, source uint32) int {
	rows, _ := st.ListSubscriptions(source)
	return len(rows)
}

// A target that can never receive the copies is refused when the subscription is made, with the
// protocol's own code for a bad target and the reason in the text: #bus itself, and a target whose
// Allow does not admit #bus, Personal or not. A Personal agent that lists #bus is a target (Q328).
func TestSubAddRefusesATargetThatCannotTakeCopiesFromTheBus(t *testing.T) {
	b := subFixture(t, nil)
	if _, err := b.Register(protocol.Record{Name: "#alice/oncall@h", Kind: protocol.KindAgent, Owner: "alice@h", Tags: []string{"personal"}, Allow: []string{"@owner"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "#nobus@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"alice@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "#opens@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{DaemonAgent}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "#alice/pager@h", Kind: protocol.KindAgent, Owner: "alice@h", Tags: []string{"personal"}, Allow: []string{"@owner", DaemonAgent}}); err != nil {
		t.Fatalf("a Personal agent may list #bus in its Allow (Q328): %v", err)
	}
	for _, tc := range []struct{ target, why string }{
		{DaemonAgent, "#bus cannot be a subscription target"},
		{"#alice/oncall@h", "does not admit #bus"},
		{"#nobus@h", "does not admit #bus"},
	} {
		_, err := b.SubAdd("alice@h", "#a@h", tc.target, "", "")
		if eventlog.Code(err) != "invalid_target" {
			t.Errorf("target %s: code %q, want the existing invalid_target", tc.target, eventlog.Code(err))
		}
		if !strings.Contains(eventlog.Reason(err), tc.why) {
			t.Errorf("target %s: reason %q does not say %q", tc.target, eventlog.Reason(err), tc.why)
		}
	}
	if rows, err := b.SubList("alice@h", "#a@h"); err != nil || len(rows) != 0 {
		t.Fatalf("a refused target left a subscription: %v %v", rows, err)
	}
	for _, target := range []string{"#opens@h", "#alice/pager@h"} {
		if _, err := b.SubAdd("alice@h", "#a@h", target, "", ""); err != nil {
			t.Fatalf("target %s admits #bus and was refused: %v", target, err)
		}
	}
	if _, err := b.SubAdd("alice@h", "#a@h", "nobody@h", "", ""); eventlog.Code(err) != "admission_refused" {
		t.Fatalf("an absent target keeps the engine's own answer: %v", err)
	}
}

func TestEventSubscriptionsAcceptAllProtocolNameComponents(t *testing.T) {
	b := New()
	b.SetDaemonOwner("owner@h")
	if err := b.EstablishDaemonAgent("node.example"); err != nil {
		t.Fatal(err)
	}
	for _, who := range []string{"u+2@h", "u@my_node", "u@x@h"} {
		if _, err := b.SetUser("owner@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	for _, target := range []string{"q+1@h", "q@my_node"} {
		if _, err := b.Register(protocol.Record{Name: target, Kind: protocol.KindQueue, Owner: "owner@h", Allow: []string{"*"}}); err != nil {
			t.Fatal(err)
		}
	}
	for i, source := range []string{"#own2@h", "#own+2@h", "#own_2@my_node", "#own@x@h"} {
		who := []string{"u+2@h", "u@my_node", "u@x@h"}[i%3]
		if _, err := b.Register(protocol.Record{Name: source, Kind: protocol.KindAgent, Owner: who, Allow: []string{"*"}}); err != nil {
			t.Fatal(err)
		}
		for _, target := range []string{"q+1@h", "q@my_node"} {
			row, err := b.SubAdd(who, source, target, "", "")
			if err != nil {
				t.Fatalf("%s sub add %s -> %s: %v", who, source, target, err)
			}
			if row.CreatedBy != who || row.Target != target {
				t.Fatalf("wrong subscription names: %+v", row)
			}
		}
		rows, err := b.SubList(who, source)
		if err != nil || len(rows) != 2 {
			t.Fatalf("%s sub list %s: %d rows %v", who, source, len(rows), err)
		}
	}
}

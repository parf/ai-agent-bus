package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

type schedClock struct{ now time.Time }

func (c *schedClock) at() time.Time        { return c.now }
func (c *schedClock) step(d time.Duration) { c.now = c.now.Add(d) }

// schedFixture: #svc@h (Owner alice, Maintainers @crew = carol) holds the
// schedules; out@h admits alice and carol but not #bus; shut@h admits nobody
// but its Owner admin.
func schedFixture(t *testing.T) (*Bus, *memory.State, *schedClock) {
	t.Helper()
	b, st := kvFixture(t)
	if err := b.EstablishDaemonAgent("node.example"); err != nil {
		t.Fatal(err)
	}
	clock := &schedClock{now: time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)}
	b.clock = clock.at
	for name, allow := range map[string][]string{"out@h": {"alice@h", "carol@h"}, "shut@h": {"admin@h"}} {
		if _, err := b.Register(protocol.Record{Kind: protocol.KindQueue, Name: name, Owner: "admin@h", Allow: allow}); err != nil {
			t.Fatal(err)
		}
	}
	return b, st, clock
}

func scheduled(t *testing.T, b *Bus, queue string) []protocol.Envelope {
	t.Helper()
	var out []protocol.Envelope
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		e, err := b.Consume(ctx, queue, "", "", false, false)
		cancel()
		if err != nil {
			return out
		}
		out = append(out, e)
	}
}

func outcomes(t *testing.T, st *memory.State, record uint32) []string {
	t.Helper()
	list, _ := st.ScheduleOutcomes(record, "")
	var out []string
	for _, o := range list {
		out = append(out, o.Outcome)
	}
	return out
}

// Only the holder's managers see or change its schedules; a stranger gets the
// same answer as for a schedule that does not exist.
func TestSchedulesAreTheHoldersManagersAlone(t *testing.T) {
	b, _, _ := schedFixture(t)
	v, _, err := b.ScheduleAdd("alice@h", "#svc@h", ScheduleRequest{Expr: "0 9 * * *", Target: "out@h", Body: "report"})
	if err != nil {
		t.Fatal(err)
	}
	for _, who := range []string{"alice@h", "carol@h", "#svc@h"} {
		if _, err := b.ScheduleGet(who, v.ID); err != nil {
			t.Errorf("%s manages #svc@h and could not read its schedule: %v", who, err)
		}
	}
	_, missing := b.ScheduleGet("bob@h", "000000000000")
	if _, err := b.ScheduleGet("bob@h", v.ID); !errors.Is(err, ErrUnknown) || err.Error() != missing.Error() {
		t.Fatalf("a stranger reading an existing schedule: %v, want the missing answer %v", err, missing)
	}
	if err := b.ScheduleDelete("bob@h", v.ID); !errors.Is(err, ErrUnknown) {
		t.Fatalf("a stranger deleted a schedule: %v", err)
	}
	if _, _, err := b.ScheduleAdd("bob@h", "#svc@h", ScheduleRequest{Expr: "* * * * *", Target: "out@h", Body: "x"}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("a stranger added a schedule: %v", err)
	}
	if list, err := b.ScheduleList("bob@h", ""); err != nil || len(list) != 0 {
		t.Fatalf("a stranger's listing: %+v %v", list, err)
	}
	if list, err := b.ScheduleList("carol@h", ""); err != nil || len(list) != 1 || list[0].Holder != "#svc@h" {
		t.Fatalf("a Maintainer's listing: %+v %v", list, err)
	}
	// A Maintainer's edit makes her the creator.
	e, err := b.ScheduleEdit("carol@h", v.ID, ScheduleRequest{Expr: "0 10 * * *", Target: "out@h", Body: "later"})
	if err != nil || e.Creator != "carol@h" {
		t.Fatalf("a Maintainer's edit: %+v %v", e, err)
	}
	if _, err := b.ScheduleEdit("carol@h", v.ID, ScheduleRequest{Expr: "0 10 * * *", Target: "shut@h", Body: "x"}); !errors.Is(err, ErrNotAllow) {
		t.Fatalf("an edit to a target the editor may not send to: %v", err)
	}
}

// A schedule fires from #bus, topic schedule, stamped with its id and creator,
// answering to the creator, into a target whose Allow does not name #bus.
func TestAFiringComesFromBusOnTheCreatorsBehalf(t *testing.T) {
	b, st, clock := schedFixture(t)
	v, _, err := b.ScheduleAdd("alice@h", "#svc@h", ScheduleRequest{In: "90s", Target: "out@h", Body: "ping"})
	if err != nil {
		t.Fatal(err)
	}
	clock.step(90 * time.Second)
	b.schedulePass()
	got := scheduled(t, b, "out@h")
	if len(got) != 1 {
		t.Fatalf("firings delivered: %d, want 1", len(got))
	}
	e := got[0]
	if e.From != DaemonAgent || e.Topic != "schedule" || e.Tag != "by alice@h" || e.Body != "ping" || e.Schedule == nil || e.Schedule.ID != v.ID || e.Schedule.By != "alice@h" || e.ReplyTo == nil || e.ReplyTo.Name != "alice@h" {
		t.Fatalf("the firing: %+v %+v", e, e.Schedule)
	}
	if _, ok, _ := st.ScheduleGet(v.ID); ok {
		t.Fatal("a fired one-shot stayed")
	}
	if o := outcomes(t, st, v.Record); len(o) != 1 || o[0] != "sent" {
		t.Fatalf("outcomes: %v", o)
	}
	b.schedulePass()
	if n := len(scheduled(t, b, "out@h")); n != 0 {
		t.Fatalf("a one-shot fired again: %d", n)
	}
}

// A sender cannot forge a schedule stamp.
func TestASenderCannotStampASchedule(t *testing.T) {
	b, _, _ := schedFixture(t)
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "out@h", Body: "x", Schedule: &protocol.ScheduleStamp{ID: "abc", By: "admin@h"}}); err != nil {
		t.Fatal(err)
	}
	if got := scheduled(t, b, "out@h"); len(got) != 1 || got[0].Schedule != nil {
		t.Fatalf("a forged stamp arrived: %+v", got)
	}
}

// A one-second schedule fires once a second, never faster, over ten seconds.
func TestAOneSecondScheduleFiresOnceASecond(t *testing.T) {
	b, _, clock := schedFixture(t)
	if _, _, err := b.ScheduleAdd("alice@h", "#svc@h", ScheduleRequest{Expr: "* * * * * *", Target: "out@h", Body: "tick"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		clock.step(time.Second)
		b.schedulePass()
		b.schedulePass()
	}
	got := scheduled(t, b, "out@h")
	if len(got) != 10 {
		t.Fatalf("firings over ten seconds: %d, want 10", len(got))
	}
	for i := 1; i < len(got); i++ {
		if !got[i].Schedule.Slot.After(got[i-1].Schedule.Slot) {
			t.Fatalf("slot %d repeated: %v", i, got[i].Schedule.Slot)
		}
	}
}

// A creator who can no longer reach the target is skipped and warned; five
// refusals in a row disable the schedule.
func TestRefusedFiringsDisableTheScheduleAfterFive(t *testing.T) {
	b, st, clock := schedFixture(t)
	v, _, err := b.ScheduleAdd("alice@h", "#svc@h", ScheduleRequest{Expr: "* * * * *", Target: "out@h", Body: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("admin@h", Management{Name: "out@h", Allow: ptr([]string{"carol@h"})}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		clock.step(time.Minute)
		b.schedulePass()
	}
	if n := len(scheduled(t, b, "out@h")); n != 0 {
		t.Fatalf("a refused creator's firing was delivered: %d", n)
	}
	cur, _, _ := st.ScheduleGet(v.ID)
	if cur.Enabled || cur.Disabled != "failure" || cur.Dropped != 5 {
		t.Fatalf("after five refusals: %+v", cur)
	}
	if o := outcomes(t, st, v.Record); len(o) != 5 || o[0] != "admission_refused" {
		t.Fatalf("outcomes: %v", o)
	}
	// Enabling again needs the enabler's own reach, and makes her the creator.
	if _, err := b.ScheduleEnable("alice@h", v.ID, true); !errors.Is(err, ErrNotAllow) {
		t.Fatalf("alice re-enabled a schedule to a target she may not reach: %v", err)
	}
	if e, err := b.ScheduleEnable("carol@h", v.ID, true); err != nil || e.Creator != "carol@h" || !e.Enabled {
		t.Fatalf("carol's enable: %+v %v", e, err)
	}
	clock.step(time.Minute)
	b.schedulePass()
	if got := scheduled(t, b, "out@h"); len(got) != 1 || got[0].Schedule.By != "carol@h" {
		t.Fatalf("after carol enabled it: %+v", got)
	}
}

// A creator who stops managing the holder fires nothing.
func TestACreatorWhoLeftTheHolderFiresNothing(t *testing.T) {
	b, st, clock := schedFixture(t)
	v, _, err := b.ScheduleAdd("carol@h", "#svc@h", ScheduleRequest{Expr: "* * * * *", Target: "out@h", Body: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("admin@h", "@crew", []string{"eve@h"}); err != nil {
		t.Fatal(err)
	}
	clock.step(time.Minute)
	b.schedulePass()
	if n := len(scheduled(t, b, "out@h")); n != 0 {
		t.Fatalf("a former Maintainer's schedule fired: %d", n)
	}
	if o := outcomes(t, st, v.Record); len(o) != 1 || o[0] != "admission_refused" {
		t.Fatalf("outcomes: %v", o)
	}
}

// A firing claimed but cut off by a stop is send_unknown at start and never sent.
func TestAFiringCutOffByAStopIsNeverRepeated(t *testing.T) {
	b, st, clock := schedFixture(t)
	v, _, err := b.ScheduleAdd("alice@h", "#svc@h", ScheduleRequest{Expr: "* * * * *", Target: "out@h", Body: "x"})
	if err != nil {
		t.Fatal(err)
	}
	slot := v.Next
	if err := st.ScheduleEdit(v.ID, v.Record, func(cur ports.Schedule, _ bool, _ ports.ScheduleCount) (ports.ScheduleChange, error) {
		cur.InFlight, cur.Last, cur.Next = slot, slot, slot+60
		return ports.ScheduleChange{Write: true, Value: cur}, nil
	}); err != nil {
		t.Fatal(err)
	}
	clock.step(30 * time.Second)
	b.ReportScheduleState()
	b.schedulePass()
	if n := len(scheduled(t, b, "out@h")); n != 0 {
		t.Fatalf("a claimed firing was sent after the stop: %d", n)
	}
	cur, _, _ := st.ScheduleGet(v.ID)
	if o := outcomes(t, st, v.Record); len(o) != 1 || o[0] != "send_unknown" || cur.InFlight != 0 {
		t.Fatalf("after start: %v %+v", o, cur)
	}
}

// After the daemon was stopped, a recurring schedule fires one catch-up with
// the slots it passed over, a past-due one-shot fires once with late_by, and
// a backward clock fires nothing twice.
func TestDowntimeCatchUpAndABackwardClock(t *testing.T) {
	b, _, clock := schedFixture(t)
	if _, _, err := b.ScheduleAdd("alice@h", "#svc@h", ScheduleRequest{Expr: "* * * * *", Target: "out@h", Body: "every"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.ScheduleAdd("alice@h", "#svc@h", ScheduleRequest{In: "2m", Target: "out@h", Body: "once"}); err != nil {
		t.Fatal(err)
	}
	clock.step(10*time.Minute + 30*time.Second)
	b.schedulePass()
	got := scheduled(t, b, "out@h")
	if len(got) != 2 {
		t.Fatalf("firings after ten minutes stopped: %d, want one catch-up and one one-shot", len(got))
	}
	for _, e := range got {
		switch e.Body {
		case "every":
			if e.Schedule.Missed != 9 || e.Schedule.LateBy == "" {
				t.Errorf("the catch-up: %+v", e.Schedule)
			}
		case "once":
			if e.Schedule.LateBy != "8m30s" {
				t.Errorf("the one-shot: %+v", e.Schedule)
			}
		}
	}
	clock.step(-time.Hour)
	b.schedulePass()
	clock.step(time.Hour)
	b.schedulePass()
	if n := len(scheduled(t, b, "out@h")); n > 1 {
		t.Fatalf("a backward clock fired %d slots again", n)
	}
}

// Run now fires once and leaves the next slot; max_late skips a late slot.
func TestRunNowAndMaxLate(t *testing.T) {
	b, st, clock := schedFixture(t)
	v, _, err := b.ScheduleAdd("alice@h", "#svc@h", ScheduleRequest{Expr: "0 9 * * *", Target: "out@h", Body: "x", MaxLate: "1m"})
	if err != nil {
		t.Fatal(err)
	}
	o, err := b.ScheduleRun("carol@h", v.ID)
	if err != nil || o.Outcome != "sent" {
		t.Fatalf("run now: %+v %v", o, err)
	}
	if cur, _, _ := st.ScheduleGet(v.ID); cur.Next != v.Next || cur.Runs != 0 {
		t.Fatalf("run now moved the schedule: %+v", cur)
	}
	if _, err := b.ScheduleRun("bob@h", v.ID); !errors.Is(err, ErrUnknown) {
		t.Fatalf("a stranger ran it: %v", err)
	}
	scheduled(t, b, "out@h")
	clock.now = time.Unix(v.Next, 0).Add(5 * time.Minute)
	b.schedulePass()
	if n := len(scheduled(t, b, "out@h")); n != 0 {
		t.Fatalf("a slot over max_late was sent: %d", n)
	}
	if o := outcomes(t, st, v.Record); o[0] != "late" {
		t.Fatalf("outcomes: %v", o)
	}
}

// Caps: a record holds at most 100 recurring schedules; a refusal names it.
// The body is required and #bus is never a target.
func TestScheduleCapsAndRefusals(t *testing.T) {
	b, _, _ := schedFixture(t)
	for i := 0; i < ScheduleMaxRecurring; i++ {
		if _, _, err := b.ScheduleAdd("alice@h", "#svc@h", ScheduleRequest{Expr: "0 9 * * *", Target: "out@h", Body: "x"}); err != nil {
			t.Fatalf("schedule %d: %v", i+1, err)
		}
	}
	_, _, err := b.ScheduleAdd("alice@h", "#svc@h", ScheduleRequest{Expr: "0 9 * * *", Target: "out@h", Body: "x"})
	if !errors.Is(err, ErrScheduleLimit) || !strings.Contains(err.Error(), "at most 100 recurring") {
		t.Fatalf("the 101st: %v", err)
	}
	for req, why := range map[ScheduleRequest]string{
		{In: "1m", Target: "out@h"}:                               "empty message",
		{In: "1m", Target: DaemonAgent, Body: "x"}:                "#bus cannot be",
		{In: "1m", Expr: "* * * * *", Target: "out@h", Body: "x"}: "give one of",
		{Expr: "0 0 30 2 *", Target: "out@h", Body: "x"}:          "can never fire",
	} {
		if _, _, err := b.ScheduleAdd("alice@h", "#svc@h", req); err == nil || !strings.Contains(err.Error(), why) {
			t.Errorf("%+v: %v, want %q", req, err, why)
		}
	}
}

// A scheduled message that expires unread, and a refused firing, are
// warnings in the holder agent's own log.
func TestScheduleWarningsLandInTheHolderAgentsLog(t *testing.T) {
	b, _, clock := schedFixture(t)
	v, _, err := b.ScheduleAdd("alice@h", "#svc@h", ScheduleRequest{Expr: "0 9 * * *", Target: "out@h", Body: "unread"})
	if err != nil {
		t.Fatal(err)
	}
	_ = clock
	// A firing already past its time to live waits in out@h; the next read
	// there prunes it.
	b.mu.Lock()
	in := b.ensure("out@h")
	in.queue = append(in.queue, protocol.Envelope{ID: "x", From: DaemonAgent, To: "out@h", Body: "unread", Expires: time.Now().Add(-time.Second), Schedule: &protocol.ScheduleStamp{ID: v.ID, By: "alice@h"}})
	b.unlock()
	if got := scheduled(t, b, "out@h"); len(got) != 0 {
		t.Fatalf("an expired firing was read: %+v", got)
	}
	b.schedulePass()
	res, err := b.EventRead("alice@h", "#svc@h", "warning")
	if err != nil {
		t.Fatal(err)
	}
	text := ""
	for _, ring := range res.Rings {
		for _, e := range ring {
			text += e.Text + "\n"
		}
	}
	if !strings.Contains(text, "expired unread in out@h") {
		t.Fatalf("the holder's warnings: %q", text)
	}
}

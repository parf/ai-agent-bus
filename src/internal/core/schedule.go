package core

// Schedules: a record's crons and ats, kept by the daemon and fired from #bus
// on their creator's behalf (Plans/R1.0-Release/cron-in-core.md).

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/schedule"
)

// Caps (Q384, Q416) and the refusals that disable a schedule (Q409).
const (
	ScheduleMaxRecurring      = 100
	ScheduleMaxOnce           = 10000
	ScheduleOwnerMaxRecurring = 1000
	ScheduleOwnerMaxOnce      = 100000
	ScheduleDisableAfter      = 5
	ScheduleTargetWarnAfter   = 3
)

var (
	ErrSchedule        = errors.New("schedule")
	ErrScheduleLimit   = errors.New("that is more schedules than allowed")
	ErrScheduleNoStore = errors.New("this daemon keeps no schedules")
)

// ScheduleRequest is what a manager asks for on add or edit: one of Expr, In
// or At; the rest optional.
type ScheduleRequest struct {
	Expr, In, At string
	Target, Body string
	Until        string // RFC 3339
	MaxRuns      int
	MaxLate      string // a duration such as 5m
	Disabled     bool
}

// ScheduleView is a schedule as a manager sees it.
type ScheduleView struct {
	ports.Schedule
	Holder      string                  `json:"holder"`
	HolderKind  string                  `json:"holder_kind"`
	LastOutcome string                  `json:"last_outcome,omitempty"`
	Outcomes    []ports.ScheduleOutcome `json:"outcomes,omitempty"`
	Upcoming    []int64                 `json:"upcoming,omitempty"`
}

// withLast names each schedule's newest outcome, from one read of its holder's.
func withLast(store ports.ScheduleStore, r protocol.Record, list []ports.Schedule) ([]ScheduleView, error) {
	outcomes, err := store.ScheduleOutcomes(r.ID, "")
	if err != nil {
		return nil, err
	}
	last := map[string]string{}
	for _, o := range outcomes {
		if _, seen := last[o.Schedule]; !seen {
			last[o.Schedule] = o.Outcome
		}
	}
	out := make([]ScheduleView, 0, len(list))
	for _, v := range list {
		out = append(out, ScheduleView{Schedule: v, Holder: r.Name, HolderKind: r.Kind, LastOutcome: last[v.ID]})
	}
	return out, nil
}

// upcoming is a recurring schedule's next n slots from its next one.
func upcoming(v ports.Schedule, n int) []int64 {
	if v.Next == 0 {
		return nil
	}
	out := []int64{v.Next}
	if v.Once {
		return out
	}
	e, err := schedule.Parse(v.Expr)
	if err != nil {
		return out
	}
	t := time.Unix(v.Next, 0)
	for len(out) < n {
		next, ok := e.Next(t)
		if !ok || (v.Until > 0 && next.Unix() > v.Until) {
			break
		}
		out = append(out, next.Unix())
		t = next
	}
	return out
}

func scheduleID() string {
	var raw [6]byte
	rand.Read(raw[:])
	return hex.EncodeToString(raw[:])
}

func scheduleErr(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrSchedule}, args...)...)
}

// holdsSchedules is the record kinds that receive messages (Q382).
func holdsSchedules(kind string) bool {
	return kind == protocol.KindAgent || kind == protocol.KindQueue || kind == protocol.KindPubSub
}

// scheduleHolder finds a live holder caller manages. Caller holds nothing.
func (b *Bus) scheduleHolder(caller, holder string) (ports.ScheduleStore, protocol.Record, error) {
	holder = LockRecord(holder)
	b.mu.Lock()
	r, live := b.entity(holder)
	may := live && b.resourceManages(caller, r) && b.acting(caller) == nil
	store, _ := b.store.(ports.ScheduleStore)
	b.unlock()
	switch {
	case !live:
		return nil, protocol.Record{}, ErrUnknown
	case !may:
		return nil, protocol.Record{}, ErrNotOwner
	case !holdsSchedules(r.Kind):
		return nil, protocol.Record{}, scheduleErr("a %s holds no schedules; agents, queues and pub/subs do", r.Kind)
	case store == nil:
		return nil, protocol.Record{}, ErrScheduleNoStore
	}
	return store, r, nil
}

// holderByID is the live record with a stored ID. Caller holds b.mu.
func (b *Bus) holderByID(id uint32) (protocol.Record, bool) {
	name, ok := b.recordByID[id]
	if !ok {
		return protocol.Record{}, false
	}
	r, live := b.entity(name)
	return r, live && r.ID == id
}

// scheduleOf finds schedule id and its holder, refusing a caller who does not
// manage the holder with the same answer as a missing schedule.
func (b *Bus) scheduleOf(caller, id string) (ports.ScheduleStore, protocol.Record, ports.Schedule, error) {
	b.mu.Lock()
	store, _ := b.store.(ports.ScheduleStore)
	b.unlock()
	if store == nil {
		return nil, protocol.Record{}, ports.Schedule{}, ErrScheduleNoStore
	}
	v, ok, err := store.ScheduleGet(id)
	if err != nil {
		return nil, protocol.Record{}, ports.Schedule{}, err
	}
	if !ok {
		return nil, protocol.Record{}, ports.Schedule{}, ErrUnknown
	}
	b.mu.Lock()
	r, live := b.holderByID(v.Record)
	may := live && b.resourceManages(caller, r) && b.acting(caller) == nil
	b.unlock()
	if !may {
		return nil, protocol.Record{}, ports.Schedule{}, ErrUnknown
	}
	return store, r, v, nil
}

// checkTarget is a target the caller may send to: an existing agent or queue,
// never #bus.
func (b *Bus) checkTarget(caller, target string) (string, error) {
	name, err := canon(target)
	if err != nil {
		return "", scheduleErr("target must be a bus name: a queue or an agent")
	}
	if name == DaemonAgent {
		return "", scheduleErr("#bus cannot be a schedule's target")
	}
	b.mu.Lock()
	r, ok := b.entity(name)
	b.unlock()
	if err := b.MaySend(caller, name); err != nil {
		return "", err
	}
	if !ok || (r.Kind != protocol.KindAgent && r.Kind != protocol.KindQueue) {
		return "", scheduleErr("target must be a queue or an agent")
	}
	return name, nil
}

// build turns a request into a schedule at now, keeping what old had.
func (b *Bus) build(caller string, req ScheduleRequest, old ports.Schedule, now time.Time) (ports.Schedule, error) {
	v := old
	forms := 0
	for _, f := range []string{req.Expr, req.In, req.At} {
		if f != "" {
			forms++
		}
	}
	if forms != 1 {
		return v, scheduleErr("give one of an expression, in <delay> or at <time>")
	}
	if req.Body == "" {
		return v, scheduleErr("body is required: an empty message is refused")
	}
	if len(req.Body) > schedule.MaxBody || !utf8.ValidString(req.Body) {
		return v, scheduleErr("a message is at most %d bytes of UTF-8", schedule.MaxBody)
	}
	target, err := b.checkTarget(caller, req.Target)
	if err != nil {
		return v, err
	}
	v.Target, v.Body, v.Creator = target, req.Body, caller
	v.Expr, v.At, v.Once = "", 0, false
	switch {
	case req.Expr != "":
		e, err := schedule.Parse(req.Expr)
		if err != nil {
			return v, scheduleErr("%s", err)
		}
		v.Expr = req.Expr
		next, ok := e.Next(now)
		if !ok {
			return v, scheduleErr("the schedule can never fire")
		}
		v.Next = next.Unix()
	case req.In != "":
		t, err := schedule.In(req.In, now)
		if err != nil {
			return v, scheduleErr("%s", err)
		}
		v.Once, v.At, v.Next = true, t.Unix(), t.Unix()
	default:
		t, err := schedule.At(req.At, now)
		if err != nil {
			return v, scheduleErr("%s", err)
		}
		v.Once, v.At, v.Next = true, t.Unix(), t.Unix()
	}
	v.Until, v.MaxRuns, v.MaxLate = 0, 0, 0
	if req.Until != "" {
		t, err := time.Parse(time.RFC3339, req.Until)
		if err != nil {
			return v, scheduleErr("until is an RFC 3339 instant with an offset")
		}
		v.Until = t.Unix()
	}
	if req.MaxRuns < 0 {
		return v, scheduleErr("max_runs is a positive count")
	}
	v.MaxRuns = req.MaxRuns
	if req.MaxLate != "" {
		d, err := time.ParseDuration(req.MaxLate)
		if err != nil || d < time.Second {
			return v, scheduleErr("max_late is a duration of at least 1s, such as 5m")
		}
		v.MaxLate = int64(d / time.Second)
	}
	v.Enabled, v.Disabled = !req.Disabled, ""
	if req.Disabled {
		v.Disabled = "hand"
	}
	v.Streak, v.Changed = 0, now.Unix()
	if v.Created == 0 {
		v.Created = now.Unix()
	}
	return v, nil
}

// ownerCounts sums the schedules of every record owner owns.
func (b *Bus) ownerCounts(store ports.ScheduleStore, owner string) (ports.ScheduleCount, error) {
	counts, err := store.ScheduleCounts()
	if err != nil {
		return ports.ScheduleCount{}, err
	}
	b.mu.Lock()
	defer b.unlock()
	var sum ports.ScheduleCount
	for _, r := range b.records {
		if r.Owner == owner {
			c := counts[r.ID]
			sum.Recurring += c.Recurring
			sum.Once += c.Once
		}
	}
	return sum, nil
}

// capCheck refuses one more schedule over a cap and warns at 80 %.
func capCheck(once bool, held, owned ports.ScheduleCount) (warn string, err error) {
	n, max, on, omax, what := held.Recurring, ScheduleMaxRecurring, owned.Recurring, ScheduleOwnerMaxRecurring, "recurring schedules"
	if once {
		n, max, on, omax, what = held.Once, ScheduleMaxOnce, owned.Once, ScheduleOwnerMaxOnce, "one-shot schedules"
	}
	if n >= max {
		return "", fmt.Errorf("%w: a record holds at most %d %s", ErrScheduleLimit, max, what)
	}
	if on >= omax {
		return "", fmt.Errorf("%w: one owner's records hold at most %d %s", ErrScheduleLimit, omax, what)
	}
	if (n+1)*5 >= max*4 {
		return fmt.Sprintf("this record holds %d of at most %d %s", n+1, max, what), nil
	}
	return "", nil
}

// ScheduleAdd adds a schedule to holder on caller's authority.
func (b *Bus) ScheduleAdd(caller, holder string, req ScheduleRequest) (ScheduleView, string, error) {
	store, r, err := b.scheduleHolder(caller, holder)
	if err != nil {
		return ScheduleView{}, "", err
	}
	b.schedMu.Lock()
	defer b.schedMu.Unlock()
	owned, err := b.ownerCounts(store, r.Owner)
	if err != nil {
		return ScheduleView{}, "", err
	}
	now := b.clock().UTC()
	v, err := b.build(caller, req, ports.Schedule{}, now)
	if err != nil {
		return ScheduleView{}, "", err
	}
	if v.Target == r.Name && r.Kind == protocol.KindPubSub {
		return ScheduleView{}, "", scheduleErr("a pub/sub cannot schedule to itself")
	}
	id, warn := scheduleID(), ""
	err = store.ScheduleEdit(id, r.ID, func(_ ports.Schedule, _ bool, held ports.ScheduleCount) (ports.ScheduleChange, error) {
		w, err := capCheck(v.Once, held, owned)
		warn = w
		return ports.ScheduleChange{Write: true, Value: v}, err
	})
	if err != nil {
		return ScheduleView{}, "", err
	}
	if warn != "" {
		b.scheduleWarn(r, "warning", "schedule %s: %s", id, warn)
	}
	// An AI session's name comes and goes with the session (Q389): its
	// schedules go with it, so say so at the only moment it matters.
	if shortLived(r) {
		note := "this record is an AI session's name; its schedules go with it when the name is renamed or removed"
		if warn != "" {
			warn += "; " + note
		} else {
			warn = note
		}
	}
	b.wakeSchedules()
	v.ID, v.Record = id, r.ID
	return ScheduleView{Schedule: v, Holder: r.Name, HolderKind: r.Kind}, warn, nil
}

// ScheduleEdit replaces a schedule's time, target and message; the caller
// becomes its creator (Q415).
func (b *Bus) ScheduleEdit(caller, id string, req ScheduleRequest) (ScheduleView, error) {
	store, r, old, err := b.scheduleOf(caller, id)
	if err != nil {
		return ScheduleView{}, err
	}
	v, err := b.build(caller, req, old, b.clock().UTC())
	if err != nil {
		return ScheduleView{}, err
	}
	v.Runs, v.Last, v.InFlight = 0, 0, old.InFlight
	err = store.ScheduleEdit(id, r.ID, func(cur ports.Schedule, found bool, held ports.ScheduleCount) (ports.ScheduleChange, error) {
		if !found {
			return ports.ScheduleChange{}, ErrUnknown
		}
		if cur.Once != v.Once {
			if _, err := capCheck(v.Once, held, ports.ScheduleCount{}); err != nil {
				return ports.ScheduleChange{}, err
			}
		}
		v.Sent, v.Dropped, v.InFlight = cur.Sent, cur.Dropped, cur.InFlight
		return ports.ScheduleChange{Write: true, Value: v}, nil
	})
	if err != nil {
		return ScheduleView{}, err
	}
	b.wakeSchedules()
	return ScheduleView{Schedule: v, Holder: r.Name, HolderKind: r.Kind}, nil
}

// ScheduleEnable enables or disables a schedule. Enabling makes the caller its
// creator after the caller's own may-send to its target (Q415).
func (b *Bus) ScheduleEnable(caller, id string, on bool) (ScheduleView, error) {
	store, r, old, err := b.scheduleOf(caller, id)
	if err != nil {
		return ScheduleView{}, err
	}
	now := b.clock().UTC()
	if on {
		if _, err := b.checkTarget(caller, old.Target); err != nil {
			return ScheduleView{}, err
		}
	}
	var out ports.Schedule
	err = store.ScheduleEdit(id, r.ID, func(v ports.Schedule, found bool, _ ports.ScheduleCount) (ports.ScheduleChange, error) {
		if !found {
			return ports.ScheduleChange{}, ErrUnknown
		}
		if on {
			v.Enabled, v.Disabled, v.Streak, v.Creator = true, "", 0, caller
			if !v.Once {
				e, err := schedule.Parse(v.Expr)
				if err != nil {
					return ports.ScheduleChange{}, scheduleErr("%s", err)
				}
				next, ok := e.Next(now)
				if !ok {
					return ports.ScheduleChange{}, scheduleErr("the schedule can never fire")
				}
				v.Next = next.Unix()
			}
		} else {
			v.Enabled, v.Disabled = false, "hand"
		}
		v.Changed = now.Unix()
		out = v
		return ports.ScheduleChange{Write: true, Value: v}, nil
	})
	if err != nil {
		return ScheduleView{}, err
	}
	b.wakeSchedules()
	return ScheduleView{Schedule: out, Holder: r.Name, HolderKind: r.Kind}, nil
}

// ScheduleDelete removes a schedule; a firing already in flight may still deliver.
func (b *Bus) ScheduleDelete(caller, id string) error {
	store, r, _, err := b.scheduleOf(caller, id)
	if err != nil {
		return err
	}
	err = store.ScheduleEdit(id, r.ID, func(_ ports.Schedule, found bool, _ ports.ScheduleCount) (ports.ScheduleChange, error) {
		if !found {
			return ports.ScheduleChange{}, ErrUnknown
		}
		return ports.ScheduleChange{Delete: true}, nil
	})
	b.wakeSchedules()
	return err
}

// ScheduleGet is one schedule with its outcomes.
func (b *Bus) ScheduleGet(caller, id string) (ScheduleView, error) {
	store, r, v, err := b.scheduleOf(caller, id)
	if err != nil {
		return ScheduleView{}, err
	}
	outcomes, err := store.ScheduleOutcomes(r.ID, id)
	if err != nil {
		return ScheduleView{}, err
	}
	last := ""
	if len(outcomes) > 0 {
		last = outcomes[0].Outcome
	}
	return ScheduleView{Schedule: v, Holder: r.Name, HolderKind: r.Kind, LastOutcome: last, Outcomes: outcomes, Upcoming: upcoming(v, 5)}, nil
}

// ScheduleList is holder's schedules, or with no holder every schedule on a
// live record the caller manages.
func (b *Bus) ScheduleList(caller, holder string) ([]ScheduleView, error) {
	if holder != "" {
		store, r, err := b.scheduleHolder(caller, holder)
		if err != nil {
			return nil, err
		}
		list, err := store.ScheduleList(r.ID)
		if err != nil {
			return nil, err
		}
		return withLast(store, r, list)
	}
	b.mu.Lock()
	store, _ := b.store.(ports.ScheduleStore)
	b.unlock()
	if store == nil {
		return nil, ErrScheduleNoStore
	}
	counts, err := store.ScheduleCounts()
	if err != nil {
		return nil, err
	}
	var holders []protocol.Record
	b.mu.Lock()
	active := b.acting(caller) == nil
	for name, r := range b.records {
		if _, held := counts[r.ID]; !held || !active {
			continue
		}
		if live, ok := b.entity(name); ok && b.resourceManages(caller, live) {
			holders = append(holders, live)
		}
	}
	b.unlock()
	sort.Slice(holders, func(i, j int) bool { return holders[i].Name < holders[j].Name })
	out := []ScheduleView{}
	for _, r := range holders {
		list, err := store.ScheduleList(r.ID)
		if err != nil {
			return nil, err
		}
		views, err := withLast(store, r, list)
		if err != nil {
			return nil, err
		}
		out = append(out, views...)
	}
	return out, nil
}

// ScheduleRun fires a schedule once now, on the caller's may-send as well as
// its creator's; its next slot does not move.
func (b *Bus) ScheduleRun(caller, id string) (ports.ScheduleOutcome, error) {
	_, r, v, err := b.scheduleOf(caller, id)
	if err != nil {
		return ports.ScheduleOutcome{}, err
	}
	if err := b.MaySend(caller, v.Target); err != nil {
		return ports.ScheduleOutcome{}, err
	}
	now := b.clock().UTC()
	o := b.fire(r, v, now.Unix(), now, true)
	if o.Outcome == "not_claimed" {
		return o, fmt.Errorf("%w: a firing of this schedule is in flight; try again", ErrBusy)
	}
	return o, nil
}

// scheduleWarn writes a schedule's warning or alert into its holder agent's
// log, and the daemon journal in every case (Q412, Q418).
func (b *Bus) scheduleWarn(r protocol.Record, level, format string, args ...any) {
	text := fmt.Sprintf(format, args...)
	sev := ports.Warning
	if level == "alert" {
		sev = ports.Alert
	}
	b.report(sev, "%s: %s", r.Name, text)
	if r.Kind != protocol.KindAgent {
		return
	}
	b.mu.Lock()
	admin := b.admin
	b.unlock()
	if admin != "" {
		_, _ = b.EventWrite(admin, r.Name, level, text)
	}
}

func lateBy(d time.Duration) string {
	if d < time.Second {
		return ""
	}
	return d.Truncate(time.Second).String()
}

// outcomeOf names a send's result in today's words.
func outcomeOf(err error) string {
	switch {
	case err == nil:
		return "sent"
	case errors.Is(err, ErrNotAllow), errors.Is(err, ErrNotOwner):
		return "admission_refused"
	case errors.Is(err, ErrUnknown):
		return "target_gone"
	case errors.Is(err, ErrFull), errors.Is(err, ErrKind), errors.Is(err, ErrOverflow):
		return "send_refused"
	}
	return "send_unknown"
}

// refused says whether an outcome counts toward disabling the schedule.
func refused(outcome string) bool {
	return outcome == "admission_refused" || outcome == "target_gone" || outcome == "send_refused"
}

// ttlFor is a firing's time to live: to the next slot, at least a minute, at
// most a day; a day for a one-shot or a catch-up.
func ttlFor(v ports.Schedule, now int64, catchUp bool) string {
	if v.Once || catchUp || v.Next == 0 {
		return "24h"
	}
	d := v.Next - now
	if d < 60 {
		d = 60
	}
	if d > 86400 {
		d = 86400
	}
	return fmt.Sprintf("%ds", d)
}

// fire claims one firing of v at slot, checks and sends it, and records the
// outcome. extra is a run-now firing, which moves no slot.
func (b *Bus) fire(r protocol.Record, v ports.Schedule, slot int64, now time.Time, extra bool) ports.ScheduleOutcome {
	b.mu.Lock()
	store, _ := b.store.(ports.ScheduleStore)
	b.unlock()
	nowS := now.Unix()
	missed := 0
	// Claim: the firing is recorded before its send, so it is never sent twice.
	var claimed ports.Schedule
	claim := store.ScheduleEdit(v.ID, v.Record, func(cur ports.Schedule, found bool, _ ports.ScheduleCount) (ports.ScheduleChange, error) {
		if !found || cur.InFlight != 0 || (!extra && (!cur.Enabled || cur.Next != slot)) {
			return ports.ScheduleChange{}, ErrUnknown
		}
		if !extra {
			cur.Last = slot
			if cur.Once {
				cur.Next = 0
			} else if e, err := schedule.Parse(cur.Expr); err == nil {
				// Only slots later than the last claimed one, from now: the
				// ones passed over while stopped are counted, not sent.
				from := time.Unix(max(slot, nowS), 0)
				for t, ok := e.Next(time.Unix(slot, 0)); ok && t.Unix() <= nowS && missed < 100000; t, ok = e.Next(t) {
					missed++
				}
				next, ok := e.Next(from)
				cur.Next = 0
				if ok {
					cur.Next = next.Unix()
				}
			}
		}
		cur.InFlight, cur.Missed = slot, missed
		claimed = cur
		return ports.ScheduleChange{Write: true, Value: cur}, nil
	})
	if claim != nil {
		return ports.ScheduleOutcome{At: nowS, Slot: slot, Outcome: "not_claimed"}
	}
	outcome, detail := b.deliverFiring(r, claimed, slot, now, missed)
	return b.recordFiring(r, claimed, slot, nowS, outcome, detail, extra)
}

// deliverFiring checks and sends one claimed firing under one lock hold.
func (b *Bus) deliverFiring(r protocol.Record, v ports.Schedule, slot int64, now time.Time, missed int) (string, string) {
	late := now.Sub(time.Unix(slot, 0))
	if v.MaxLate > 0 && late > time.Duration(v.MaxLate)*time.Second {
		return "late", fmt.Sprintf("%s late, over max_late %ds", lateBy(late), v.MaxLate)
	}
	stamp := &protocol.ScheduleStamp{ID: v.ID, By: v.Creator, Slot: time.Unix(slot, 0).UTC(), LateBy: lateBy(late), Missed: missed}
	guard := func() error {
		holder, live := b.entity(r.Name)
		if !live || holder.ID != r.ID {
			return fmt.Errorf("%s is not live: %w", r.Name, ErrUnknown)
		}
		if !b.resourceManages(v.Creator, holder) {
			return fmt.Errorf("%s no longer manages %s: %w", v.Creator, r.Name, ErrNotAllow)
		}
		return nil
	}
	e := protocol.Envelope{From: DaemonAgent, To: v.Target, Topic: "schedule", Tag: "by " + v.Creator, Body: v.Body,
		TTL: ttlFor(v, now.Unix(), missed > 0 || late > time.Minute), ReplyTo: &protocol.ReplyTo{Name: v.Creator}}
	_, err := b.send(e, sendOpts{onBehalf: v.Creator, guard: guard, stamp: stamp})
	o := outcomeOf(err)
	detail := ""
	if err != nil {
		detail = err.Error()
	}
	return o, detail
}

// record writes a firing's outcome and counts, ends a finished schedule and
// disables one refused too often.
func (b *Bus) recordFiring(r protocol.Record, v ports.Schedule, slot, nowS int64, outcome, detail string, extra bool) ports.ScheduleOutcome {
	b.mu.Lock()
	store, _ := b.store.(ports.ScheduleStore)
	b.unlock()
	o := ports.ScheduleOutcome{At: nowS, Slot: slot, Outcome: outcome, Detail: detail}
	var after ports.Schedule
	disabled, ended := false, false
	err := store.ScheduleEdit(v.ID, v.Record, func(cur ports.Schedule, found bool, _ ports.ScheduleCount) (ports.ScheduleChange, error) {
		if !found {
			return ports.ScheduleChange{Outcome: &o}, nil
		}
		cur.InFlight, cur.Missed = 0, 0
		if !extra {
			cur.Runs++
		}
		switch {
		case outcome == "sent":
			cur.Sent++
			cur.Streak = 0
		case refused(outcome):
			cur.Dropped++
			cur.Streak++
		default:
			cur.Dropped++
		}
		if cur.Streak >= ScheduleDisableAfter && cur.Enabled {
			cur.Enabled, cur.Disabled, disabled = false, "failure", true
		}
		if cur.Once && !extra {
			after = cur
			return ports.ScheduleChange{Delete: true, Outcome: &o}, nil
		}
		if !cur.Once && cur.Enabled && ((cur.MaxRuns > 0 && cur.Runs >= cur.MaxRuns) || (cur.Until > 0 && cur.Next > cur.Until)) {
			cur.Enabled, cur.Disabled, ended = false, "ended", true
		}
		after = cur
		return ports.ScheduleChange{Write: true, Value: cur, Outcome: &o}, nil
	})
	if err != nil {
		b.report(ports.Error, "schedule %s on %s: recording outcome %s: %v", v.ID, r.Name, outcome, err)
	}
	switch {
	case disabled:
		b.scheduleWarn(r, "alert", "schedule %s disabled after %d refused firings in a row to %s; last: %s", v.ID, ScheduleDisableAfter, v.Target, detail)
	case refused(outcome):
		b.scheduleWarn(r, "warning", "schedule %s to %s: %s (%s)", v.ID, v.Target, outcome, detail)
		if after.Streak == ScheduleTargetWarnAfter {
			b.scheduleWarn(r, "warning", "schedule %s: %s refused %d firings in a row", v.ID, v.Target, ScheduleTargetWarnAfter)
		}
	case outcome == "send_unknown":
		b.scheduleWarn(r, "warning", "schedule %s to %s: send_unknown, dropped (%s)", v.ID, v.Target, detail)
	case outcome == "late":
		b.scheduleWarn(r, "warning", "schedule %s skipped: %s", v.ID, detail)
	}
	if ended {
		b.wakeSchedules()
	}
	return o
}

// ReportScheduleState sets aside what cannot fire and settles firings a stop
// cut off, at start: a claimed firing whose outcome is missing is send_unknown
// and never repeated.
func (b *Bus) ReportScheduleState() {
	b.mu.Lock()
	store, _ := b.store.(ports.ScheduleStore)
	b.unlock()
	if store == nil {
		return
	}
	if ids, err := store.ScheduleOrphans(); err == nil && len(ids) > 0 {
		b.report(ports.Alert, "schedules held by %d record IDs that name no stored record were set aside", len(ids))
	}
	if ids, err := store.ScheduleBroken(); err == nil && len(ids) > 0 {
		b.report(ports.Alert, "invalid stored schedules set aside, never fired: %s", strings.Join(ids, ", "))
	}
	inflight, err := store.ScheduleInFlight()
	if err != nil {
		b.report(ports.Error, "schedules in flight at start: %v", err)
		return
	}
	for _, v := range inflight {
		b.mu.Lock()
		r, ok := b.holderByID(v.Record)
		b.unlock()
		if !ok {
			continue
		}
		b.recordFiring(r, v, v.InFlight, b.clock().Unix(), "send_unknown", "the daemon stopped between claim and outcome", false)
	}
	now := b.clock().Unix()
	due, err := store.ScheduleDue(now, ScheduleMaxOnce)
	if err == nil && len(due) > 0 {
		b.report(ports.Warning, "%d schedules fell due while the daemon was stopped; each fires once now", len(due))
	}
}

// wakeSchedules tells the engine the earliest slot may have changed.
func (b *Bus) wakeSchedules() {
	select {
	case b.schedWake <- struct{}{}:
	default:
	}
}

// ScheduleBatch is how many due firings one pass reads.
const ScheduleBatch = 100

// ServeSchedules runs the engine until ctx ends: it sleeps to the earliest
// slot, at most a minute so a wall-clock step is noticed, fires what is due
// one firing at a time, and alerts when firings run more than a minute late.
func (b *Bus) ServeSchedules(done <-chan struct{}) {
	b.ReportScheduleState()
	for {
		wait := b.schedulePass()
		timer := time.NewTimer(wait)
		select {
		case <-done:
			timer.Stop()
			return
		case <-b.schedWake:
		case <-timer.C:
		}
		timer.Stop()
	}
}

// expiredFiring is a scheduled message that expired unread in its target.
type expiredFiring struct{ id, target string }

// reportExpired warns each holder whose scheduled message expired unread.
func (b *Bus) reportExpired(store ports.ScheduleStore) {
	b.mu.Lock()
	expired := b.schedExpired
	b.schedExpired = nil
	b.unlock()
	for _, x := range expired {
		v, ok, err := store.ScheduleGet(x.id)
		if err != nil || !ok {
			continue
		}
		b.mu.Lock()
		r, live := b.holderByID(v.Record)
		b.unlock()
		if live {
			b.scheduleWarn(r, "warning", "schedule %s: a message expired unread in %s", x.id, x.target)
		}
	}
}

// schedulePass fires what is due now and says how long to sleep.
func (b *Bus) schedulePass() time.Duration {
	b.mu.Lock()
	store, _ := b.store.(ports.ScheduleStore)
	b.unlock()
	if store != nil {
		b.reportExpired(store)
	}
	if store == nil {
		return time.Minute
	}
	for rounds := 0; rounds < 100; rounds++ {
		now := b.clock().UTC()
		due, err := store.ScheduleDue(now.Unix(), ScheduleBatch)
		if err != nil {
			b.report(ports.Error, "schedules: %v", err)
			return time.Minute
		}
		if len(due) == 0 {
			break
		}
		if late := now.Unix() - due[0].Next; late > 60 && !b.schedStarted {
			// The start-up catch-up is not the engine falling behind.
		} else if late > 60 {
			b.warnRepeated("schedule engine late: %d firings due more than a minute ago are unsent", len(due))
		}
		for _, v := range due {
			b.mu.Lock()
			r, live := b.holderByID(v.Record)
			b.unlock()
			if !live {
				continue // an inactive holder's slots wait for it
			}
			b.fire(r, v, v.Next, b.clock().UTC(), false)
		}
	}
	b.schedStarted = true
	next, ok, err := store.ScheduleNext()
	if err != nil || !ok {
		return time.Minute
	}
	wait := time.Until(time.Unix(next, 0))
	if wait < 0 {
		wait = 0
	}
	if wait > time.Minute {
		wait = time.Minute
	}
	return wait
}

// shortLived is a holder whose name lasts as long as an AI session: the
// launchers register those agents with the ai-cli tag (docs/06-launchers.md).
func shortLived(r protocol.Record) bool {
	for _, t := range r.Tags {
		if t == "ai-cli" {
			return true
		}
	}
	return false
}

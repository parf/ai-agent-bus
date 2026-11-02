package core

import (
	"sort"
	"sync"
	"time"

	"github.com/parf/ai-agent-bus/internal/eventlog"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/subscription"
)

// Unbound embedded buses keep private reservations in memory. A bound store
// must implement the durable port: never silently downgrade a real store.
type eventIDs struct {
	mu   sync.Mutex
	high map[uint32]uint64
}

func (s *eventIDs) EventHighWater(id uint32) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.high[id], nil
}
func (s *eventIDs) ReserveEventIDs(id uint32, n uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n < s.high[id] {
		return ports.ErrEventStateInvalid
	}
	s.high[id] = n
	return nil
}

func (b *Bus) eventReader(caller string, r protocol.Record) bool {
	if caller == b.admin {
		return true
	}
	if r.Name == DaemonAgent {
		return false
	}
	return caller == r.Owner || b.maintains(caller, r)
}

// eventAccess decides from current registry state, before looking at log
// existence. Returned data is checked again after external store work.
func (b *Bus) eventAccess(caller, target string, write bool) (protocol.Record, *eventlog.Engine, eventlog.Policy, error) {
	caller, err := canon(caller)
	if err != nil {
		return protocol.Record{}, nil, nil, eventlog.Error("forbidden")
	}
	target, err = canon(target)
	if err != nil {
		return protocol.Record{}, nil, nil, eventlog.Error("invalid_log")
	}
	b.mu.Lock()
	defer b.unlock()
	if b.acting(caller) != nil {
		return protocol.Record{}, nil, nil, eventlog.Error("forbidden")
	}
	// A record that is not live (inactive, or its User inactive) is the zero record here, as a missing
	// name is, so every caller gets the same answer for the two and a former rights-holder cannot tell
	// "suspended" from "gone" (docs/01-identity-and-authority.md#user-states). The ACL decides first
	// on what remains.
	r, ok := b.entity(target)
	allowed := b.eventReader(caller, r)
	if write {
		allowed = caller == b.admin || caller == target
	}
	if !allowed {
		return protocol.Record{}, nil, nil, eventlog.Error("forbidden")
	}
	if !ok || r.Kind != protocol.KindAgent {
		return protocol.Record{}, nil, nil, eventlog.Error("not_found")
	}
	if b.events == nil {
		var ids ports.EventIDStore
		var alerts ports.EventAlertStore
		if b.store == nil {
			ids = &eventIDs{high: map[uint32]uint64{}}
		} else {
			ids, _ = b.store.(ports.EventIDStore)
			alerts, _ = b.store.(ports.EventAlertStore)
		}
		b.events = eventlog.New(ids, alerts, b.clock)
	}
	p := eventlog.Defaults()
	values, _ := busObject(b.records[DaemonAgent].Config)
	for _, l := range eventlog.Levels {
		rule := p[l]
		if n, ok := values[l+"_entries"].(interface{ Int64() (int64, error) }); ok {
			v, _ := n.Int64()
			rule.Size = int(v)
		}
		if n, ok := values[l+"_days"].(interface{ Int64() (int64, error) }); ok {
			v, _ := n.Int64()
			rule.TTL = time.Duration(v) * 24 * time.Hour
		}
		p[l] = rule
	}
	return r, b.events, p, nil
}
func (b *Bus) eventRecheck(caller string, r protocol.Record, write bool) error {
	current, _, _, err := b.eventAccess(caller, r.Name, write)
	if err != nil {
		return err
	}
	if current.ID != r.ID {
		return eventlog.Error("not_found")
	}
	return nil
}
func (b *Bus) EventWrite(caller, target, level, text string) (ports.EventEntry, error) {
	v, _, err := b.EventWriteWithin(caller, target, level, text, time.Time{})
	return v, err
}

// subscribersNeed is the least time a write must have left to be stored when a
// subscription could match it: stored and then forwarded, or not stored at all.
const subscribersNeed = 3 * time.Second

// EventWriteWithin appends to the caller's own log and sends the copies the
// entry's subscriptions ask for, once each, before it returns: the write is
// done only when its fan-out is, which keeps a pull or clean of the same log
// behind it (the daemon schedules one mutation per log at a time). The fan-out
// holds no registry or ring lock. A write with a deadline that leaves a
// possible subscriber less than three seconds is refused before it is stored.
func (b *Bus) EventWriteWithin(caller, target, level, text string, deadline time.Time) (ports.EventEntry, subscription.Result, error) {
	r, e, p, err := b.eventAccess(caller, target, true)
	if err != nil {
		return ports.EventEntry{}, subscription.Result{}, err
	}
	defer b.eventRelease(r, e)
	if !deadline.IsZero() && eventlog.IsLevel(level) && time.Until(deadline) < subscribersNeed && b.subsHaveCandidates(r, level) {
		return ports.EventEntry{}, subscription.Result{}, eventlog.Error("deadline")
	}
	v, err := e.WriteChecked(r.ID, level, text, p, func() error { return b.eventRecheck(caller, r, true) })
	if err != nil {
		return v, subscription.Result{}, err
	}
	end := deadline
	if end.IsZero() {
		end = time.Now().Add(subscription.BatchBudget)
	}
	res := b.deliverEntry(r, v, end)
	if err = b.eventRecheck(caller, r, true); err != nil {
		return ports.EventEntry{}, subscription.Result{}, err
	}
	return v, res, nil
}
func (b *Bus) EventRead(caller, target, level string) (eventlog.ReadResult, error) {
	r, e, p, err := b.eventAccess(caller, target, false)
	if err != nil {
		return eventlog.ReadResult{}, err
	}
	defer b.eventRelease(r, e)
	v, err := e.ReadChecked(r.ID, r.Name, level, p, func() error { return b.eventRecheck(caller, r, false) })
	if err != nil {
		return v, err
	}
	if err = b.eventRecheck(caller, r, false); err != nil {
		return eventlog.ReadResult{}, err
	}
	return v, nil
}
func (b *Bus) EventPull(caller, target, level string, count int) (eventlog.PullResult, error) {
	r, e, p, err := b.eventAccess(caller, target, false)
	if err != nil {
		return eventlog.PullResult{}, err
	}
	defer b.eventRelease(r, e)
	v, err := e.PullChecked(r.ID, r.Name, level, count, p, func() error { return b.eventRecheck(caller, r, false) })
	if err != nil {
		return v, err
	}
	if err = b.eventRecheck(caller, r, false); err != nil {
		return eventlog.PullResult{}, err
	}
	return v, nil
}
func (b *Bus) EventClean(caller, target, level string, upper uint64) (eventlog.CleanResult, error) {
	r, e, p, err := b.eventAccess(caller, target, false)
	if err != nil {
		return eventlog.CleanResult{}, err
	}
	defer b.eventRelease(r, e)
	v, err := e.CleanChecked(r.ID, r.Name, level, upper, p, func() error { return b.eventRecheck(caller, r, false) })
	if err != nil {
		return v, err
	}
	if err = b.eventRecheck(caller, r, false); err != nil {
		return eventlog.CleanResult{}, err
	}
	return v, nil
}
func (b *Bus) EventList(caller string) (map[string]any, error) {
	b.mu.Lock()
	now := b.clock()
	names := []string{}
	for name, r := range b.records {
		// b.live is defence in depth: EventRead below answers not_found for a hidden log, and the loop skips it.
		if r.Kind == protocol.KindAgent && b.live(r) && b.eventReader(caller, r) {
			names = append(names, name)
		}
	}
	b.unlock()
	sort.Strings(names)
	out := []eventlog.Summary{}
	for _, name := range names {
		v, err := b.EventRead(caller, name, "")
		if eventlog.Code(err) == "not_found" || eventlog.Code(err) == "forbidden" || eventlog.Code(err) == "storage_invalid" || eventlog.Code(err) == "storage_unknown" {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, v.Summary)
	}
	return map[string]any{"logs": out, "now": now.UTC().Format("2006-01-02T15:04:05.000Z")}, nil
}

// EventCapabilities drives method discovery without granting access to data.
func (b *Bus) EventCapabilities(caller string) (write, read bool) {
	b.mu.Lock()
	defer b.unlock()
	if b.acting(caller) != nil {
		return false, false
	}
	write = caller == b.admin || b.records[caller].Kind == protocol.KindAgent
	for _, r := range b.records {
		if r.Kind == protocol.KindAgent && b.live(r) && b.eventReader(caller, r) {
			read = true
			break
		}
	}
	return
}

// A request admitted just before deletion may finish afterwards. Its final
// release removes any old-ID catalog entry it loaded after the delete hook.
func (b *Bus) eventRelease(r protocol.Record, e *eventlog.Engine) {
	b.mu.Lock()
	live := b.records[r.Name].ID == r.ID
	pruner, _ := b.store.(ports.EventStatePruner)
	b.unlock()
	if !live {
		e.Forget(r.ID)
		if pruner != nil {
			if err := pruner.DropEventState(r.ID); err != nil {
				b.mu.Lock()
				b.report(ports.Error, "retired agent-event state could not be removed for %s", r.Name)
				b.unlock()
			}
		}
	}
}

// EventAdmissionDrop counts queue admission refusal without I/O or waiting on
// a log's mutation lock. Untrusted targets cannot change someone else's count.
func (b *Bus) EventAdmissionDrop(caller, target string) {
	r, e, _, err := b.eventAccess(caller, target, true)
	if err == nil {
		e.AdmissionDrop(r.ID)
	}
}

// EventAdmit is an in-memory reader preflight only. It reserves no authority;
// operations must still perform their execution and precommit checks.
func (b *Bus) EventAdmit(caller, target string, write bool) (string, error) {
	r, _, _, err := b.eventAccess(caller, target, write)
	return r.Name, err
}

// FlushEvents drains accepted alert writes after request handlers have stopped.
func (b *Bus) FlushEvents() error {
	b.mu.Lock()
	e := b.events
	b.unlock()
	if e == nil {
		return nil
	}
	return e.Close()
}

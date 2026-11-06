package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/parf/ai-agent-bus/internal/eventlog"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/subscription"
)

// Subscriptions of an agent's log (Plans/R1.0-Release/agent-events/README.md,
// docs/agents/subscriptions.md#the-shared-behaviour). The engine is the shared
// one; this is its daemon side: who may subscribe (the source's managers,
// checked when the subscription is created and asked again after the store is
// read, never at a send: Q325), what a send still needs (the author's reach to
// the target under the target's ordinary ACL), and who sends the copy: #bus, the
// service, never the source agent, whose own words these are not. The target
// must therefore admit #bus under its ordinary ACL, like any other sender.

// subMemory is the store of an unbound embedded bus, as eventIDs is for ids.
type subMemory struct {
	mu   sync.Mutex
	rows map[string]struct {
		source uint32
		body   []byte
	}
}

func (s *subMemory) ListSubscriptions(source uint32) (map[string][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string][]byte{}
	for id, r := range s.rows {
		if r.source == source {
			out[id] = append([]byte(nil), r.body...)
		}
	}
	return out, nil
}

func (s *subMemory) AddSubscription(source uint32, id string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, taken := s.rows[id]; taken {
		return ports.ErrSubscriptionExists
	}
	if s.rows == nil {
		s.rows = map[string]struct {
			source uint32
			body   []byte
		}{}
	}
	s.rows[id] = struct {
		source uint32
		body   []byte
	}{source, append([]byte(nil), value...)}
	return nil
}

func (s *subMemory) DeleteSubscription(source uint32, id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.rows[id]
	if !ok || r.source != source {
		return false, nil
	}
	delete(s.rows, id)
	return true, nil
}

func (s *subMemory) SetSubscriptionCounts(source uint32, id string, sent, dropped int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.rows[id]
	if !ok || r.source != source {
		return ports.ErrSubscriptionAbsent
	}
	var m map[string]any
	if err := json.Unmarshal(r.body, &m); err != nil {
		return err
	}
	m["sent"], m["dropped"] = sent, dropped
	r.body, _ = json.Marshal(m)
	s.rows[id] = r
	return nil
}

func (s *subMemory) FindSubscription(id string) (uint32, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.rows[id]
	return r.source, ok, nil
}

func (s *subMemory) SubscriptionSources(creator string) ([]uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[uint32]bool{}
	for _, r := range s.rows {
		var row struct {
			CreatedBy string `json:"created_by"`
		}
		if json.Unmarshal(r.body, &row) == nil && row.CreatedBy == creator {
			seen[r.source] = true
		}
	}
	out := make([]uint32, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// subStore binds the port to one source record.
type subStore struct {
	port   ports.SubscriptionStore
	source uint32
}

func (s subStore) List() (map[string][]byte, error) { return s.port.ListSubscriptions(s.source) }
func (s subStore) Add(id string, v []byte) error {
	err := s.port.AddSubscription(s.source, id, v)
	if errors.Is(err, ports.ErrSubscriptionExists) {
		return subscription.ErrExists
	}
	return err
}
func (s subStore) Delete(id string) (bool, error) { return s.port.DeleteSubscription(s.source, id) }
func (s subStore) SetCounts(id string, sent, dropped int64) error {
	err := s.port.SetSubscriptionCounts(s.source, id, sent, dropped)
	if errors.Is(err, ports.ErrSubscriptionAbsent) {
		return subscription.ErrAbsent
	}
	return err
}

// subBus is what the engine asks of the daemon for one source.
type subBus struct {
	b      *Bus
	source string
	id     uint32
}

// MaySend is the ordinary send ACL, whatever the target is, the source itself
// included: the author's reach to the target, never a management question.
func (s subBus) MaySend(who, target string) subscription.Admission {
	if !s.b.activeNow(who) { // a suspended or removed author reaches nothing
		return subscription.Refused
	}
	err := s.b.MaySend(who, target)
	switch {
	case err == nil:
		return subscription.Allowed
	case errors.Is(err, ErrNotAllow), errors.Is(err, ErrUnknown), errors.Is(err, ErrKind):
		return subscription.Refused
	}
	return subscription.Unavailable
}

// MayManage is the author's standing over the source log now: still the same
// registration, and still its Owner, a Maintainer or the daemon Owner.
func (s subBus) MayManage(who string) subscription.Admission {
	if s.b.managesNow(who, s.source, s.id) {
		return subscription.Allowed
	}
	return subscription.Refused
}

func (s subBus) Kind(target string) (string, string, error) {
	name, err := canon(target)
	if err != nil {
		return "", "", err
	}
	s.b.mu.Lock()
	defer s.b.unlock()
	r, ok := s.b.entity(name)
	if !ok {
		return "", "", subscription.ErrNoSuchTarget
	}
	return string(r.Kind), r.Name, nil
}

// Send is one non-waiting send from #bus: a full, revoked or unreachable
// target is a refusal, anything else an unknown outcome. It returns at once if
// the attempt's time is already gone and delivers nothing after returning.
func (s subBus) Send(ctx context.Context, target, body, topic, tag string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := s.b.Send(protocol.Envelope{From: DaemonAgent, To: target, Topic: topic, Tag: tag, Body: body})
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrFull), errors.Is(err, ErrNotAllow), errors.Is(err, ErrUnknown), errors.Is(err, ErrKind), errors.Is(err, ErrOverflow):
		return subscription.ErrRefused
	}
	return err
}

func subID() string {
	var raw [6]byte
	rand.Read(raw[:])
	return hex.EncodeToString(raw[:])
}

// subSelector is a subscription's selector in an agent's log: the level, or
// nothing for every level.
func subSelector(m map[string]string) (map[string]string, error) {
	for k, v := range m {
		if k != "level" || !eventlog.IsLevel(v) {
			return nil, errors.New("selector")
		}
	}
	return m, nil
}

func subMatches(selector map[string]string, event map[string]any) bool {
	want, has := selector["level"]
	got, _ := event["level"].(string)
	return !has || want == got
}

// subsFor is the engine of one source's subscriptions, made once and kept.
func (b *Bus) subsFor(r protocol.Record) *subscription.Engine {
	b.mu.Lock()
	defer b.unlock()
	if e, ok := b.subEngines[r.ID]; ok {
		return e
	}
	var port ports.SubscriptionStore = &b.subFallback
	if b.store != nil {
		if p, ok := b.store.(ports.SubscriptionStore); ok {
			port = p
		}
	}
	source := r.Name
	e, err := subscription.New(subStore{port, r.ID}, subBus{b, source, r.ID}, subscription.Config{
		Source: source, ValidateSelector: subSelector, SelectorMatches: subMatches, NewID: subID,
		Clock: b.clock,
		Warning: func(code string) {
			b.mu.Lock()
			b.warnRepeated("an agent-event subscription of %s dropped a copy: %s", source, code)
			b.unlock()
		},
	})
	if err != nil {
		return nil // the source's name is a registered one; New fails only on a bad configuration
	}
	if b.subEngines == nil {
		b.subEngines = map[uint32]*subscription.Engine{}
	}
	b.subEngines[r.ID] = e
	return e
}

// activeNow is whether who is still a registered, active principal.
func (b *Bus) activeNow(who string) bool {
	b.mu.Lock()
	defer b.unlock()
	return b.acting(who) == nil
}

// authError is a refusal of a caller that is no longer active as forbidden, as
// the log's own calls answer it, and any other refusal as it is.
func (b *Bus) authError(caller string, err error) error {
	if errors.Is(err, subscription.ErrNotAuthor) && !b.activeNow(caller) {
		return eventlog.Error("forbidden")
	}
	return subError(err)
}

// managesNow is whether who, now, is the active owner or a maintainer of the
// source registration id under name, or the daemon Owner: asked again after any
// external work, since standing and even the registration can change meanwhile.
func (b *Bus) managesNow(who, name string, id uint32) bool {
	b.mu.Lock()
	defer b.unlock()
	r, ok := b.entity(name)
	return ok && r.ID == id && b.acting(who) == nil && b.eventReader(who, r)
}

func (b *Bus) forgetSubs(id uint32) {
	delete(b.subEngines, id)
}

// subError is an engine refusal in the log's error form, so the calls answer in
// one vocabulary.
func subError(err error) error {
	var coded subscription.Error
	if errors.As(err, &coded) {
		return eventlog.Error(string(coded))
	}
	return err
}

// SubAdd subscribes target to the entries of source's log, optionally of one
// level and matching a filter, from now on. Only the source's managers may.
func (b *Bus) SubAdd(caller, source, target, level, filter string) (subscription.Row, error) {
	r, _, _, err := b.eventAccess(caller, source, false)
	if err != nil {
		return subscription.Row{}, err
	}
	selector := map[string]string{}
	if level != "" {
		selector["level"] = level
	}
	if err := b.subTargetTakesBus(caller, target); err != nil {
		return subscription.Row{}, err
	}
	e := b.subsFor(r)
	if e == nil {
		return subscription.Row{}, eventlog.Error("internal_error")
	}
	standing := func() bool { return b.managesNow(caller, r.Name, r.ID) }
	active := func() bool { return b.activeNow(caller) }
	row, err := e.Add(subscription.Caller{Name: caller, Manager: true, Current: standing, Active: active}, target, selector, filter)
	if errors.Is(err, subscription.ErrNotAuthor) {
		return subscription.Row{}, eventlog.Error("forbidden") // the standing went while the store was read
	}
	if err != nil {
		return subscription.Row{}, subError(err)
	}
	if !standing() { // and if it went after the row was stored, the row goes
		_ = e.Delete(subscription.Caller{Name: caller, Manager: true, Current: func() bool { return true }}, row.ID)
		return subscription.Row{}, eventlog.Error("forbidden")
	}
	return row, nil
}

// subTargetTakesBus refuses, when a subscription is made, a target that can never receive its copies:
// #bus itself, and any target whose Allow does not admit #bus; a Personal agent may list #bus (Q328).
// A target that does not exist, or is the wrong kind, is left to the engine's own answer.
func (b *Bus) subTargetTakesBus(caller, target string) error {
	name, err := canon(target)
	if err != nil {
		return nil
	}
	if name == DaemonAgent {
		return eventlog.Refusal{Code: "invalid_target", Text: "#bus cannot be a subscription target."}
	}
	// Do not disclose target admission to a caller who cannot reach it.
	// The engine keeps the same answer as for an absent target.
	if err := b.MaySend(caller, name); err != nil {
		return nil
	}
	if err := b.MaySend(DaemonAgent, name); errors.Is(err, ErrNotAllow) {
		return eventlog.Refusal{Code: "invalid_target", Text: "The target does not admit #bus: list #bus in its Allow."}
	}
	return nil
}

// SubscriptionEntry is one listed subscription and the log it reads.
type SubscriptionEntry struct {
	Source string
	subscription.Listed
}

func (s SubscriptionEntry) MarshalJSON() ([]byte, error) {
	raw, err := s.Listed.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err = json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	m["source"], _ = json.Marshal(s.Source)
	return json.Marshal(m)
}

// SubList lists subscriptions: those of one source the caller manages, or with
// no source every one of the sources the caller manages and, in the others, the
// ones the caller made.
func (b *Bus) SubList(caller, source string) ([]SubscriptionEntry, error) {
	out := []SubscriptionEntry{}
	collect := func(r protocol.Record, manager bool) error {
		e := b.subsFor(r)
		if e == nil {
			return eventlog.Error("internal_error")
		}
		rows, err := e.List(subscription.Caller{Name: caller, Manager: manager, Current: func() bool { return b.managesNow(caller, r.Name, r.ID) }, Active: func() bool { return b.activeNow(caller) }})
		if err != nil {
			return b.authError(caller, err)
		}
		if manager && !b.managesNow(caller, r.Name, r.ID) && source != "" {
			return eventlog.Error("forbidden") // revoked while the rows were read: nothing of theirs is shown
		}
		for _, row := range rows {
			out = append(out, SubscriptionEntry{Source: r.Name, Listed: row})
		}
		return nil
	}
	if source != "" {
		r, _, _, err := b.eventAccess(caller, source, false)
		if err == nil {
			return out, collect(r, true)
		}
		// The bare listing narrowed to one log: an author who does not manage it
		// sees their own rows there. With none, the refusal is the one a missing
		// or unmanaged log gives, so the narrowed form reveals nothing more.
		if eventlog.Code(err) != "forbidden" {
			return nil, err
		}
		r, ok := b.authoredSource(caller, source)
		if !ok {
			return nil, err
		}
		if cerr := collect(r, false); cerr != nil {
			return nil, cerr
		}
		if len(out) == 0 {
			return nil, err
		}
		return out, nil
	}
	b.mu.Lock()
	if b.acting(caller) != nil {
		b.unlock()
		return nil, eventlog.Error("forbidden")
	}
	managed := map[string]protocol.Record{}
	for name, r := range b.records {
		if r.Kind == protocol.KindAgent && b.live(r) && b.eventReader(caller, r) {
			managed[name] = r
		}
	}
	var port ports.SubscriptionStore = &b.subFallback
	if b.store != nil {
		if p, ok := b.store.(ports.SubscriptionStore); ok {
			port = p
		}
	}
	b.unlock()
	authored, err := port.SubscriptionSources(caller)
	if err != nil {
		return nil, eventlog.Error("store_unknown")
	}
	b.mu.Lock()
	for _, id := range authored {
		if name := b.recordByID[id]; name != "" {
			if _, ok := managed[name]; !ok {
				if r, live := b.entity(name); live {
					managed[name] = r
				}
			}
		}
	}
	b.unlock()
	names := make([]string, 0, len(managed))
	for name := range managed {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		r := managed[name]
		b.mu.Lock()
		manager := b.eventReader(caller, r)
		b.unlock()
		if err := collect(r, manager); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// authoredSource is the live agent log source when the caller, acting, made a
// subscription of it; the bare listing reaches the same logs the same way.
func (b *Bus) authoredSource(caller, source string) (protocol.Record, bool) {
	caller, err := canon(caller)
	if err != nil {
		return protocol.Record{}, false
	}
	source, err = canon(source)
	if err != nil {
		return protocol.Record{}, false
	}
	b.mu.Lock()
	if b.acting(caller) != nil {
		b.unlock()
		return protocol.Record{}, false
	}
	r, live := b.entity(source)
	var port ports.SubscriptionStore = &b.subFallback
	if b.store != nil {
		if p, ok := b.store.(ports.SubscriptionStore); ok {
			port = p
		}
	}
	b.unlock()
	if !live || r.Kind != protocol.KindAgent {
		return protocol.Record{}, false
	}
	authored, err := port.SubscriptionSources(caller)
	if err != nil {
		return protocol.Record{}, false
	}
	for _, id := range authored {
		if id == r.ID {
			return r, true
		}
	}
	return protocol.Record{}, false
}

// SubDelete removes a subscription by its id: its author, or a manager of its source.
func (b *Bus) SubDelete(caller, id string) error {
	b.mu.Lock()
	if b.acting(caller) != nil {
		b.unlock()
		return eventlog.Error("forbidden")
	}
	var port ports.SubscriptionStore = &b.subFallback
	if b.store != nil {
		if p, ok := b.store.(ports.SubscriptionStore); ok {
			port = p
		}
	}
	b.unlock()
	source, found, err := port.FindSubscription(id)
	if err != nil {
		return eventlog.Error("store_unknown")
	}
	b.mu.Lock()
	name := b.recordByID[source]
	r, live := b.entity(name)
	manager := live && b.eventReader(caller, r)
	b.unlock()
	if !found || !live {
		return eventlog.Error("not_found")
	}
	e := b.subsFor(r)
	if e == nil {
		return eventlog.Error("internal_error")
	}
	err = e.Delete(subscription.Caller{Name: caller, Manager: manager, Current: func() bool { return b.managesNow(caller, r.Name, r.ID) }, Active: func() bool { return b.activeNow(caller) }}, id)
	return b.authError(caller, err)
}

// subsHaveCandidates is the selector-only preflight before an entry is stored.
func (b *Bus) subsHaveCandidates(r protocol.Record, level string) bool {
	e := b.subsFor(r)
	return e != nil && e.HasCandidates(map[string]any{"level": level})
}

// deliverEntry sends the copies of an entry that was just stored, once, within
// the write's deadline. Nothing here holds the registry lock or a ring's.
func (b *Bus) deliverEntry(r protocol.Record, entry ports.EventEntry, deadline time.Time) subscription.Result {
	e := b.subsFor(r)
	if e == nil {
		return subscription.Result{}
	}
	body, err := json.Marshal(map[string]any{"log": r.Name, "id": entry.ID, "level": entry.Level, "text": entry.Text,
		"at": entry.At, "last_at": entry.LastAt, "count": entry.Count})
	if err != nil {
		return subscription.Result{}
	}
	return e.Deliver(map[string]any{"level": entry.Level, "id": entry.ID}, entry.Text, string(body), r.Name, entry.Level, deadline)
}

// Package eventlog implements the shared eventlog model, without registry
// authority or delivery. Each record has its own lock; I/O never holds the
// catalog lock. Core supplies authority and never-reused record IDs.
package eventlog

import (
	"errors"
	"regexp"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/parf/ai-agent-bus/internal/ports"
)

const Safe = ports.EventMaxID

var Levels = []string{"alert", "warning", "notice"}
var ansi = regexp.MustCompile("\x1b(?:\\[[0-?]*[ -/]*[@-~]|\\][^\x07\x1b]*(?:\x07|\x1b\\\\))")

type Error string

func (e Error) Error() string { return string(e) }
func IsLevel(s string) bool   { return s == "alert" || s == "warning" || s == "notice" }

type Rule struct {
	Size int
	TTL  time.Duration
}
type Policy map[string]Rule

func Defaults() Policy {
	return Policy{"notice": {200, 24 * time.Hour}, "warning": {200, 7 * 24 * time.Hour}, "alert": {50, 30 * 24 * time.Hour}}
}
func (p Policy) Valid() bool {
	n := 0
	for _, l := range Levels {
		r := p[l]
		if r.Size < 1 || r.TTL <= 0 {
			return false
		}
		n += r.Size
	}
	return n <= 500
}
func Line(s string) (string, error) {
	if !utf8.ValidString(s) {
		return "", Error("invalid_text")
	}
	rs := []rune{}
	for _, r := range ansi.ReplaceAllString(s, "") {
		if unicode.IsOneOf([]*unicode.RangeTable{unicode.Cc, unicode.Cf, unicode.Cs, unicode.Zl, unicode.Zp}, r) {
			continue
		}
		rs = append(rs, r)
		if len(rs) == 200 {
			break
		}
	}
	if len(rs) == 0 {
		return "", Error("invalid_text")
	}
	return string(rs), nil
}
func stamp(t time.Time) string          { return t.UTC().Format("2006-01-02T15:04:05.000Z") }
func parse(s string) (time.Time, error) { return time.Parse("2006-01-02T15:04:05.000Z", s) }

type log struct {
	mu                       sync.Mutex
	loaded, active, poisoned bool
	next, high               uint64
	last                     time.Time
	rings                    map[string][]ports.EventEntry
	tokens, alertTokens      float64
	rateAt                   time.Time
	dropped                  atomic.Uint64
}
type Engine struct {
	mu     sync.Mutex
	logs   map[uint32]*log
	ids    ports.EventIDStore
	alerts ports.EventAlertStore
	clock  func() time.Time
}

func New(ids ports.EventIDStore, alerts ports.EventAlertStore, clock func() time.Time) *Engine {
	if clock == nil {
		clock = time.Now
	}
	return &Engine{logs: map[uint32]*log{}, ids: ids, alerts: alerts, clock: clock}
}
func (e *Engine) get(id uint32) *log {
	e.mu.Lock()
	defer e.mu.Unlock()
	l := e.logs[id]
	if l == nil {
		l = &log{rings: map[string][]ports.EventEntry{}, tokens: 20, alertTokens: 20}
		e.logs[id] = l
	}
	return l
}
func (e *Engine) load(id uint32, l *log) error {
	if l.poisoned {
		return Error("storage_unknown")
	}
	if l.loaded {
		return nil
	}
	if e.ids == nil {
		return Error("storage_unknown")
	}
	high, err := e.ids.EventHighWater(id)
	if errors.Is(err, ports.ErrEventStateInvalid) || high > Safe {
		return Error("storage_invalid")
	}
	if err != nil {
		return Error("storage_unknown")
	}
	l.high = high
	l.next = high + 1
	if e.alerts != nil {
		entries, active, err := e.alerts.LoadEventAlerts(id)
		if errors.Is(err, ports.ErrEventStateInvalid) {
			return Error("storage_invalid")
		}
		if err != nil {
			return Error("storage_unknown")
		}
		prev := uint64(0)
		for _, v := range entries {
			a, ae := parse(v.At)
			z, ze := parse(v.LastAt)
			clean, ce := Line(v.Text)
			if v.Level != "alert" || v.ID <= prev || v.ID > high || v.Count < 1 || v.Count > Safe || ae != nil || ze != nil || z.Before(a) || ce != nil || clean != v.Text {
				return Error("storage_invalid")
			}
			prev = v.ID
		}
		if len(entries) > 500 || (!active && len(entries) > 0) {
			return Error("storage_invalid")
		}
		l.rings["alert"] = append([]ports.EventEntry{}, entries...)
		l.active = active
	}
	l.loaded = true
	return nil
}
func clone(in map[string][]ports.EventEntry) map[string][]ports.EventEntry {
	out := map[string][]ports.EventEntry{}
	for _, s := range Levels {
		out[s] = append([]ports.EventEntry{}, in[s]...)
	}
	return out
}
func (e *Engine) save(id uint32, l *log, next map[string][]ports.EventEntry, active bool) error {
	if e.alerts != nil && (active != l.active || !slices.Equal(next["alert"], l.rings["alert"])) {
		if err := e.alerts.ReplaceEventAlerts(id, append([]ports.EventEntry{}, next["alert"]...), active); err != nil {
			l.poisoned = true
			return Error("storage_unknown")
		}
	}
	l.rings = next
	l.active = active
	return nil
}
func sweep(l *log, p Policy, now time.Time) map[string][]ports.EventEntry {
	out := clone(l.rings)
	for _, s := range Levels {
		keep := out[s][:0]
		for _, v := range out[s] {
			at, _ := parse(v.LastAt)
			if now.Sub(at) < p[s].TTL {
				keep = append(keep, v)
			}
		}
		if len(keep) > p[s].Size {
			keep = keep[len(keep)-p[s].Size:]
		}
		out[s] = keep
	}
	return out
}
func (e *Engine) lock(id uint32, p Policy) (*log, time.Time, error) {
	if !p.Valid() {
		return nil, time.Time{}, Error("invalid_config")
	}
	l := e.get(id)
	l.mu.Lock()
	if err := e.load(id, l); err != nil {
		l.mu.Unlock()
		return nil, time.Time{}, err
	}
	now := e.clock()
	if now.Unix() < 0 || now.Year() >= 10000 {
		l.mu.Unlock()
		return nil, now, Error("invalid_clock")
	}
	return l, now, nil
}
func (e *Engine) Write(id uint32, level, text string, p Policy) (ports.EventEntry, error) {
	return e.WriteChecked(id, level, text, p, nil)
}
func (e *Engine) WriteChecked(id uint32, level, text string, p Policy, check func() error) (ports.EventEntry, error) {
	if !IsLevel(level) {
		return ports.EventEntry{}, Error("invalid_level")
	}
	text, err := Line(text)
	if err != nil {
		return ports.EventEntry{}, err
	}
	l, now, err := e.lock(id, p)
	if err != nil {
		return ports.EventEntry{}, err
	}
	defer l.mu.Unlock()
	if !l.rateAt.IsZero() && now.After(l.rateAt) {
		l.tokens += now.Sub(l.rateAt).Seconds() * 10
		l.alertTokens += now.Sub(l.rateAt).Seconds() * 10
		if l.alertTokens > 20 {
			l.alertTokens = 20
		}
		if l.tokens > 20 {
			l.tokens = 20
		}
	}
	if now.After(l.rateAt) {
		l.rateAt = now
	}
	bucket := &l.tokens
	if level == "alert" {
		bucket = &l.alertTokens
	}
	if *bucket < 1 {
		l.dropped.Add(1)
		return ports.EventEntry{}, Error("rate_limit")
	}
	next := sweep(l, p, now)
	entries := next[level]
	folded := -1
	for i := len(entries) - 1; i >= 0; i-- {
		v := entries[i]
		at, _ := parse(v.At)
		last, _ := parse(v.LastAt)
		if !now.Before(l.last) && v.Text == text && !now.Before(last) && now.Sub(at) >= 0 && now.Sub(at) < 60*time.Second {
			folded = i
			break
		}
	}
	var v ports.EventEntry
	if folded >= 0 {
		v = entries[folded]
		if v.Count < Safe {
			v.Count++
		}
		v.LastAt = stamp(now)
		entries[folded] = v
	} else {
		if l.next > Safe-64 {
			return v, Error("id_limit")
		}
		if l.next > l.high {
			high := l.next + 63
			if err = e.ids.ReserveEventIDs(id, high); err != nil {
				l.poisoned = true
				return v, Error("storage_unknown")
			}
			l.high = high
		}
		v = ports.EventEntry{ID: l.next, Level: level, Text: text, At: stamp(now), LastAt: stamp(now), Count: 1}
		entries = append(entries, v)
		if len(entries) > p[level].Size {
			entries = entries[len(entries)-p[level].Size:]
		}
	}
	next[level] = entries
	if check != nil {
		if err := check(); err != nil {
			return ports.EventEntry{}, err
		}
	}
	if err = e.save(id, l, next, true); err != nil {
		return ports.EventEntry{}, err
	}
	if folded < 0 {
		l.next++
	}
	l.active = true
	*bucket--
	if now.After(l.last) {
		l.last = now
	}
	return v, nil
}

type Summary struct {
	Dropped       uint64         `json:"dropped,omitempty"`
	Log           string         `json:"log"`
	Counts        map[string]int `json:"counts"`
	LatestAlertAt *string        `json:"latest_alert_at"`
}
type ReadResult struct {
	Summary
	Rings map[string][]ports.EventEntry `json:"rings"`
	Now   string                        `json:"now"`
}

func summary(name string, l *log) Summary {
	s := Summary{Log: name, Counts: map[string]int{}, Dropped: l.dropped.Load()}
	for _, level := range Levels {
		s.Counts[level] = len(l.rings[level])
		for _, v := range l.rings["alert"] {
			if s.LatestAlertAt == nil || v.LastAt > *s.LatestAlertAt {
				x := v.LastAt
				s.LatestAlertAt = &x
			}
		}
	}
	return s
}
func (e *Engine) Read(id uint32, name, level string, p Policy) (ReadResult, error) {
	return e.ReadChecked(id, name, level, p, nil)
}
func (e *Engine) ReadChecked(id uint32, name, level string, p Policy, check func() error) (ReadResult, error) {
	if level != "" && !IsLevel(level) {
		return ReadResult{}, Error("invalid_level")
	}
	l, now, err := e.lock(id, p)
	if err != nil {
		return ReadResult{}, err
	}
	defer l.mu.Unlock()
	if !l.active {
		return ReadResult{}, Error("not_found")
	}
	if check != nil {
		if err := check(); err != nil {
			return ReadResult{}, err
		}
	}
	if err = e.save(id, l, sweep(l, p, now), l.active); err != nil {
		return ReadResult{}, err
	}
	out := ReadResult{Summary: summary(name, l), Rings: map[string][]ports.EventEntry{}, Now: stamp(now)}
	for _, s := range Levels {
		if level != "" && level != s {
			continue
		}
		vs := append([]ports.EventEntry{}, l.rings[s]...)
		for i, j := 0, len(vs)-1; i < j; i, j = i+1, j-1 {
			vs[i], vs[j] = vs[j], vs[i]
		}
		out.Rings[s] = vs
	}
	return out, nil
}

type PullResult struct {
	Log       string             `json:"log"`
	Entries   []ports.EventEntry `json:"entries"`
	Remaining int                `json:"remaining"`
	Now       string             `json:"now"`
}

func (e *Engine) Pull(id uint32, name, level string, count int, p Policy) (PullResult, error) {
	return e.PullChecked(id, name, level, count, p, nil)
}
func (e *Engine) PullChecked(id uint32, name, level string, count int, p Policy, check func() error) (PullResult, error) {
	if level != "" && !IsLevel(level) || count < 1 || count > 500 {
		return PullResult{}, Error("invalid_request")
	}
	l, now, err := e.lock(id, p)
	if err != nil {
		return PullResult{}, err
	}
	defer l.mu.Unlock()
	if !l.active {
		return PullResult{}, Error("not_found")
	}
	next := sweep(l, p, now)
	all := []ports.EventEntry{}
	for _, s := range Levels {
		if level == "" || s == level {
			all = append(all, next[s]...)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	if len(all) > count {
		all = all[:count]
	}
	ids := map[uint64]bool{}
	for _, v := range all {
		ids[v.ID] = true
	}
	remaining := 0
	for _, s := range Levels {
		keep := next[s][:0]
		for _, v := range next[s] {
			if !ids[v.ID] {
				keep = append(keep, v)
			}
		}
		next[s] = keep
		remaining += len(keep)
	}
	if check != nil {
		if err := check(); err != nil {
			return PullResult{}, err
		}
	}
	if err = e.save(id, l, next, l.active); err != nil {
		return PullResult{}, err
	}
	return PullResult{name, all, remaining, stamp(now)}, nil
}

type CleanResult struct {
	Log     string         `json:"log"`
	Removed int            `json:"removed"`
	Counts  map[string]int `json:"counts"`
}

func (e *Engine) Clean(id uint32, name, level string, upper uint64, p Policy) (CleanResult, error) {
	return e.CleanChecked(id, name, level, upper, p, nil)
}
func (e *Engine) CleanChecked(id uint32, name, level string, upper uint64, p Policy, check func() error) (CleanResult, error) {
	if level != "" && !IsLevel(level) || upper > Safe {
		return CleanResult{}, Error("invalid_request")
	}
	l, now, err := e.lock(id, p)
	if err != nil {
		return CleanResult{}, err
	}
	defer l.mu.Unlock()
	if !l.active {
		return CleanResult{}, Error("not_found")
	}
	next := sweep(l, p, now)
	removed := 0
	for _, s := range Levels {
		if level != "" && s != level {
			continue
		}
		keep := next[s][:0]
		for _, v := range next[s] {
			if upper > 0 && v.ID > upper {
				keep = append(keep, v)
			} else {
				removed++
			}
		}
		next[s] = keep
	}
	if check != nil {
		if err := check(); err != nil {
			return CleanResult{}, err
		}
	}
	if err = e.save(id, l, next, !(level == "" && upper == 0)); err != nil {
		return CleanResult{}, err
	}
	return CleanResult{name, removed, summary(name, l).Counts}, nil
}
func (e *Engine) Dropped(id uint32) uint64 {
	l := e.get(id)
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.dropped.Load()
}

// Refusal is an Error that keeps its code on the wire and says why in its own words: the code is one
// the protocol already has, the reason is only text for the caller.
type Refusal struct {
	Code Error
	Text string
}

func (r Refusal) Error() string { return string(r.Code) }
func (r Refusal) Unwrap() error { return r.Code }

// Reason is the refusal's own words, or "" when err has none.
func Reason(err error) string {
	var r Refusal
	if errors.As(err, &r) {
		return r.Text
	}
	return ""
}

func Code(err error) string {
	var e Error
	if errors.As(err, &e) {
		return string(e)
	}
	return "internal_error"
}

// Forget releases only this registration's loaded state. A running operation
// may finish on its old pointer, but cannot republish it into the catalog.
func (e *Engine) Forget(id uint32) { e.mu.Lock(); delete(e.logs, id); e.mu.Unlock() }
func (e *Engine) Loaded() int      { e.mu.Lock(); defer e.mu.Unlock(); return len(e.logs) }

func (e *Engine) AdmissionDrop(id uint32) { e.get(id).dropped.Add(1) }

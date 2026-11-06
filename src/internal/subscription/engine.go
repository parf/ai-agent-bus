// Package subscription is the shared subscription behaviour of the daemon's
// agent logs: state, authority, matching and once-only delivery
// (docs/agents/subscriptions.md#the-shared-behaviour,
// src/agents/shared/subscriptions.md). It is the Go twin of the Python engine
// and passes the same vectors, src/testdata/subscriptions.json.
//
// The engine knows nothing of the daemon. A Store keeps one source's rows, a
// Bus answers who may send where and carries the copy, and the caller says
// whether the acting principal manages the source. A copy is sent once and
// never retried; a refused, unreachable or slow target drops that copy and
// counts it, and the subscription stays for the next entry.
package subscription

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/subfilter"
)

const (
	MaxSafe         = int64(1<<53 - 1)
	MaxRowBytes     = 4096
	MaxSelectorSize = 2048
)

// The send budgets, variables only so a test can shorten them.
var (
	AttemptBudget = time.Second     // one target's send
	BatchBudget   = 5 * time.Second // a whole write's fan-out
)

// Error is a stable refusal code; its prose is not the contract.
type Error string

func (e Error) Error() string { return string(e) }

const (
	ErrInvalidTarget     = Error("invalid_target")
	ErrInvalidSelector   = Error("invalid_selector")
	ErrInvalidFilter     = Error("invalid_filter")
	ErrInvalidID         = Error("invalid_id")
	ErrLimit             = Error("limit")
	ErrNotFound          = Error("not_found")
	ErrNotAuthor         = Error("not_author")
	ErrAdmissionRefused  = Error("admission_refused")
	ErrAdmissionUnavail  = Error("admission_unavailable")
	ErrStoreUnknown      = Error("store_unknown")
	warnFilterInvalid    = "filter_invalid"
	warnFilterTimeout    = "filter_timeout"
	warnInvalidText      = "invalid_text"
	warnSendRefused      = "send_refused"
	warnSendUnknown      = "send_unknown"
	warnDeadline         = "deadline"
	warnAdmissionRefused = "admission_refused"
	warnAdmissionUnavail = "admission_unavailable"
	warnStoreUnknown     = "store_unknown"
)

// Row is a stored subscription.
type Row struct {
	ID        string            `json:"id"`
	Target    string            `json:"target"`
	Selector  map[string]string `json:"selector"`
	Filter    string            `json:"filter"`
	CreatedBy string            `json:"created_by"`
	CreatedAt string            `json:"created_at"`
	Sent      int64             `json:"sent"`
	Dropped   int64             `json:"dropped"`
}

// Listed is one entry of a listing: a row, with Reason "invalid_filter" when
// its stored filter is no longer accepted, or only an id with Reason
// "invalid_state" when the stored entry is not a subscription at all.
type Listed struct {
	Row    *Row
	ID     string
	Reason string
}

func (l Listed) MarshalJSON() ([]byte, error) {
	if l.Row == nil {
		return json.Marshal(map[string]string{"id": l.ID, "reason": l.Reason})
	}
	raw, err := json.Marshal(l.Row)
	if err != nil || l.Reason == "" {
		return raw, err
	}
	var m map[string]json.RawMessage
	if err = json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	m["reason"], _ = json.Marshal(l.Reason)
	return json.Marshal(m)
}

// Store keeps the rows of one source. Values are the stored JSON, which the
// engine validates: a stored entry that is not a subscription is damage to be
// shown to the managers, never a reason to stop.
type Store interface {
	// List returns every stored entry by id.
	List() (map[string][]byte, error)
	// Add stores a new entry; ErrExists when the id is taken.
	Add(id string, value []byte) error
	// Delete removes an entry, and says whether there was one.
	Delete(id string) (bool, error)
	// SetCounts writes the two counters of an entry.
	SetCounts(id string, sent, dropped int64) error
}

// ErrExists is Store.Add's answer for a taken id.
var ErrExists = errors.New("subscription id exists")

// ErrAbsent is Store.SetCounts's answer for a row deleted while its copy was
// being sent: nothing to count, and no failure.
var ErrAbsent = errors.New("subscription is gone")

// Admission is the answer to "may who send to target".
type Admission int

const (
	Allowed Admission = iota
	Refused
	Unavailable
)

// Bus is what the engine needs of the daemon.
type Bus interface {
	// MaySend asks whether who may reach target under the target's own ACL.
	// A 404-like refusal is Refused; anything unexpected is Unavailable, never
	// Allowed.
	MaySend(who, target string) Admission
	// MayManage asks whether who may read and manage the source log. It is the
	// authority over the source and nothing about reaching it as a target.
	MayManage(who string) Admission
	// Kind is the registry kind of target and its canonical name;
	// ErrNoSuchTarget when there is none, any other error when it cannot be
	// told.
	Kind(target string) (kind, name string, err error)
	// Send puts one copy in the target's inbox. It returns promptly when ctx
	// ends and never delivers after it has returned. ErrRefused is a refusal
	// (full, revoked); any other error is an unknown outcome.
	Send(ctx context.Context, target, body, topic, tag string) error
}

// ErrRefused is Bus.Send's answer for a refused copy.
var ErrRefused = errors.New("send refused")

// ErrNoSuchTarget is Bus.Kind's answer for a name that is not registered.
var ErrNoSuchTarget = errors.New("no such target")

// Config is an engine's fixed settings.
type Config struct {
	Source           string
	ValidateSelector func(map[string]string) (map[string]string, error)
	SelectorMatches  func(selector map[string]string, event map[string]any) bool
	Warning          func(code string)
	Clock            func() time.Time
	NewID            func() string
	MaxSubscriptions int // at most 100
	MaxPerCreator    int // at most 20
}

// Engine is one source's subscriptions.
type Engine struct {
	cfg   Config
	store Store
	bus   Bus
	mu    sync.Mutex
}

var (
	idPattern   = regexp.MustCompile(`^[0-9a-f]{12}$`)
	timePattern = regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}Z$`)
)

// New builds an engine.
func New(store Store, bus Bus, cfg Config) (*Engine, error) {
	source, err := targetText(cfg.Source)
	if err != nil {
		return nil, err
	}
	cfg.Source = source
	if cfg.MaxSubscriptions == 0 {
		cfg.MaxSubscriptions = 100
	}
	if cfg.MaxPerCreator == 0 {
		cfg.MaxPerCreator = 20
	}
	if cfg.MaxSubscriptions < 1 || cfg.MaxSubscriptions > 100 || cfg.MaxPerCreator < 1 || cfg.MaxPerCreator > 20 {
		return nil, ErrLimit
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	return &Engine{cfg: cfg, store: store, bus: bus}, nil
}

func targetText(v string) (string, error) {
	name, err := protocol.ParseName(v)
	if err != nil {
		return "", ErrInvalidTarget
	}
	return name.String(), nil
}

func identity(v string) (string, error) {
	t, err := targetText(v)
	if err != nil {
		return "", ErrNotAuthor
	}
	return t, nil
}

func validID(v string) (string, error) {
	if !idPattern.MatchString(v) {
		return "", ErrInvalidID
	}
	return v, nil
}

func (e *Engine) warn(code string) {
	if e.cfg.Warning != nil {
		e.cfg.Warning(code) // a diagnostic never causes a message to be sent again
	}
}

func (e *Engine) selector(v map[string]string) (map[string]string, error) {
	if v == nil {
		return nil, ErrInvalidSelector
	}
	raw, err := json.Marshal(v)
	if err != nil || len(raw) > MaxSelectorSize || e.cfg.ValidateSelector == nil {
		return nil, ErrInvalidSelector
	}
	out, err := e.cfg.ValidateSelector(copyMap(v))
	if err != nil || out == nil {
		return nil, ErrInvalidSelector
	}
	return out, nil
}

func copyMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// parseRow validates a stored value as a subscription under key.
func (e *Engine) parseRow(raw []byte, key string) (Row, error) {
	var row Row
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var fields map[string]any
	if err := dec.Decode(&fields); err != nil || dec.More() {
		return row, errors.New("not an object")
	}
	for _, f := range []string{"id", "target", "selector", "filter", "created_by", "created_at", "sent", "dropped"} {
		if _, ok := fields[f]; !ok {
			return row, errors.New("missing field")
		}
	}
	if len(fields) != 8 {
		return row, errors.New("unknown field")
	}
	str := func(f string) (string, bool) { s, ok := fields[f].(string); return s, ok }
	var ok bool
	if row.ID, ok = str("id"); !ok || row.ID != key {
		return row, errors.New("id")
	}
	if _, err := validID(key); err != nil {
		return row, err
	}
	if row.Target, ok = str("target"); !ok {
		return row, errors.New("target")
	}
	if _, err := targetText(row.Target); err != nil {
		return row, err
	}
	if row.CreatedBy, ok = str("created_by"); !ok {
		return row, errors.New("created_by")
	}
	if _, err := identity(row.CreatedBy); err != nil {
		return row, err
	}
	if row.Filter, ok = str("filter"); !ok {
		return row, errors.New("filter")
	}
	sel, ok := fields["selector"].(map[string]any)
	if !ok {
		return row, errors.New("selector")
	}
	selector := map[string]string{}
	for k, v := range sel {
		s, isString := v.(string)
		if !isString {
			return row, errors.New("selector")
		}
		selector[k] = s
	}
	var err error
	if row.Selector, err = e.selector(selector); err != nil {
		return row, err
	}
	if row.CreatedAt, ok = str("created_at"); !ok || !timePattern.MatchString(row.CreatedAt) {
		return row, errors.New("created_at")
	}
	if _, err = time.Parse("2006-01-02T15:04:05.000Z", row.CreatedAt); err != nil {
		return row, err
	}
	for f, into := range map[string]*int64{"sent": &row.Sent, "dropped": &row.Dropped} {
		n, isNumber := fields[f].(json.Number)
		if !isNumber {
			return row, errors.New(f)
		}
		v, perr := n.Int64()
		if perr != nil || strings.ContainsAny(n.String(), ".eE") || v < 0 || v > MaxSafe {
			return row, errors.New(f)
		}
		*into = v
	}
	if len(raw) > MaxRowBytes {
		return row, errors.New("too long")
	}
	return row, nil
}

// load returns the valid rows and the damaged ids; a store that cannot be read
// is store_unknown.
func (e *Engine) load() (map[string]Row, map[string]bool, error) {
	stored, err := e.store.List()
	if err != nil {
		return nil, nil, ErrStoreUnknown
	}
	if len(stored) > e.cfg.MaxSubscriptions {
		return nil, nil, ErrLimit
	}
	rows, damaged := map[string]Row{}, map[string]bool{}
	for id, raw := range stored {
		if _, err := validID(id); err != nil {
			return nil, nil, err
		}
		row, err := e.parseRow(raw, id)
		if err != nil {
			damaged[id] = true
			continue
		}
		rows[id] = row
	}
	return rows, damaged, nil
}

// Caller is the authenticated acting principal: its canonical name, and whether
// it manages the source (the source's Owner, a Maintainer or the daemon Owner).
// Neither comes from a request body. Current, when set, is asked again after
// the rows have been read and before anything is shown or removed: standing can
// change while the store is read, and the answer then is the one that counts.
type Caller struct {
	Name    string
	Manager bool
	Current func() bool
	// Active, when set, is asked at the same moments: whether the caller is still
	// a registered, active principal, whatever it manages. A suspended author
	// shows and removes nothing, even its own.
	Active func() bool
}

func (c Caller) gone() bool { return c.Active != nil && !c.Active() }

func (c Caller) manager() bool {
	if c.Current != nil {
		return c.Current()
	}
	return c.Manager
}

// admissions caches a batch's answers by who was asked, about what, and which
// question: reaching a target and managing the source are different questions
// even when the target is the source.
type admissions map[admissionKey]error

type admissionKey struct {
	who, name string
	manage    bool
}

func (e *Engine) admit(who, target string, cache admissions) error {
	pair := admissionKey{who, target, false}
	if code, ok := cache[pair]; ok {
		return code
	}
	var code error
	switch e.bus.MaySend(who, target) {
	case Allowed:
	case Refused:
		code = ErrAdmissionRefused
	default:
		code = ErrAdmissionUnavail
	}
	cache[pair] = code
	return code
}

// admitManage is the author's standing over the source, cached with the batch's
// other questions under the source's own name.
func (e *Engine) admitManage(who string, cache admissions) error {
	pair := admissionKey{who, e.cfg.Source, true}
	if code, ok := cache[pair]; ok {
		return code
	}
	var code error
	switch e.bus.MayManage(who) {
	case Allowed:
	case Refused:
		code = ErrAdmissionRefused
	default:
		code = ErrAdmissionUnavail
	}
	cache[pair] = code
	return code
}

func (e *Engine) destination(target string) (string, error) {
	kind, name, err := e.bus.Kind(target)
	if err != nil {
		return "", err
	}
	if kind != "agent" && kind != "queue" {
		return "", ErrInvalidTarget
	}
	return targetText(name)
}

func (e *Engine) stamp() (string, error) {
	now := e.cfg.Clock()
	if now.Unix() < 0 || now.Unix() >= 253402300799 {
		return "", ErrStoreUnknown
	}
	return now.UTC().Truncate(time.Millisecond).Format("2006-01-02T15:04:05.000Z"), nil
}

// Add creates a subscription from the moment of the call; nothing earlier is
// replayed. The creator comes only from the authenticated caller.
func (e *Engine) Add(c Caller, target string, selector map[string]string, filter string) (Row, error) {
	who, err := identity(c.Name)
	if err != nil {
		return Row{}, err
	}
	if target, err = targetText(target); err != nil {
		return Row{}, err
	}
	if selector, err = e.selector(selector); err != nil {
		return Row{}, err
	}
	if _, err = subfilter.Compile(filter); err != nil {
		return Row{}, ErrInvalidFilter
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	rows, damaged, err := e.load()
	if err != nil {
		return Row{}, err
	}
	if c.gone() || c.Current != nil && !c.Current() { // asked after the rows were read
		return Row{}, ErrNotAuthor
	}
	mine := 0
	for _, r := range rows {
		if r.CreatedBy == who {
			mine++
		}
	}
	if len(rows)+len(damaged) >= e.cfg.MaxSubscriptions || mine >= e.cfg.MaxPerCreator {
		return Row{}, ErrLimit
	}
	cache := admissions{}
	if err = e.admit(who, target, cache); err != nil {
		return Row{}, err
	}
	if err = e.admitManage(who, cache); err != nil {
		return Row{}, err
	}
	if target, err = e.destination(target); err != nil {
		var coded Error
		if errors.As(err, &coded) {
			return Row{}, err
		}
		return Row{}, ErrAdmissionUnavail
	}
	created, err := e.stamp()
	if err != nil {
		return Row{}, err
	}
	for range 3 {
		id := ""
		if e.cfg.NewID != nil {
			id = e.cfg.NewID()
		}
		if _, err = validID(id); err != nil {
			return Row{}, err
		}
		row := Row{ID: id, Target: target, Selector: selector, Filter: filter, CreatedBy: who, CreatedAt: created}
		raw, merr := json.Marshal(row)
		if merr != nil {
			return Row{}, ErrStoreUnknown
		}
		switch err = e.store.Add(id, raw); {
		case err == nil:
			return row, nil
		case errors.Is(err, ErrExists):
			continue
		default:
			return Row{}, ErrStoreUnknown
		}
	}
	return Row{}, ErrLimit
}

// List shows the caller's own rows, or every row to the source's managers. A
// stored filter the grammar no longer accepts is listed with its reason, and a
// damaged entry only to the managers.
func (e *Engine) List(c Caller) ([]Listed, error) {
	who, err := identity(c.Name)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	rows, damaged, err := e.load()
	if err != nil {
		return nil, err
	}
	if c.gone() {
		return nil, ErrNotAuthor
	}
	manager := c.manager() // asked after the rows were read: a revoked manager sees its own only
	ids := make([]string, 0, len(rows)+len(damaged))
	for id := range rows {
		ids = append(ids, id)
	}
	for id := range damaged {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := []Listed{}
	for _, id := range ids {
		if damaged[id] {
			if manager {
				out = append(out, Listed{ID: id, Reason: "invalid_state"})
			}
			continue
		}
		row := rows[id]
		if !manager && row.CreatedBy != who {
			continue
		}
		listed := Listed{Row: &row}
		if _, err := subfilter.Compile(row.Filter); err != nil {
			listed.Reason = "invalid_filter"
		}
		out = append(out, listed)
	}
	return out, nil
}

// Delete removes a subscription: its creator, or a manager of the source, may.
// A damaged entry is repairable only by a manager.
func (e *Engine) Delete(c Caller, id string) error {
	if _, err := validID(id); err != nil {
		return err
	}
	who, err := identity(c.Name)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	rows, damaged, err := e.load()
	if err != nil {
		return err
	}
	if c.gone() {
		return ErrNotAuthor
	}
	row, found := rows[id]
	if !found && !damaged[id] {
		return ErrNotFound
	}
	if !c.manager() && (!found || row.CreatedBy != who) { // asked now, before the row goes
		return ErrNotAuthor
	}
	deleted, err := e.store.Delete(id)
	if err != nil {
		return ErrStoreUnknown
	}
	if !deleted {
		return ErrNotFound
	}
	return nil
}

// HasCandidates is the selector-only preflight over the valid rows, before an
// entry is stored. It tests no filter and sends nothing.
func (e *Engine) HasCandidates(event map[string]any) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	rows, _, err := e.load()
	if err != nil {
		return false
	}
	for _, row := range rows {
		if e.cfg.SelectorMatches != nil && e.cfg.SelectorMatches(row.Selector, event) {
			return true
		}
	}
	return false
}

// Result is a write's deliveries: matched candidates, copies sent, copies dropped.
type Result struct {
	Matched int `json:"matched"`
	Sent    int `json:"sent"`
	Dropped int `json:"dropped"`
}

type outcome struct {
	sent bool
	code string
}

// Deliver sends the entry's copies once, in id order, before the caller goes on:
// the selector first, the bounded filter second, then one checked send per
// match, each at most AttemptBudget and all within BatchBudget or the caller's
// deadline, whichever is shorter. A copy not attempted when the budget ends is
// dropped and counted once. Counters are written after every attempt has
// finished, once per row.
func (e *Engine) Deliver(event map[string]any, text, body, topic, tag string, deadline time.Time) Result {
	// The rows are read under the lock; the filters and the sends, which take
	// time, run without it, so a held target never holds a listing of the same
	// source. The counters are written under it again.
	e.mu.Lock()
	rows, _, err := e.load()
	e.mu.Unlock()
	if err != nil {
		e.warn(warnStoreUnknown)
		return Result{}
	}
	// One deadline from entry bounds the whole write: the filters and then the
	// sends, within BatchBudget or the caller's deadline, whichever is shorter.
	end := time.Now().Add(BatchBudget)
	if deadline.Before(end) {
		end = deadline
	}
	var candidates []Row
	for _, row := range rows {
		if e.cfg.SelectorMatches != nil && e.cfg.SelectorMatches(row.Selector, event) {
			candidates = append(candidates, row)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })
	outcomes := map[string]outcome{}
	var active []Row
	switch {
	case !time.Now().Before(end):
		for _, r := range candidates {
			outcomes[r.ID] = outcome{code: warnDeadline}
		}
	case len(text) > subfilter.MaxText || !utf8.ValidString(text):
		for _, r := range candidates {
			outcomes[r.ID] = outcome{code: warnInvalidText}
		}
	default:
		matching := time.Now()
		for _, r := range candidates {
			if r.Filter == "" {
				active = append(active, r)
				continue
			}
			left := time.Until(end)
			if left <= 0 { // the write's own budget is gone: this copy was never attempted
				outcomes[r.ID] = outcome{code: warnDeadline}
				continue
			}
			if time.Since(matching) >= time.Second { // all filters together get one second
				outcomes[r.ID] = outcome{code: warnFilterTimeout}
				continue
			}
			f, cerr := subfilter.Compile(r.Filter)
			if cerr != nil {
				outcomes[r.ID] = outcome{code: warnFilterInvalid}
				continue
			}
			ok, merr := f.Match(text, patternBudget(left))
			switch {
			case errors.Is(merr, subfilter.ErrTimeout):
				outcomes[r.ID] = outcome{code: warnFilterTimeout}
			case merr != nil:
				outcomes[r.ID] = outcome{code: warnFilterInvalid}
			case ok:
				active = append(active, r)
			}
		}
	}
	if len(active) > 0 {
		e.send(active, body, topic, tag, end, outcomes)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	result := Result{Matched: len(outcomes)}
	failed := false
	for _, r := range candidates {
		o, ok := outcomes[r.ID]
		if !ok {
			continue
		}
		if o.sent {
			result.Sent++
		} else {
			result.Dropped++
			e.warn(o.code)
		}
		if o.sent {
			r.Sent = min(MaxSafe, r.Sent+1)
		} else {
			r.Dropped = min(MaxSafe, r.Dropped+1)
		}
		if err := e.store.SetCounts(r.ID, r.Sent, r.Dropped); err != nil && !errors.Is(err, ErrAbsent) {
			failed = true
		}
	}
	if failed {
		e.warn(warnStoreUnknown)
	}
	return result
}

// patternBudget is one filter's time: 50 ms, and never more than what is left of
// the write's own budget.
func patternBudget(left time.Duration) time.Duration { return min(50*time.Millisecond, left) }

// send attempts each active row's copy, one at a time, in id order.
func (e *Engine) send(rows []Row, body, topic, tag string, end time.Time, out map[string]outcome) {
	cache := admissions{}
	for _, row := range rows {
		if !time.Now().Before(end) {
			out[row.ID] = outcome{code: warnDeadline}
			continue
		}
		if code := e.admit(row.CreatedBy, row.Target, cache); code != nil {
			out[row.ID] = outcome{code: admissionWarning(code)}
			continue
		}
		if name, err := e.destination(row.Target); err != nil || name != row.Target {
			code := warnAdmissionRefused
			var coded Error
			if err != nil && !errors.As(err, &coded) && !errors.Is(err, ErrNoSuchTarget) {
				code = warnAdmissionUnavail
			}
			out[row.ID] = outcome{code: code}
			continue
		}
		if !time.Now().Before(end) {
			out[row.ID] = outcome{code: warnDeadline}
			continue
		}
		budget := time.Until(end)
		if budget > AttemptBudget {
			budget = AttemptBudget
		}
		ctx, cancel := context.WithTimeout(context.Background(), budget)
		err := e.bus.Send(ctx, row.Target, body, topic, tag)
		cancel()
		switch {
		case err == nil:
			out[row.ID] = outcome{sent: true}
		case errors.Is(err, ErrRefused):
			out[row.ID] = outcome{code: warnSendRefused}
		default:
			out[row.ID] = outcome{code: warnSendUnknown}
		}
	}
}

func admissionWarning(err error) string {
	if errors.Is(err, ErrAdmissionRefused) {
		return warnAdmissionRefused
	}
	return warnAdmissionUnavail
}

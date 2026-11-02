package subscription

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

// The shared operation vectors, src/testdata/subscriptions.json: the same 33
// cases the Python and TypeScript engines pass, run on the Go engine with the
// same mocks, the queries asked, the sends attempted and the warnings raised
// checked exactly, not only the answers.

type vectorStore struct {
	rows    map[string][]byte
	writes  int
	step    map[string]any
	removed int
}

func (s *vectorStore) List() (map[string][]byte, error) {
	out := map[string][]byte{}
	for k, v := range s.rows {
		out[k] = append([]byte(nil), v...)
	}
	return out, nil
}

func (s *vectorStore) Add(id string, value []byte) error {
	s.writes++
	if _, taken := s.rows[id]; taken {
		return ErrExists
	}
	s.rows[id] = append([]byte(nil), value...)
	if s.step["store"] == "unknown_after" {
		return errors.New("not confirmed")
	}
	return nil
}

func (s *vectorStore) Delete(id string) (bool, error) {
	_, had := s.rows[id]
	delete(s.rows, id)
	return had, nil
}

func (s *vectorStore) SetCounts(id string, sent, dropped int64) error {
	s.writes++
	if s.step["counter_store"] == "unknown" {
		return errors.New("not confirmed")
	}
	raw, ok := s.rows[id]
	if !ok {
		return errors.New("absent")
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	m["sent"], m["dropped"] = sent, dropped
	s.rows[id], _ = json.Marshal(m)
	return nil
}

type query struct {
	Who    string `json:"who"`
	Target string `json:"target"`
}

type vectorBus struct {
	t        *testing.T
	step     map[string]any
	answers  []any
	queries  []query
	attempts int
}

func (b *vectorBus) MaySend(who, target string) Admission {
	b.queries = append(b.queries, query{who, target})
	var answer any = true
	if len(b.answers) > 0 {
		answer, b.answers = b.answers[0], b.answers[1:]
	}
	switch answer {
	case true:
		return Allowed
	case false, "404":
		return Refused
	}
	return Unavailable
}

func (b *vectorBus) Kind(target string) (string, string, error) {
	kind, _ := b.step["kind"].(string)
	if kind == "" {
		kind = "queue"
	}
	name := target
	if !strings.Contains(name, "@") {
		name += "@fixture.test"
	}
	return kind, name, nil
}

func (b *vectorBus) Send(_ context.Context, target, body, topic, tag string) error {
	b.attempts++
	switch b.step["send"] {
	case "refused":
		return ErrRefused
	case "unknown":
		return errors.New("not confirmed")
	case "must-not-send":
		b.t.Error("a copy was sent that must not be")
	}
	return nil
}

var jsonSurrogate = regexp.MustCompile(`(?i)\\ud[89ab][0-9a-f]{2}(\\ud[c-f][0-9a-f]{2})?|\\ud[c-f][0-9a-f]{2}`)

// vectorText is a JSON string's value, a lone surrogate escape becoming bytes
// that are not UTF-8: a Go string cannot hold one, and the engine must refuse it.
func vectorText(raw json.RawMessage) string {
	var s string
	_ = json.Unmarshal(raw, &s)
	for _, m := range jsonSurrogate.FindAllString(string(raw), -1) {
		if len(m) == 6 {
			return "\xed\xa0\x80"
		}
	}
	return s
}

func vectorSelector(m map[string]string) (map[string]string, error) {
	for k := range m {
		if k != "log" && k != "level" {
			return nil, errors.New("selector")
		}
	}
	return m, nil
}

func vectorMatches(selector map[string]string, event map[string]any) bool {
	for k, want := range selector {
		if got, _ := event[k].(string); got != want {
			return false
		}
	}
	return true
}

func TestSharedOperationVectors(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/subscriptions.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Cases []struct {
			Name    string
			Source  string
			Now     string
			IDs     []string
			Initial []map[string]any
			Limits  map[string]int
			Steps   []json.RawMessage
		}
	}
	if err = json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Cases) != 33 {
		t.Fatalf("%d operation vectors, want 33: the file changed", len(file.Cases))
	}
	for _, c := range file.Cases {
		t.Run(c.Name, func(t *testing.T) {
			store := &vectorStore{rows: map[string][]byte{}}
			for _, row := range c.Initial {
				value, _ := json.Marshal(row)
				store.rows[row["id"].(string)] = value
			}
			bus := &vectorBus{t: t}
			var warnings []string
			now := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
			if c.Now != "" {
				now, _ = time.Parse("2006-01-02T15:04:05.000Z", c.Now)
			}
			ids, serial := append([]string(nil), c.IDs...), 0
			cfg := Config{
				Source: c.Source, ValidateSelector: vectorSelector, SelectorMatches: vectorMatches,
				Warning: func(code string) { warnings = append(warnings, code) },
				Clock:   func() time.Time { return now },
				NewID: func() string {
					serial++
					if c.IDs != nil {
						if len(ids) == 0 {
							t.Fatal("id factory exhausted")
						}
						id := ids[0]
						ids = ids[1:]
						return id
					}
					return fmt.Sprintf("%012x", serial)
				},
				MaxSubscriptions: c.Limits["max_subscriptions"], MaxPerCreator: c.Limits["max_per_creator"],
			}
			engine, err := New(store, bus, cfg)
			if err != nil {
				t.Fatal(err)
			}
			for n, rawStep := range c.Steps {
				var step map[string]any
				if err := json.Unmarshal(rawStep, &step); err != nil {
					t.Fatal(err)
				}
				var fields map[string]json.RawMessage
				_ = json.Unmarshal(rawStep, &fields)
				store.step, bus.step = step, step
				bus.answers, _ = step["checks"].([]any)
				bus.queries, bus.attempts, store.writes, warnings = nil, 0, 0, nil
				actual := map[string]any{}
				runStep(t, engine, c.Source, step, fields, actual)
				actual["queries"] = append([]query{}, bus.queries...)
				actual["attempts"] = bus.attempts
				actual["warnings"] = append([]string{}, warnings...)
				actual["writes"] = store.writes
				expect, _ := step["expect"].(map[string]any)
				for field, want := range expect {
					got, _ := json.Marshal(actual[field])
					var gotValue any
					_ = json.Unmarshal(got, &gotValue)
					if !reflect.DeepEqual(gotValue, want) {
						t.Errorf("step %d (%s) %s = %s, want %v", n, step["op"], field, got, want)
					}
				}
			}
		})
	}
}

func runStep(t *testing.T, engine *Engine, source string, step map[string]any, fields map[string]json.RawMessage, actual map[string]any) {
	t.Helper()
	author, _ := step["author"].(string)
	roles, _ := step["roles"].([]any)
	manager := false
	for _, r := range roles {
		if r == "owner" || r == "maintainer" {
			manager = true
		}
	}
	if original, has := step["original_to"].(string); has && original != "" && !strings.EqualFold(original, source) {
		manager = false
	}
	caller := Caller{Name: author, Manager: manager}
	fail := func(err error) {
		var coded Error
		if errors.As(err, &coded) {
			actual["error"] = string(coded)
		} else {
			t.Errorf("unexpected error %v", err)
		}
	}
	switch step["op"] {
	case "add":
		var selector map[string]string
		if sel, ok := step["selector"].(map[string]any); ok {
			selector = map[string]string{}
			for k, v := range sel {
				s, isString := v.(string)
				if !isString {
					selector = nil
					break
				}
				selector[k] = s
			}
		}
		target, _ := step["target"].(string)
		filter, _ := step["filter"].(string)
		row, err := engine.Add(caller, target, selector, filter)
		if err != nil {
			fail(err)
			return
		}
		actual["row"] = row
	case "list":
		rows, err := engine.List(caller)
		if err != nil {
			fail(err)
			return
		}
		actual["rows"] = rows
	case "delete":
		id, _ := step["id"].(string)
		if err := engine.Delete(caller, id); err != nil {
			fail(err)
			return
		}
		actual["deleted"] = true
	case "deliver":
		event, _ := step["event"].(map[string]any)
		var ev struct{ Text json.RawMessage }
		_ = json.Unmarshal(fields["event"], &ev)
		text := vectorText(ev.Text)
		deadline := time.Now().Add(5 * time.Second)
		if step["deadline"] == "expired" {
			deadline = time.Now().Add(-time.Second)
		}
		body, _ := json.Marshal(event)
		log, _ := event["log"].(string)
		level, _ := event["level"].(string)
		res := engine.Deliver(event, text, string(body), log, level, deadline)
		actual["matched"], actual["sent"], actual["dropped"] = res.Matched, res.Sent, res.Dropped
	default:
		t.Fatalf("unknown vector operation %v", step["op"])
	}
}

// holdBus blocks each send until its context ends, as a target whose inbox
// admission hangs, and records whether anything was delivered.
type holdBus struct {
	vectorBus
	started   chan string
	delivered []string
}

func (b *holdBus) Send(ctx context.Context, target, body, topic, tag string) error {
	b.started <- target
	<-ctx.Done()
	return ctx.Err() // returns when the context ends and delivers nothing
}

func rowJSON(id, creator string) []byte {
	raw, _ := json.Marshal(Row{ID: id, Target: "events@fixture.test", Selector: map[string]string{"log": "deploys"}, CreatedBy: creator, CreatedAt: "2026-10-29T00:00:00.000Z"})
	return raw
}

// One held target costs one attempt budget; a write's whole fan-out stops at its
// budget and the copies not attempted are dropped and counted once; nothing is
// delivered after Deliver has returned.
func TestHeldSendsAreBoundedPerTargetAndPerWrite(t *testing.T) {
	oldA, oldB := AttemptBudget, BatchBudget
	AttemptBudget, BatchBudget = 100*time.Millisecond, 250*time.Millisecond
	defer func() { AttemptBudget, BatchBudget = oldA, oldB }()
	store := &vectorStore{rows: map[string][]byte{}}
	for i := 1; i <= 6; i++ {
		store.rows[fmt.Sprintf("%012x", i)] = rowJSON(fmt.Sprintf("%012x", i), "#writer@fixture.test")
	}
	bus := &holdBus{vectorBus: vectorBus{t: t, step: map[string]any{}}, started: make(chan string, 16)}
	var warnings []string
	engine, err := New(store, bus, Config{Source: "#eventlog@fixture.test", ValidateSelector: vectorSelector, SelectorMatches: vectorMatches,
		Warning: func(c string) { warnings = append(warnings, c) }})
	if err != nil {
		t.Fatal(err)
	}
	began := time.Now()
	res := engine.Deliver(map[string]any{"log": "deploys", "level": "alert"}, "x", "{}", "deploys", "alert", time.Now().Add(time.Minute))
	took := time.Since(began)
	if took > 600*time.Millisecond {
		t.Fatalf("a write's fan-out took %s with a 250 ms budget", took)
	}
	if res.Matched != 6 || res.Sent != 0 || res.Dropped != 6 {
		t.Fatalf("result %+v, want 6 matched, none sent, 6 dropped (each counted once)", res)
	}
	if n := len(bus.started); n < 2 || n > 3 {
		t.Fatalf("%d sends were attempted in a 250 ms budget of 100 ms attempts, want 2 or 3", n)
	}
	unattempted := 0
	for _, w := range warnings {
		if w == "deadline" {
			unattempted++
		}
	}
	if unattempted != 6-len(bus.started) || len(warnings) != 6 {
		t.Fatalf("warnings %v for %d attempts: want one per copy, the rest 'deadline'", warnings, len(bus.started))
	}
	// Every counter was written once, and nothing sends after the call returned.
	before := len(bus.started)
	time.Sleep(150 * time.Millisecond)
	if len(bus.started) != before {
		t.Fatal("a send began after Deliver had returned")
	}
	for i := 1; i <= 6; i++ {
		var row Row
		_ = json.Unmarshal(store.rows[fmt.Sprintf("%012x", i)], &row)
		if row.Dropped != 1 || row.Sent != 0 {
			t.Errorf("row %d counters sent %d dropped %d, want 0 and 1", i, row.Sent, row.Dropped)
		}
	}
}

// A counter stops at the largest exact integer.
func TestCountersSaturate(t *testing.T) {
	store := &vectorStore{rows: map[string][]byte{}}
	raw, _ := json.Marshal(Row{ID: "000000000001", Target: "events@fixture.test", Selector: map[string]string{}, CreatedBy: "#writer@fixture.test", CreatedAt: "2026-10-29T00:00:00.000Z", Sent: MaxSafe})
	store.rows["000000000001"] = raw
	bus := &vectorBus{t: t, step: map[string]any{}}
	engine, _ := New(store, bus, Config{Source: "#eventlog@fixture.test", ValidateSelector: vectorSelector, SelectorMatches: vectorMatches})
	engine.Deliver(map[string]any{"log": "x", "level": "alert"}, "x", "{}", "x", "alert", time.Now().Add(time.Minute))
	var row Row
	_ = json.Unmarshal(store.rows["000000000001"], &row)
	if row.Sent != MaxSafe {
		t.Fatalf("sent %d, want it to stop at %d", row.Sent, MaxSafe)
	}
}

// A caller whose name is not a valid actor is refused before anything is read,
// asked or written: the creator comes only from the authenticated caller.
func TestAnUnauthenticatedCallerNameTouchesNothing(t *testing.T) {
	store := &vectorStore{rows: map[string][]byte{}}
	bus := &vectorBus{t: t, step: map[string]any{}}
	engine, _ := New(store, bus, Config{Source: "#eventlog@fixture.test", ValidateSelector: vectorSelector, SelectorMatches: vectorMatches,
		NewID: func() string { return "000000000001" }})
	for _, who := range []string{"", "not a name", "#caf\u00e9@fixture.test"} {
		bad := Caller{Name: who, Manager: true}
		if _, err := engine.Add(bad, "events@fixture.test", map[string]string{}, ""); !errors.Is(err, ErrNotAuthor) {
			t.Errorf("add as %q: %v, want not_author", who, err)
		}
		if _, err := engine.List(bad); !errors.Is(err, ErrNotAuthor) {
			t.Errorf("list as %q: %v, want not_author", who, err)
		}
		if err := engine.Delete(bad, "000000000001"); !errors.Is(err, ErrNotAuthor) {
			t.Errorf("delete as %q: %v, want not_author", who, err)
		}
	}
	if len(bus.queries) != 0 || store.writes != 0 || len(store.rows) != 0 {
		t.Fatalf("a refused caller reached the bus (%d queries) or the store (%d writes)", len(bus.queries), store.writes)
	}
}

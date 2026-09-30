package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// kvFixture: alice owns #svc@h, whose allow list admits bob and whose
// Maintainers are the @crew group carol is in.
func kvFixture(t *testing.T) (*Bus, *memory.State) {
	t.Helper()
	st := memory.NewState()
	b := New()
	b.Persistence(st)
	b.SetDaemonOwner("admin@h")
	for _, who := range []string{"alice@h", "bob@h", "carol@h", "eve@h"} {
		if _, err := b.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.SetGroup("admin@h", "@crew", []string{"carol@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "alice@h", Allow: []string{"bob@h"}, Maintainers: protocol.MaintainerList{"@crew"}}); err != nil {
		t.Fatal(err)
	}
	return b, st
}

func str(s string) ports.KVValue { return ports.KVValue{Bytes: []byte(s)} }

func TestKVIsTheRecordsManagersAlone(t *testing.T) {
	b, _ := kvFixture(t)
	// The Owner, a Maintainer through a group, and the record's own Agent.
	for i, who := range []string{"alice@h", "carol@h", "#svc@h"} {
		name := fmt.Sprintf("n%d", i)
		if err := b.KVSet(who, "#svc@h", ports.KVString, name, str(who), ""); err != nil {
			t.Fatalf("%s could not write its record's store: %v", who, err)
		}
		if v, err := b.KVGet("alice@h", "#svc@h", ports.KVString, name); err != nil || string(v.Bytes) != who {
			t.Fatalf("%s's write read back as %q, %v", who, v.Bytes, err)
		}
	}
	// The allow list grants use of the record, not of its store.
	for _, who := range []string{"bob@h", "eve@h"} {
		if _, err := b.KVGet(who, "#svc@h", ports.KVString, "n0"); !errors.Is(err, ErrNotOwner) {
			t.Errorf("%s read the store: %v", who, err)
		}
		if err := b.KVSet(who, "#svc@h", ports.KVString, "x", str("x"), ""); !errors.Is(err, ErrNotOwner) {
			t.Errorf("%s wrote the store: %v", who, err)
		}
	}
	if _, err := b.KVGet("alice@h", "nothing@h", ports.KVString, "n0"); !errors.Is(err, ErrUnknown) {
		t.Errorf("a store of no record answered %v", err)
	}
}

func TestKVStoreIsNoSuchEntityWhileItsRecordIsInactive(t *testing.T) {
	b, _ := kvFixture(t)
	if err := b.KVSet("alice@h", "#svc@h", ports.KVInt, "n", ports.KVValue{Int: 7}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "#svc@h", Status: ptr("inactive")}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.KVGet("alice@h", "#svc@h", ports.KVInt, "n"); !errors.Is(err, ErrUnknown) {
		t.Fatalf("an inactive record's store answered %v", err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "#svc@h", Status: ptr("active")}); err != nil {
		t.Fatal(err)
	}
	if v, err := b.KVGet("alice@h", "#svc@h", ports.KVInt, "n"); err != nil || v.Int != 7 {
		t.Fatalf("the store did not come back with its record: %v, %v", v, err)
	}
}

func TestKVNameRemovedAndRegisteredAgainStartsEmpty(t *testing.T) {
	b, st := kvFixture(t)
	for _, kind := range []ports.KVKind{ports.KVString, ports.KVJSON} {
		if err := b.KVSet("alice@h", "#svc@h", kind, "k", str(`{"a":1}`), ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.KVIntInc("alice@h", "#svc@h", "k", 1); err != nil {
		t.Fatal(err)
	}
	if err := b.Unregister("#svc@h", "alice@h"); err != nil {
		t.Fatal(err)
	}
	// The removal took the values with it, rather than leaving them behind an
	// ID nothing will reach again.
	if ids, err := st.KVOrphans(); err != nil || len(ids) != 0 {
		t.Fatalf("the removed record's values stayed behind: %v %v", ids, err)
	}
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "eve@h"}); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []ports.KVKind{ports.KVString, ports.KVInt, ports.KVJSON} {
		if _, err := b.KVGet("eve@h", "#svc@h", kind, "k"); !errors.Is(err, ErrKVAbsent) {
			t.Errorf("the new holder of the name read the old %s value: %v", kind, err)
		}
	}
}

func TestKVKindsAreSeparateNamespaces(t *testing.T) {
	b, _ := kvFixture(t)
	if err := b.KVSet("alice@h", "#svc@h", ports.KVString, "count", str("seven"), ""); err != nil {
		t.Fatal(err)
	}
	if err := b.KVSet("alice@h", "#svc@h", ports.KVInt, "count", ports.KVValue{Int: 7}, ""); err != nil {
		t.Fatal(err)
	}
	s, _ := b.KVGet("alice@h", "#svc@h", ports.KVString, "count")
	n, _ := b.KVGet("alice@h", "#svc@h", ports.KVInt, "count")
	if string(s.Bytes) != "seven" || n.Int != 7 {
		t.Fatalf("one kind overwrote the other: %q, %d", s.Bytes, n.Int)
	}
	if _, err := b.KVGet("alice@h", "#svc@h", ports.KVJSON, "count"); !errors.Is(err, ErrKVAbsent) {
		t.Fatalf("the third kind saw a value: %v", err)
	}
}

func TestKVSetModes(t *testing.T) {
	b, _ := kvFixture(t)
	if err := b.KVSet("alice@h", "#svc@h", ports.KVString, "k", str("1"), KVReplace); !errors.Is(err, ErrKVAbsent) {
		t.Fatalf("replace of an absent name: %v", err)
	}
	if _, err := b.KVGet("alice@h", "#svc@h", ports.KVString, "k"); !errors.Is(err, ErrKVAbsent) {
		t.Fatalf("a refused replace wrote: %v", err)
	}
	if err := b.KVSet("alice@h", "#svc@h", ports.KVString, "k", str("1"), KVAdd); err != nil {
		t.Fatal(err)
	}
	if err := b.KVSet("alice@h", "#svc@h", ports.KVString, "k", str("2"), KVAdd); !errors.Is(err, ErrKVPresent) {
		t.Fatalf("add over a present name: %v", err)
	}
	if v, _ := b.KVGet("alice@h", "#svc@h", ports.KVString, "k"); string(v.Bytes) != "1" {
		t.Fatalf("a refused add wrote %q", v.Bytes)
	}
	if err := b.KVSet("alice@h", "#svc@h", ports.KVString, "k", str("3"), KVReplace); err != nil {
		t.Fatal(err)
	}
	if err := b.KVSet("alice@h", "#svc@h", ports.KVString, "k", str("4"), ""); err != nil {
		t.Fatal(err)
	}
	if v, _ := b.KVGet("alice@h", "#svc@h", ports.KVString, "k"); string(v.Bytes) != "4" {
		t.Fatalf("set left %q", v.Bytes)
	}
	if err := b.KVSet("alice@h", "#svc@h", ports.KVString, "k", str("5"), "upsert"); !errors.Is(err, ErrKVValue) {
		t.Fatalf("an unknown mode: %v", err)
	}
	if was, err := b.KVDelete("alice@h", "#svc@h", ports.KVString, "k"); err != nil || !was {
		t.Fatalf("delete: %v, %v", was, err)
	}
	if was, err := b.KVDelete("alice@h", "#svc@h", ports.KVString, "k"); err != nil || was {
		t.Fatalf("a second delete said it removed something: %v, %v", was, err)
	}
}

func TestKVConcurrentAddsHaveOneWinner(t *testing.T) {
	b, _ := kvFixture(t)
	const n = 50
	var wg sync.WaitGroup
	var mu sync.Mutex
	won := 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := b.KVSet("alice@h", "#svc@h", ports.KVString, "leader", str(fmt.Sprint(i)), KVAdd)
			if err == nil {
				mu.Lock()
				won++
				mu.Unlock()
			} else if !errors.Is(err, ErrKVPresent) {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if won != 1 {
		t.Fatalf("%d concurrent adds of one name succeeded", won)
	}
}

func TestKVIntIncIsExactUnderConcurrency(t *testing.T) {
	b, _ := kvFixture(t)
	const workers, each = 20, 50
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < each; j++ {
				if _, err := b.KVIntInc("alice@h", "#svc@h", "hits", 1); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
	v, err := b.KVGet("alice@h", "#svc@h", ports.KVInt, "hits")
	if err != nil || v.Int != workers*each {
		t.Fatalf("hits = %d, %v; want %d", v.Int, err, workers*each)
	}
}

func ops(t *testing.T, spec string) []KVOp {
	t.Helper()
	var o []KVOp
	if err := json.Unmarshal([]byte(spec), &o); err != nil {
		t.Fatal(err)
	}
	return o
}

func jsonDoc(t *testing.T, b *Bus, name string) string {
	t.Helper()
	v, err := b.KVGet("alice@h", "#svc@h", ports.KVJSON, name)
	if err != nil {
		t.Fatal(err)
	}
	return string(v.Bytes)
}

func TestKVJSONOperationsDoWhatTheirRowSays(t *testing.T) {
	b, _ := kvFixture(t)
	for _, c := range []struct{ ops, doc, results string }{
		{`[{"op":"set","key":"a","value":{"x":1}}]`, `{"a":{"x":1}}`, `[true]`},
		{`[{"op":"inc","key":"n","value":2},{"op":"inc","key":"n","value":-5}]`, `{"a":{"x":1},"n":-3}`, `[true,true]`},
		{`[{"op":"push","key":"q","value":"b"},{"op":"push","key":"q","value":"c"},{"op":"unshift","key":"q","value":"a"}]`, `{"a":{"x":1},"n":-3,"q":["a","b","c"]}`, `[true,true,true]`},
		{`[{"op":"shift","key":"q"}]`, `{"a":{"x":1},"n":-3,"q":["b","c"]}`, `[true]`},
		{`[{"op":"pop","key":"q"}]`, `{"a":{"x":1},"n":-3,"q":["b"]}`, `[true]`},
		{`[{"op":"add_to_set","key":"q","value":"b"},{"op":"add_to_set","key":"q","value":"d"}]`, `{"a":{"x":1},"n":-3,"q":["b","d"]}`, `[false,true]`},
		{`[{"op":"push","key":"q","value":"d"},{"op":"remove_from_set","key":"q","value":"d"}]`, `{"a":{"x":1},"n":-3,"q":["b"]}`, `[true,true]`},
		{`[{"op":"unset","key":"a"},{"op":"unset","key":"gone"}]`, `{"n":-3,"q":["b"]}`, `[true,false]`},
		{`[{"op":"shift","key":"none"},{"op":"pop","key":"none"},{"op":"remove_from_set","key":"none","value":1}]`, `{"n":-3,"q":["b"]}`, `[false,false,false]`},
	} {
		res, err := b.KVJSON("alice@h", "#svc@h", "doc", ops(t, c.ops))
		if err != nil {
			t.Fatalf("%s: %v", c.ops, err)
		}
		changed := make([]bool, len(res))
		for i, r := range res {
			changed[i] = r.Changed
		}
		if got, _ := json.Marshal(changed); string(got) != c.results {
			t.Errorf("%s changed %s, want %s", c.ops, got, c.results)
		}
		if got := jsonDoc(t, b, "doc"); got != c.doc {
			t.Errorf("%s left %s, want %s", c.ops, got, c.doc)
		}
	}
	// shift and pop answer what they took, and inc what it left.
	if _, err := b.KVJSON("alice@h", "#svc@h", "doc", ops(t, `[{"op":"push","key":"q","value":{"id":2}}]`)); err != nil {
		t.Fatal(err)
	}
	res, err := b.KVJSON("alice@h", "#svc@h", "doc", ops(t, `[{"op":"shift","key":"q"},{"op":"pop","key":"q"},{"op":"inc","key":"n","value":3}]`))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(res[0].Value) + " " + string(res[1].Value) + " " + string(res[2].Value); got != `"b" {"id":2} 0` {
		t.Fatalf("results carried %s", got)
	}
	// An empty array's shift is no error and takes nothing.
	res, err = b.KVJSON("alice@h", "#svc@h", "doc", ops(t, `[{"op":"shift","key":"q"}]`))
	if err != nil || res[0].Changed || res[0].Value != nil {
		t.Fatalf("an empty shift: %+v, %v", res, err)
	}
}

func TestKVJSONEqualityIgnoresKeyOrderAndNumberSpelling(t *testing.T) {
	b, _ := kvFixture(t)
	res, err := b.KVJSON("alice@h", "#svc@h", "set", ops(t, `[
		{"op":"add_to_set","key":"s","value":{"a":1,"b":[2]}},
		{"op":"add_to_set","key":"s","value":{"b":[2.0],"a":1e0}},
		{"op":"add_to_set","key":"s","value":1},
		{"op":"add_to_set","key":"s","value":"1"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if !res[0].Changed || res[1].Changed || !res[2].Changed || !res[3].Changed {
		t.Fatalf("add_to_set changed %+v", res)
	}
	if _, err := b.KVJSON("alice@h", "#svc@h", "set", ops(t, `[{"op":"remove_from_set","key":"s","value":1.00}]`)); err != nil {
		t.Fatal(err)
	}
	if got := jsonDoc(t, b, "set"); got != `{"s":[{"a":1,"b":[2]},"1"]}` {
		t.Fatalf("left %s", got)
	}
}

func TestKVJSONListIsAllOrNone(t *testing.T) {
	b, _ := kvFixture(t)
	if _, err := b.KVJSON("alice@h", "#svc@h", "doc", ops(t, `[{"op":"set","key":"s","value":"text"},{"op":"push","key":"q","value":1}]`)); err != nil {
		t.Fatal(err)
	}
	before := jsonDoc(t, b, "doc")
	for _, bad := range []string{
		`[{"op":"push","key":"q","value":2},{"op":"push","key":"s","value":3}]`,
		`[{"op":"push","key":"q","value":2},{"op":"inc","key":"s","value":1}]`,
		`[{"op":"push","key":"q","value":2},{"op":"flip","key":"q"}]`,
		`[{"op":"push","key":"q","value":2},{"op":"shift","key":"q","value":1}]`,
		`[{"op":"push","key":"q","value":2},{"op":"set","key":""}]`,
	} {
		if _, err := b.KVJSON("alice@h", "#svc@h", "doc", ops(t, bad)); err == nil {
			t.Errorf("%s was accepted", bad)
		}
		if got := jsonDoc(t, b, "doc"); got != before {
			t.Fatalf("%s left %s, a partial list", bad, got)
		}
	}
	// A value that is not an object takes no operations and is not stored.
	if err := b.KVSet("alice@h", "#svc@h", ports.KVJSON, "arr", str(`[1]`), ""); !errors.Is(err, ErrKVValue) {
		t.Fatalf("a JSON array was stored: %v", err)
	}
}

func TestKVConcurrentShiftsHandEachElementOnce(t *testing.T) {
	b, _ := kvFixture(t)
	const items = 200
	var push []string
	for i := 0; i < items; i++ {
		push = append(push, fmt.Sprintf(`{"op":"push","key":"jobs","value":%d}`, i))
	}
	if _, err := b.KVJSON("alice@h", "#svc@h", "batch", ops(t, "["+strings.Join(push[:100], ",")+"]")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.KVJSON("alice@h", "#svc@h", "batch", ops(t, "["+strings.Join(push[100:], ",")+"]")); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var got []string
	for w := 0; w < 10; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				res, err := b.KVJSON("alice@h", "#svc@h", "batch", []KVOp{{Op: "shift", Key: "jobs"}})
				if err != nil {
					t.Error(err)
					return
				}
				if !res[0].Changed {
					return
				}
				mu.Lock()
				got = append(got, string(res[0].Value))
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	sort.Slice(got, func(i, j int) bool { return len(got[i]) < len(got[j]) || len(got[i]) == len(got[j]) && got[i] < got[j] })
	if len(got) != items {
		t.Fatalf("%d elements were handed out, want %d", len(got), items)
	}
	for i, v := range got {
		if v != fmt.Sprint(i) {
			t.Fatalf("element %d handed out as %s: duplicated or lost", i, v)
		}
	}
}

func TestKVLimits(t *testing.T) {
	b, _ := kvFixture(t)
	if err := b.KVSet("alice@h", "#svc@h", ports.KVString, strings.Repeat("n", KVMaxName+1), str("x"), ""); !errors.Is(err, ErrKVValue) {
		t.Errorf("a long name: %v", err)
	}
	if err := b.KVSet("alice@h", "#svc@h", ports.KVString, "", str("x"), ""); !errors.Is(err, ErrKVValue) {
		t.Errorf("an empty name: %v", err)
	}
	if err := b.KVSet("alice@h", "#svc@h", ports.KVString, "big", ports.KVValue{Bytes: make([]byte, KVMaxValue+1)}, ""); !errors.Is(err, ErrKVLimit) {
		t.Errorf("a value over the limit: %v", err)
	}
	if err := b.KVSet("alice@h", "#svc@h", ports.KVString, "big", ports.KVValue{Bytes: make([]byte, KVMaxValue)}, ""); err != nil {
		t.Errorf("a value at the limit: %v", err)
	}
	if _, err := b.KVIntInc("alice@h", "#svc@h", "max", 1<<62); err != nil {
		t.Fatal(err)
	}
	if _, err := b.KVIntInc("alice@h", "#svc@h", "max", 1<<62); !errors.Is(err, ErrKVLimit) {
		t.Errorf("an overflowing inc: %v", err)
	}
}

// A record is named as everywhere: trimmed and lower-case, so an equivalent
// spelling reaches the same store.
func TestKVRecordNamesAreCanonical(t *testing.T) {
	b, _ := kvFixture(t)
	if err := b.KVSet("alice@h", " #SVC@H ", ports.KVString, "x", str("ok"), ""); err != nil {
		t.Fatal(err)
	}
	if v, err := b.KVGet("alice@h", "#svc@h", ports.KVString, "x"); err != nil || string(v.Bytes) != "ok" {
		t.Fatalf("an equivalent spelling reached another store: %q %v", v.Bytes, err)
	}
}

// A JSON value is one value and nothing after it.
func TestKVJSONValueIsOneValue(t *testing.T) {
	b, _ := kvFixture(t)
	for _, bad := range []string{`{} }`, `{}{}`, `{} x`, `{"a":1}]`} {
		if err := b.KVSet("alice@h", "#svc@h", ports.KVJSON, "doc", str(bad), ""); !errors.Is(err, ErrKVValue) {
			t.Errorf("%q was stored: %v", bad, err)
		}
		if _, err := b.KVJSON("alice@h", "#svc@h", "doc", []KVOp{{Op: "set", Key: "k", Value: json.RawMessage(bad)}}); !errors.Is(err, ErrKVValue) {
			t.Errorf("%q was taken as an operation's value: %v", bad, err)
		}
	}
}

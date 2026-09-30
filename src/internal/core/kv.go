// Each record's key-value store: string, int and JSON values under names, in
// the daemon's database and never its memory. The registry answers who may
// reach a store — the record's Owner, Maintainers and own Agent, while it is
// active — and the store's own transaction makes each edit atomic
// (Plans/R1.0-Release/kv.md#per-record-storage).
package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"unicode/utf8"

	"github.com/parf/ai-agent-bus/internal/ports"
)

var (
	ErrKVAbsent  = errors.New("no value is stored under that name")
	ErrKVPresent = errors.New("a value is already stored under that name")
	ErrKVValue   = errors.New("that is not a value this store takes")
	ErrKVType    = errors.New("that key holds a different type")
	ErrKVLimit   = errors.New("that is more than a record's store holds")
	ErrKVNoStore = errors.New("this daemon keeps no key-value store")
)

// Limits on one record's store.
const (
	KVMaxName  = 256
	KVMaxValue = 512 << 10
	KVMaxNames = 10000
	KVMaxOps   = 100
)

// KVHow is a write's mode.
const (
	KVSet     = "set"
	KVAdd     = "add"
	KVReplace = "replace"
)

// KVOp is one JSON operation on a top-level key.
type KVOp struct {
	Op    string          `json:"op"`
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value,omitempty"`
}

// KVResult is what one JSON operation did: Changed is false for an op that
// found nothing to do, Value what shift or pop took or the number inc left.
type KVResult struct {
	Op      string          `json:"op"`
	Key     string          `json:"key"`
	Changed bool            `json:"changed"`
	Value   json.RawMessage `json:"value,omitempty"`
}

// kvStore finds the store and the record's ID, asking the registry whether
// caller may use it. Caller holds nothing.
func (b *Bus) kvStore(caller, record, name string) (ports.KVStore, uint32, error) {
	if name == "" || len(name) > KVMaxName || !utf8.ValidString(name) {
		return nil, 0, fmt.Errorf("%w: a name is 1 to %d bytes of UTF-8", ErrKVValue, KVMaxName)
	}
	b.mu.Lock()
	r, live := b.entity(record)
	may := live && b.resourceManages(caller, r)
	store, _ := b.store.(ports.KVStore)
	b.unlock()
	switch {
	case !live:
		return nil, 0, ErrUnknown
	case !may:
		return nil, 0, ErrNotOwner
	case store == nil:
		return nil, 0, ErrKVNoStore
	}
	return store, r.ID, nil
}

// ReportKVOrphans reports values whose record is not stored, found at start.
// They are nobody's: an ID is never reused, so nothing will reach them, and
// they are left for the operator rather than deleted or reattached.
func (b *Bus) ReportKVOrphans() {
	store, _ := b.store.(ports.KVStore)
	if store == nil {
		return
	}
	ids, err := store.KVOrphans()
	if err != nil {
		b.report(ports.Error, "the key-value store could not be checked at start: %s", err)
		return
	}
	for _, id := range ids {
		b.report(ports.Alert, "stored key-value values of record ID %d name no stored record and are ignored", id)
	}
}

// kvErr says a record removed between the check and the write as the removal
// it is.
func kvErr(err error) error {
	if errors.Is(err, ports.ErrKVNoRecord) {
		return ErrUnknown
	}
	return err
}

// KVGet reads one value.
func (b *Bus) KVGet(caller, record string, kind ports.KVKind, name string) (ports.KVValue, error) {
	store, id, err := b.kvStore(caller, record, name)
	if err != nil {
		return ports.KVValue{}, err
	}
	v, found, err := store.KVGet(kind, id, name)
	if err != nil {
		return ports.KVValue{}, err
	}
	if !found {
		return ports.KVValue{}, ErrKVAbsent
	}
	return v, nil
}

// KVSet writes one value: how is set, add (only if absent) or replace (only
// if present). A JSON value is an object, stored in canonical form.
func (b *Bus) KVSet(caller, record string, kind ports.KVKind, name string, v ports.KVValue, how string) error {
	if how == "" {
		how = KVSet
	}
	if how != KVSet && how != KVAdd && how != KVReplace {
		return fmt.Errorf("%w: how is set, add or replace", ErrKVValue)
	}
	if kind == ports.KVJSON {
		doc, err := kvObject(v.Bytes)
		if err != nil {
			return err
		}
		if v.Bytes, err = kvEncode(doc); err != nil {
			return err
		}
	}
	if len(v.Bytes) > KVMaxValue {
		return fmt.Errorf("%w: a value is at most %d bytes", ErrKVLimit, KVMaxValue)
	}
	store, id, err := b.kvStore(caller, record, name)
	if err != nil {
		return err
	}
	return kvErr(store.KVEdit(kind, id, name, func(_ ports.KVValue, found bool, names int) (ports.KVEdit, error) {
		switch {
		case how == KVAdd && found:
			return ports.KVEdit{}, ErrKVPresent
		case how == KVReplace && !found:
			return ports.KVEdit{}, ErrKVAbsent
		case !found && names >= KVMaxNames:
			return ports.KVEdit{}, fmt.Errorf("%w: a record holds at most %d names of one kind", ErrKVLimit, KVMaxNames)
		}
		return ports.KVEdit{Write: true, Value: v}, nil
	}))
}

// KVDelete removes one name, saying whether it was there.
func (b *Bus) KVDelete(caller, record string, kind ports.KVKind, name string) (bool, error) {
	store, id, err := b.kvStore(caller, record, name)
	if err != nil {
		return false, err
	}
	var was bool
	err = store.KVEdit(kind, id, name, func(_ ports.KVValue, found bool, _ int) (ports.KVEdit, error) {
		was = found
		return ports.KVEdit{Delete: found}, nil
	})
	return was, kvErr(err)
}

// KVIntInc adds n to an int, an absent one counting from zero, and answers
// what it is now.
func (b *Bus) KVIntInc(caller, record, name string, n int64) (int64, error) {
	store, id, err := b.kvStore(caller, record, name)
	if err != nil {
		return 0, err
	}
	var now int64
	err = store.KVEdit(ports.KVInt, id, name, func(old ports.KVValue, found bool, names int) (ports.KVEdit, error) {
		if !found && names >= KVMaxNames {
			return ports.KVEdit{}, fmt.Errorf("%w: a record holds at most %d names of one kind", ErrKVLimit, KVMaxNames)
		}
		sum, ok := addInt(old.Int, n)
		if !ok {
			return ports.KVEdit{}, fmt.Errorf("%w: the sum overflows a 64-bit int", ErrKVLimit)
		}
		now = sum
		return ports.KVEdit{Write: true, Value: ports.KVValue{Int: sum}}, nil
	})
	return now, kvErr(err)
}

func addInt(a, b int64) (int64, bool) {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		return 0, false
	}
	return a + b, true
}

// KVJSON applies ops to a JSON value's top-level keys, all of them or none,
// an absent name starting as {}.
func (b *Bus) KVJSON(caller, record, name string, ops []KVOp) ([]KVResult, error) {
	if len(ops) == 0 || len(ops) > KVMaxOps {
		return nil, fmt.Errorf("%w: a list holds 1 to %d operations", ErrKVValue, KVMaxOps)
	}
	store, id, err := b.kvStore(caller, record, name)
	if err != nil {
		return nil, err
	}
	var results []KVResult
	err = store.KVEdit(ports.KVJSON, id, name, func(old ports.KVValue, found bool, names int) (ports.KVEdit, error) {
		if !found && names >= KVMaxNames {
			return ports.KVEdit{}, fmt.Errorf("%w: a record holds at most %d names of one kind", ErrKVLimit, KVMaxNames)
		}
		doc := map[string]any{}
		if found {
			var err error
			if doc, err = kvObject(old.Bytes); err != nil {
				return ports.KVEdit{}, err
			}
		}
		results = results[:0]
		changed := false
		for i, op := range ops {
			r, err := applyOp(doc, op)
			if err != nil {
				return ports.KVEdit{}, fmt.Errorf("operation %d, %s %q: %w", i+1, op.Op, op.Key, err)
			}
			changed = changed || r.Changed
			results = append(results, r)
		}
		if !changed {
			return ports.KVEdit{}, nil
		}
		out, err := kvEncode(doc)
		if err != nil {
			return ports.KVEdit{}, err
		}
		if len(out) > KVMaxValue {
			return ports.KVEdit{}, fmt.Errorf("%w: a value is at most %d bytes", ErrKVLimit, KVMaxValue)
		}
		return ports.KVEdit{Write: true, Value: ports.KVValue{Bytes: out}}, nil
	})
	if err != nil {
		return nil, kvErr(err)
	}
	return results, nil
}

// kvObject decodes a JSON value that must be an object, numbers kept as
// written.
func kvObject(raw []byte) (map[string]any, error) {
	v, err := kvDecode(raw)
	if err != nil {
		return nil, err
	}
	doc, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: a JSON value is an object", ErrKVValue)
	}
	return doc, nil
}

func kvDecode(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("%w: not JSON: %v", ErrKVValue, err)
	}
	if dec.More() {
		return nil, fmt.Errorf("%w: more than one JSON value", ErrKVValue)
	}
	return v, nil
}

// kvEncode is the canonical form: compact, keys sorted, nothing escaped that
// JSON does not require.
func kvEncode(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// applyOp is one operation on doc, which it edits in place.
func applyOp(doc map[string]any, op KVOp) (KVResult, error) {
	r := KVResult{Op: op.Op, Key: op.Key}
	if op.Key == "" {
		return r, fmt.Errorf("%w: every operation names a key", ErrKVValue)
	}
	takesValue := map[string]bool{"set": true, "inc": true, "push": true, "unshift": true, "add_to_set": true, "remove_from_set": true}
	known := takesValue[op.Op] || op.Op == "unset" || op.Op == "shift" || op.Op == "pop"
	if !known {
		return r, fmt.Errorf("%w: no operation %q", ErrKVValue, op.Op)
	}
	var val any
	if takesValue[op.Op] {
		if len(op.Value) == 0 {
			return r, fmt.Errorf("%w: %s takes a value", ErrKVValue, op.Op)
		}
		var err error
		if val, err = kvDecode(op.Value); err != nil {
			return r, err
		}
	} else if len(op.Value) != 0 {
		return r, fmt.Errorf("%w: %s takes no value", ErrKVValue, op.Op)
	}
	cur, has := doc[op.Key]
	array := func() ([]any, error) {
		if !has {
			return nil, nil
		}
		a, ok := cur.([]any)
		if !ok {
			return nil, fmt.Errorf("%w: not an array", ErrKVType)
		}
		return a, nil
	}
	switch op.Op {
	case "set":
		doc[op.Key], r.Changed = val, true
	case "unset":
		if has {
			delete(doc, op.Key)
			r.Changed = true
		}
	case "inc":
		n, ok := val.(json.Number)
		by, err := n.Int64()
		if !ok || err != nil {
			return r, fmt.Errorf("%w: inc adds an integer", ErrKVValue)
		}
		var now int64
		if has {
			c, ok := cur.(json.Number)
			if !ok {
				return r, fmt.Errorf("%w: not a number", ErrKVType)
			}
			if now, err = c.Int64(); err != nil {
				return r, fmt.Errorf("%w: not an integer", ErrKVType)
			}
		}
		sum, fits := addInt(now, by)
		if !fits {
			return r, fmt.Errorf("%w: the sum overflows a 64-bit int", ErrKVLimit)
		}
		doc[op.Key] = json.Number(fmt.Sprint(sum))
		r.Changed, r.Value = true, json.RawMessage(fmt.Sprint(sum))
	case "push", "unshift", "add_to_set":
		a, err := array()
		if err != nil {
			return r, err
		}
		if op.Op == "add_to_set" && contains(a, val) {
			break
		}
		if op.Op == "unshift" {
			a = append([]any{val}, a...)
		} else {
			a = append(a, val)
		}
		doc[op.Key], r.Changed = a, true
	case "shift", "pop":
		a, err := array()
		if err != nil || len(a) == 0 {
			return r, err
		}
		var took any
		if op.Op == "shift" {
			took, a = a[0], a[1:]
		} else {
			took, a = a[len(a)-1], a[:len(a)-1]
		}
		raw, err := kvEncode(took)
		if err != nil {
			return r, err
		}
		doc[op.Key], r.Changed, r.Value = a, true, raw
	case "remove_from_set":
		a, err := array()
		if err != nil {
			return r, err
		}
		kept := make([]any, 0, len(a))
		for _, e := range a {
			if !jsonEqual(e, val) {
				kept = append(kept, e)
			}
		}
		if len(kept) != len(a) {
			doc[op.Key], r.Changed = kept, true
		}
	}
	return r, nil
}

func contains(a []any, v any) bool {
	for _, e := range a {
		if jsonEqual(e, v) {
			return true
		}
	}
	return false
}

// jsonEqual compares two decoded JSON values: key order does not matter, and
// numbers are equal when their values are, however they are spelled.
func jsonEqual(a, b any) bool {
	switch x := a.(type) {
	case json.Number:
		y, ok := b.(json.Number)
		if !ok {
			return false
		}
		p, okp := new(big.Rat).SetString(string(x))
		q, okq := new(big.Rat).SetString(string(y))
		return okp && okq && p.Cmp(q) == 0
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for k, v := range x {
			w, has := y[k]
			if !has || !jsonEqual(v, w) {
				return false
			}
		}
		return true
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !jsonEqual(x[i], y[i]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}

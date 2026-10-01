package bus

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"
	"unicode/utf8"
)

type Record struct {
	Name  string
	bus   *Bus
	KV    KV
	Locks Locks
}

func (r Record) Lookup(ctx context.Context) (json.RawMessage, error) {
	var v json.RawMessage
	e := r.bus.json(ctx, "GET", "/lookup", url.Values{"name": {r.Name}}, nil, &v)
	return v, e
}
func (r Record) Manage(ctx context.Context, changes map[string]any) (json.RawMessage, error) {
	v := map[string]any{"name": r.Name}
	for k, x := range changes {
		if k != "name" {
			v[k] = x
		}
	}
	var out json.RawMessage
	e := r.bus.json(ctx, "POST", "/manage", nil, v, &out)
	return out, e
}
func (r Record) Unregister(ctx context.Context) error {
	return r.bus.json(ctx, "POST", "/unregister", nil, map[string]any{"name": r.Name}, nil)
}
func (r Record) Config(ctx context.Context) (json.RawMessage, error) {
	var v json.RawMessage
	e := r.bus.json(ctx, "GET", "/config", url.Values{"name": {r.Name}}, nil, &v)
	if string(v) == "null" {
		v = nil
	}
	return v, e
}

type Secret struct{ data []byte }

func (s Secret) String() string                { return "[redacted]" }
func (s Secret) GoString() string              { return "Secret([redacted])" }
func (s Secret) Format(f fmt.State, verb rune) { fmt.Fprint(f, "Secret([redacted])") }
func (s Secret) Bytes() []byte                 { return append([]byte(nil), s.data...) }
func (s Secret) MarshalJSON() ([]byte, error)  { return []byte(`"[redacted]"`), nil }
func (r Record) Secret(ctx context.Context) (Secret, error) {
	if e := r.bus.ensure(ctx); e != nil {
		return Secret{}, e
	}
	v, e := r.bus.request(ctx, "GET", "/secret", url.Values{"name": {r.Name}}, nil)
	return Secret{v}, e
}

type KVKind string

const (
	KVString KVKind = "string"
	KVInt    KVKind = "int"
	KVJSON   KVKind = "json"
)

type KVMode string

const (
	KVSet     KVMode = "set"
	KVAdd     KVMode = "add"
	KVReplace KVMode = "replace"
)

type KVValue struct {
	Kind  KVKind
	Bytes []byte
	Int   int64
	JSON  json.RawMessage
}
type KVOp struct {
	Op    string          `json:"op"`
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value,omitempty"`
}
type KVResult struct {
	Changed bool            `json:"changed"`
	Value   json.RawMessage `json:"value"`
}
type KV struct {
	bus    *Bus
	record string
}

func (k KV) Get(ctx context.Context, kind KVKind, key string) (KVValue, error) {
	var v struct {
		Value json.RawMessage `json:"value"`
		B64   *string         `json:"value_base64"`
	}
	e := k.bus.json(ctx, "GET", "/kv", url.Values{"record": {k.record}, "name": {key}, "kind": {string(kind)}}, nil, &v)
	out := KVValue{Kind: kind}
	if e != nil {
		return out, e
	}
	switch kind {
	case KVInt:
		e = json.Unmarshal(v.Value, &out.Int)
	case KVJSON:
		out.JSON = append(json.RawMessage(nil), v.Value...)
	case KVString:
		if v.B64 != nil {
			out.Bytes, e = base64.StdEncoding.DecodeString(*v.B64)
		} else {
			var text string
			e = json.Unmarshal(v.Value, &text)
			out.Bytes = []byte(text)
		}
	default:
		e = errors.New("unknown KV kind")
	}
	return out, e
}
func (k KV) Set(ctx context.Context, kind KVKind, key string, value any, mode KVMode) error {
	if mode != KVSet && mode != KVAdd && mode != KVReplace {
		return errors.New("set mode required")
	}
	v := map[string]any{"record": k.record, "name": key, "kind": kind, "how": mode}
	switch kind {
	case KVString:
		var data []byte
		switch x := value.(type) {
		case []byte:
			data = x
		case string:
			data = []byte(x)
		default:
			return errors.New("string KV requires bytes or string")
		}
		if utf8.Valid(data) {
			v["value"] = string(data)
		} else {
			v["value_base64"] = base64.StdEncoding.EncodeToString(data)
		}
	case KVInt:
		n, ok := value.(int64)
		if !ok {
			return errors.New("integer KV requires int64")
		}
		v["value"] = n
	case KVJSON:
		data, e := json.Marshal(value)
		if e != nil {
			return e
		}
		var object map[string]json.RawMessage
		if e = json.Unmarshal(data, &object); e != nil || object == nil {
			return errors.New("JSON KV requires an object")
		}
		v["value"] = json.RawMessage(data)
	default:
		return errors.New("unknown KV kind")
	}
	return k.bus.json(ctx, "POST", "/kv/set", nil, v, nil)
}
func (k KV) Delete(ctx context.Context, kind KVKind, key string) (bool, error) {
	var v struct {
		Deleted bool `json:"deleted"`
	}
	e := k.bus.json(ctx, "POST", "/kv/delete", nil, map[string]any{"record": k.record, "name": key, "kind": kind}, &v)
	return v.Deleted, e
}
func (k KV) Increment(ctx context.Context, key string, n int64) (int64, error) {
	var v struct {
		Value int64 `json:"value"`
	}
	e := k.bus.json(ctx, "POST", "/kv/inc", nil, map[string]any{"record": k.record, "name": key, "n": n}, &v)
	return v.Value, e
}
func (k KV) JSON(ctx context.Context, key string, ops []KVOp) ([]KVResult, error) {
	if len(ops) > 100 {
		return nil, errors.New("at most 100 JSON operations")
	}
	var v struct {
		Results []KVResult `json:"results"`
	}
	e := k.bus.json(ctx, "POST", "/kv/json", nil, map[string]any{"record": k.record, "name": key, "ops": ops}, &v)
	return v.Results, e
}
func (k KV) List(ctx context.Context) (json.RawMessage, error) {
	var v json.RawMessage
	e := k.bus.json(ctx, "GET", "/kv/list", url.Values{"record": {k.record}}, nil, &v)
	return v, e
}

type Locks struct {
	bus    *Bus
	record string
}
type Lease struct {
	mu      sync.Mutex
	locks   Locks
	Name    string
	expires time.Time
	retired bool
}

func validTTL(ttl time.Duration) error {
	if ttl <= 0 || ttl > 24*time.Hour {
		return errors.New("TTL must be positive and at most 24h")
	}
	return nil
}
func (l Locks) take(ctx context.Context, path, name string, opts ...Option) (*Lease, error) {
	s := config(opts)
	if s.ttl == 0 {
		s.ttl = 30 * time.Second
	}
	if e := validTTL(s.ttl); e != nil {
		return nil, e
	}
	if s.wait < 0 || s.wait > 60*time.Second {
		return nil, errors.New("lock wait capped at 60s")
	}
	start := time.Now()
	e := l.bus.json(ctx, "POST", path, url.Values{"wait": {s.wait.String()}}, map[string]any{"record": l.record, "name": name, "ttl": s.ttl.String()}, nil)
	if e != nil {
		return nil, e
	}
	return &Lease{locks: l, Name: name, expires: start.Add(s.ttl)}, nil
}
func (l Locks) Acquire(ctx context.Context, name string, opts ...Option) (*Lease, error) {
	return l.take(ctx, "/lock", name, opts...)
}
func (l Locks) TryAcquire(ctx context.Context, name string, ttl time.Duration) (*Lease, error) {
	return l.take(ctx, "/try-lock", name, WithTTL(ttl), WithWait(0))
}
func (l Locks) Extend(ctx context.Context, name string, ttl time.Duration) error {
	if e := validTTL(ttl); e != nil {
		return e
	}
	return l.bus.json(ctx, "POST", "/extend", nil, map[string]any{"record": l.record, "name": name, "ttl": ttl.String()}, nil)
}
func (l Locks) Release(ctx context.Context, name string) error {
	return l.bus.json(ctx, "POST", "/release", nil, map[string]any{"record": l.record, "name": name}, nil)
}
func (l Locks) ForceRelease(ctx context.Context, name string) (string, error) {
	var v struct {
		Holder string `json:"holder"`
	}
	e := l.bus.json(ctx, "POST", "/release-force", nil, map[string]any{"record": l.record, "name": name}, &v)
	return v.Holder, e
}
func (l Locks) Holders(ctx context.Context) (json.RawMessage, error) {
	var v json.RawMessage
	e := l.bus.json(ctx, "GET", "/holders", url.Values{"record": {l.record}}, nil, &v)
	return v, e
}
func (l *Lease) live() error {
	if l.retired || !time.Now().Before(l.expires) {
		return errors.New("lease retired or expired")
	}
	return nil
}
func (l *Lease) Release(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e := l.live(); e != nil {
		return e
	}
	l.retired = true
	return l.locks.Release(ctx, l.Name)
}
func (l *Lease) Extend(ctx context.Context, ttl time.Duration) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e := l.live(); e != nil {
		return e
	}
	if e := validTTL(ttl); e != nil {
		return e
	}
	l.retired = true
	start := time.Now()
	if e := l.locks.Extend(ctx, l.Name, ttl); e != nil {
		return e
	}
	l.expires = start.Add(ttl)
	l.retired = false
	return nil
}

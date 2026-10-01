package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Record is a handle binding one named record to the Bus's authenticated
// identity. Its KV, Locks, Config and Secret namespaces share the binding.
type Record struct {
	bus  *Bus
	name string
}

// Record returns a handle for the named record, sharing this Bus.
func (b *Bus) Record(name string) *Record { return &Record{bus: b, name: name} }

// Lookup reads the record this handle binds.
func (r *Record) Lookup(ctx context.Context) (*Status, error) {
	return r.bus.Status(ctx)
}

// ---- KV ----

// KVKind is which of a record's three stores a name is in.
type KVKind string

const (
	KVString KVKind = "string"
	KVInt    KVKind = "int"
	KVJSON   KVKind = "json"
)

type kvValue struct {
	Bytes json.RawMessage `json:"bytes,omitempty"`
	Int   *int64          `json:"int,omitempty"`
}

// KVGet reads one value from the record's store.
func (r *Record) KVGet(ctx context.Context, kind KVKind, name string) (json.RawMessage, error) {
	b, code, err := r.bus.call(ctx, "GET", "/kv", map[string]string{"kind": string(kind), "name": name}, nil)
	if err != nil {
		return nil, err
	}
	if code >= 400 {
		return nil, busError(code, b)
	}
	return json.RawMessage(b), nil
}

// KVSet writes one value: mode is set, add or replace.
func (r *Record) KVSet(ctx context.Context, kind KVKind, name string, value json.RawMessage, mode string) error {
	_, code, err := r.bus.call(ctx, "POST", "/kv/set",
		map[string]string{"kind": string(kind), "name": name},
		map[string]any{"record": r.name, "kind": kind, "name": name, "value": value, "how": mode})
	if err != nil {
		return err
	}
	if code >= 400 {
		return busError(code, nil)
	}
	return nil
}

// KVDelete removes one name.
func (r *Record) KVDelete(ctx context.Context, kind KVKind, name string) error {
	_, code, err := r.bus.call(ctx, "POST", "/kv/delete",
		map[string]string{"kind": string(kind), "name": name},
		map[string]any{"record": r.name, "kind": kind, "name": name})
	if err != nil {
		return err
	}
	if code >= 400 {
		return busError(code, nil)
	}
	return nil
}

// KVIncrement adds n to an integer, returning the new value.
func (r *Record) KVIncrement(ctx context.Context, name string, n int64) (int64, error) {
	b, code, err := r.bus.call(ctx, "POST", "/kv/inc",
		map[string]string{"kind": "int", "name": name},
		map[string]any{"record": r.name, "kind": "int", "name": name, "value": n})
	if err != nil {
		return 0, err
	}
	if code >= 400 {
		return 0, busError(code, b)
	}
	var out struct {
		Value int64 `json:"value"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return 0, err
	}
	return out.Value, nil
}

// ---- Locks ----

func (r *Record) lockTake(ctx context.Context, name, ttl, wait string) error {
	_, code, err := r.bus.call(ctx, "POST", "/lock",
		map[string]string{"record": r.name, "name": name},
		map[string]any{"record": r.name, "name": name, "ttl": ttl})
	_ = wait
	if err != nil {
		return err
	}
	if code >= 400 {
		return busError(code, nil)
	}
	return nil
}

// LocksAcquire takes a named lock, refusing if held.
func (r *Record) LocksAcquire(ctx context.Context, name string, ttl time.Duration) error {
	return r.lockTake(ctx, name, durationString(ttl), "")
}

// LocksRelease gives a lock back.
func (r *Record) LocksRelease(ctx context.Context, name string) error {
	_, code, err := r.bus.call(ctx, "POST", "/release",
		map[string]string{"record": r.name, "name": name},
		map[string]any{"record": r.name, "name": name})
	if err != nil {
		return err
	}
	if code >= 400 {
		return busError(code, nil)
	}
	return nil
}

// ---- Config and Secret ----

// Config reads the record's stored configuration.
func (r *Record) Config(ctx context.Context) (json.RawMessage, error) {
	b, code, err := r.bus.call(ctx, "GET", "/config?name="+r.name, nil, nil)
	if err != nil {
		return nil, err
	}
	if code >= 400 {
		return nil, busError(code, b)
	}
	return json.RawMessage(b), nil
}

// Secret reads the record's stored secret.
func (r *Record) Secret(ctx context.Context) (string, error) {
	b, code, err := r.bus.call(ctx, "GET", "/secret?name="+r.name, nil, nil)
	if err != nil {
		return "", err
	}
	if code >= 400 {
		return "", busError(code, b)
	}
	return string(b), nil
}

// ---- helpers ----

func busError(code int, body []byte) error {
	msg := ""
	if len(body) > 0 {
		var e struct{ Error string }
		if json.Unmarshal(body, &e) == nil {
			msg = e.Error
		}
	}
	return fmt.Errorf("agent-bus: %d %s", code, msg)
}

func durationString(d time.Duration) string {
	return d.String()
}

var _ = http.StatusOK
var _ = context.Background
var _ = fmt.Sprintf

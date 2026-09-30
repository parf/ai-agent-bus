package memory

import (
	"fmt"
	"sort"

	"github.com/parf/ai-agent-bus/internal/ports"
)

// Each record's key-value store, as the database keeps it: by kind, record ID
// and name (Plans/R1.0-Release/kv.md#per-record-storage).

type kvKey struct {
	kind   ports.KVKind
	record uint32
	name   string
}

func knownKind(kind ports.KVKind) error {
	switch kind {
	case ports.KVString, ports.KVInt, ports.KVJSON:
		return nil
	}
	return fmt.Errorf("no key-value kind %q", kind)
}

// dropKV removes every value of one record. Caller holds s.mu.
func (s *State) dropKV(record uint32) {
	for k := range s.kv {
		if k.record == record {
			delete(s.kv, k)
		}
	}
}

func (s *State) holds(record uint32) bool {
	for _, r := range s.records {
		if r.ID == record {
			return true
		}
	}
	return false
}

func cloneKV(v ports.KVValue) ports.KVValue {
	if v.Bytes != nil {
		v.Bytes = append([]byte{}, v.Bytes...)
	}
	return v
}

func (s *State) KVGet(kind ports.KVKind, record uint32, name string) (ports.KVValue, bool, error) {
	if err := knownKind(kind); err != nil {
		return ports.KVValue{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, found := s.kv[kvKey{kind, record, name}]
	return cloneKV(v), found, nil
}

// KVEdit holds s.mu across the read, the edit and the write, as the
// database's one transaction does.
func (s *State) KVEdit(kind ports.KVKind, record uint32, name string, edit func(ports.KVValue, bool, int) (ports.KVEdit, error)) error {
	if err := knownKind(kind); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	if !s.holds(record) {
		return ports.ErrKVNoRecord
	}
	k := kvKey{kind, record, name}
	old, found := s.kv[k]
	names := 0
	for o := range s.kv {
		if o.kind == kind && o.record == record {
			names++
		}
	}
	e, err := edit(cloneKV(old), found, names)
	if err != nil {
		return err
	}
	switch {
	case e.Delete:
		delete(s.kv, k)
	case e.Write:
		s.kv[k] = cloneKV(e.Value)
	}
	return nil
}

func (s *State) KVOrphans() ([]uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[uint32]bool{}
	var out []uint32
	for k := range s.kv {
		if !seen[k.record] && !s.holds(k.record) {
			seen[k.record] = true
			out = append(out, k.record)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

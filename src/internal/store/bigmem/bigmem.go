// Package bigmem is a memory-only BigStore for core unit tests.
// Production and client conformance use package bigfile, which supports both
// file-backed values and explicitly requested memory values.
package bigmem

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
)

type key struct {
	record uint32
	name   string
}

type value struct {
	data []byte
	info ports.BigInfo
}

// Store is safe for concurrent use.
type Store struct {
	mu   sync.Mutex
	keys map[key]value
}

func New() *Store { return &Store{keys: map[key]value{}} }

// expire removes every expired value. Caller holds s.mu.
func (s *Store) expire(now time.Time) {
	for k, v := range s.keys {
		if !now.Before(v.info.Expires) {
			delete(s.keys, k)
		}
	}
}

func (s *Store) BigPut(record uint32, name string, data io.Reader, sum string, memory bool, expires, now time.Time, lim ports.BigLimits) (ports.BigInfo, bool, error) {
	buf, err := io.ReadAll(io.LimitReader(data, lim.Value+1))
	if err != nil {
		return ports.BigInfo{}, false, err
	}
	switch {
	case int64(len(buf)) > lim.Value:
		return ports.BigInfo{}, false, ports.ErrBigTooLarge
	case len(buf) == 0:
		return ports.BigInfo{}, false, ports.ErrBigEmpty
	}
	h := sha256.Sum256(buf)
	hexsum := hex.EncodeToString(h[:])
	if sum != "" && sum != hexsum {
		return ports.BigInfo{}, false, ports.ErrBigHash
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire(now)
	var inRecord, total int64
	names := 0
	for k, v := range s.keys {
		if k.record == record && k.name == name {
			continue // replaced by this put
		}
		total += v.info.Size
		if k.record == record {
			inRecord += v.info.Size
			names++
		}
	}
	size := int64(len(buf))
	switch {
	case names+1 > lim.Names:
		return ports.BigInfo{}, false, fmt.Errorf("%w: a record holds at most %d bigfiles", ports.ErrBigFull, lim.Names)
	case inRecord+size > lim.Record:
		return ports.BigInfo{}, false, fmt.Errorf("%w: a record holds at most %d bytes of bigfiles", ports.ErrBigFull, lim.Record)
	case total+size > lim.Total:
		return ports.BigInfo{}, false, fmt.Errorf("%w: the daemon holds at most %d bytes of bigfiles", ports.ErrBigFull, lim.Total)
	}
	_, replaced := s.keys[key{record, name}]
	info := ports.BigInfo{Name: name, Size: size, SHA256: hexsum, Held: "memory", Expires: expires}
	s.keys[key{record, name}] = value{buf, info}
	return info, replaced, nil
}

func (s *Store) BigGet(record uint32, name string, now time.Time) (io.ReadCloser, ports.BigInfo, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire(now)
	v, ok := s.keys[key{record, name}]
	if !ok {
		return nil, ports.BigInfo{}, false, nil
	}
	return io.NopCloser(bytes.NewReader(v.data)), v.info, true, nil
}

func (s *Store) BigList(record uint32, now time.Time) ([]ports.BigInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire(now)
	out := []ports.BigInfo{}
	for k, v := range s.keys {
		if k.record == record {
			out = append(out, v.info)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Store) BigDelete(record uint32, name string, now time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire(now)
	_, ok := s.keys[key{record, name}]
	delete(s.keys, key{record, name})
	return ok, nil
}

func (s *Store) BigDrop(record uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.keys {
		if k.record == record {
			delete(s.keys, k)
		}
	}
	return nil
}

package bigfile

import (
	"sort"
	"sync"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
)

// MemoryIndex is a ports.BigIndex that lives in memory: the index for a test,
// or for a store that has no database. It keeps nothing across a restart of
// the process, so it is never the daemon's.
type MemoryIndex struct {
	mu   sync.Mutex
	rows map[key]ports.BigRow
}

func NewMemoryIndex() *MemoryIndex { return &MemoryIndex{rows: map[key]ports.BigRow{}} }

func (m *MemoryIndex) sorted(keep func(ports.BigRow) bool) []ports.BigRow {
	var out []ports.BigRow
	for _, r := range m.rows {
		if keep(r) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Record != out[j].Record {
			return out[i].Record < out[j].Record
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (m *MemoryIndex) BigRows() ([]ports.BigRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sorted(func(ports.BigRow) bool { return true }), nil
}

func (m *MemoryIndex) BigRowsOf(record uint32) ([]ports.BigRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sorted(func(r ports.BigRow) bool { return r.Record == record }), nil
}

func (m *MemoryIndex) BigRowGet(record uint32, name string) (ports.BigRow, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[key{record, name}]
	return r, ok, nil
}

func (m *MemoryIndex) BigRowPut(row ports.BigRow) (ports.BigRow, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.rows[key{row.Record, row.Name}]
	m.rows[key{row.Record, row.Name}] = row
	return old, ok, nil
}

func (m *MemoryIndex) BigRowDelete(record uint32, name string) (ports.BigRow, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.rows[key{record, name}]
	delete(m.rows, key{record, name})
	return old, ok, nil
}

func (m *MemoryIndex) drop(keep func(ports.BigRow) bool) []ports.BigRow {
	out := m.sorted(keep)
	for _, r := range out {
		delete(m.rows, key{r.Record, r.Name})
	}
	return out
}

func (m *MemoryIndex) BigRowsDropRecord(record uint32) ([]ports.BigRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.drop(func(r ports.BigRow) bool { return r.Record == record }), nil
}

func (m *MemoryIndex) BigRowsDropExpired(now time.Time) ([]ports.BigRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.drop(func(r ports.BigRow) bool { return !now.Before(r.Expires) }), nil
}

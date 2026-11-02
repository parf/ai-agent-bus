package memory

import (
	"fmt"

	"github.com/parf/ai-agent-bus/internal/ports"
)

// eventLog is one record's alert snapshot and whether its log exists.
type eventLog struct {
	alerts []ports.EventEntry
	active bool
}

// LoadEventAlerts returns a copy of the record's last stored snapshot.
func (s *State) LoadEventAlerts(id uint32) ([]ports.EventEntry, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return nil, false, s.Err
	}
	l := s.eventLogs[id]
	return append([]ports.EventEntry(nil), l.alerts...), l.active, nil
}

// ReplaceEventAlerts stores the whole snapshot; a refused write changes nothing.
func (s *State) ReplaceEventAlerts(id uint32, alerts []ports.EventEntry, active bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	if id == 0 || len(alerts) > 500 {
		return fmt.Errorf("invalid event alert snapshot")
	}
	var last uint64
	for _, e := range alerts {
		if e.Level != "alert" || e.ID == 0 || e.ID <= last || e.Count == 0 {
			return fmt.Errorf("invalid event alert snapshot")
		}
		last = e.ID
	}
	if s.eventLogs == nil {
		s.eventLogs = map[uint32]eventLog{}
	}
	s.eventLogs[id] = eventLog{alerts: append([]ports.EventEntry(nil), alerts...), active: active}
	return nil
}

// dropEventLog removes one record's alerts. Caller holds s.mu.
func (s *State) dropEventLog(id uint32) {
	delete(s.eventLogs, id)
	delete(s.eventHigh, id)
}

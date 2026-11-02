package memory

import (
	"context"
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
	return s.ReplaceEventAlertBatch(context.Background(), []ports.EventAlertSnapshot{{RecordID: id, Alerts: alerts, Active: active}})
}
func (s *State) ReplaceEventAlertBatch(ctx context.Context, batch []ports.EventAlertSnapshot) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	for _, v := range batch {
		if v.RecordID == 0 || len(v.Alerts) > 500 {
			return fmt.Errorf("invalid event alert snapshot")
		}
		var last uint64
		for _, e := range v.Alerts {
			if e.Level != "alert" || e.ID == 0 || e.ID <= last || e.Count == 0 || e.ID > ports.EventMaxID || e.Count > ports.EventMaxID {
				return fmt.Errorf("invalid event alert snapshot")
			}
			last = e.ID
		}
	}
	if s.eventLogs == nil {
		s.eventLogs = map[uint32]eventLog{}
	}
	for _, v := range batch {
		s.eventLogs[v.RecordID] = eventLog{alerts: append([]ports.EventEntry(nil), v.Alerts...), active: v.Active}
	}
	return nil
}

// dropEventLog removes one record's alerts. Caller holds s.mu.
func (s *State) dropEventLog(id uint32) {
	delete(s.eventLogs, id)
	delete(s.eventHigh, id)
}

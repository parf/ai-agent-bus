package memory

import (
	"fmt"
	"github.com/parf/ai-agent-bus/internal/ports"
)

func (s *State) EventHighWater(id uint32) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return 0, s.Err
	}
	return s.eventHigh[id], nil
}
func (s *State) ReserveEventIDs(id uint32, through uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	if id == 0 || through == 0 || through > ports.EventMaxID || through < s.eventHigh[id] {
		return fmt.Errorf("invalid event ID reservation")
	}
	if s.eventHigh == nil {
		s.eventHigh = map[uint32]uint64{}
	}
	s.eventHigh[id] = through
	return nil
}

func (s *State) DropEventState(id uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	delete(s.eventHigh, id)
	s.dropSubscriptions(id)
	return nil
}

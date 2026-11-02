package memory

import (
	"encoding/json"
	"errors"
	"sort"

	"github.com/parf/ai-agent-bus/internal/ports"
)

// memorySub is one stored subscription: the source's record ID and the stored JSON.
type memorySub struct {
	source uint32
	body   []byte
}

func (s *State) ListSubscriptions(sourceID uint32) (map[string][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return nil, s.Err
	}
	out := map[string][]byte{}
	for id, sub := range s.subs {
		if sub.source == sourceID {
			out[id] = append([]byte(nil), sub.body...)
		}
	}
	return out, nil
}

func (s *State) AddSubscription(sourceID uint32, id string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	if sourceID == 0 || id == "" {
		return errors.New("invalid subscription")
	}
	if _, taken := s.subs[id]; taken {
		return ports.ErrSubscriptionExists
	}
	if s.subs == nil {
		s.subs = map[string]memorySub{}
	}
	s.subs[id] = memorySub{source: sourceID, body: append([]byte(nil), value...)}
	return nil
}

func (s *State) DeleteSubscription(sourceID uint32, id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return false, s.Err
	}
	sub, ok := s.subs[id]
	if !ok || sub.source != sourceID {
		return false, nil
	}
	delete(s.subs, id)
	return true, nil
}

func (s *State) SetSubscriptionCounts(sourceID uint32, id string, sent, dropped int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	sub, ok := s.subs[id]
	if !ok || sub.source != sourceID {
		return errors.New("absent")
	}
	var m map[string]any
	if err := json.Unmarshal(sub.body, &m); err != nil {
		return err
	}
	m["sent"], m["dropped"] = sent, dropped
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	s.subs[id] = memorySub{source: sourceID, body: body}
	return nil
}

func (s *State) FindSubscription(id string) (uint32, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return 0, false, s.Err
	}
	sub, ok := s.subs[id]
	return sub.source, ok, nil
}

func (s *State) SubscriptionSources(creator string) ([]uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return nil, s.Err
	}
	seen := map[uint32]bool{}
	for _, sub := range s.subs {
		var row struct {
			CreatedBy string `json:"created_by"`
		}
		if json.Unmarshal(sub.body, &row) == nil && row.CreatedBy == creator {
			seen[sub.source] = true
		}
	}
	out := make([]uint32, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// dropSubscriptions removes a retired source's subscriptions. Caller holds s.mu.
func (s *State) dropSubscriptions(source uint32) {
	for id, sub := range s.subs {
		if sub.source == source {
			delete(s.subs, id)
		}
	}
}

// dropSubscriptionsBy removes what a removed principal authored. Caller holds s.mu.
func (s *State) dropSubscriptionsBy(creator string) {
	for id, sub := range s.subs {
		var row struct {
			CreatedBy string `json:"created_by"`
		}
		if json.Unmarshal(sub.body, &row) == nil && row.CreatedBy == creator {
			delete(s.subs, id)
		}
	}
}

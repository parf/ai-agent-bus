package memory

import "github.com/parf/ai-agent-bus/internal/ports"

func (s *State) ExportState(ids []uint32, names []string) (ports.TransferState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens.mu.Lock()
	defer s.tokens.mu.Unlock()
	var out ports.TransferState
	selected := map[uint32]bool{}
	for _, id := range ids {
		selected[id] = true
	}
	for k, v := range s.kv {
		if selected[k.record] {
			out.KV = append(out.KV, ports.StoredKV{RecordID: k.record, Kind: k.kind, Name: k.name, Value: cloneKV(v)})
		}
	}
	for _, name := range names {
		if c, ok := s.tokens.creds[name]; ok {
			out.Credentials = append(out.Credentials, c)
		}
	}
	return out, nil
}

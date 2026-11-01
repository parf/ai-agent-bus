package bus

import (
	"encoding/json"
	"fmt"
	"log"
)

const ClientProtocol = 1

func checkProtocol(data []byte) (bool, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return false, fmt.Errorf("protocol error: identity must be an object")
	}
	values := [2]int{}
	for i, field := range []string{"protocol", "protocol_min"} {
		raw, found := fields[field]
		if found && (json.Unmarshal(raw, &values[i]) != nil || string(raw) == "null" || values[i] < 0) {
			return false, fmt.Errorf("protocol error: %s must be a nonnegative integer", field)
		}
	}
	if values[1] > values[0] {
		return false, fmt.Errorf("protocol error: protocol_min exceeds protocol")
	}
	if ClientProtocol < values[1] {
		return false, fmt.Errorf("daemon too new for this client")
	}
	return values[0] < ClientProtocol, nil
}
func (b *Bus) checkProtocol(data []byte) error {
	old, err := checkProtocol(data)
	if err != nil {
		return err
	}
	b.mu.Lock()
	warn := old && !b.protocolWarned
	b.daemonOld = old
	b.protocolWarned = b.protocolWarned || old
	b.mu.Unlock()
	if warn {
		log.Print("daemon too old: protocol is older than this client")
	}
	return nil
}
func (b *Bus) refusal(status int, data []byte, operation string) error {
	var v struct {
		Error string `json:"error"`
		Kind  string `json:"kind"`
	}
	_ = json.Unmarshal(data, &v)
	if v.Error == "" {
		v.Error = string(data)
	}
	if status == 500 {
		v.Error = "internal daemon error"
		v.Kind = ""
	}
	b.mu.Lock()
	old := b.daemonOld
	b.mu.Unlock()
	if old && v.Kind == "no_route" {
		v.Error = "daemon too old for " + operation
	}
	return &BusError{Status: status, Message: v.Error, Kind: v.Kind}
}

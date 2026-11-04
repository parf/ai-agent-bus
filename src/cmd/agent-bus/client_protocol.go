package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

const clientProtocol = 1

type protocolKey struct {
	client      *http.Client
	base, token string
}
type protocolResult struct {
	done chan struct{}
	err  error
}

var protocolChecks = struct {
	sync.Mutex
	entries map[protocolKey]*protocolResult
}{entries: make(map[protocolKey]*protocolResult)}

func checkClientProtocol(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return fmt.Errorf("protocol error: identity must be an object")
	}
	values := [2]int{}
	for i, field := range []string{"protocol", "protocol_min"} {
		if raw, ok := fields[field]; !ok || json.Unmarshal(raw, &values[i]) != nil || string(raw) == "null" || values[i] < 0 {
			return fmt.Errorf("protocol error: %s must be a nonnegative integer", field)
		}
	}
	if values[1] > values[0] {
		return fmt.Errorf("protocol error: protocol_min exceeds protocol")
	}
	if clientProtocol < values[1] {
		return fmt.Errorf("daemon too new for this client")
	}
	if values[0] != clientProtocol || values[1] != clientProtocol {
		return fmt.Errorf("daemon protocol is unsupported")
	}
	return nil
}

func ensureDaemonProtocol(ctx context.Context) error {
	client, base := transport()
	key := protocolKey{client, base, os.Getenv("AGENT_BUS_TOKEN")}
	protocolChecks.Lock()
	if state := protocolChecks.entries[key]; state != nil {
		protocolChecks.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-state.done:
			return state.err
		}
	}
	state := &protocolResult{done: make(chan struct{})}
	protocolChecks.entries[key] = state
	protocolChecks.Unlock()
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	data, code, err := callRawCtx(bounded, "GET", "/identity", nil, nil)
	if err == nil && code >= 400 {
		err = fmt.Errorf("identity refused (%d): %s", code, data)
	}
	if err == nil {
		err = checkClientProtocol(data)
	}
	state.err = err
	protocolChecks.Lock()
	if err != nil {
		delete(protocolChecks.entries, key)
	}
	close(state.done)
	protocolChecks.Unlock()
	return err
}

func protocolRefusal(data []byte, code int) []byte {
	if code == 500 {
		return []byte(`{"error":"internal daemon error"}`)
	}
	return data
}

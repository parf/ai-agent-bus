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
	old  bool
	err  error
}

var protocolChecks = struct {
	sync.Mutex
	entries map[protocolKey]*protocolResult
}{entries: make(map[protocolKey]*protocolResult)}

func checkClientProtocol(data []byte) (bool, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return false, fmt.Errorf("protocol error: identity must be an object")
	}
	values := [2]int{}
	for i, field := range []string{"protocol", "protocol_min"} {
		if raw, ok := fields[field]; ok && (json.Unmarshal(raw, &values[i]) != nil || string(raw) == "null" || values[i] < 0) {
			return false, fmt.Errorf("protocol error: %s must be a nonnegative integer", field)
		}
	}
	if values[1] > values[0] {
		return false, fmt.Errorf("protocol error: protocol_min exceeds protocol")
	}
	if clientProtocol < values[1] {
		return false, fmt.Errorf("daemon too new for this client")
	}
	return values[0] < clientProtocol, nil
}

func ensureDaemonProtocol(ctx context.Context) (bool, error) {
	client, base := transport()
	key := protocolKey{client, base, os.Getenv("AGENT_BUS_TOKEN")}
	protocolChecks.Lock()
	if state := protocolChecks.entries[key]; state != nil {
		protocolChecks.Unlock()
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-state.done:
			return state.old, state.err
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
		state.old, err = checkClientProtocol(data)
	}
	state.err = err
	if err == nil && state.old {
		fmt.Fprintln(os.Stderr, "daemon too old: protocol is older than this client")
	}
	protocolChecks.Lock()
	if err != nil {
		delete(protocolChecks.entries, key)
	}
	close(state.done)
	protocolChecks.Unlock()
	return state.old, err
}

func protocolRefusal(data []byte, code int, old bool, operation string) []byte {
	if code < 400 {
		return data
	}
	if code == 500 {
		return []byte(`{"error":"internal daemon error"}`)
	}
	var row map[string]any
	if json.Unmarshal(data, &row) != nil {
		return data
	}
	if code != 500 && !(old && row["kind"] == "no_route") {
		return data
	}
	if code == 500 {
		row = map[string]any{"error": "internal daemon error"}
	} else if old && row["kind"] == "no_route" {
		row["error"] = "daemon too old for " + operation
	}
	out, err := json.Marshal(row)
	if err != nil {
		return data
	}
	return out
}

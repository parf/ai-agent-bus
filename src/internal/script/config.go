package script

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// ConfigEnv replaces any inherited value with this start's stored configuration.
// The API answers null for an unconfigured record; {} is a configured value.
func ConfigEnv(env []string, raw []byte) ([]string, error) {
	out := make([]string, 0, len(env)+1)
	for _, value := range env {
		if !strings.HasPrefix(value, "AGENT_BUS_CONFIG=") {
			out = append(out, value)
		}
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return out, nil
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return nil, fmt.Errorf("configuration is not JSON: %w", err)
	}
	return append(out, "AGENT_BUS_CONFIG="+compact.String()), nil
}

package agentmeta

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ConfigWarnings names undeclared keys and enum mismatches, never refuses or
// repeats a configured value. Descriptions do not replace agent validation.
func (m Manifest) ConfigWarnings(env []string) []string {
	var values map[string]json.RawMessage
	for _, kv := range env {
		if strings.HasPrefix(kv, "AGENT_BUS_CONFIG=") {
			_ = json.Unmarshal([]byte(strings.TrimPrefix(kv, "AGENT_BUS_CONFIG=")), &values)
		}
	}
	var out []string
	for _, name := range keys(values) {
		c, ok := m.Config[name]
		if !ok {
			out = append(out, fmt.Sprintf("Configuration key %q is not described in agent.json", name))
			continue
		}
		kind, choices, _ := c.Type.Values()
		if kind != "enum" {
			continue
		}
		var value string
		valid := false
		if json.Unmarshal(values[name], &value) == nil {
			for _, choice := range choices {
				if value == choice {
					valid = true
				}
			}
		}
		if !valid {
			out = append(out, fmt.Sprintf("Configuration key %q is outside its declared enum", name))
		}
	}
	return out
}

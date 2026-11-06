package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// sessionKeys are what a launcher's session file decides for the CLI: who is
// calling and which bus it reaches.
var sessionKeys = []string{"AGENT_BUS_TOKEN", "AGENT_BUS_NAME", "AGENT_BUS_ADDR", "AGENT_BUS_TLS_FINGERPRINT"}

// loadSessionFile: a launcher session's runtime carries AGENT_BUS_SESSION_FILE,
// the private file the launcher rewrites on a rename, while the environment it
// started with keeps the old identity. When the variable is set, the file alone
// decides: a key it lacks is unset, and a file that cannot be read refuses the
// call rather than fall back to that stale identity
// (docs/06-launchers.md#the-sessions-shell-after-a-rename).
func loadSessionFile() error {
	path := os.Getenv("AGENT_BUS_SESSION_FILE")
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("AGENT_BUS_SESSION_FILE: %v; the launcher session may have ended (unset the variable to use the environment's identity)", err)
	}
	var env map[string]any
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("AGENT_BUS_SESSION_FILE %s is not a session file: %v", path, err)
	}
	for _, key := range sessionKeys {
		if v, ok := env[key].(string); ok && v != "" {
			os.Setenv(key, v)
		} else {
			os.Unsetenv(key)
		}
	}
	return nil
}

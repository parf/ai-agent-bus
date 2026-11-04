package protocol

import (
	"encoding/json"
	"testing"
)

func TestMaintainersRequireArray(t *testing.T) {
	for _, raw := range []string{`"@ops"`, `""`, `null`, `true`, `7`, `{}`} {
		t.Run(raw, func(t *testing.T) {
			var list MaintainerList
			if err := json.Unmarshal([]byte(raw), &list); err == nil {
				t.Fatalf("non-array maintainers accepted: %s", raw)
			}
		})
	}
	for _, raw := range []string{`[]`, `["@ops"]`} {
		var list MaintainerList
		if err := json.Unmarshal([]byte(raw), &list); err != nil {
			t.Fatalf("array refused: %s: %v", raw, err)
		}
	}
}

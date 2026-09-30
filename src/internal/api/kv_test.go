package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// A value at its limit crosses the API however it escapes: every byte a
// control character is six on the wire.
func TestAKVValueAtItsLimitCrossesTheAPI(t *testing.T) {
	b := core.New()
	b.Persistence(memory.NewState())
	s, tok := serverFor(t, b, "admin@h")
	if _, err := b.Register(protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "admin@h"}); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"record": "jobs@h", "name": "big", "value": strings.Repeat("\x00", core.KVMaxValue)})
	if code, out := send(s, tok("admin@h"), "POST", "/kv/set", string(body), ""); code != http.StatusOK {
		t.Fatalf("a %d-byte value in a %d-byte body: %d %.200s", core.KVMaxValue, len(body), code, out)
	}
	over, _ := json.Marshal(map[string]string{"record": "jobs@h", "name": "big", "value": strings.Repeat("\x00", core.KVMaxValue+1)})
	if code, _ := send(s, tok("admin@h"), "POST", "/kv/set", string(over), ""); code != http.StatusRequestEntityTooLarge {
		t.Fatalf("a value over the limit answered %d", code)
	}
}

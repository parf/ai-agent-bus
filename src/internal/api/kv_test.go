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

// /kv/list answers a record's store, a string that is not UTF-8 as base64
// and one cut mid-character still as text; with no record, every store the
// caller may use.
func TestKVListCrossesTheAPI(t *testing.T) {
	b := core.New()
	b.Persistence(memory.NewState())
	s, tok := serverFor(t, b, "admin@h")
	if _, err := b.Register(protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "admin@h"}); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`{"record":"jobs@h","name":"bin","value_base64":"AP8="}`,
		`{"record":"jobs@h","name":"text","value":"a` + strings.Repeat("é", 150) + `"}`, // the cut at 200 bytes splits an é
		`{"record":"jobs@h","kind":"int","name":"n","value":5}`,
	} {
		if code, out := send(s, tok("admin@h"), "POST", "/kv/set", body, ""); code != http.StatusOK {
			t.Fatalf("%s: %d %s", body, code, out)
		}
	}
	code, out := send(s, tok("admin@h"), "GET", "/kv/list?record=jobs@h", "", "")
	if code != http.StatusOK {
		t.Fatalf("%d %s", code, out)
	}
	var got struct {
		Kind   string `json:"kind"`
		Values map[string][]struct {
			Name          string          `json:"name"`
			PreviewBase64 string          `json:"preview_base64"`
			Size          int             `json:"size"`
			Preview       json.RawMessage `json:"preview"`
		} `json:"values"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	str := got.Values["string"]
	if got.Kind != "queue" || len(str) != 2 || str[0].Name != "bin" || str[0].PreviewBase64 != "AP8=" || str[1].Size != 301 || str[1].PreviewBase64 != "" {
		t.Fatalf("strings %+v", got)
	}
	var text string
	if json.Unmarshal(str[1].Preview, &text); text != "a"+strings.Repeat("é", 99) {
		t.Fatalf("a preview cut mid-character came back as %q", text)
	}
	if ints := got.Values["int"]; len(ints) != 1 || string(ints[0].Preview) != "5" {
		t.Fatalf("ints %+v", ints)
	}
	code, out = send(s, tok("admin@h"), "GET", "/kv/list", "", "")
	if code != http.StatusOK || !strings.Contains(out, `"record":"jobs@h","kind":"queue","string":2,"int":1,"json":0`) {
		t.Fatalf("every store: %d %s", code, out)
	}
}

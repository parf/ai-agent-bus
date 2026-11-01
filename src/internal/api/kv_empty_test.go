package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
	"github.com/parf/ai-agent-bus/internal/store/sqlite"
)

func TestEmptyBase64KVWriteMatchesOrdinaryEmptyBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bus.db")
	store, err := sqlite.Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	b := core.New()
	b.Persistence(store)
	s, tok := serverFor(t, b, "admin@h")
	if _, err = b.Register(protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "admin@h"}); err != nil {
		t.Fatal(err)
	}
	if err = b.KVSet("admin@h", "jobs@h", ports.KVString, "ordinary", ports.KVValue{Bytes: []byte{}}, core.KVSet); err != nil {
		t.Fatal(err)
	}
	if code, out := send(s, tok("admin@h"), "POST", "/kv/set", `{"record":"jobs@h","kind":"string","name":"api","value_base64":""}`, ""); code != http.StatusOK {
		t.Fatalf("empty base64 set = %d %s, want 200", code, out)
	}
	code, out := send(s, tok("admin@h"), "GET", "/kv?record=jobs@h&kind=string&name=api", "", "")
	var got struct {
		Value *string `json:"value"`
	}
	if err = json.Unmarshal([]byte(out), &got); err != nil || code != http.StatusOK || got.Value == nil || *got.Value != "" {
		t.Fatalf("empty bytes get = %d %s (%v)", code, out, err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, name := range []string{"ordinary", "api"} {
		var typ string
		var size int
		if err = db.QueryRow("SELECT typeof(value),length(value) FROM kv WHERE name=?", name).Scan(&typ, &size); err != nil {
			t.Fatal(err)
		}
		if typ != "blob" || size != 0 {
			t.Errorf("%s storage = %s length %d, want empty blob", name, typ, size)
		}
	}
}

func TestKVValuePresenceAndDuplicateKeys(t *testing.T) {
	b := core.New()
	b.Persistence(memory.NewState())
	s, tok := serverFor(t, b, "admin@h")
	if _, err := b.Register(protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "admin@h"}); err != nil {
		t.Fatal(err)
	}
	for _, members := range []string{
		``, `,"value_base64":null`, `,"value_base64":0`, `,"value_base64":false`,
		`,"value":"","value_base64":""`, `,"kind":"int","value_base64":""`,
		`,"value_base64":"","value_base64":""`, `,"value":"","value":""`,
		`,"value":null`,
	} {
		body := `{"record":"jobs@h","name":"missing"` + members + `}`
		code, out := send(s, tok("admin@h"), "POST", "/kv/set", body, "")
		var got struct {
			Kind  string `json:"kind"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal([]byte(out), &got); err != nil || code != http.StatusBadRequest || got.Kind != "malformed" {
			t.Errorf("members %s = %d %s, want malformed", members, code, out)
		}
		want := "value"
		if strings.Contains(members, "value_base64") {
			want = "value_base64"
		}
		if !strings.Contains(got.Error, want) {
			t.Errorf("refusal %q does not name %s", got.Error, want)
		}
	}
}

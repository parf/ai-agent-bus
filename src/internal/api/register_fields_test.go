package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Registration refuses the fields it does not write — never stores and
// ignores them — and its answer carries the stored created_at (K.34).
func TestRegistrationRefusesWhatItDoesNotWrite(t *testing.T) {
	bus := core.New()
	s, tok := serverFor(t, bus, "owner@h")
	if _, err := bus.SetUser("owner@h", protocol.User{Name: "alice@h"}, true); err != nil {
		t.Fatal(err)
	}
	alice := tok("alice@h")
	for field, body := range map[string]string{
		"status":     `{"kind":"queue","name":"q@h","status":"inactive"}`,
		"owner":      `{"kind":"queue","name":"q@h","owner":"owner@h"}`,
		"queued":     `{"kind":"queue","name":"q@h","queued":5}`,
		"in":         `{"kind":"queue","name":"q@h","in":3}`,
		"created_at": `{"kind":"queue","name":"q@h","created_at":"2001-01-01T00:00:00Z"}`,
		"secret":     `{"kind":"agent","name":"#a@h","secret":"K=V"}`,
	} {
		if code, body := send(s, alice, "POST", "/register", body, ""); code != http.StatusBadRequest && code != http.StatusForbidden {
			t.Errorf("a registration stating %s answered %d %s", field, code, body)
		}
	}
	if _, known := bus.Lookup("alice@h", "q@h"); known {
		t.Fatal("a refused registration stored the record")
	}
	// Her own name as owner is no claim: it is who she is.
	code, answer := send(s, alice, "POST", "/register", `{"kind":"queue","name":"q@h","owner":"alice@h"}`, "")
	if code != http.StatusOK {
		t.Fatalf("registering as herself: %d %s", code, answer)
	}
	// An Agent stating the User it acts for states who will own it: no claim.
	if _, err := bus.Register(protocol.Record{Name: "#bot@h", Kind: protocol.KindAgent, Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	if code, body := send(s, tok("#bot@h"), "POST", "/register", `{"kind":"queue","name":"botq@h","owner":"alice@h"}`, ""); code != http.StatusOK {
		t.Fatalf("an agent naming its own User as owner: %d %s", code, body)
	}
	if code, _ := send(s, tok("#bot@h"), "POST", "/register", `{"kind":"queue","name":"botq2@h","owner":"owner@h"}`, ""); code != http.StatusForbidden {
		t.Fatalf("an agent naming somebody else as owner answered %d", code)
	}
	var first protocol.Record
	json.Unmarshal([]byte(answer), &first)
	if first.Created.IsZero() {
		t.Fatalf("the answer carries no created_at: %s", answer)
	}
	time.Sleep(10 * time.Millisecond)
	_, answer = send(s, alice, "POST", "/register", `{"kind":"queue","name":"q@h","descr":"again"}`, "")
	var again protocol.Record
	json.Unmarshal([]byte(answer), &again)
	if !again.Created.Equal(first.Created) {
		t.Fatalf("a re-registration answered created_at %v, the stored one is %v", again.Created, first.Created)
	}
}

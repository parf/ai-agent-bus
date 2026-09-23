package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestPersonalServiceThroughAPI(t *testing.T) {
	bus := core.New()
	s, token := serverFor(t, bus, "admin@h")
	for _, name := range []string{"alice@h", "bob@h"} {
		if _, err := bus.SetUser("admin@h", protocol.User{Name: name}, true); err != nil {
			t.Fatal(err)
		}
	}
	call := func(who, path, body string, want int) string {
		t.Helper()
		req := httptest.NewRequest("POST", path, strings.NewReader(body))
		if body == "" {
			req.Method = "GET"
		}
		req.Header.Set(HeaderToken, token(who))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s %s: got %d %s, want %d", who, path, w.Code, w.Body.String(), want)
		}
		return w.Body.String()
	}
	call("alice@h", "/register", `{"kind":"agent","name":"#peer@h"}`, 200)
	call("alice@h", "/register", `{"kind":"agent","name":"#reports@h","personal":true,"allow":["#peer@h"]}`, 200)
	var got protocol.Record
	if err := json.Unmarshal([]byte(call("alice@h", "/lookup?name=%23reports@h", "", 200)), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Personal {
		t.Fatalf("public record lost Personal: %+v", got)
	}
	call("alice@h", "/manage", `{"kind":"agent","name":"#reports@h","allow":["bob@h"]}`, 400)
	call("alice@h", "/manage", `{"kind":"agent","name":"#reports@h","personal":false,"allow":["bob@h"]}`, 200)
}

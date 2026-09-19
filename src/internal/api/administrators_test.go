package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestAdministratorRoleUsesItsOwnPublicName(t *testing.T) {
	b := core.New()
	s, token := serverFor(t, b, "owner@h")
	if err := b.SetGroup("owner@h", core.AdministratorsGroup, []string{"owner@h", "admin@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetUser("owner@h", protocol.User{Name: "ordinary@h"}, true); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set(HeaderToken, token("owner@h"))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("users: %d %s", w.Code, w.Body.String())
	}
	var users []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &users); err != nil {
		t.Fatal(err)
	}
	if len(users) != 3 {
		t.Fatalf("users=%v", users)
	}
	for _, user := range users {
		if _, old := user["maintainer"]; old {
			t.Fatalf("old administrative role on wire: %v", user)
		}
		want := user["name"] != "ordinary@h"
		if (user["administrator"] == true) != want {
			t.Fatalf("wrong administrator standing: %v", user)
		}
	}
	// @maintainers named the administrators before they were renamed. The
	// rename migration went with the rest of the pre-1.1 compatibility code,
	// so the name carries no authority and is an ordinary group again — what
	// must stay refused is the protected group below, not this spelling.
	if code, _ := post(t, s, token, "owner@h", "/group", `{"name":"@maintainers","members":["ordinary@h"]}`); code != 200 {
		t.Fatalf("an ordinary group named @maintainers: %d, want 200", code)
	}
	if code, body := post(t, s, token, "owner@h", "/register", `{"kind":"agent","name":"svc@h"}`); code != 200 {
		t.Fatalf("register svc@h: %d %s", code, body)
	}
	if code, body := post(t, s, token, "owner@h", "/manage", `{"name":"svc@h","maintainers":["@maintainers"]}`); code != 200 {
		t.Fatalf("@maintainers as an ordinary maintainers grant: %d %s", code, body)
	}
	if code, _ := post(t, s, token, "admin@h", "/group", `{"name":"@administrators","members":["owner@h","admin@h","ordinary@h"]}`); code != 403 {
		t.Fatalf("administrator self-promotion through protected group: %d, want 403", code)
	}
}

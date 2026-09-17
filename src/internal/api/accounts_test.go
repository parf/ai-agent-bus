package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestAccountMapUsesExistingAdministrationAndHostValidation(t *testing.T) {
	b := core.New()
	s, token := serverFor(t, b, "owner@h")
	for _, name := range []string{"admin@h", "ordinary@h", "mapped@h"} {
		if _, err := b.SetUser("owner@h", protocol.User{Name: name}, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.SetGroup("owner@h", core.AdministratorsGroup, []string{"owner@h", "admin@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.EstablishAccounts(map[string]string{"existing-os": "ordinary@h"}); err != nil {
		t.Fatal(err)
	}
	validated := []string{}
	s.LocalAccounts("agent-busd", func(account string) error {
		validated = append(validated, account)
		if account == "missing-os" {
			return errors.New("not in passwd")
		}
		return nil
	})
	call := func(who, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set(HeaderToken, token(who))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}

	w := call("admin@h", http.MethodPost, "/account", `{"account":"new-os","principal":"mapped@h"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("Administrator set: %d %s", w.Code, w.Body.String())
	}
	var view protocol.AccountMappings
	if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil || !view.RestartRequired || len(view.Mappings) != 2 {
		t.Fatalf("set answer: %+v, %v", view, err)
	}
	if len(validated) != 1 || validated[0] != "new-os" {
		t.Fatalf("host validation did not run exactly once: %v", validated)
	}
	if w := call("ordinary@h", http.MethodPost, "/account", `{"account":"other-os","principal":"mapped@h"}`); w.Code != http.StatusForbidden {
		t.Fatalf("ordinary user changed map: %d %s", w.Code, w.Body.String())
	}
	if w := call("owner@h", http.MethodPost, "/account", `{"account":"missing-os","principal":"mapped@h"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("missing OS account was stored: %d %s", w.Code, w.Body.String())
	}
	if w := call("owner@h", http.MethodPost, "/account", `{"account":"agent-busd","principal":"mapped@h"}`); w.Code != http.StatusForbidden {
		t.Fatalf("implicit daemon socket was reassigned: %d %s", w.Code, w.Body.String())
	}
	if w := call("owner@h", http.MethodPost, "/account", `{"account":"new-os","remove":true}`); w.Code != http.StatusOK {
		t.Fatalf("remove: %d %s", w.Code, w.Body.String())
	}
	if len(validated) != 2 { // new-os and the deliberately missing set; removal performs no lookup
		t.Fatalf("removal unexpectedly required the retired OS account: %v", validated)
	}
	if w := call("owner@h", http.MethodPost, "/account", `{"account":"x","principal":"mapped@h","unexpected":true}`); w.Code != http.StatusBadRequest {
		t.Fatalf("account endpoint ignored an unknown field: %d %s", w.Code, w.Body.String())
	}
	if w := call("admin@h", http.MethodGet, "/accounts", ""); w.Code != http.StatusOK {
		t.Fatalf("Administrator list: %d %s", w.Code, w.Body.String())
	}
	if w := call("ordinary@h", http.MethodGet, "/accounts", ""); w.Code != http.StatusForbidden {
		t.Fatalf("ordinary user read map: %d %s", w.Code, w.Body.String())
	}
}

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestFreshLocalPrincipalSessionAuthenticatesWithoutPriorTokenRequest(t *testing.T) {
	b := core.New()
	s, _ := serverFor(t, b, "owner@h")
	if len(s.tokens.Pairs()) != 0 {
		t.Fatal("fixture already has a token")
	}
	owner, _ := protocol.ParseName("owner@h")
	w := httptest.NewRecorder()
	s.HandlerFor(owner).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/session", nil))
	var result struct {
		Session string `json:"session"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Session == "" {
		t.Fatalf("local session: %d %s", w.Code, w.Body.String())
	}
	r := httptest.NewRequest(http.MethodGet, "/status", nil)
	r.Header.Set(HeaderToken, result.Session)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("fresh session cannot authenticate: %d %s", w.Code, w.Body.String())
	}
	who, pair, ok := s.tokens.Credential(result.Session)
	if !ok || who != owner.String() || pair.UserID == 0 || b.CheckCredential(who, pair) != nil {
		t.Fatalf("session does not carry the current principal IDs: %s %+v %v", who, pair, ok)
	}
}

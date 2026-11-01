package api

import (
	"encoding/json"
	"errors"
	"maps"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
)

func TestWireRouteAndDataRefusalsAreDistinct(t *testing.T) {
	b := core.New()
	s, token := serverFor(t, b, "owner@h")
	for _, tc := range []struct {
		method, path, kind string
		code               int
		allow              bool
	}{
		{"GET", "/not-a-route", "no_route", 404, false},
		{"POST", "/identity", "no_route", 405, true},
		{"POST", "/subscriber/remove", "no_route", 404, false},
		{"GET", "/lookup?name=missing@h", "unknown", 404, false},
	} {
		before := b.Status().Refused
		r := httptest.NewRequest(tc.method, tc.path, nil)
		r.Header.Set(HeaderToken, token("owner@h"))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		var body map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if w.Code != tc.code || body["kind"] != tc.kind || body["error"] == "" {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
		if tc.allow && !strings.Contains(w.Header().Get("Allow"), "GET") {
			t.Fatalf("405 missing Allow: %v", w.Header())
		}
		if tc.kind == "no_route" && !maps.Equal(b.Status().Refused, before) {
			t.Fatal("route error counted as refusal")
		}
	}
}

func TestWireRefusalsCarryTheirReasonButInternalErrorsDoNot(t *testing.T) {
	s, _ := serverFor(t, core.New(), "owner@h")
	for _, c := range codes {
		w := httptest.NewRecorder()
		s.reply(w, nil, c.err)
		var body map[string]string
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if w.Code != c.code || body["kind"] != c.kind {
			t.Fatalf("%v: %d %s", c.err, w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	s.refuse(w, 403, "authority", "local socket required")
	if !strings.Contains(w.Body.String(), `"kind":"authority"`) {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	oops(w, errors.New("internal"))
	var body map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != 500 || body["error"] != "internal" || len(body) != 1 {
		t.Fatalf("internal error: %d %s", w.Code, w.Body.String())
	}
	want := []string{"acl", "authority", "busy", "credential", "enrolment", "full", "malformed", "name-taken", "second-reader", "suspended", "unavailable", "unknown"}
	if !slices.Equal(Reasons(), want) {
		t.Fatalf("Reasons=%v", Reasons())
	}
}

func TestWireUnavailableAndDryRefusalHaveExplicitKinds(t *testing.T) {
	s, _ := serverFor(t, core.New(), "owner@h")
	w := httptest.NewRecorder()
	s.reply(w, nil, core.ErrBigNoStore)
	if w.Code != 503 || !strings.Contains(w.Body.String(), `"kind":"unavailable"`) {
		t.Fatalf("unavailable: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	dryRefusal(w, core.ErrNotAllow)
	if w.Code != 403 || !strings.Contains(w.Body.String(), `"kind":"acl"`) {
		t.Fatalf("dry refusal: %d %s", w.Code, w.Body.String())
	}
}

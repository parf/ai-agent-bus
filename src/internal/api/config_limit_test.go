package api

import (
	"bytes"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/script"
)

func TestConfigurationCompactLimitAndScriptEnvironment(t *testing.T) {
	b := core.New()
	s, token := serverFor(t, b, "owner@h")
	known(t, b, "#svc@h")
	h := s.Handler()
	set := func(raw string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest("POST", "/configure", strings.NewReader(`{"name":"#svc@h","config":`+raw+`}`))
		r.Header.Set(HeaderToken, token("fixture-owner@h"))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	// The wire input exceeds the limit in whitespace; only compact bytes count.
	compact := `{"v":"` + strings.Repeat("x", 65536-len(`{"v":""}`)) + `"}`
	if w := set(" \n" + compact + "\n "); w.Code != 200 {
		t.Fatalf("64 KiB rejected: %d %s", w.Code, w.Body.String())
	}
	get := func() []byte {
		r := httptest.NewRequest("GET", "/config?name=%23svc@h", nil)
		r.Header.Set(HeaderToken, token("#svc@h"))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("config read: %d", w.Code)
		}
		return bytes.TrimSpace(w.Body.Bytes())
	}
	stored := get()
	if string(stored) != compact {
		t.Fatal("boundary configuration was not stored exactly")
	}
	env, err := script.ConfigEnv([]string{"PATH=/bin:/usr/bin"}, stored)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/sh", "-c", `printf '%s' "$AGENT_BUS_CONFIG"`)
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil || !bytes.Equal(out, stored) {
		t.Fatalf("boundary configuration did not reach script: %v (%d bytes)", err, len(out))
	}
	tooLarge := `{"v":"` + strings.Repeat("x", 65537-len(`{"v":""}`)) + `"}`
	if w := set(tooLarge); w.Code != 400 || !strings.Contains(w.Body.String(), "65536") {
		t.Fatalf("64 KiB+1 not refused with limit: %d %s", w.Code, w.Body.String())
	}
	if !bytes.Equal(get(), stored) {
		t.Fatal("oversize refusal changed stored configuration")
	}
}

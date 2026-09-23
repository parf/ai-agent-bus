package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// serverFor builds a daemon owned by the first name, and hands back the
// credential each principal authenticates with. A name is bound to its token
// now, so a test cannot state one without the other — which is the point.
func serverFor(t *testing.T, bus *core.Bus, owner string) (*Server, func(string) string) {
	t.Helper()
	tokens, err := auth.Load(memory.NewTokens(), owner)
	if err != nil {
		t.Fatal(err)
	}
	return New(bus, tokens, owner), func(name string) string {
		tok, err := tokens.Issue(name)
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}
}

// Two readers of one configuration must not race on the stored bytes: the
// handler used to append a newline into their spare capacity.
func TestConcurrentConfigReads(t *testing.T) {
	bus := core.New()
	s, tok := serverFor(t, bus, "owner@h")
	known(t, bus, "#svc@h")
	h := s.Handler()
	set := httptest.NewRequest("POST", "/configure",
		strings.NewReader(`{"kind":"agent","name":"#svc@h","config":{"a":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`))
	set.Header.Set(HeaderToken, tok("#svc@h"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, set)
	if w.Code != 200 {
		t.Fatalf("configure: %d %s", w.Code, w.Body.String())
	}
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest(http.MethodGet, "/config?name=%23svc@h", nil)
			r.Header.Set(HeaderToken, tok("#svc@h"))
			h.ServeHTTP(httptest.NewRecorder(), r)
		}()
	}
	wg.Wait()
}

// A record leaves the daemon without its configuration, whichever answer
// carries it. This escaped once on register and once on configure.
func TestNoAnswerCarriesAConfiguration(t *testing.T) {
	bus := core.New()
	s, tok := serverFor(t, bus, "owner@h")
	known(t, bus, "#svc@h")
	h := s.Handler()
	post := func(path, user, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", path, strings.NewReader(body))
		r.Header.Set(HeaderToken, tok(user))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	// Every participant is somebody first. A name the daemon holds nothing for
	// cannot even be issued a credential, and this test is about
	// configuration rather than about the gate.
	for _, who := range []string{"other@h"} {
		if _, err := bus.SetUser("owner@h", protocol.User{Name: who}, true); err != nil {
			t.Fatalf("fixture principal %s: %v", who, err)
		}
	}
	if w := post("/configure", "owner@h", `{"kind":"agent","name":"#mail/parf@h","config":{"password":"FIXTURE"}}`); w.Code != 200 {
		t.Fatalf("configure: %d %s", w.Code, w.Body.String())
	} else if strings.Contains(w.Body.String(), "FIXTURE") {
		t.Fatalf("configure echoed the configuration back: %s", w.Body.String())
	}
	if w := post("/register", "other@h", `{"kind":"agent","name":"#mail/parf@h"}`); strings.Contains(w.Body.String(), "FIXTURE") {
		t.Fatalf("register handed out the configuration: %s", w.Body.String())
	}
	r := httptest.NewRequest("GET", "/ls", nil)
	r.Header.Set(HeaderToken, tok("other@h"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if strings.Contains(w.Body.String(), "FIXTURE") {
		t.Fatalf("ls handed out the configuration: %s", w.Body.String())
	}
}

// Setup data goes in and is used; it does not come back out to be looked at.
// The owner who set it cannot read it either — only the service can.
func TestAConfigurationIsPrivateToItsService(t *testing.T) {
	bus := core.New()
	s, tok := serverFor(t, bus, "owner@h")
	known(t, bus, "#svc@h")
	h := s.Handler()
	do := func(method, path, user, body string) *httptest.ResponseRecorder {
		var r *http.Request
		if body == "" {
			r = httptest.NewRequest(method, path, nil)
		} else {
			r = httptest.NewRequest(method, path, strings.NewReader(body))
		}
		r.Header.Set(HeaderToken, tok(user))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	// Every participant is somebody first. A name the daemon holds nothing for
	// cannot even be issued a credential, and this test is about
	// configuration rather than about the gate.
	for _, who := range []string{"nosy@h"} {
		if _, err := bus.SetUser("owner@h", protocol.User{Name: who}, true); err != nil {
			t.Fatalf("fixture principal %s: %v", who, err)
		}
	}
	if w := do("POST", "/configure", "owner@h", `{"kind":"agent","name":"#mail/parf@h","config":{"password":"FIXTURE"}}`); w.Code != 200 {
		t.Fatalf("configure: %d %s", w.Code, w.Body.String())
	}
	for _, who := range []string{"owner@h", "nosy@h"} {
		w := do("GET", "/config?name=%23mail/parf@h", who, "")
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s read it: %d %s", who, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "FIXTURE") {
			t.Fatalf("the refusal to %s carried the configuration: %s", who, w.Body.String())
		}
	}
	w := do("GET", "/config?name=%23mail/parf@h", "#mail/parf@h", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "FIXTURE") {
		t.Fatalf("the service could not read its own: %d %s", w.Code, w.Body.String())
	}
	// The owner can still SET one; it just never comes back.
	if w := do("POST", "/configure", "owner@h", `{"kind":"agent","name":"#mail/parf@h","config":{"password":"SECOND"}}`); w.Code != 200 {
		t.Fatalf("the owner lost the right to configure: %d %s", w.Code, w.Body.String())
	}
}

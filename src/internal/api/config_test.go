package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
)

// Two readers of one configuration must not race on the stored bytes: the
// handler used to append a newline into their spare capacity.
func TestConcurrentConfigReads(t *testing.T) {
	bus := core.New()
	s := New(bus, "tok")
	h := s.Handler()
	set := httptest.NewRequest("POST", "/configure",
		strings.NewReader(`{"name":"svc@h","config":{"a":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`))
	set.Header.Set(HeaderUser, "svc@h")
	set.Header.Set(HeaderToken, "tok")
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
			r := httptest.NewRequest(http.MethodGet, "/config?name=svc@h", nil)
			r.Header.Set(HeaderUser, "svc@h")
			r.Header.Set(HeaderToken, "tok")
			h.ServeHTTP(httptest.NewRecorder(), r)
		}()
	}
	wg.Wait()
}

// A record leaves the daemon without its configuration, whichever answer
// carries it. This escaped once on register and once on configure.
func TestNoAnswerCarriesAConfiguration(t *testing.T) {
	bus := core.New()
	s := New(bus, "tok")
	h := s.Handler()
	post := func(path, user, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", path, strings.NewReader(body))
		r.Header.Set(HeaderUser, user)
		r.Header.Set(HeaderToken, "tok")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := post("/configure", "owner@h", `{"name":"mail/parf@h","config":{"password":"FIXTURE"}}`); w.Code != 200 {
		t.Fatalf("configure: %d %s", w.Code, w.Body.String())
	} else if strings.Contains(w.Body.String(), "FIXTURE") {
		t.Fatalf("configure echoed the configuration back: %s", w.Body.String())
	}
	if w := post("/register", "other@h", `{"name":"mail/parf@h"}`); strings.Contains(w.Body.String(), "FIXTURE") {
		t.Fatalf("register handed out the configuration: %s", w.Body.String())
	}
	r := httptest.NewRequest("GET", "/ls", nil)
	r.Header.Set(HeaderUser, "other@h")
	r.Header.Set(HeaderToken, "tok")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if strings.Contains(w.Body.String(), "FIXTURE") {
		t.Fatalf("ls handed out the configuration: %s", w.Body.String())
	}
}

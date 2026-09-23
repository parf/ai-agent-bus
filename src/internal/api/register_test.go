package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
)

func TestConditionalRegistrationClaimsNameOnce(t *testing.T) {
	bus := core.New()
	s, token := serverFor(t, bus, "owner@h")
	credential := token("owner@h")
	h := s.Handler()
	var wg sync.WaitGroup
	results := make(chan int, 16)
	for i := 0; i < cap(results); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := httptest.NewRequest("POST", "/register", strings.NewReader(fmt.Sprintf(`{"kind":"agent","name":"#Session@h","descr":"claim %d"}`, i)))
			r.Header.Set(HeaderToken, credential)
			r.Header.Set("If-None-Match", "*")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			results <- w.Code
		}(i)
	}
	wg.Wait()
	close(results)
	wins := 0
	for code := range results {
		if code == http.StatusOK {
			wins++
		} else if code != http.StatusPreconditionFailed {
			t.Fatalf("unexpected claim response: %d", code)
		}
	}
	if wins != 1 {
		t.Fatalf("concurrent claims succeeded %d times; want one", wins)
	}
	// Updating an existing record is still the ordinary registration contract.
	r := httptest.NewRequest("POST", "/register", strings.NewReader(`{"kind":"agent","name":"#session@h","descr":"renamed"}`))
	r.Header.Set(HeaderToken, credential)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "renamed") {
		t.Fatalf("ordinary update failed: %d %s", w.Code, w.Body.String())
	}
}

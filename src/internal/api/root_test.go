package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
)

// A person who typed the API's address into a browser is sent to the
// dashboard, and only from the root: a mistyped route must stay a 404 rather
// than become a redirect that hides the typo.
// See docs/05-discovery.md#where-it-listens.
func TestRootRedirectsToDashboard(t *testing.T) {
	s, _ := serverFor(t, core.New(), "owner@h")
	s.Dashboard("http://127.0.0.1:6780")
	h := s.Handler()

	get := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}

	// No credential: the redirect is the one answer a browser can act on,
	// and it carries nothing a caller could not have guessed.
	w := get("/")
	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("GET / answered %d, want %d", w.Code, http.StatusMovedPermanently)
	}
	if got, want := w.Header().Get("Location"), "http://127.0.0.1:6780/"; got != want {
		t.Fatalf("sent to %q, want %q", got, want)
	}

	if w := get("/no-such-route"); w.Code == http.StatusMovedPermanently {
		t.Fatal("a mistyped route redirected; only the root is the dashboard's")
	}

	// Told nothing, and told nowhere, it serves no root at all — what it did
	// before. An empty address must not normalise into "/", which would be
	// this root redirecting a browser to itself.
	for _, told := range []func(*Server){nil, func(s *Server) { s.Dashboard("") }} {
		quiet, _ := serverFor(t, core.New(), "owner@h")
		if told != nil {
			told(quiet)
		}
		w := httptest.NewRecorder()
		quiet.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		if w.Code == http.StatusMovedPermanently {
			t.Fatalf("redirected to %q without a dashboard to redirect to", w.Header().Get("Location"))
		}
	}
}

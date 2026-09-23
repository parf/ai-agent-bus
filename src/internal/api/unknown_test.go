package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Holding a credential is not being somebody. A name the daemon has no profile
// and no record for is refused everything, and told *who are you* rather than
// *you may not* — the two are different answers and a caller acts on them
// differently (docs/05-discovery.md#refusals).
//
// And it cannot get itself out of that: a credential is issued **to somebody**,
// so minting one for a name the daemon knows nothing about is refused at the
// source (docs/02-access.md#getting-a-token). That is the operation that filled
// this daemon's directory with names answering for nothing. A name becomes real
// because somebody registers it or a maintainer creates it, and only then can
// it hold a credential.
func TestAnUnknownNameCannotActAndCannotBeIssuedACredential(t *testing.T) {
	b := core.New()
	s, token := serverFor(t, b, "admin@h")
	call := func(who, method, path, body string) (int, string) {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set(HeaderToken, token(who))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w.Code, w.Body.String()
	}

	// A leftover credential, of the kind an older store still holds. Every
	// route is the same answer, and it is 401.
	for _, probe := range []struct{ method, path, body string }{
		{"GET", "/status", ""},
		{"GET", "/ls", ""},
		{"GET", "/users", ""},
		{"POST", "/send", `{"to":"admin@h","body":"x"}`},
		{"POST", "/manage", `{"kind":"agent","name":"admin@h","disabled":true}`},
		{"POST", "/unregister", `{"kind":"agent","name":"admin@h"}`},
		{"POST", "/register", `{"kind":"agent","name":"#theirs@h"}`},
		{"POST", "/register", `{"kind":"agent","name":"#ghost@h"}`},
	} {
		if code, body := call("#ghost@h", probe.method, probe.path, probe.body); code != 401 {
			t.Errorf("%s %s as an unknown name: %d %s, want 401", probe.method, probe.path, code, body)
		}
	}
	if _, known := b.Lookup("admin@h", "#theirs@h"); known {
		t.Fatal("a refused registration left a record behind")
	}

	// Nobody is handed a credential either, however authorised the asker.
	if code, body := call("admin@h", "POST", "/token", `{"kind":"agent","name":"#nobody@h"}`); code != 401 {
		t.Fatalf("a credential was minted for a name the daemon knows nothing about: %d %s", code, body)
	}

	// Somebody registers the name; now it is real, may hold a credential, and
	// may own things of its own.
	if code, body := call("admin@h", "POST", "/register", `{"kind":"agent","name":"#ghost@h"}`); code != 200 {
		t.Fatalf("the owner could not register the name: %d %s", code, body)
	}
	if code, body := call("admin@h", "POST", "/token", `{"kind":"agent","name":"#ghost@h"}`); code != 200 {
		t.Fatalf("a registered name could not be issued a credential: %d %s", code, body)
	}
	if code, body := call("#ghost@h", "GET", "/status", ""); code != 200 {
		t.Fatalf("a registered name is still refused: %d %s", code, body)
	}
	if code, body := call("#ghost@h", "POST", "/register", `{"kind":"agent","name":"#theirs@h"}`); code != 200 {
		t.Fatalf("a known name could not register a service: %d %s", code, body)
	}

	// A known user that is paused is a different refusal: it is not *who are
	// you*, it is a state, and it keeps its own code and reason.
	if _, err := b.SetUser("admin@h", protocol.User{Name: "paused@h"}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetUserState("admin@h", "paused@h", "paused"); err != nil {
		t.Fatal(err)
	}
	if code, body := call("paused@h", "GET", "/status", ""); code != 403 {
		t.Fatalf("a paused user was answered %d %s, want 403 suspended", code, body)
	}

	// A service that owns itself has no profile and is not collateral.
	known(t, b, "#svc@h")
	if code, body := call("#svc@h", "GET", "/status", ""); code != 200 {
		t.Fatalf("a self-owned service was refused: %d %s", code, body)
	}

	// Counted where it was decided, as a credential refusal and not as a
	// second reason nobody asked for.
	counts := b.Status().Refused
	if counts["credential"] != 9 || counts["suspended"] != 1 {
		t.Errorf("refusals counted as %v, want 9 credential and 1 suspended", counts)
	}
}

// privateRoutes is every route behind the guard. Kept as a list so that a route
// added without one shows up here as a gap rather than as nothing at all.
var privateRoutes = []string{
	"GET /status", "POST /register", "POST /unregister", "POST /manage",
	"GET /groups", "GET /users", "POST /user", "POST /user/state",
	"POST /identity/remove", "GET /activity", "POST /group", "GET /ls",
	"GET /lookup", "GET /recent", "GET /names", "POST /subscribe",
	"POST /subscriber/remove", "POST /configure", "GET /config", "POST /send",
	"GET /consume?wait=0s", "POST /token", "POST /session", "DELETE /session",
}

// The rule has to hold at every door, not at the ones a test happened to pick.
// Three ways a credential arrives — a leftover token, a browser session, and
// the mapped account on a socket — against every private route: an unregistered
// name is refused all of them, and leaves nothing behind for having tried.
// Enrolment is absent on purpose: it carries no credential, because it is where
// one comes from (docs/02-access.md#proving-possession).
//
// Route coverage of this shape was worked out by the codex peer during its
// access audit; the checks are restated here rather than kept in a scratch
// overlay, so that they run with everything else.
func TestEveryPrivateRouteRefusesAnUnregisteredName(t *testing.T) {
	b := core.New()
	s, token := serverFor(t, b, "admin@h")
	leftover := token("#ghost@h")
	session, err := s.tokens.StartSession("#ghost@h")
	if err != nil {
		t.Fatal(err)
	}
	ghost, err := protocol.ParseName("#ghost@h")
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range privateRoutes {
		method, path, _ := strings.Cut(route, " ")
		for _, how := range []struct {
			label, credential string
		}{
			{"a leftover token", leftover},
			{"a browser session", session},
			{"the mapped account on its socket", ""},
		} {
			body := `{"kind":"agent","name":"#ghost@h","config":{},"create":true}`
			r := httptest.NewRequest(method, path, strings.NewReader(body))
			w := httptest.NewRecorder()
			if how.credential == "" {
				s.HandlerFor(ghost).ServeHTTP(w, r)
			} else {
				r.Header.Set(HeaderToken, how.credential)
				s.Handler().ServeHTTP(w, r)
			}
			if w.Code != http.StatusUnauthorized {
				t.Errorf("%s with %s: %d %s, want 401", route, how.label, w.Code, strings.TrimSpace(w.Body.String()))
			}
		}
	}
	if _, made := b.Lookup("admin@h", "#ghost@h"); made {
		t.Fatal("probing left a record behind for a name nobody registered")
	}
}

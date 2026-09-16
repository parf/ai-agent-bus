package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestUserAdministrationAndLifecycle(t *testing.T) {
	b := core.New()
	s, tok := serverFor(t, b, "admin@h")
	call := func(who, method, path, body string, want int) string {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set(HeaderToken, tok(who))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: %d %s, want %d", who, path, w.Code, w.Body.String(), want)
		}
		return w.Body.String()
	}
	call("admin@h", "POST", "/user", `{"name":"alice@h","person_name":"Alice","email":" Alice@Example.COM ","github_user":"Alice-Code","create":true}`, 200)
	call("admin@h", "POST", "/user", `{"name":"maint@h","person_name":"Maintainer","create":true}`, 200)
	call("admin@h", "POST", "/user", `{"name":"peer@h","person_name":"Peer","create":true}`, 200)
	call("admin@h", "POST", "/group", `{"name":"@maintainers","members":["admin@h","maint@h","peer@h"]}`, 200)
	call("maint@h", "POST", "/user", `{"name":"alice@h","person_name":"Vouched","email":"alice@example.com","github_user":"alice-code"}`, 200)
	for _, target := range []string{"maint@h", "peer@h", "admin@h"} {
		call("maint@h", "POST", "/user", `{"name":"`+target+`","person_name":"forged"}`, 403)
	}
	call("alice@h", "POST", "/user", `{"name":"alice@h","person_name":"self-vouched"}`, 403)
	call("maint@h", "POST", "/group", `{"name":"@maintainers","members":["maint@h"]}`, 403)
	call("admin@h", "POST", "/group", `{"name":"@maintainers","members":[]}`, 403)
	// Not 403 any more: the removal is refused before anyone asks whose group
	// it is, so the daemon owner is refused it on the same terms as everybody.
	call("admin@h", "POST", "/group", `{"name":"@maintainers","remove":true}`, 400)
	call("admin@h", "POST", "/user/state", `{"name":"admin@h","state":"paused"}`, 403)
	call("admin@h", "POST", "/user", `{"name":"duplicate@h","email":"ALICE@EXAMPLE.COM","create":true}`, 400)
	call("admin@h", "POST", "/user", `{"name":"duplicate@h","github_user":"ALICE-CODE","create":true}`, 400)
	call("admin@h", "POST", "/user", `{"name":"distinct@h","email":"alice+alerts@example.com","create":true}`, 200)
	var users []protocol.User
	json.Unmarshal([]byte(call("alice@h", "GET", "/users", "", 200)), &users)
	if len(users) != 1 || users[0].Name != "alice@h" || users[0].Email != "alice@example.com" {
		t.Fatalf("unfiltered or unnormalized people view: %+v", users)
	}
	call("alice@h", "POST", "/register", `{"name":"svc@h"}`, 200)
	call("admin@h", "POST", "/send", `{"to":"alice@h","body":"retained"}`, 200)
	session, err := s.tokens.StartSession("alice@h")
	if err != nil {
		t.Fatal(err)
	}
	call("maint@h", "POST", "/user/state", `{"name":"alice@h","state":"paused"}`, 200)
	call("alice@h", "GET", "/status", "", 403)
	var pausedRecord protocol.Record
	json.Unmarshal([]byte(call("admin@h", "GET", "/lookup?name=alice@h", "", 200)), &pausedRecord)
	if !pausedRecord.Disabled {
		t.Fatal("paused user inbox is advertised as active")
	}
	call("admin@h", "POST", "/send", `{"to":"alice@h","body":"new"}`, 403)
	for _, h := range []struct {
		local bool
		token string
	}{{false, session}, {true, ""}} {
		r := httptest.NewRequest("GET", "/status", nil)
		r.Header.Set(HeaderToken, h.token)
		w := httptest.NewRecorder()
		if h.local {
			name, _ := protocol.ParseName("alice@h")
			s.HandlerFor(name).ServeHTTP(w, r)
		} else {
			s.Handler().ServeHTTP(w, r)
		}
		if w.Code != 403 {
			t.Fatalf("suspended user bypassed through session/local socket: %d", w.Code)
		}
	}
	// A service identity is *not* independent of its owner's access, which is
	// what H.5.7 changed: kept is not accepted, so while the state lasts the
	// credential grants nothing, theirs or their services'
	// (docs/01-identity.md#services-of-a-user-who-is-paused-or-banned). The
	// credential itself survives — the call below proves it works again once
	// the state is lifted, so nothing was revoked.
	call("svc@h", "GET", "/status", "", 403)
	restored := core.New()
	snapshot := b.Snapshot()
	data, _ := json.Marshal(snapshot)
	json.Unmarshal(data, &snapshot)
	restored.Restore(snapshot)
	s = New(restored, s.tokens, "admin@h")
	call("alice@h", "GET", "/status", "", 403)
	call("maint@h", "POST", "/user/state", `{"name":"alice@h","state":"active"}`, 200)
	// The service answers again the moment the state is lifted. That the
	// *same bytes* still work — kept rather than reissued — is pinned in
	// TestSuspensionDestroysNothing, which holds the credential across the
	// ban; this helper mints one per call and could not tell the difference.
	call("svc@h", "GET", "/status", "", 200)
	if got := call("alice@h", "GET", "/consume?wait=0s", "", 200); !strings.Contains(got, "retained") {
		t.Fatal("pause lost queued work")
	}
	call("maint@h", "POST", "/user/state", `{"name":"alice@h","state":"banned"}`, 200)
	call("maint@h", "POST", "/user/state", `{"name":"alice@h","state":"active"}`, 403)
	call("maint@h", "POST", "/user", `{"name":"alice@h","state":"active"}`, 403)
	call("alice@h", "POST", "/register", `{"name":"another@h"}`, 403)
	call("admin@h", "POST", "/user/state", `{"name":"alice@h","state":"active"}`, 200)
	call("alice@h", "GET", "/status", "", 200)
	// Removing a maintainer affects an already issued credential.
	call("admin@h", "POST", "/group", `{"name":"@maintainers","members":["admin@h","peer@h"]}`, 200)
	call("maint@h", "POST", "/user", `{"name":"alice@h","person_name":"stale authority"}`, 403)
}

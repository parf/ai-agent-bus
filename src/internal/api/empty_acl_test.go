package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestEmptyACLIsHiddenAndRefusesOtherCallersOverHTTP(t *testing.T) {
	b := core.New()
	s, token := serverFor(t, b, "admin@h")
	for _, name := range []string{"alice@h", "outsider@h"} {
		if _, err := b.SetUser("admin@h", protocol.User{Name: name}, true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.Register(protocol.Record{Name: "private@h", Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	request := func(who, method, path, body string, status int) string {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set(HeaderToken, token(who))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: status=%d want=%d body=%s", who, path, w.Code, status, w.Body.String())
		}
		return w.Body.String()
	}
	request("alice@h", "GET", "/lookup?name=private@h", "", 200)
	request("admin@h", "GET", "/lookup?name=private@h", "", 200)
	request("admin@h", "POST", "/send", `{"to":"private@h","body":"forbidden"}`, 403)
	for _, who := range []string{"outsider@h"} {
		hidden := request(who, "GET", "/lookup?name=private@h", "", 404)
		missing := request(who, "GET", "/lookup?name=missing@h", "", 404)
		// Each response may echo the requested name; compare the same name.
		if strings.ReplaceAll(hidden, "private@h", "missing@h") != missing {
			t.Fatal("hidden and missing lookup answers differ")
		}
		var rows []protocol.Record
		if err := json.Unmarshal([]byte(request(who, "GET", "/ls", "", 200)), &rows); err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if r.Name == "private@h" {
				t.Fatalf("%s lists hidden record", who)
			}
		}
		request(who, "POST", "/send", `{"to":"private@h","body":"forbidden"}`, 403)
	}
	request("alice@h", "POST", "/manage", `{"name":"private@h","allow":["*"]}`, 200)
	request("outsider@h", "GET", "/lookup?name=private@h", "", 200)
	request("outsider@h", "POST", "/send", `{"to":"private@h","body":"granted"}`, 200)
	if body := request("private@h", "GET", "/consume?wait=1ms", "", 200); !strings.Contains(body, "granted") {
		t.Fatal("own inbox did not deliver the granted message")
	}
	request("alice@h", "POST", "/manage", `{"name":"private@h","allow":[]}`, 200)
	request("outsider@h", "GET", "/lookup?name=private@h", "", 404)
}

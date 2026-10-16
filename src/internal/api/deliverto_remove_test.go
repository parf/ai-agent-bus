package api

import (
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A manager takes a recipient off a 📣's Deliver-To list at POST
// /deliver-to/remove; POST /subscriber/remove with `subscriber`, the names
// before 0.8.109, still does it for one release.
func TestARecipientIsTakenOffByItsNameAndByTheOldOne(t *testing.T) {
	bus := core.New()
	s, token := serverFor(t, bus, "admin@h")
	if _, err := bus.SetUser("admin@h", protocol.User{Name: "alice@h"}, true); err != nil {
		t.Fatal(err)
	}
	for _, r := range []protocol.Record{
		{Kind: protocol.KindAgent, Name: "#a@h", Owner: "alice@h", Allow: []string{"*"}},
		{Kind: protocol.KindAgent, Name: "#b@h", Owner: "alice@h", Allow: []string{"*"}},
		{Kind: protocol.KindPubSub, Name: "news@h", Owner: "alice@h", Allow: []string{"*"}, Subs: []string{"#a@h", "#b@h"}},
	} {
		if _, err := bus.Register(r); err != nil {
			t.Fatal(err)
		}
	}
	post := func(path, body string) {
		t.Helper()
		req := httptest.NewRequest("POST", path, strings.NewReader(body))
		req.Header.Set(HeaderToken, token("alice@h"))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	subs := func() []string { r, _ := bus.Lookup("alice@h", "news@h"); return r.Subs }
	post("/deliver-to/remove", `{"channel":"news@h","recipient":"#a@h"}`)
	if slices.Contains(subs(), "#a@h") {
		t.Fatalf("POST /deliver-to/remove left the recipient: %v", subs())
	}
	post("/subscriber/remove", `{"channel":"news@h","subscriber":"#b@h"}`)
	if slices.Contains(subs(), "#b@h") {
		t.Fatalf("the old route left the recipient: %v", subs())
	}
}

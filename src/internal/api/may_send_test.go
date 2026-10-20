package api

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestMaySendMatchesSendWithoutMessagesOrCounters(t *testing.T) {
	for _, tc := range []struct {
		label, caller, target string
		code                  int
	}{
		{"allowed", "alice@h", "#private@h", 200},
		{"invisible ACL", "bob@h", "#private@h", 403},
		{"visible to admin but refused by ACL", "admin@h", "#private@h", 403},
		{"inactive", "alice@h", "#inactive@h", 404},
		{"missing", "alice@h", "#missing@h", 404},
		{"external", "alice@h", "web@h", 400},
		{"personal allowed agent", "#peer@h", "#personal@h", 200},
		{"personal refused user", "bob@h", "#personal@h", 403},
		{"agent reply to allowed user", "#peer@h", "bob@h", 200},
		{"agent reply to refused user", "#private@h", "bob@h", 403},
		{"user to another user", "alice@h", "bob@h", 403},
		{"invalid name", "alice@h", "bad name", 400},
	} {
		t.Run(tc.label, func(t *testing.T) {
			b := core.New()
			now := time.Now()
			b.Clock(func() time.Time { return now })
			s, token := serverFor(t, b, "admin@h")
			for _, who := range []string{"alice@h", "bob@h"} {
				if _, err := b.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
					t.Fatal(err)
				}
			}
			for _, rec := range []protocol.Record{
				{Name: "#private@h", Kind: protocol.KindAgent, Owner: "alice@h"},
				{Name: "#inactive@h", Kind: protocol.KindAgent, Owner: "alice@h"},
				{Name: "web@h", Kind: protocol.KindService, Owner: "alice@h", Addr: "https://example.test", Proto: "https"},
				{Name: "#peer@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"bob@h"}},
				{Name: "#personal@h", Kind: protocol.KindAgent, Owner: "alice@h", Personal: true, Allow: []string{"#peer@h"}},
			} {
				if _, err := b.Register(rec); err != nil {
					t.Fatal(err)
				}
			}
			inactive := protocol.StatusInactive
			if _, err := b.Manage("alice@h", core.Management{Name: "#inactive@h", Status: &inactive}); err != nil {
				t.Fatal(err)
			}
			credential := token(tc.caller)
			request := func(method, path, body string) *httptest.ResponseRecorder {
				t.Helper()
				r := httptest.NewRequest(method, path, strings.NewReader(body))
				r.Header.Set(HeaderToken, credential)
				w := httptest.NewRecorder()
				s.Handler().ServeHTTP(w, r)
				return w
			}
			before := b.Status()
			before.Up = ""
			queues := b.Snapshot().Queues
			recent := b.Recent("admin@h")
			activity, err := b.Activity("alice@h", "")
			if err != nil {
				t.Fatal(err)
			}
			dry := request("GET", "/may-send?"+url.Values{"name": {tc.target}}.Encode(), "")
			after := b.Status()
			after.Up = ""
			afterActivity, err := b.Activity("alice@h", "")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(queues, b.Snapshot().Queues) || !reflect.DeepEqual(recent, b.Recent("admin@h")) || !reflect.DeepEqual(activity, afterActivity) {
				t.Fatalf("dry run changed messages or counters: before=%+v after=%+v", before, after)
			}
			body, _ := json.Marshal(protocol.Envelope{To: tc.target, Body: "real send"})
			sent := request("POST", "/send", string(body))
			if dry.Code != tc.code || sent.Code != tc.code {
				t.Fatalf("may-send=%d %s; send=%d %s; want %d", dry.Code, dry.Body, sent.Code, sent.Body, tc.code)
			}
			if tc.code == 200 {
				if strings.TrimSpace(dry.Body.String()) != `{"may_send":true}` {
					t.Fatalf("dry run: %s", dry.Body)
				}
				if b.Status().Queued != before.Queued+1 {
					t.Fatal("positive control send did not enqueue exactly one")
				}
			} else if dry.Body.String() != sent.Body.String() {
				t.Fatalf("dry run reason differs from send: %s != %s", dry.Body, sent.Body)
			}
		})
	}
}

func TestMaySendNeverAcceptsAnOnBehalfOfCaller(t *testing.T) {
	b := core.New()
	s, token := serverFor(t, b, "admin@h")
	for _, path := range []string{
		"/may-send?name=%23private@h&who=admin@h", "/may-send?name=a@h&from=admin@h",
		"/may-send?name=a@h&name=b@h", "/may-send", "/may-send?caller=admin@h",
	} {
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set(HeaderToken, token("admin@h"))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	r := httptest.NewRequest("GET", "/may-send?name=a@h", strings.NewReader(`{"from":"admin@h"}`))
	r.Header.Set(HeaderToken, token("admin@h"))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatalf("body: %d %s", w.Code, w.Body)
	}
	if len(b.Status().Refused) != 0 {
		t.Fatal("dry-run argument refusals moved counters")
	}
}

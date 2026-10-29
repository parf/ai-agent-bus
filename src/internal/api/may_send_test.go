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
		"/may-send?name=a@h&from=admin@h",
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

// With who, the answer is Send's own for that name, true or false, only for a
// target the caller can see; a hidden target answers as an absent one.
func TestMaySendForAnotherNameFollowsSendAndLimitsDisclosure(t *testing.T) {
	b := core.New()
	s, token := serverFor(t, b, "admin@h")
	for _, who := range []string{"alice@h", "bob@h", "carol@h"} {
		if _, err := b.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	for _, rec := range []protocol.Record{
		{Name: "#private@h", Kind: protocol.KindAgent, Owner: "alice@h"},
		{Name: "#peer@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"bob@h"}},
		{Name: "#personal@h", Kind: protocol.KindAgent, Owner: "alice@h", Personal: true, Allow: []string{"#peer@h"}},
		{Name: "#gone@h", Kind: protocol.KindAgent, Owner: "alice@h"},
		{Name: "web@h", Kind: protocol.KindService, Owner: "alice@h", Addr: "https://example.test", Proto: "https"},
	} {
		if _, err := b.Register(rec); err != nil {
			t.Fatal(err)
		}
	}
	inactive := protocol.StatusInactive
	if _, err := b.Manage("alice@h", core.Management{Name: "#gone@h", Status: &inactive}); err != nil {
		t.Fatal(err)
	}
	get := func(caller, who, name string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest("GET", "/may-send?"+url.Values{"who": {who}, "name": {name}}.Encode(), nil)
		r.Header.Set(HeaderToken, token(caller))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	// answered reads one answer, which must carry the canonical who it was asked about.
	answered := func(t *testing.T, w *httptest.ResponseRecorder, who string) bool {
		t.Helper()
		var got struct {
			MaySend bool   `json:"may_send"`
			Who     string `json:"who"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || w.Code != 200 || got.Who != who {
			t.Fatalf("answer %d %s for who=%s: %v", w.Code, w.Body, who, err)
		}
		return got.MaySend
	}
	before := b.Status()
	before.Up = ""
	queues := b.Snapshot().Queues
	recent := b.Recent("admin@h")
	// The caller is the admin, who sees everything here, so every row is a visible target:
	// what differs is only who is asked about.
	for _, who := range []string{"alice@h", "bob@h", "carol@h", "#peer@h", "#private@h", "ghost@h", "#gone@h"} {
		for _, name := range []string{"#private@h", "#peer@h", "#personal@h", "web@h", "bob@h", "alice@h"} {
			w := get("admin@h", who, name)
			if w.Code != 200 {
				t.Fatalf("who=%s name=%s: %d %s", who, name, w.Code, w.Body)
			}
			want := false
			// What Send itself would answer for who, where who is a live name.
			if err := b.MaySend(who, name); err == nil {
				want = true
			}
			if who == "ghost@h" || who == "#gone@h" {
				want = false
			}
			if got := answered(t, w, who); got != want {
				t.Errorf("who=%s name=%s: answered %s, Send's rule says %v", who, name, w.Body, want)
			}
		}
	}
	// Known cases, so the parity above is not two copies of one mistake.
	for _, c := range []struct {
		who, name string
		want      bool
	}{
		{"bob@h", "#peer@h", true}, {"carol@h", "#peer@h", false}, {"alice@h", "#private@h", true},
		{"bob@h", "#private@h", false}, {"#peer@h", "#personal@h", true}, {"#private@h", "#personal@h", false},
		{"alice@h", "web@h", false}, {"ghost@h", "#peer@h", false},
	} {
		if got := answered(t, get("admin@h", c.who, c.name), c.who); got != c.want {
			t.Errorf("who=%s name=%s: got %v, want %v", c.who, c.name, got, c.want)
		}
	}
	// An unknown who, an inactive one and a refused one are one answer in
	// every byte but the echo: no oracle on who exists.
	refused, unknown, retired := get("admin@h", "carol@h", "#private@h"), get("admin@h", "ghost@h", "#private@h"), get("admin@h", "#gone@h", "#private@h")
	for _, w := range []*httptest.ResponseRecorder{unknown, retired} {
		if w.Code != refused.Code || w.Header().Get("Content-Type") != refused.Header().Get("Content-Type") || len(w.Body.Bytes()) < 10 {
			t.Fatalf("who answers differ: %d %q vs %d %q", w.Code, w.Body, refused.Code, refused.Body)
		}
	}
	if strings.ReplaceAll(unknown.Body.String(), "ghost@h", "carol@h") != refused.Body.String() {
		t.Fatalf("an unknown who answers differently from a refused one: %q vs %q", unknown.Body, refused.Body)
	}
	// A caller who cannot see the target learns nothing: hidden, absent and
	// inactive are one answer, and who is not consulted.
	hidden := get("carol@h", "alice@h", "#private@h")
	absent := get("carol@h", "alice@h", "#nothing@h")
	gone := get("carol@h", "alice@h", "#gone@h")
	if hidden.Code != 404 || absent.Code != 404 || gone.Code != 404 {
		t.Fatalf("hidden %d, absent %d, inactive %d; want 404 each", hidden.Code, absent.Code, gone.Code)
	}
	if hidden.Body.String() != strings.ReplaceAll(absent.Body.String(), "#nothing@h", "#private@h") {
		t.Fatalf("a hidden target answers differently from an absent one: %q vs %q", hidden.Body, absent.Body)
	}
	if strings.Contains(get("carol@h", "alice@h", "#private@h").Body.String(), "may_send") {
		t.Fatal("a hidden target was answered")
	}
	// Bad arguments.
	for _, path := range []string{
		"/may-send?name=%23peer@h&who=", "/may-send?name=%23peer@h&who=a@h&who=b@h",
		"/may-send?name=%23peer@h&who=bob@h&from=bob@h", "/may-send?who=bob@h", "/may-send?name=%23peer@h&who=bad%20name",
	} {
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set(HeaderToken, token("alice@h"))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != 400 {
			t.Errorf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	after := b.Status()
	after.Up = ""
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(queues, b.Snapshot().Queues) || !reflect.DeepEqual(recent, b.Recent("admin@h")) {
		t.Fatalf("a who query changed counters or messages: %+v -> %+v", before, after)
	}
}

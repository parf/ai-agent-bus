package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestEventParserBoundsTypesAndDuplicateFields(t *testing.T) {
	for _, body := range []string{`{"method":"write","log":"#a@h","level":"alert","text":""}`, "write #a@h notice full line with spaces", `{"method":"read","log":"#a@h","level":"alert"}`, "pull #a@h alert 7", "clean #a@h 9", "list"} {
		if _, err := parseEventCall(body); err != nil {
			t.Fatalf("valid %s: %v", body, err)
		}
	}
	for _, body := range []string{`{"method":"read","log":"#a@h","log":"#b@h"}`, `{"method":"read","log":null}`, `{"method":"read","log":"#a@h","level":null}`, `{"method":"pull","log":"#a@h","count":1.5}`, `{"method":"write","log":"#a@h","level":"alert"}`, `{"method":"clean","log":"#a@h","id":0}`, "read #a@h 2", "pull #a@h 501", "clean #a@h 9007199254740992", "write #a@h invalid x", "write #a@h notice " + strings.Repeat("x", 16384)} {
		if _, err := parseEventCall(body); err == nil {
			t.Fatalf("invalid request accepted %s", body)
		}
	}
}
func TestDaemonWritesAnswerOnlyWaitingCallsAndNeverLeakProcessInput(t *testing.T) {
	b := core.New()
	s, token := serverFor(t, b, "admin@h")
	if err := b.EstablishDaemonAgent("node.example"); err != nil {
		t.Fatal(err)
	}
	for _, r := range []protocol.Record{{Name: "#a@h", Kind: protocol.KindAgent, Owner: "admin@h", Allow: []string{"*"}}, {Name: "answers@h", Kind: protocol.KindQueue, Owner: "admin@h", Allow: []string{"*"}}} {
		if _, err := b.Register(r); err != nil {
			t.Fatal(err)
		}
	}
	stop := s.ServeDaemonAgent(context.Background())
	defer stop()
	send := func(who, body, wait string, back *protocol.ReplyTo) protocol.Envelope {
		t.Helper()
		raw, _ := json.Marshal(protocol.Envelope{To: core.DaemonAgent, Body: body, Wait: wait, ReplyTo: back})
		r := httptest.NewRequest("POST", "/send", strings.NewReader(string(raw)))
		r.Header.Set(HeaderToken, token(who))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("send %d %s", w.Code, w.Body.String())
		}
		var accepted protocol.Envelope
		if err := json.Unmarshal(w.Body.Bytes(), &accepted); err != nil {
			t.Fatal(err)
		}
		return accepted
	}
	consume := func(name string) protocol.Envelope {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		v, err := b.Consume(ctx, name, "", "", false, false)
		if err != nil {
			t.Fatalf("waiting answer: %v", err)
		}
		return v
	}
	// The second waiting mutation is a completion fence on the same agent;
	// inspecting an inbox immediately after HTTP acceptance would be vacuous.
	for i, body := range []string{"write #a@h notice plain", "write #a@h invalid refused"} {
		send("#a@h", body, "", &protocol.ReplyTo{Name: "answers@h"})
		accepted := send("#a@h", "write #a@h alert fence"+string(rune('a'+i)), "2s", nil)
		answer := consume("#a@h")
		if answer.Re != accepted.ID || strings.HasPrefix(answer.Body, "error:") {
			t.Fatalf("waiting write not exactly correlated: %+v", answer)
		}
		for _, name := range []string{"#a@h", "answers@h"} {
			r, _ := b.Lookup("admin@h", name)
			if r.Queued != 0 {
				t.Fatalf("no-wait write leaked %d process-input messages to %s", r.Queued, name)
			}
		}
	}
	accepted := send("#a@h", "write #a@h invalid waiting", "2s", &protocol.ReplyTo{Name: "answers@h"})
	answer := consume("answers@h")
	if answer.Re != accepted.ID || !strings.HasPrefix(answer.Body, "error: invalid_level:") {
		t.Fatalf("write refusal route/prefix %+v", answer)
	}
	accepted = send("admin@h", "status", "", nil)
	answer = consume("admin@h")
	if answer.Re != accepted.ID || !strings.HasPrefix(answer.Body, "ok: ") {
		t.Fatalf("old verb stopped answering %+v", answer)
	}
	for _, tc := range []struct{ who, body, code string }{{"#a@h", "read #a@h", "forbidden"}, {"admin@h", "read #a@h warning", ""}} {
		accepted = send(tc.who, tc.body, "2s", nil)
		answer = consume(tc.who)
		if answer.Re != accepted.ID {
			t.Fatal("uncorrelated read")
		}
		var result map[string]json.RawMessage
		if json.Unmarshal([]byte(answer.Body), &result) != nil {
			t.Fatal(answer.Body)
		}
		if tc.code != "" {
			if !strings.Contains(string(result["error"]), `"code":"`+tc.code+`"`) {
				t.Fatal(answer.Body)
			}
		} else if string(result["rings"]) != `{"warning":[]}` {
			t.Fatalf("empty selected ring not a list: %s", answer.Body)
		}
	}
}

func TestBusRingSumRefusesAtTheConfigurationBoundary(t *testing.T) {
	b := core.New()
	s, token := serverFor(t, b, "admin@h")
	if err := b.EstablishDaemonAgent("node.example"); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/configure", strings.NewReader(`{"name":"#bus","kind":"agent","config":{"realm":"invalid","notice_entries":251}}`))
	r.Header.Set(HeaderToken, token("admin@h"))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code < 400 || !strings.Contains(w.Body.String(), "500") {
		t.Fatalf("501-entry setup accepted: %d %s", w.Code, w.Body.String())
	}
	if b.Realm() != "node.example" {
		t.Fatal("refused setup was published")
	}
}

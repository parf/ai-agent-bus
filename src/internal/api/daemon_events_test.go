package api

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/parf/ai-agent-bus/internal/store/memory"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/eventlog"
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
	r := httptest.NewRequest("POST", "/configure", strings.NewReader(`{"name":"#bus","config":{"realm":"invalid","notice_entries":251}}`))
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

func TestEventHelpHidesMethodsWithoutAuthority(t *testing.T) {
	b := core.New()
	s, _ := serverFor(t, b, "admin@h")
	if _, err := b.SetUser("admin@h", protocol.User{Name: "stranger@h"}, true); err != nil {
		t.Fatal(err)
	}
	b.EstablishDaemonAgent("node.example")
	for _, e := range []protocol.Envelope{{From: "stranger@h", Ask: protocol.AskHelp}, {From: "stranger@h", Ask: protocol.AskHelpJSON}, {From: "stranger@h", Body: "help"}} {
		got, err := s.daemonAnswer(e)
		if err != nil {
			t.Fatal(err)
		}
		if e.Ask == protocol.AskHelpJSON {
			var v struct{ Methods map[string]any }
			if json.Unmarshal([]byte(got), &v) != nil {
				t.Fatal(got)
			}
			for _, method := range []string{"write", "read", "list", "pull", "clean"} {
				if _, ok := v.Methods[method]; ok {
					t.Fatalf("stranger saw %s in metadata", method)
				}
			}
		} else {
			for _, line := range strings.Split(got, "\n") {
				for _, method := range []string{"write", "read", "list", "pull", "clean"} {
					if strings.HasPrefix(line, method+" ") || strings.HasPrefix(line, method+" —") {
						t.Fatalf("stranger saw event help: %s", line)
					}
				}
			}
		}
	}
}
func TestEventErrorsNeverEchoStoreInternals(t *testing.T) {
	const secret = "private-store-secret-42"
	generic := eventFailure(errors.New(secret), false)
	if generic != `{"error":{"code":"internal_error","text":"The operation could not be completed."}}` {
		t.Fatalf("generic internal error changed: %s", generic)
	}
	b := core.New()
	s, _ := serverFor(t, b, "admin@h")
	b.EstablishDaemonAgent("node.example")
	b.Register(protocol.Record{Name: "#a@h", Kind: protocol.KindAgent, Owner: "admin@h"})
	st := memory.NewState()
	b.Persistence(st)
	st.Err = errors.New(secret)
	got, err := s.daemonAnswer(protocol.Envelope{From: "#a@h", Body: "write #a@h alert safe", Wait: "1s"})
	if err != nil || got != "error: storage_unknown: Ring storage outcome is unknown; reopen validated state." || strings.Contains(got, secret) {
		t.Fatalf("private storage diagnostic escaped: %q %v", got, err)
	}
}
func TestEventRefusalBytesDoNotRevealTargetExistence(t *testing.T) {
	b := core.New()
	s, _ := serverFor(t, b, "admin@h")
	b.EstablishDaemonAgent("node.example")
	b.SetUser("admin@h", protocol.User{Name: "stranger@h"}, true)
	for _, name := range []string{"#written@h", "#never@h"} {
		b.Register(protocol.Record{Name: name, Kind: protocol.KindAgent, Owner: "admin@h", Allow: []string{"*"}})
	}
	b.EventWrite("#written@h", "#written@h", "alert", "private")
	for _, method := range []string{"read", "write", "invalid"} {
		var previous string
		for _, name := range []string{"#written@h", "#never@h", "#missing@h"} {
			body := method + " " + name
			if method == "write" {
				body += " alert attempted"
			} else if method == "invalid" {
				body = "write " + name + " invalid attempted"
			}
			got, err := s.daemonAnswer(protocol.Envelope{From: "stranger@h", Body: body, Wait: "1s"})
			if err != nil {
				t.Fatal(err)
			}
			if previous != "" && got != previous {
				t.Fatalf("%s target existence changes bytes: %q != %q", method, got, previous)
			}
			previous = got
		}
	}
}
func TestExpiredAndWriteLookingObservationsQueueNoAnswer(t *testing.T) {
	b := core.New()
	s, _ := serverFor(t, b, "admin@h")
	b.EstablishDaemonAgent("node.example")
	b.Register(protocol.Record{Name: "#a@h", Kind: protocol.KindAgent, Owner: "admin@h", Allow: []string{"*"}})
	expired, err := b.Send(protocol.Envelope{From: "#a@h", To: core.DaemonAgent, Body: "write #a@h notice expired", Wait: "1s"})
	if err != nil {
		t.Fatal(err)
	}
	snap := b.Snapshot()
	for i := range snap.Queues {
		for j := range snap.Queues[i].Messages {
			if snap.Queues[i].Messages[j].ID == expired.ID {
				snap.Queues[i].Messages[j].Deadline = time.Now().Add(-time.Hour)
			}
		}
	}
	if _, err = b.Consume(context.Background(), core.DaemonAgent, "", "", false, false); err != nil {
		t.Fatal(err)
	}
	b.Restore(snap)
	for _, body := range []string{"WRITE #a@h notice uppercase", `{"method":"Write","log":"#a@h","level":"notice","text":"upper"}`, `{"method":"write","log":`} {
		if _, err = b.Send(protocol.Envelope{From: "#a@h", To: core.DaemonAgent, Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	fence, err := b.Send(protocol.Envelope{From: "#a@h", To: core.DaemonAgent, Body: "write #a@h notice fence", Wait: "2s"})
	if err != nil {
		t.Fatal(err)
	}
	stop := s.ServeDaemonAgent(context.Background())
	defer stop()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	answer, err := b.Consume(ctx, "#a@h", "", "", false, false)
	if err != nil || answer.Re != fence.ID {
		t.Fatalf("expired/write-looking write queued stray answer: %+v %v", answer, err)
	}
	r, _ := b.Lookup("admin@h", "#a@h")
	if r.Queued != 0 {
		t.Fatalf("expired/write-looking answers queued: %d", r.Queued)
	}
	unknown, err := b.Send(protocol.Envelope{From: "#a@h", To: core.DaemonAgent, Body: "unrecognized-method"})
	if err != nil {
		t.Fatal(err)
	}
	answer, err = b.Consume(ctx, "#a@h", "", "", false, false)
	if err != nil || answer.Re != unknown.ID || !strings.HasPrefix(answer.Body, "error: unknown method") {
		t.Fatalf("ordinary unknown no-wait verb stopped answering: %+v %v", answer, err)
	}

}

func TestDaemonPrivateAgentWriteStoresWithoutWideningReplyACL(t *testing.T) {
	b := core.New()
	s, _ := serverFor(t, b, "admin@h")
	b.EstablishDaemonAgent("node.example")
	b.Register(protocol.Record{Name: "#private@h", Kind: protocol.KindAgent, Owner: "admin@h"})
	b.Register(protocol.Record{Name: "answers@h", Kind: protocol.KindQueue, Owner: "admin@h", Allow: []string{"*"}})
	stop := s.ServeDaemonAgent(context.Background())
	defer stop()
	if _, err := b.Send(protocol.Envelope{From: "#private@h", To: core.DaemonAgent, Body: "write #private@h notice private", Wait: "2s"}); err != nil {
		t.Fatal(err)
	}
	// An authorized write to the same log with an admitted explicit return route
	// fences completion without granting #bus access to the private inbox.
	fence, err := b.Send(protocol.Envelope{From: "admin@h", To: core.DaemonAgent, Body: "write #private@h notice fence", Wait: "2s", ReplyTo: &protocol.ReplyTo{Name: "answers@h"}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	answer, err := b.Consume(ctx, "answers@h", "", "", false, false)
	if err != nil || answer.Re != fence.ID {
		t.Fatalf("fence did not complete: %+v %v", answer, err)
	}
	r, _ := b.Lookup("admin@h", "#private@h")
	if r.Queued != 0 {
		t.Fatalf("reply bypassed private Allow: %d messages", r.Queued)
	}
	v, err := b.EventRead("admin@h", "#private@h", "")
	if err != nil || v.Counts["notice"] != 2 {
		t.Fatalf("private waiting write was not stored: %+v %v", v, err)
	}
}

func TestOversizedDaemonInputDoesNotRunFullBodyScanners(t *testing.T) {
	for _, body := range []string{strings.Repeat("x", 1024*1024), "write #a@h alert " + strings.Repeat("x", 1024*1024), `{"method":"write","text":"` + strings.Repeat("x", 1024*1024) + `"}`} {
		scans := 0
		c, err := inspectDaemonInput(protocol.Envelope{Body: body}, func(body string) (eventCall, error) { scans++; return scanDaemonInput(body) })
		if eventlog.Code(err) != "invalid_request" || scans != 0 {
			t.Fatalf("oversized body reached full scanner: bytes=%d scans=%d method=%s err=%v", len(body), scans, c.Method, err)
		}
	}
	scans := 0
	inspectDaemonInput(protocol.Envelope{Body: strings.Repeat("x", daemonMaxInput)}, func(body string) (eventCall, error) { scans++; return eventCall{}, nil })
	if scans != 1 {
		t.Fatal("input at the bound was not scanned")
	}
}
func TestForbiddenEventRepliesUseExactAuthorityTextPerVerb(t *testing.T) {
	b := core.New()
	s, _ := serverFor(t, b, "admin@h")
	b.EstablishDaemonAgent("node.example")
	b.Register(protocol.Record{Name: "#victim@h", Kind: protocol.KindAgent, Owner: "admin@h"})
	b.Register(protocol.Record{Name: "#attacker@h", Kind: protocol.KindAgent, Owner: "admin@h"})
	for _, method := range []string{"read", "pull", "clean", "write"} {
		t.Run(method, func(t *testing.T) {
			body := method + " #victim@h"
			want := `{"error":{"code":"forbidden","text":"Only the log’s Owner, Maintainers or daemon Owner may read or manage it."}}`
			if method == "write" {
				body += " alert attempted"
				want = "error: forbidden: Only the agent itself or daemon Owner may write this log."
			}
			got, err := s.daemonAnswer(protocol.Envelope{From: "#attacker@h", Body: body, Wait: "1s"})
			if err != nil || got != want {
				t.Fatalf("%s forbidden bytes changed: %q %v", method, got, err)
			}
		})
	}
}

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/version"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestDaemonAgentListsOnlyTheOriginalCallersRecords(t *testing.T) {
	b := core.New()
	s, token := serverFor(t, b, "admin@h")
	for _, who := range []string{"alice@h", "bob@h"} {
		if _, err := b.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.EstablishDaemonAgent(); err != nil {
		t.Fatal(err)
	}
	for _, r := range []protocol.Record{
		{Name: "#private@h", Kind: protocol.KindAgent, Owner: "admin@h"},
		{Name: "#personal@h", Kind: protocol.KindAgent, Owner: "alice@h", Personal: true},
		{Name: "#worker@h", Kind: protocol.KindAgent, Owner: "alice@h", Tags: []string{"team"}, Allow: []string{"*"}},
		{Name: "#visitor@h", Kind: protocol.KindAgent, Owner: "bob@h", Allow: []string{"#bus"}},
	} {
		if _, err := b.Register(r); err != nil {
			t.Fatal(err)
		}
	}
	stop := s.ServeDaemonAgent(context.Background())
	defer stop()
	for _, who := range []string{"admin@h", "alice@h", "bob@h", "#worker@h", "#visitor@h"} {
		credential := token(who)
		request := func(method, path, body string) *httptest.ResponseRecorder {
			r := httptest.NewRequest(method, path, strings.NewReader(body))
			r.Header.Set(HeaderToken, credential)
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			return w
		}
		for _, tc := range []struct{ body, query string }{{"ls --all", ""}, {"ls --kind agent", "?kind=agent"}, {"ls --tag=team", "?tag=team"}} {
			t.Run(who+tc.body, func(t *testing.T) {
				listed := request("GET", "/ls"+tc.query, "")
				if listed.Code != 200 {
					t.Fatal(listed.Body.String())
				}
				var rs []protocol.Record
				if err := json.Unmarshal(listed.Body.Bytes(), &rs); err != nil {
					t.Fatal(err)
				}
				want := []string{}
				for _, r := range rs {
					want = append(want, r.Name)
				}
				slices.Sort(want)
				body, _ := json.Marshal(protocol.Envelope{From: "admin@h", To: core.DaemonAgent, Body: tc.body, Topic: "rt24", Tag: "list"})
				sent := request("POST", "/send", string(body))
				if sent.Code != 200 {
					t.Fatalf("send: %d %s", sent.Code, sent.Body.String())
				}
				var accepted protocol.Envelope
				json.Unmarshal(sent.Body.Bytes(), &accepted)
				got := request("GET", "/consume?topic=rt24&tag=list&wait=1s", "")
				if got.Code != 200 {
					t.Fatalf("consume: %d %s", got.Code, got.Body.String())
				}
				var answer protocol.Envelope
				if err := json.Unmarshal(got.Body.Bytes(), &answer); err != nil {
					t.Fatal(err)
				}
				if answer.From != core.DaemonAgent || answer.Re != accepted.ID {
					t.Fatalf("uncorrelated: %+v", answer)
				}
				names := []string{}
				if answer.Body != "No matching records." {
					for _, line := range strings.Split(answer.Body, "\n") {
						names = append(names, strings.Split(line, "\t")[0])
					}
				}
				slices.Sort(names)
				if !reflect.DeepEqual(names, want) {
					t.Fatalf("#bus exposed different records for %s: got %v want %v", who, names, want)
				}
			})
		}
	}
}

func TestDaemonAgentReadOnlyMethodsAndAsks(t *testing.T) {
	b := core.New()
	s, _ := serverFor(t, b, "admin@h")
	if err := b.EstablishDaemonAgent(); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"help", "status", "version", "ls --all", "--help", "manage #bus", "status extra"} {
		got, err := s.daemonAnswer(protocol.Envelope{From: "admin@h", Body: body})
		switch body {
		case "--help", "manage #bus", "status extra":
			if err == nil {
				t.Fatalf("ordinary unsupported input %q was accepted: %s", body, got)
			}
		case "status":
			if err != nil || !strings.HasPrefix(got, fmt.Sprintf("ok: %d records,", b.Status().Records)) || !strings.Contains(got, " readers, up ") {
				t.Fatalf("status: %s %v", got, err)
			}
		case "version":
			if err != nil || !strings.Contains(got, version.String) {
				t.Fatalf("version: %s %v", got, err)
			}
		default:
			if err != nil || got == "" {
				t.Fatalf("%s: %s %v", body, got, err)
			}
		}
	}
	for _, ask := range []string{protocol.AskHelp, protocol.AskHelpJSON, protocol.AskMCP, protocol.AskHelpConfig, protocol.AskHelpSecret, protocol.AskStatus, "not-a-known-ask"} {
		got, err := s.daemonAnswer(protocol.Envelope{From: "admin@h", Ask: ask})
		if err != nil {
			t.Fatal(err)
		}
		if ask == "not-a-known-ask" && got != "unsupported: "+ask {
			t.Fatal(got)
		}
		if ask == protocol.AskStatus && !strings.HasPrefix(got, "ok: ") {
			t.Fatal(got)
		}
		if ask == protocol.AskHelpJSON {
			var data struct{ Methods map[string]any }
			if json.Unmarshal([]byte(got), &data) != nil || len(data.Methods) != 4 {
				t.Fatal(got)
			}
		}
		if ask == protocol.AskMCP && got != `{"tools":[]}` {
			t.Fatal(got)
		}
	}
	if _, err := b.SetUser("admin@h", protocol.User{Name: "paused@h"}, true); err != nil {
		t.Fatal(err)
	}
	b.SetUserState("admin@h", "paused@h", "inactive")
	if _, err := s.daemonAnswer(protocol.Envelope{From: "paused@h", Body: "ls --all"}); err == nil {
		t.Fatal("inactive caller was allowed")
	}
	// Cancellation joins the internal reader promptly instead of leaking it.
	stop := s.ServeDaemonAgent(context.Background())
	ended := make(chan struct{})
	go func() { stop(); close(ended) }()
	select {
	case <-ended:
	case <-time.After(time.Second):
		t.Fatal("reader did not stop")
	}
}

func TestDaemonAgentReplyToAndLiteralAskBoundary(t *testing.T) {
	b := core.New()
	s, _ := serverFor(t, b, "admin@h")
	if err := b.EstablishDaemonAgent(); err != nil {
		t.Fatal(err)
	}
	_, err := b.Register(protocol.Record{Name: "answers@h", Kind: protocol.KindQueue, Owner: "admin@h", Allow: []string{"*"}})
	if err != nil {
		t.Fatal(err)
	}
	stop := s.ServeDaemonAgent(context.Background())
	defer stop()
	sent, err := b.Send(protocol.Envelope{From: "admin@h", To: core.DaemonAgent, Ask: protocol.AskHelp, ReplyTo: &protocol.ReplyTo{Name: "answers@h", Topic: "back", Tag: "help"}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	reply, err := b.ConsumeAs(ctx, "admin@h", "answers@h", "back", "help", true, false)
	if err != nil || reply.Re != sent.ID || !strings.Contains(reply.Body, "status") {
		t.Fatalf("reply_to: %+v %v", reply, err)
	}
	_, err = b.Send(protocol.Envelope{From: "admin@h", To: core.DaemonAgent, Ask: protocol.AskHelp, Body: "--help"})
	if err == nil {
		t.Fatal("ask with body accepted")
	}
	caller, _ := protocol.ParseName("admin@h")
	_, err = s.listRecords(caller, url.Values{"tag": []string{"bad tag"}})
	if err == nil {
		t.Fatal("invalid tag accepted")
	}
}

func TestDaemonAgentSurvivesSQLiteRestartWithoutCredentialOrSpuriousWarnings(t *testing.T) {
	n := startNode(t, filepath.Join(t.TempDir(), "bus.db"))
	if err := n.bus.EstablishDaemonAgent(); err != nil {
		t.Fatal(err)
	}
	if _, err := n.issue("admin@h", core.DaemonAgent); err == nil {
		t.Fatal("issued an external daemon credential")
	}
	if _, err := n.bus.Send(protocol.Envelope{From: "admin@h", To: core.DaemonAgent, Body: "status"}); err != nil {
		t.Fatal(err)
	}
	if err := n.bus.FlushQueues(true); err != nil {
		t.Fatal(err)
	}
	n.srv.Close()
	n = n.restart()
	defer n.st.Close()
	defer n.srv.Close()
	if n.log.has("stored #bus metadata conflicts") {
		t.Fatal(n.log.all())
	}
	if err := n.bus.EstablishDaemonAgent(); err != nil {
		t.Fatal(err)
	}
	e, err := n.bus.Consume(context.Background(), core.DaemonAgent, "", "", false, false)
	if err != nil || e.Body != "status" {
		t.Fatalf("restart lost request: %+v %v", e, err)
	}
	req := httptest.NewRequest("GET", "/ls", nil)
	w := httptest.NewRecorder()
	principal, _ := protocol.ParseName(core.DaemonAgent)
	n.srv.HandlerFor(principal).ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("socket impersonation: %d %s", w.Code, w.Body.String())
	}
}

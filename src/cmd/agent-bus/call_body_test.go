//go:build linux

package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func TestCallBodyUsesOnlyTheAnswerOnAPrivateBus(t *testing.T) {
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens())
	if err != nil {
		t.Fatal(err)
	}
	daemon := api.New(b, tokens, "owner@h")
	defer daemon.Close()
	server := httptest.NewServer(daemon.Handler())
	defer server.Close()
	if _, err := b.SetUser("owner@h", protocol.User{Name: "bob@h", PersonName: "Bob"}, true); err != nil {
		t.Fatal(err)
	}
	token, err := tokens.Issue("bob@h")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []protocol.Record{
		{Name: "#time-simple@h", Kind: protocol.KindAgent, Owner: "owner@h", Allow: []string{"bob@h"}},
		{Name: "#private@h", Kind: protocol.KindAgent, Owner: "owner@h", Allow: []string{"@owner"}},
	} {
		if _, err := b.Register(r); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			e, err := b.Consume(ctx, "#time-simple@h", "", "", false, false)
			if err != nil {
				return
			}
			reply := protocol.Envelope{From: e.To, To: e.From, Topic: e.Topic, Tag: e.Tag, Re: e.ID}
			switch e.Body {
			case "epoch":
				reply.Body = "1700000000"
			case "multiline":
				reply.Body = "first\nsecond\n"
			case "done":
				reply.Receipt = protocol.ReceiptDone
			case "ack":
				reply.Receipt = protocol.ReceiptAck
			default:
				return
			}
			if _, err := b.Send(reply); err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("private agent failed to stop")
		}
	})
	for _, request := range []string{"epoch", "multiline"} {
		t.Run(request, func(t *testing.T) {
			raw, stderr, err := cliPresentation(t, false, server.URL, token, "call", "#time-simple@h", request, "--wait=1s")
			if err != nil || stderr != "" {
				t.Fatalf("envelope call: %v %s", err, stderr)
			}
			var answer protocol.Envelope
			if err := json.Unmarshal([]byte(raw), &answer); err != nil {
				t.Fatal(err)
			}
			out, stderr, err := cliPresentation(t, false, server.URL, token, "call", "#time-simple@h", request, "--body", "--wait=1s")
			// jq -r .body emits the decoded string followed by a newline, including
			// when the string already ends with a newline.
			if err != nil || stderr != "" || out != answer.Body+"\n" {
				t.Fatalf("body differs from jq -r .body: err=%v stderr=%q got=%q want=%q", err, stderr, out, answer.Body+"\n")
			}
		})
	}
	for _, c := range []struct{ target, request, refusal string }{
		{"#time-simple@h", "done", "finished and sent no answer"},
		{"#time-simple@h", "ack", "no answer within"},
		{"#private@h", "epoch", `"kind":"acl"`},
	} {
		t.Run(c.request+" "+c.target, func(t *testing.T) {
			out, stderr, err := cliPresentation(t, false, server.URL, token, "call", c.target, c.request, "--body", "--wait=200ms")
			if err == nil || out != "" || !strings.Contains(stderr, c.refusal) {
				t.Fatalf("receipt/refusal must fail on stderr only: err=%v stdout=%q stderr=%q", err, out, stderr)
			}
		})
	}
}

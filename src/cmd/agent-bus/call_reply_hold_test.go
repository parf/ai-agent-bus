package main

import (
	"context"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// A `call` made under a name whose inbox a session's face reads unfiltered must
// get its own answer: the agent answers the moment it consumes, before the CLI
// has started its filtered read, and the face is blocked on the same inbox.
// See docs/04-messaging.md#request-and-reply.
func TestCallKeepsItsAnswerFromTheFaceReadingItsInbox(t *testing.T) {
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens(), "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	daemon := api.New(b, tokens, "owner@h")
	defer daemon.Close()
	server := httptest.NewServer(daemon.Handler())
	defer server.Close()
	if _, err = b.SetUser("owner@h", protocol.User{Name: "bob@h", PersonName: "Bob"}, true); err != nil {
		t.Fatal(err)
	}
	token, err := tokens.Issue("bob@h")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "#fast@h", Kind: protocol.KindAgent, Owner: "owner@h", Allow: []string{"bob@h"}}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// The agent: answers at once, with an ack first, as a runner agent can.
	go func() {
		for ctx.Err() == nil {
			e, err := b.Consume(ctx, "#fast@h", "", "", false, false)
			if err != nil {
				return
			}
			_, _ = b.Send(protocol.Envelope{From: "#fast@h", To: e.From, Topic: e.Topic, Tag: e.Tag, Re: e.ID, Receipt: protocol.ReceiptAck})
			_, _ = b.Send(protocol.Envelope{From: "#fast@h", To: e.From, Topic: e.Topic, Tag: e.Tag, Re: e.ID, Body: "fast answer"})
		}
	}()
	// The face: an unfiltered reader of bob's inbox, restarted after every
	// message it takes, which it records.
	taken := make(chan protocol.Envelope, 64)
	go func() {
		for ctx.Err() == nil {
			e, err := b.Consume(ctx, "bob@h", "", "", false, false)
			if err != nil {
				return
			}
			taken <- e
		}
	}()
	for i := 0; i < 5; i++ {
		deadline, stop := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(deadline, os.Args[0], "-test.run=^TestFlagCLIHelper$", "--", "call", "#fast@h", "--wait", "2s", "status")
		env := []string{}
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "AGENT_BUS_") {
				env = append(env, v)
			}
		}
		cmd.Env = append(env, "AB_CLI_FLAG_HELPER=1", "GORACE=atexit_sleep_ms=0", "AGENT_BUS_ADDR="+server.URL, "AGENT_BUS_TOKEN="+token, "XDG_CACHE_HOME="+t.TempDir())
		out, err := cmd.Output()
		stop()
		if err != nil || !strings.Contains(string(out), "fast answer") {
			t.Fatalf("call %d lost its answer: err=%v out=%q", i, err, out)
		}
		select {
		case e := <-taken:
			t.Fatalf("call %d: the face took %+v from the caller's inbox", i, e)
		default:
		}
	}
}

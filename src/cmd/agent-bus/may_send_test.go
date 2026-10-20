package main

import (
	"bytes"
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

func TestMaySendCLIUsesAuthenticatedCallerAndQueuesNothing(t *testing.T) {
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens(), "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	daemon := api.New(b, tokens, "owner@h")
	defer daemon.Close()
	server := httptest.NewServer(daemon.Handler())
	defer server.Close()
	if _, err := b.SetUser("owner@h", protocol.User{Name: "bob@h"}, true); err != nil {
		t.Fatal(err)
	}
	for _, rec := range []protocol.Record{
		{Name: "#open@h", Kind: protocol.KindAgent, Owner: "owner@h", Allow: []string{"bob@h"}},
		{Name: "#closed@h", Kind: protocol.KindAgent, Owner: "owner@h"},
	} {
		if _, err := b.Register(rec); err != nil {
			t.Fatal(err)
		}
	}
	token, err := tokens.Issue("bob@h")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		ok   bool
		want string
	}{
		{[]string{"may-send", "#open@h"}, true, `{"may_send":true}`},
		{[]string{"may-send", "#closed@h"}, false, "bob@h may not send"},
		{[]string{"may-send", "#missing@h"}, false, "no such receiver"},
		{[]string{"may-send"}, false, "one target name"},
		{[]string{"may-send", "#open@h", "#closed@h"}, false, "one target name"},
		{[]string{"may-send", "#open@h", "--from", "owner@h"}, false, "unknown option"},
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, os.Args[0], append([]string{"-test.run=^TestFlagCLIHelper$", "--"}, tc.args...)...)
		for _, e := range os.Environ() {
			if !strings.HasPrefix(e, "AGENT_BUS_") {
				cmd.Env = append(cmd.Env, e)
			}
		}
		cmd.Env = append(cmd.Env, "AB_CLI_FLAG_HELPER=1", "GORACE=atexit_sleep_ms=0", "AGENT_BUS_ADDR="+server.URL, "AGENT_BUS_TOKEN="+token)
		var out, stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		err := cmd.Run()
		cancel()
		if tc.ok {
			if err != nil || strings.TrimSpace(out.String()) != tc.want || stderr.Len() != 0 {
				t.Fatalf("%v: %v out=%s stderr=%s", tc.args, err, &out, &stderr)
			}
		} else if err == nil || out.Len() != 0 || !strings.Contains(stderr.String(), tc.want) {
			t.Fatalf("%v: refusal must be stderr/nonzero: %v out=%s stderr=%s", tc.args, err, &out, &stderr)
		}
	}
	if b.Status().Queued != 0 || len(b.Recent("owner@h")) != 0 || len(b.Status().Refused) != 0 {
		t.Fatal("CLI dry runs changed messages or counters")
	}
}

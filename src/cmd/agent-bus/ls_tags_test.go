package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// The real CLI against a real daemon handler: repeated --tag options on ls,
// and repeated --tagged on call, select the records carrying every tag, the
// same as --tag=a,b. The answers are COUNTED, so a listing that kept only the
// last tag (three records) or all of them cannot pass.
func TestRepeatedTagOptionsSelectRecordsCarryingEveryTag(t *testing.T) {
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens(), "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	daemon := api.New(b, tokens, "owner@h")
	defer daemon.Close()
	server := httptest.NewServer(daemon.Handler())
	defer server.Close()
	token, err := tokens.Issue("owner@h")
	if err != nil {
		t.Fatal(err)
	}
	for name, tags := range map[string][]string{
		"#only-a@h": {"alpha"}, "#only-b@h": {"beta"}, "#both-1@h": {"alpha", "beta"}, "#both-2@h": {"alpha", "beta", "gamma"}, "#neither@h": {"gamma"},
	} {
		if _, err := b.Register(protocol.Record{Name: name, Kind: protocol.KindAgent, Owner: "owner@h", Tags: tags}); err != nil {
			t.Fatal(err)
		}
	}
	run := func(args ...string) string {
		t.Helper()
		deadline, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		cmd := exec.CommandContext(deadline, os.Args[0], append([]string{"-test.run=^TestFlagCLIHelper$", "--"}, args...)...)
		env := []string{}
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "AGENT_BUS_") {
				env = append(env, v)
			}
		}
		cmd.Env = append(env, "AB_CLI_FLAG_HELPER=1", "GORACE=atexit_sleep_ms=0", "AGENT_BUS_ADDR="+server.URL, "AGENT_BUS_TOKEN="+token, "XDG_CACHE_HOME="+t.TempDir())
		out, _ := cmd.CombinedOutput() // a call with no answerer ends in an error; the names are what is counted
		return string(out)
	}
	listed := func(args ...string) []string {
		t.Helper()
		out := run(append([]string{"ls", "--kind=agent"}, args...)...)
		var recs []protocol.Record
		if err := json.Unmarshal([]byte(out), &recs); err != nil {
			t.Fatalf("ls %v: %v\n%s", args, err, out)
		}
		var names []string
		for _, r := range recs {
			if strings.HasSuffix(r.Name, "@h") {
				names = append(names, r.Name)
			}
		}
		slices.Sort(names)
		return names
	}
	both := []string{"#both-1@h", "#both-2@h"}
	for _, args := range [][]string{
		{"--tag=alpha", "--tag=beta"},
		{"--tag", "alpha", "--tag", "beta"},
		{"--tag=beta", "--tag=alpha"},
		{"--tag=alpha,beta"},
		{"--tag=alpha", "--tag=beta", "--tag=alpha"},
	} {
		if got := listed(args...); !slices.Equal(got, both) {
			t.Errorf("ls %v listed %v, want exactly %v", args, got, both)
		}
	}
	if got := listed("--tag=alpha"); len(got) != 3 {
		t.Errorf("ls --tag=alpha listed %v, want its 3 records as before", got)
	}
	if got := listed("--tag=alpha", "--tag=nosuch"); len(got) != 0 {
		t.Errorf("an unknown second tag listed %v, want none", got)
	}
	// call --tagged names every visible agent it reaches; nobody answers, so each is listed as not answering.
	for _, args := range [][]string{
		{"call", "--tagged=alpha", "--tagged=beta", "--wait=300ms", "x"},
		{"call", "--tagged=alpha,beta", "--wait=300ms", "x"},
	} {
		out := run(args...)
		for _, name := range both {
			if strings.Count(out, "- "+name+":") != 1 {
				t.Errorf("%v: %s not reached exactly once:\n%s", args, name, out)
			}
		}
		for _, name := range []string{"#only-a@h", "#only-b@h", "#neither@h"} {
			if strings.Contains(out, name) {
				t.Errorf("%v reached %s, which lacks a tag:\n%s", args, name, out)
			}
		}
	}
}

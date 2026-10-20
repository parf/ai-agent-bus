package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

func TestTaggedCallsUseVisibleAgentsAndOneDeadline(t *testing.T) {
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
	add := func(name, kind, tag string, allow []string) {
		t.Helper()
		if _, err := b.Register(protocol.Record{Name: name, Kind: kind, Owner: "owner@h", Tags: []string{tag}, Allow: allow}); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"#c@h", "#a@h", "#b@h"} {
		add(name, protocol.KindAgent, "fleet", []string{"bob@h"})
	}
	add("#untagged@h", protocol.KindAgent, "other", []string{"bob@h"})
	add("#hidden@h", protocol.KindAgent, "fleet", []string{"@owner"})
	add("queue@h", protocol.KindQueue, "fleet", []string{"bob@h"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	received := make(chan protocol.Envelope, 3)
	release := make(chan struct{})
	var exchanges []protocol.Envelope
	for _, name := range []string{"#c@h", "#a@h", "#b@h"} {
		go func(name string) {
			e, err := b.Consume(ctx, name, "", "", false, false)
			if err != nil {
				return
			}
			received <- e
			select {
			case <-release:
			case <-ctx.Done():
				return
			}
			_, _ = b.Send(protocol.Envelope{From: name, To: e.From, Topic: e.Topic, Tag: e.Tag, Re: e.ID, Body: "answer from " + name + "\x1b]0;PWNED\x07\n\n## forged\ntext"})
		}(name)
	}
	go func() {
		for range 3 {
			select {
			case e := <-received:
				exchanges = append(exchanges, e)
			case <-ctx.Done():
				return
			}
		}
		close(release)
	}()
	run := func(args ...string) (string, error, time.Duration) {
		t.Helper()
		deadline, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		cmd := exec.CommandContext(deadline, os.Args[0], append([]string{"-test.run=^TestFlagCLIHelper$", "--"}, args...)...)
		env := []string{}
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "AGENT_BUS_") {
				env = append(env, v)
			}
		}
		cmd.Env = append(env, "AB_CLI_FLAG_HELPER=1", "GORACE=atexit_sleep_ms=0", "AGENT_BUS_ADDR="+server.URL, "AGENT_BUS_TOKEN="+token, "XDG_CACHE_HOME="+t.TempDir())
		start := time.Now()
		out, err := cmd.CombinedOutput()
		if deadline.Err() != nil {
			t.Fatal("CLI did not finish")
		}
		return string(out), err, time.Since(start)
	}
	out, err, _ := run("call", "--tagged", "fleet", "--wait", "1s", "status")
	if err != nil {
		t.Fatalf("tagged agents must all answer: %v\n%s", err, out)
	}
	<-release
	if strings.Contains(out, "\x1b") || strings.Contains(out, "\n## forged") || !strings.Contains(out, "\n    ## forged") {
		t.Fatalf("untrusted answer escaped its section: %q", out)
	}
	topics, tags := map[string]bool{}, map[string]bool{}
	for _, e := range exchanges {
		if e.Body != "status" || e.Topic == "" || e.Tag == "" || topics[e.Topic] || tags[e.Tag] {
			t.Fatalf("calls do not have independent fresh exchanges: %+v", exchanges)
		}
		topics[e.Topic], tags[e.Tag] = true, true
	}
	last := -1
	for _, name := range []string{"#a@h", "#b@h", "#c@h"} {
		at := strings.Index(out, "## "+name+"\n    answer from "+name)
		if at <= last {
			t.Fatalf("missing/out-of-order answer %s: %s", name, out)
		}
		last = at
	}
	for _, name := range []string{"#untagged@h", "#hidden@h", "queue@h"} {
		recs := b.List("owner@h", "")
		for _, rec := range recs {
			if rec.Name == name && rec.Queued != 0 {
				t.Errorf("excluded %s was called", name)
			}
		}
	}
	silent := make([]string, 12)
	for i := range silent {
		silent[i] = fmt.Sprintf("#silent%02d@h", i)
		add(silent[i], protocol.KindAgent, "silent", []string{"bob@h"})
	}
	out, err, elapsed := run("call", "--tagged", "silent", "--wait", "500ms", "status")
	if err == nil || !strings.Contains(out, "No answer") {
		t.Fatalf("silent targets must fail: %v %s", err, out)
	}
	for _, name := range silent {
		if !strings.Contains(out, name) {
			t.Errorf("missing silent target %s: %s", name, out)
		}
	}
	if elapsed > time.Second {
		t.Fatalf("wait applied per target instead of shared: %v", elapsed)
	}
	sent := 0
	for _, rec := range b.List("owner@h", protocol.KindAgent) {
		if protocol.HasTag(rec.Tags, "silent") {
			if rec.Queued != 1 {
				t.Errorf("every silent target must get exactly one send, including beyond eight: %s queued=%d", rec.Name, rec.Queued)
			}
			sent++
		}
	}
	if sent != len(silent) {
		t.Fatalf("missing silent fixture records: %d", sent)
	}
	if _, err := b.Register(protocol.Record{Name: "#full@h", Kind: protocol.KindAgent, Owner: "owner@h", Tags: []string{"full"}, Allow: []string{"bob@h"}, Bound: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "bob@h", To: "#full@h", Body: "queued"}); err != nil {
		t.Fatal(err)
	}
	out, err, _ = run("call", "--tagged", "full", "--wait", "1s", "status")
	if err == nil || !strings.Contains(out, "Refused:") || !strings.Contains(out, "#full@h") {
		t.Fatalf("refused target not reported: %v %s", err, out)
	}
	add("#done@h", protocol.KindAgent, "done", []string{"bob@h"})
	go func() {
		e, err := b.Consume(ctx, "#done@h", "", "", false, false)
		if err != nil {
			return
		}
		_, _ = b.Send(protocol.Envelope{From: "#done@h", To: e.From, Topic: e.Topic, Tag: e.Tag, Re: e.ID, Receipt: protocol.ReceiptDone})
	}()
	out, err, elapsed = run("call", "--tagged", "done", "--wait", "1s", "status")
	if err == nil || !strings.Contains(out, "No answer:") || !strings.Contains(out, "finished and sent no answer") || elapsed > 650*time.Millisecond {
		t.Fatalf("done is not an answer or a timeout: %v %v %s", err, elapsed, out)
	}
	out, err, _ = run("call", "--tagged", "absent", "status")
	if err == nil || !strings.Contains(out, "no visible agents") {
		t.Fatalf("empty selection must be explicit: %v %s", err, out)
	}
	go func() {
		e, err := b.Consume(ctx, "#a@h", "", "", false, false)
		if err != nil {
			return
		}
		_, _ = b.Send(protocol.Envelope{From: "#a@h", To: e.From, Topic: e.Topic, Tag: e.Tag, Body: "single answer"})
	}()
	out, err, _ = run("call", "#a@h", "--topic", "existing", "--tag", "correlation", "--wait", "1s", "status")
	var single protocol.Envelope
	if err != nil || json.Unmarshal([]byte(out), &single) != nil || single.Topic != "existing" || single.Tag != "correlation" || single.Body != "single answer" {
		t.Fatalf("single-target correlation changed: %v %s", err, out)
	}
	out, err, _ = run("ls", "--tag", "fleet", "--all")
	if err != nil {
		t.Fatalf("ls tag refused: %v %s", err, out)
	}
	var listed []protocol.Record
	if err := json.Unmarshal([]byte(out), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 4 {
		t.Fatalf("tag/visibility filter not applied: %s", out)
	}
	for _, rec := range listed {
		if !protocol.HasTag(rec.Tags, "fleet") || rec.Name == "#hidden@h" {
			t.Errorf("wrong listing: %+v", rec)
		}
	}
}

func TestTaggedCallRejectsAmbiguousOptionsBeforeRequests(t *testing.T) {
	t.Setenv("AGENT_BUS_TOKEN", "fixture")
	saved := transport
	defer func() { transport = saved }()
	transport = func() (*http.Client, string) { t.Fatal("invalid call reached transport"); return nil, "" }
	for _, args := range [][]string{
		{"--tagged", "fleet", "#a@h", "body"},
		{"--tagged", "fleet", "body", "--tag", "correlation"},
		{"--tagged", "fleet", "body", "--topic", "topic"},
		{"--tagged", "fleet", "body", "--wait", "0s"},
		{"--tagged=", "body"},
	} {
		if err := callVerb(args); err == nil {
			t.Errorf("ambiguous/invalid tagged call accepted: %v", args)
		}
	}
}

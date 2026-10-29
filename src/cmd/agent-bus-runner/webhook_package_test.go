package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/sandbox"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

type webhookTestLog struct{ t *testing.T }

func (w webhookTestLog) Write(p []byte) (int, error) { w.t.Log(string(p)); return len(p), nil }

// Use the real runner keeper and actual bubblewrap network policy, with a
// disposable API and no host unit/journal writes. Cgroup enforcement has its
// own runner checks; this fixture exercises package/load/launch/network/reload.
func TestWebhookKeeperBindsHostLoopbackAndStopsBeforeReplacement(t *testing.T) {
	if err := sandbox.ProbeBwrap(); err != nil {
		t.Skipf("actual runner network proof requires bubblewrap: %v", err)
	}
	root := t.TempDir()
	agents := filepath.Join(root, "agents")
	dest := filepath.Join(agents, "webhook")
	cmd := exec.Command("sh", filepath.Join("..", "..", "agents", "webhook", "package.sh"), dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("package: %v: %s", err, out)
	}
	installed, err := load(agents, filepath.Join(root, "host"))
	if err != nil || len(installed) != 1 {
		t.Fatalf("load: %v %+v", err, installed)
	}
	in := installed[0]
	if in.Refused != "" || in.Agent.Serves != "itself" || in.Span.Min != 1 || in.Span.Max != 1 || !wantsNet(in) {
		t.Fatalf("webhook kept network contract lost: refused=%q serves=%q span=%+v network=%v", in.Refused, in.Agent.Serves, in.Span, wantsNet(in))
	}
	b := core.New()
	b.Restore(ports.Snapshot{Clean: true, Users: []protocol.User{{Name: "owner@h", Status: "active"}}, Records: []protocol.Record{{Name: "owner@h", Kind: protocol.KindUser, Owner: "owner@h", Full: protocol.OverflowStrict}}})
	b.Persistence(memory.NewState())
	for _, rec := range []protocol.Record{{Name: "#fixture@h", Kind: protocol.KindAgent, Owner: "owner@h", Allow: []string{"@owner"}}, {Name: "events@h", Kind: protocol.KindQueue, Owner: "owner@h", Allow: []string{"#fixture@h"}}} {
		if _, err = b.Register(rec); err != nil {
			t.Fatal(err)
		}
	}
	cfg := []byte(`[{"name":"event","path":"/event","target":"events@h"}]`)
	if _, err = b.Configure("#fixture@h", "owner@h", cfg); err != nil {
		t.Fatal(err)
	}
	tokens, err := auth.Load(memory.NewTokens(), "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	server := api.New(b, tokens, "owner@h")
	defer server.Close()
	token, err := tokens.Issue("#fixture@h")
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	as := &client{http: httpServer.Client(), base: httpServer.URL, token: token}
	r := &runner{ctx: context.Background(), name: "#runner@h", host: filepath.Join(root, "host"), agents: agents, addr: httpServer.URL, say: webhookTestLog{t}, served: map[string]*served{}}
	in.Host.Name = "#fixture@h"
	env := []string{"WEBHOOK_LISTEN=127.0.0.1:0"}
	r.mu.Lock()
	s, err := r.adopt(r.ctx, in, "#fixture@h", as, env, nil, sandbox.Bwrap{}, &jail{})
	if err == nil {
		r.served[s.name] = s
	}
	r.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.kept.stop() }()
	address := ""
	sequence := 0
	waitReady := func() {
		t.Helper()
		end := time.Now().Add(4 * time.Second)
		for time.Now().Before(end) {
			sequence++
			tag := strconv.Itoa(sequence)
			if _, e := b.Send(protocol.Envelope{From: "owner@h", To: s.name, Ask: protocol.AskStatus, Topic: "ready", Tag: tag}); e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			for {
				message, e := b.ConsumeAs(ctx, "owner@h", "owner@h", "ready", tag, true, false)
				if e != nil {
					break
				}
				if message.Receipt != "" {
					continue
				}
				match := regexp.MustCompile(`^ok: listening (127\.0\.0\.1:[0-9]+);`).FindStringSubmatch(message.Body)
				if len(match) == 2 {
					address = match[1]
					cancel()
					return
				}
				break
			}
			cancel()
		}
		t.Fatal("runner/Bwrap webhook never reported its actual port-0 listener")
	}
	waitReady()
	response, err := http.Post("http://"+address+"/event", "text/plain", strings.NewReader("exact fixture body"))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if response.StatusCode != 202 {
		t.Fatalf("post status %d", response.StatusCode)
	}
	rec, ok := b.Lookup("owner@h", "events@h")
	if !ok || rec.In != 1 {
		t.Fatalf("one post must yield one bus send: %+v", rec)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	message, err := b.ConsumeAs(ctx, "owner@h", "events@h", "", "", false, false)
	if err != nil || message.From != "#fixture@h" || message.Body != "exact fixture body" {
		t.Fatalf("own identity/raw body: %v %+v", err, message)
	}
	// An unrelated second process cannot share this port while the first drains.
	sock, err := net.Listen("tcp", address)
	if err == nil {
		sock.Close()
		t.Fatal("active listener unexpectedly allowed a second bind")
	}
	// The runner's retirement path waits for the old process before replacement.
	s.kept.stop()
	r.mu.Lock()
	s.kept = r.keep(s, token)
	r.mu.Unlock()
	waitReady()
	if n := s.kept.count(); n != 1 {
		t.Fatalf("replacement kept %d processes", n)
	}
	c := &http.Client{Timeout: time.Second}
	resp, err := c.Post("http://"+address+"/event", "text/plain", strings.NewReader("after replacement"))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	rec, _ = b.Lookup("owner@h", "events@h")
	if resp.StatusCode != 202 || rec.In != 2 {
		t.Fatalf("replacement did not deliver exactly once: status=%d in=%d", resp.StatusCode, rec.In)
	}
}

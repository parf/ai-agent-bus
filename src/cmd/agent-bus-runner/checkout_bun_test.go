package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

// The source agent, client and dependency must all be outside /usr: its
// blanket system-program bind otherwise conceals a missing checkout read.
func TestGitBundledBunAnswersWithTheDefaultSandbox(t *testing.T) {
	if err := sandbox.ProbeBwrap(); err != nil {
		t.Skipf("checkout Bun proof requires bubblewrap: %v", err)
	}
	root := t.TempDir()
	if strings.HasPrefix(root, "/usr/") {
		t.Fatal("checkout Bun proof must be outside /usr")
	}
	src := filepath.Join(root, "src")
	writeFile(t, filepath.Join(root, ".git"), "gitdir: fixture\n")
	writeFile(t, filepath.Join(src, "go.mod"), "module fixture\n")
	for _, path := range []string{"agents/slack", "agents/shared", "clients/typescript/src", "internal/version"} {
		if err := os.CopyFS(filepath.Join(src, path), os.DirFS(filepath.Join("..", "..", path))); err != nil {
			t.Fatal(err)
		}
	}
	// Copy the actual runtime dependency, never a link back into /usr and
	// never a precompiled bundle that could hide the client's import graph.
	dependency := filepath.Join("..", "..", "clients", "typescript", "node_modules", "lossless-json")
	if err := os.CopyFS(filepath.Join(src, "clients", "typescript", "node_modules", "lossless-json"), os.DirFS(dependency)); err != nil {
		t.Fatal(err)
	}
	host := filepath.Join(root, "runner")
	writeFile(t, filepath.Join(host, "agents.json"), `{"slack":{"enabled":true}}`)
	r := &runner{ctx: context.Background(), host: host, agents: filepath.Join(host, "agent.d"), bundled: filepath.Join(src, "agents"), say: io.Discard}
	list, err := loadSources(r.sources(), host)
	if err != nil {
		t.Fatal(err)
	}
	var in installed
	for _, candidate := range list {
		if candidate.Key == "slack" {
			in = candidate
		}
	}
	if in.Key != "slack" || in.Refused != "" || !in.enabled() || in.Agent.Serves != "itself" {
		t.Fatalf("enabled source Slack: %+v", in)
	}
	box, why := r.sandboxFor(in)
	if why != "" || box.Name() != "bwrap" {
		t.Fatalf("default sandbox: %v %s", box, why)
	}

	// The provider lives INSIDE the child's isolated network namespace. The
	// bus uses a bound Unix socket; there is no shared network or host route.
	const provider = "http://127.0.0.1:34567"
	b := core.New()
	b.Restore(ports.Snapshot{Clean: true, Users: []protocol.User{{Name: "owner@h", Status: "active"}}, Records: []protocol.Record{{Name: "owner@h", Kind: protocol.KindUser, Owner: "owner@h", Full: protocol.OverflowStrict}}})
	b.Persistence(memory.NewState())
	const name = "#fixture@h"
	if _, err := b.Register(protocol.Record{Name: name, Kind: protocol.KindAgent, Owner: "owner@h", Allow: []string{"@owner"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetSecret(name, "owner@h", "SLACK_BOT_TOKEN=fixture\nSLACK_APP_TOKEN=fixture-app\nSLACK_API_URL="+provider+"\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Configure(name, "owner@h", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	tokens, err := auth.Load(memory.NewTokens(), "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	server := api.New(b, tokens, "owner@h")
	t.Cleanup(server.Close)
	token, err := tokens.Issue(name)
	if err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(shortSocketDir(t), "bus.sock")
	listener, err := listenTestUnix(t, socket)
	if err != nil {
		t.Fatal(err)
	}
	bus := &http.Server{Handler: server.Handler(), ConnContext: api.ConnContext}
	go bus.Serve(listener)
	t.Cleanup(func() { bus.Close() })
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	t.Cleanup(transport.CloseIdleConnections)
	r.addr = socket
	work := filepath.Join(host, "work", "slack")
	if err := os.MkdirAll(work, 0700); err != nil {
		t.Fatal(err)
	}
	bootstrap := filepath.Join(work, "provider-and-agent.ts")
	writeFile(t, bootstrap, fmt.Sprintf(`
const originalFetch = globalThis.fetch;
globalThis.fetch = (input, ...rest) => {
  const url = new URL(input instanceof Request ? input.url : String(input));
  if (url.hostname !== "127.0.0.1" || url.port !== "34567") {
    console.error("non-local provider attempted: refused by J54 fixture");
    process.exit(70);
  }
  return originalFetch(input, ...rest);
};
const fake = Bun.serve({hostname:"127.0.0.1", port:34567, async fetch(request) {
  const path = new URL(request.url).pathname;
  if (path === "/auth.test") return Response.json({ok:true,user:"checkout_fixture",user_id:"U1",team:"local_fixture",team_id:"T1",url:"http://127.0.0.1:34567"});
  if (path === "/users.list") return Response.json({ok:true,members:[]});
  if (path === "/conversations.list") return Response.json({ok:true,channels:[]});
  if (path === "/apps.connections.open") return Response.json({ok:false,error:"fixture_socket_disabled"});
  console.error("unexpected local provider request"); process.exit(71);
}});
process.on("SIGTERM", () => fake.stop(true));
await import(%q);
`, filepath.Join(src, "agents", "slack", "slack.ts")))
	// Only the fixture bootstrap changes the command: it imports the actual
	// bundled source, with its unmodified imports and bus serve loop.
	s := &served{in: in, name: name, bus: &client{http: &http.Client{Transport: transport}, base: "http://unix", token: token}, dir: r.codeDir(in), work: work, cmd: "bun " + bootstrap, box: box, net: false, jail: &jail{}}
	k := &keeper{r: r, s: s, token: token}
	cmd, _, err := k.command()
	if err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "bun-output")
	log, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		k.signalOne(cmd, s.jail, syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			k.signalOne(cmd, s.jail, syscall.SIGKILL)
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Error("sandboxed Bun process was not reaped")
			}
		}
	})
	sent, err := b.Send(protocol.Envelope{From: "owner@h", To: name, Body: "whoami", Topic: "checkout", Tag: "bun"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	var answer protocol.Envelope
	answers, acks := 0, 0
	for {
		message, err := b.ConsumeAs(ctx, "owner@h", "owner@h", "checkout", "bun", true, false)
		if err != nil {
			diagnostic, _ := os.ReadFile(logPath)
			t.Fatalf("real checkout Bun must answer through its sandbox: %v; output: %s", err, diagnostic)
		}
		if message.Receipt == "ack" {
			acks++
			continue
		}
		answers++
		answer = message
		break
	}
	if answers != 1 || acks != 1 || answer.From != name || answer.Re != sent.ID || !strings.Contains(answer.Body, "checkout_fixture on local_fixture ("+provider+")") {
		t.Fatalf("one real source-agent answer to its request: answers=%d acks=%d sent=%s answer=%+v", answers, acks, sent.ID, answer)
	}
	quiet, stop := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer stop()
	if extra, err := b.ConsumeAs(quiet, "owner@h", "owner@h", "checkout", "bun", true, false); err == nil {
		t.Fatalf("sandboxed Bun answered twice: %+v", extra)
	}
}

package main

import (
	"bytes"
	"context"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func TestDebugDoctorRealBus(t *testing.T) {
	b := core.New()
	b.Persistence(memory.NewState())
	tokens, err := auth.Load(memory.NewTokens(), "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	server := api.New(b, tokens, "owner@h")
	defer server.Close()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	if _, err = b.Register(protocol.Record{Name: "#debug@h", Kind: protocol.KindAgent, Owner: "owner@h", Allow: []string{"@owner"}}); err != nil {
		t.Fatal(err)
	}
	token, err := tokens.Issue("#debug@h")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "debug")
	pack := exec.Command("../../agents/debug/package.sh", dir)
	if out, err := pack.CombinedOutput(); err != nil {
		t.Fatalf("package: %v %s", err, out)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", filepath.Join(dir, "debug.py"))
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "AGENT_BUS_ADDR=" + httpServer.URL, "AGENT_BUS_NAME=#debug@h", "AGENT_BUS_TOKEN=" + token, "PYTHONDONTWRITEBYTECODE=1"}
	var log bytes.Buffer
	cmd.Stdout = &log
	cmd.Stderr = &log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); cmd.Wait() }()
	for _, body := range []string{"selftest", "env", "roles", "help"} {
		if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#debug@h", Body: body}); err != nil {
			t.Fatal(err)
		}
		reply := configurationReplyLong(t, b)
		if strings.Contains(reply.Body, token) {
			t.Fatal("debug leaked credential")
		}
		switch body {
		case "selftest":
			if strings.Contains(reply.Body, "❌") || strings.Count(reply.Body, "✅") != 6 {
				t.Fatalf("selftest: %s", reply.Body)
			}
		case "env":
			if !strings.Contains(reply.Body, "AGENT_BUS_TOKEN=***") {
				t.Fatal("credential mask absent")
			}
		case "roles":
			if !strings.Contains(reply.Body, "owner") {
				t.Fatal("delivered owner absent")
			}
		case "help":
			if !strings.Contains(reply.Body, "may-send") {
				t.Fatal("permission guidance absent")
			}
		}
	}
}

func configurationReplyLong(t *testing.T, b *core.Bus) protocol.Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		e, err := b.Consume(ctx, "owner@h", "", "", false, false)
		if err != nil {
			t.Fatal(err)
		}
		if e.Receipt == "" {
			return e
		}
	}
}

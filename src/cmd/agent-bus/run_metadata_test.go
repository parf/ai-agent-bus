package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/sandbox"
)

func TestRunAnswersPackageAsksWithoutScript(t *testing.T) {
	saved := transport
	defer func() { transport = saved }()
	t.Setenv("AGENT_BUS_TOKEN", "fixture")
	posts := make(chan protocol.Envelope, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/secret" { // an agent with no bus secret
			http.NotFound(w, r)
			return
		}
		if r.URL.Path != "/send" {
			t.Errorf("metadata ask reached %s", r.URL.Path)
			io.WriteString(w, "null")
			return
		}
		var e protocol.Envelope
		if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
			t.Error(err)
		}
		posts <- e
		io.WriteString(w, `{}`)
	}))
	defer server.Close()
	transport = func() (*http.Client, string) { return server.Client(), server.URL }
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "agent.json"), []byte(`{"description":"Manifest fixture","methods":{"read":{"help":"Read facts"}}}`), 0600)
	os.WriteFile(filepath.Join(dir, "env.dist"), []byte("# Token purpose\nTOKEN=sentinel-no-output\n"), 0600)
	ag, err := describe([]string{"#fixture@h", "--algo", "args", "--package-dir", dir, "printf script-was-run"})
	if err != nil {
		t.Fatal(err)
	}
	ag.box = sandbox.Off{}
	ag.work = dir
	ag.say = io.Discard
	for _, ask := range []string{protocol.AskHelp, protocol.AskHelpConfig, protocol.AskHelpSecret, protocol.AskHelpJSON, protocol.AskMCP} {
		handle(ag, protocol.Envelope{From: "caller@h", Ask: ask})
		if len(posts) != 1 {
			t.Fatalf("help should answer once, got %d", len(posts))
		}
		e := <-posts
		if e.Receipt != "" || strings.Contains(e.Body, "script-was-run") || strings.Contains(e.Body, "sentinel-no-output") {
			t.Fatalf("package ask ran script or leaked: %+v", e)
		}
		if json.Valid([]byte(e.Body)) != (ask == protocol.AskHelpJSON || ask == protocol.AskMCP) {
			t.Fatalf("wrong answer form for %s: %s", ask, e.Body)
		}
		want := map[string]string{"help": "Manifest fixture", "help-config": "No Configuration keys", "help-secret": "Token purpose", "help-json": `"methods"`, "mcp": `{"tools":[]}`}[ask]
		if !strings.Contains(e.Body, want) {
			t.Fatalf("%s: %s", ask, e.Body)
		}
	}
	ag.metadata = nil
	handle(ag, protocol.Envelope{From: "caller@h", Ask: protocol.AskMCP})
	if len(posts) != 1 {
		t.Fatalf("unpackaged mcp answered %d times", len(posts))
	}
	if e := <-posts; e.Body != `{"tools":[]}` {
		t.Fatalf("unpackaged mcp ran script: %s", e.Body)
	}
	os.WriteFile(filepath.Join(dir, "agent.json"), []byte(`{"description":"x","methods":{"bad":{"help":"x","typo":true}}}`), 0600)
	if _, err := describe([]string{"#fixture@h", "--package-dir", dir, "true"}); err == nil || !strings.Contains(err.Error(), "typo") {
		t.Fatalf("unknown nested field accepted: %v", err)
	}
}
func TestMetadataAskFlags(t *testing.T) {
	for _, ask := range []string{"help-config", "help-secret", "help-json"} {
		pos, flags, err := split([]string{"#fixture@h", "--" + ask})
		if err != nil {
			t.Fatal(err)
		}
		got, err := askOf(pos, flags)
		if err != nil || got != ask {
			t.Fatalf("ask %s: %s %v", ask, got, err)
		}
		if _, err := askOf([]string{"#fixture@h", "body"}, flags); err == nil {
			t.Fatal("body accepted with ask")
		}
	}
}

func TestRunForwardsNativeHelpAsks(t *testing.T) {
	saved := transport
	defer func() { transport = saved }()
	t.Setenv("AGENT_BUS_TOKEN", "fixture")
	posts := make(chan protocol.Envelope, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/secret" { // an agent with no bus secret
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/config" {
			io.WriteString(w, `{"units":"scoped.service"}`)
			return
		}
		if r.URL.Path != "/send" {
			t.Errorf("unexpected %s", r.URL.Path)
		}
		var e protocol.Envelope
		if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
			t.Error(err)
		}
		posts <- e
		io.WriteString(w, `{}`)
	}))
	defer server.Close()
	transport = func() (*http.Client, string) { return server.Client(), server.URL }
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "agent.json"), []byte(`{"description":"native fixture","help":"native"}`), 0600)
	ag, err := describe([]string{"#fixture@h", "--algo", "args", "--package-dir", dir, `printf 'native:%s:%s' "$AGENT_BUS_ASK" "$AGENT_BUS_CONFIG"; :`})
	if err != nil {
		t.Fatal(err)
	}
	ag.box, ag.work, ag.say = sandbox.Off{}, dir, io.Discard
	for _, ask := range []string{"help", "help-json", "help-config", "help-secret", "mcp"} {
		handle(ag, protocol.Envelope{From: "caller@h", Ask: ask})
		if len(posts) != 1 {
			t.Fatalf("native help answered %d times", len(posts))
		}
		e := <-posts
		if e.Body != "native:"+ask+`:{"units":"scoped.service"}` {
			t.Fatalf("native %s intercepted or config lost: %s", ask, e.Body)
		}
	}
	parsed, err := describe([]string{"#fixture@h", "--no-asks", "true"})
	if err != nil || !parsed.NoAsks {
		t.Fatalf("no-asks flag: %+v %v", parsed, err)
	}
	ag.Script = `echo run >> marker; printf 'native:%s:%s' "$AGENT_BUS_ASK" "$1"; :`
	metadata := ag.metadata
	for _, policy := range []string{"flag", "package", "unpackaged"} {
		ag.metadata = metadata
		ag.NoAsks = policy != "package"
		ag.metadata.Asks = ""
		if policy == "package" {
			ag.metadata.Asks = "none"
		}
		ag.metadata.Help = ""
		if policy == "unpackaged" {
			ag.metadata = nil
		}
		for _, ask := range []string{"help", "help-config", "help-secret", "help-json", "mcp", "status", "unknown"} {
			handle(ag, protocol.Envelope{From: "caller@h", Ask: ask})
			e := <-posts
			if ask == "status" || ask == "unknown" || policy == "unpackaged" && ask != "mcp" {
				if e.Body != "unsupported: "+ask {
					t.Fatal(e.Body)
				}
			}
			if _, err := os.Stat(filepath.Join(dir, "marker")); !os.IsNotExist(err) {
				t.Fatalf("%s %s ran script", policy, ask)
			}
		}
	}
	ag.NoAsks = false
	ag.metadata = metadata
	ag.metadata.Asks = ""
	handle(ag, protocol.Envelope{From: "caller@h", Ask: "status", Body: "discard"})
	if e := <-posts; e.Body != "native:status:" {
		t.Fatal(e.Body)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ag.stop = ctx
	ag.NoAsks = true
	handle(ag, protocol.Envelope{From: "caller@h", Ask: "status"})
	if e := <-posts; !strings.HasPrefix(e.Body, "stopping:") {
		t.Fatal(e.Body)
	}
	if got := scriptStatus(ctx); !strings.HasPrefix(got, "stopping:") {
		t.Fatal(got)
	}
	if _, err := describe([]string{"#fixture@h", "--package-dir", dir, "true"}); err != nil {
		t.Fatal(err)
	}
}

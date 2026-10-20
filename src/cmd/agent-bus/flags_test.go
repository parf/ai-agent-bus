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
	"sync/atomic"
	"testing"
	"time"
)

// Exercise the actual dispatch, including verbs which used to discard flags.
func TestFlagCLIHelper(t *testing.T) {
	if os.Getenv("AB_CLI_FLAG_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"agent-bus"}, os.Args[i+1:]...)
			main()
			os.Exit(0)
		}
	}
	os.Exit(99)
}

func TestEveryVerbRefusesUnknownFlagsBeforeRequests(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "fixture boundary reached", 403)
	}))
	defer server.Close()
	cases := []string{
		"status", "register fixture --kind agent", "unregister fixture", "lock fixture key", "try-lock fixture key", "release fixture key", "extend fixture key", "holders fixture",
		"kv get fixture key", "kv set fixture key value", "kv delete fixture key", "kv inc fixture key", "kv json fixture key []", "ls", "send fixture text", "send --agent", "call fixture text", "consume --wait 1ms", "ack fixture", "done fixture", "reply --to fixture text", "channel create fixture", "publish --channel fixture text", "unsubscribe fixture", "agent-template fixture {}", "secret fixture TOKEN=x", "run fixture /bin/true --sandbox off", "stop fixture", "logs fixture", "manage fixture --descr text", "group @fixture member", "enrol fixture", "enroll fixture", "help", "-h", "--help", "--version", "-version",
	}
	for _, line := range cases {
		t.Run(line, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			args := append([]string{"-test.run=^TestFlagCLIHelper$", "--"}, strings.Fields(line)...)
			args = append(args, "--mistyped=value")
			cmd := exec.CommandContext(ctx, os.Args[0], args...)
			cmd.Env = append(os.Environ(), "AB_CLI_FLAG_HELPER=1", "GORACE=atexit_sleep_ms=0", "AGENT_BUS_ADDR="+server.URL, "AGENT_BUS_TOKEN=fixture", "XDG_CACHE_HOME="+t.TempDir(), "XDG_STATE_HOME="+t.TempDir())
			before := requests.Load()
			out, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatal("flag refusal did not finish")
			}
			if err == nil || !strings.Contains(string(out), "--mistyped") {
				t.Errorf("must refuse the named flag: err=%v output=%.200s", err, out)
			}
			if got := requests.Load() - before; got != 0 {
				t.Errorf("unknown flag reached daemon: %d requests", got)
			}
		})
	}
}

func TestBooleanFlagsLeaveFollowingName(t *testing.T) {
	for _, flag := range []string{"follow", "personal", "share", "network", "int", "json", "add", "replace", "help", "mcp", "force", "all", "template"} {
		pos, f, err := split([]string{"--" + flag, "fixture"})
		if err != nil {
			t.Fatal(err)
		}
		if len(pos) != 1 || pos[0] != "fixture" || !has(f, flag) {
			t.Errorf("--%s swallowed following name: %v %v", flag, pos, f)
		}
	}
}

func TestMisplacedFlagsAreNamed(t *testing.T) {
	cases := []struct {
		run  func([]string) error
		args []string
		flag string
	}{
		{send, []string{"fixture", "text", "--wait=1s"}, "--wait"},
		{callVerb, []string{"fixture", "text", "--ttl=1s"}, "--ttl"},
		{publish, []string{"--channel=fixture", "text", "--topic=x"}, "--topic"},
		{stopVerb, []string{"fixture", "--lines=5"}, "--lines"},
		{lockTake(true), []string{"fixture", "key", "--wait="}, "--wait"},
	}
	for _, c := range cases {
		if err := c.run(c.args); err == nil || !strings.Contains(err.Error(), c.flag) {
			t.Errorf("%v: must refuse %s, got %v", c.args, c.flag, err)
		}
	}
}

func TestDelimiterSendsLiteralOptions(t *testing.T) {
	for _, verb := range []string{"send", "call", "reply", "publish"} {
		t.Run(verb, func(t *testing.T) {
			body := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/status" {
					fmt.Fprint(w, `{"you":"fixture@h"}`)
					return
				}
				if r.URL.Path == "/lookup" {
					fmt.Fprint(w, `{"name":"fixture@h","kind":"agent"}`)
					return
				}
				if r.URL.Path == "/send" {
					var envelope struct {
						Body string `json:"body"`
					}
					if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
						t.Error(err)
					}
					body <- envelope.Body
					fmt.Fprint(w, `{"message_id":"fixture"}`)
					return
				}
				if r.URL.Path == "/consume" {
					fmt.Fprint(w, `{"body":"answered"}`)
					return
				}
				http.Error(w, "unexpected request", 400)
			}))
			defer server.Close()
			var args []string
			switch verb {
			case "send", "call":
				args = []string{verb, "#fixture@h"}
			case "reply":
				args = []string{verb, "--to", "#fixture@h"}
			case "publish":
				args = []string{verb, "--channel", "news@h"}
			}
			args = append(args, "--", "--verbose please", "--agent", "--mistyped")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], append([]string{"-test.run=^TestFlagCLIHelper$", "--"}, args...)...)
			cmd.Env = append(os.Environ(), "AB_CLI_FLAG_HELPER=1", "GORACE=atexit_sleep_ms=0", "AGENT_BUS_ADDR="+server.URL, "AGENT_BUS_TOKEN=fixture", "XDG_CACHE_HOME="+t.TempDir())
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("literal text was refused: %v %.200s", err, out)
			}
			select {
			case text := <-body:
				if text != "--verbose please --agent --mistyped" {
					t.Errorf("literal text changed: %q", text)
				}
			default:
				t.Error("literal text was not sent")
			}
		})
	}
}

func TestMissingValuedFlagsFailBeforeRequests(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); http.Error(w, "fixture boundary", 403) }))
	defer server.Close()
	for _, line := range []string{"send fixture text --topic", "send fixture text --tag --help", "call fixture text --wait", "register fixture --descr", "ls --kind", "consume --inbox", "logs fixture --lines", "enrol fixture --key", "run fixture /bin/true --algo"} {
		t.Run(line, func(t *testing.T) {
			args := strings.Fields(line)
			missing := args[len(args)-1]
			if missing == "--help" {
				missing = "--tag"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], append([]string{"-test.run=^TestFlagCLIHelper$", "--"}, args...)...)
			cmd.Env = append(os.Environ(), "AB_CLI_FLAG_HELPER=1", "GORACE=atexit_sleep_ms=0", "AGENT_BUS_ADDR="+server.URL, "AGENT_BUS_TOKEN=fixture", "XDG_STATE_HOME="+t.TempDir())
			before := requests.Load()
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), missing+" wants a value") {
				t.Errorf("missing value not refused: err=%v output=%.200s", err, out)
			}
			if requests.Load() != before {
				t.Error("missing value reached daemon")
			}
		})
	}
}

func TestExplicitEmptyValuesRemainAvailable(t *testing.T) {
	pos, flags, err := split([]string{"fixture", "--descr=", "--personal=false"})
	if err := only(flags, err, "descr", "personal"); err != nil {
		t.Fatal(err)
	}
	if len(pos) != 1 || pos[0] != "fixture" || !has(flags, "descr") || flags["descr"] != "" || !has(flags, "personal") {
		t.Errorf("explicit clear or boolean presence lost: %v %v", pos, flags)
	}
}

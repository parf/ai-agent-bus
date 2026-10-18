package main

import (
	"context"
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
		"kv get fixture key", "kv set fixture key value", "kv delete fixture key", "kv inc fixture key", "kv json fixture key []", "ls", "send fixture text", "send --agent", "call fixture text", "consume --wait 1ms", "ack fixture", "done fixture", "reply --to fixture text", "channel create fixture", "publish --channel fixture text", "subscribe fixture", "unsubscribe fixture", "agent-template fixture {}", "secret fixture TOKEN=x", "run fixture /bin/true --sandbox off", "stop fixture", "logs fixture", "manage fixture --descr text", "group @fixture member", "enrol fixture", "enroll fixture", "help", "-h", "--help", "--version", "-version",
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
		pos, f := split([]string{"--" + flag, "fixture"})
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

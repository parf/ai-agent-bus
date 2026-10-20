package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
	"golang.org/x/sys/unix"
)

func TestAgentHelpPlainAndTerminalOutput(t *testing.T) {
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens(), "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	daemon := api.New(b, tokens, "owner@h")
	defer daemon.Close()
	server := httptest.NewServer(daemon.Handler())
	defer server.Close()
	if _, err := b.SetUser("owner@h", protocol.User{Name: "bob@h", PersonName: "Bob"}, true); err != nil {
		t.Fatal(err)
	}
	token, err := tokens.Issue("bob@h")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"#helper@h", "#silent@h"} {
		if _, err := b.Register(protocol.Record{Name: name, Kind: protocol.KindAgent, Owner: "owner@h", Allow: []string{"bob@h"}}); err != nil {
			t.Fatal(err)
		}
	}
	bodies := map[string]string{
		"help":        "Service checks\nVersion: fixture\nrestart <unit>: Restart it; role: operate|admin; destructive\n  unit (string): Service name\n",
		"help-config": "units (string): Comma-separated service names\n",
		"help-secret": "BOT_TOKEN (required): A bot password\nAPI_URL (optional): Endpoint\n",
		"help-json":   `{"description":"Service checks","methods":{"restart":{"role":["operate","admin"],"destructive":true}},"config":{"count":{"default":9007199254740993}},"secrets":[],"ready":false}`,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	observed := make(chan protocol.Envelope, 20)
	go func() {
		for {
			e, err := b.Consume(ctx, "#helper@h", "", "", false, false)
			if err != nil {
				return
			}
			observed <- e
			_, _ = b.Send(protocol.Envelope{From: e.To, To: e.From, Topic: e.Topic, Tag: e.Tag, Re: e.ID, Body: bodies[e.Ask]})
		}
	}()
	strip := regexp.MustCompile("\x1b\\[[0-9;]*m")
	run := func(terminal bool, noColour *string, args ...string) (string, string, error) {
		t.Helper()
		deadline, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		cmd := exec.CommandContext(deadline, os.Args[0], append([]string{"-test.run=^TestFlagCLIHelper$", "--"}, args...)...)
		for _, entry := range os.Environ() {
			if !strings.HasPrefix(entry, "AGENT_BUS_") && !strings.HasPrefix(entry, "NO_COLOR=") {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		cmd.Env = append(cmd.Env, "AB_CLI_FLAG_HELPER=1", "GORACE=atexit_sleep_ms=0", "AGENT_BUS_ADDR="+server.URL, "AGENT_BUS_TOKEN="+token, "XDG_CACHE_HOME="+t.TempDir())
		if noColour != nil {
			cmd.Env = append(cmd.Env, "NO_COLOR="+*noColour)
		}
		var stderr, stdout bytes.Buffer
		cmd.Stderr = &stderr
		if !terminal {
			cmd.Stdout = &stdout
			err := cmd.Run()
			return stdout.String(), stderr.String(), err
		}
		fd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
		if err != nil {
			t.Fatal(err)
		}
		master := os.NewFile(uintptr(fd), "pty")
		defer master.Close()
		if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
			t.Fatal(err)
		}
		number, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
		if err != nil {
			t.Fatal(err)
		}
		slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|unix.O_NOCTTY, 0)
		if err != nil {
			t.Fatal(err)
		}
		cmd.Stdout = slave
		if err := cmd.Start(); err != nil {
			slave.Close()
			t.Fatal(err)
		}
		slave.Close()
		out, readErr := io.ReadAll(master)
		err = cmd.Wait()
		if readErr != nil && !errors.Is(readErr, unix.EIO) {
			t.Fatal(readErr)
		}
		if deadline.Err() != nil {
			t.Fatal("help command did not finish")
		}
		return strings.ReplaceAll(string(out), "\r\n", "\n"), stderr.String(), err
	}
	tags := map[string]bool{}
	for _, form := range []string{"", "config", "secret", "json"} {
		args := []string{"help", "#helper@h"}
		ask := "help"
		if form != "" {
			args = append(args, form)
			ask += "-" + form
		}
		out, stderr, err := run(false, nil, args...)
		if err != nil || stderr != "" || strings.Contains(out, "\x1b") {
			t.Fatalf("piped help must be plain: %v stderr=%q out=%q", err, stderr, out)
		}
		if form == "json" {
			if !json.Valid([]byte(out)) || !strings.Contains(out, "\n  \"description\"") || !strings.Contains(out, "9007199254740993") {
				t.Fatalf("pretty JSON lost structure or precision: %s", out)
			}
		} else if out != bodies[ask] {
			t.Fatalf("help must print only answer text: %q", out)
		}
		e := <-observed
		if e.Ask != ask || e.Body != "" || e.Topic != "help" || e.Tag == "" || tags[e.Tag] {
			t.Fatalf("wrong help request: %+v", e)
		}
		tags[e.Tag] = true
	}
	for _, form := range []string{"", "secret", "json"} {
		args := []string{"help", "#helper@h"}
		if form != "" {
			args = append(args, form)
		}
		out, stderr, err := run(true, nil, args...)
		if err != nil || stderr != "" || !strings.Contains(out, "\x1b[") {
			t.Fatalf("terminal help needs colour: %v %q %q", err, stderr, out)
		}
		plain := strip.ReplaceAllString(out, "")
		if form == "json" && !json.Valid([]byte(plain)) {
			t.Fatalf("coloured JSON is not valid after stripping: %q", plain)
		}
		if form == "" && (!strings.Contains(out, "\x1b[33mrole:") || !strings.Contains(out, "\x1b[31mdestructive")) {
			t.Fatalf("role/destructive marks need colour: %q", out)
		}
		if form == "secret" && !strings.Contains(out, "\x1b[1;31m(required)") {
			t.Fatalf("required Secret is not marked: %q", out)
		}
	}
	empty := ""
	for _, test := range []struct {
		name string
		env  *string
		flag bool
	}{{"NO_COLOR", &empty, false}, {"flag", nil, true}} {
		args := []string{"help", "#helper@h"}
		if test.flag {
			args = append(args, "--no-color")
		}
		out, stderr, err := run(true, test.env, args...)
		if err != nil || stderr != "" || out != bodies["help"] || strings.Contains(out, "\x1b") {
			t.Fatalf("%s must disable colour: %v %q %q", test.name, err, stderr, out)
		}
	}
	for _, args := range [][]string{{"help", "#missing@h"}, {"help", "#silent@h", "--wait", "100ms"}} {
		out, stderr, err := run(false, nil, args...)
		if err == nil || out != "" || stderr == "" || strings.HasPrefix(stderr, "{") {
			t.Fatalf("refusal/timeout must be a plain stderr error: %v %q %q", err, out, stderr)
		}
		if args[1] == "#silent@h" && !strings.Contains(stderr, "no answer within") {
			t.Fatalf("missing timeout reason: %q", stderr)
		}
	}
	out, stderr, err := run(false, nil, "call", "#helper@h", "--help")
	var envelope protocol.Envelope
	if err != nil || stderr != "" || json.Unmarshal([]byte(out), &envelope) != nil || envelope.Body != bodies["help"] {
		t.Fatalf("script call help envelope changed: %v %q %q", err, stderr, out)
	}
}

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Timestamp the actual log write, not a later successful poll response.
type restartLog struct {
	lockedBuffer
	reconnected atomic.Int64
}

func (l *restartLog) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte("reconnected after")) {
		l.reconnected.Store(time.Now().UnixNano())
	}
	return l.lockedBuffer.Write(p)
}

// Start at the production retry ceiling without spending a long outage waiting
// for exponential backoff to reach it. Only the first delay is seeded: the cap,
// jitter, dispatch, HTTP transport and daemon are production code.
func TestRunRestartCLIHelper(t *testing.T) {
	if os.Getenv("AB_CLI_FLAG_HELPER") != "1" {
		return
	}
	awayFirst = awayMost
	TestFlagCLIHelper(t)
}

// Restart the real daemon, including its account socket, while the real run
// verb remains alive. Every wait is bounded: a lost reader fails an assertion.
func TestRunKeepsItsNameAndSerialReaderAfterDaemonRestart(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "agent-busd")
	// Compile before the runtime deadline: a cold Go cache is not an outage.
	// The owning go test timeout still bounds a wedged build.
	build := exec.Command("go", "build", "-o", binary, "./cmd/agent-busd")
	build.Dir = "../.."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build disposable daemon: %v\n%s", err, out)
	}
	account, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	env := []string{}
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "AGENT_BUS_") && !strings.HasPrefix(value, "AB_CLI_FLAG_HELPER=") {
			env = append(env, value)
		}
	}
	socket := filepath.Join(dir, "bus.sock")
	client, base := api.Dial(socket)
	client.Timeout = 1500 * time.Millisecond
	defer client.CloseIdleConnections()
	ownerToken := ""
	request := func(method, path string, value any) ([]byte, int, error) {
		var input bytes.Buffer
		if value != nil {
			if err := json.NewEncoder(&input).Encode(value); err != nil {
				return nil, 0, err
			}
		}
		req, err := http.NewRequestWithContext(ctx, method, base+path, &input)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set(api.HeaderToken, ownerToken)
		r, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		return body, r.StatusCode, err
	}
	wait := func(what string, seconds time.Duration, probe func() bool) {
		t.Helper()
		until := time.Now().Add(seconds)
		for time.Now().Before(until) && ctx.Err() == nil {
			if probe() && time.Now().Before(until) {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("%s did not happen within %s", what, seconds)
	}
	startDaemon := func() func() {
		t.Helper()
		log := &lockedBuffer{}
		cmd := exec.CommandContext(ctx, binary, "-socket", socket, "-addr", "127.0.0.1:0", "-owner", "owner@fixture",
			"-db", filepath.Join(dir, "bus.db"), "-create", "-flush-every", "0", "-dashboard", "",
			"-ssh-keys", filepath.Join(dir, "authorized_keys"), "-ssh-token", "/bin/false")
		cmd.Env = env
		cmd.Stdout, cmd.Stderr = log, log
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		stopped := false
		stop := func() {
			if stopped {
				return
			}
			stopped = true
			_ = cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				_ = cmd.Process.Kill()
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Error("disposable daemon did not stop")
				}
			}
		}
		t.Cleanup(stop)
		wait("disposable daemon startup", 5*time.Second, func() bool {
			_, code, err := request("GET", "/status", nil)
			return err == nil && (code == 200 || code == 401)
		})
		return stop
	}
	stopDaemon := startDaemon()
	cli := func(args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, os.Args[0], append([]string{"-test.run=^TestFlagCLIHelper$", "--"}, args...)...)
		cmd.Env = append(append([]string{}, env...), "AB_CLI_FLAG_HELPER=1", "GORACE=atexit_sleep_ms=0",
			"AGENT_BUS_ADDR="+api.UserSocket(dir, account.Username), "XDG_STATE_HOME="+dir,
			"XDG_CACHE_HOME="+dir)
		return cmd
	}
	token, err := cli("token", "owner@fixture").Output()
	if err != nil {
		t.Fatalf("fixture owner token: %v", err)
	}
	ownerToken = strings.TrimSpace(string(token))
	script := filepath.Join(dir, "reply.sh")
	busy, overlap := filepath.Join(dir, "busy"), filepath.Join(dir, "overlap")
	if err := os.WriteFile(script, []byte(fmt.Sprintf("#!/bin/sh\ncat >/dev/null\nmkdir %q || { touch %q; exit 1; }\ntrap 'rmdir %q' EXIT\nsleep .05\nprintf answered\n", busy, overlap, busy)), 0700); err != nil {
		t.Fatal(err)
	}
	log := &restartLog{}
	runner := cli("run", "#restart@fixture", "--algo=json", script)
	runner.Args[1] = "-test.run=^TestRunRestartCLIHelper$"
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("run log:\n%s", log.String())
		}
	})
	runner.Stdout, runner.Stderr = log, log
	if err := runner.Start(); err != nil {
		t.Fatal(err)
	}
	runnerDone := make(chan error, 1)
	go func() { runnerDone <- runner.Wait() }()
	t.Cleanup(func() {
		_ = runner.Process.Signal(syscall.SIGTERM)
		select {
		case <-runnerDone:
		case <-time.After(2 * time.Second):
			_ = runner.Process.Kill()
			select {
			case <-runnerDone:
			case <-time.After(time.Second):
				t.Error("disposable run did not stop")
			}
		}
	})
	reader := func() bool {
		body, code, err := request("GET", "/lookup?name=%23restart%40fixture", nil)
		var r protocol.Record
		return err == nil && code == 200 && json.Unmarshal(body, &r) == nil && r.Name == "#restart@fixture" && r.Readers != nil && *r.Readers == 1
	}
	wait("initial single reader: "+log.String(), 3*time.Second, reader)
	stopDaemon()
	wait("runner observing the outage", 3*time.Second, func() bool { return strings.Contains(log.String(), "daemon away:") })
	restartAt := time.Now()
	startDaemon()
	readyAt := time.Now()
	// No mail is sent yet: a successful pending reader must be visible and
	// logged within the bound, rather than after the healthy 55 s poll ends.
	wait("same-name reader after daemon restart", 10*time.Second, reader)
	wait("reconnect log timestamp before the ten-second deadline", max(time.Until(readyAt.Add(10*time.Second)), time.Millisecond), func() bool {
		return log.reconnected.Load() != 0
	})
	loggedAt := time.Unix(0, log.reconnected.Load())
	if loggedAt.Before(restartAt) || loggedAt.After(readyAt.Add(10*time.Second)) {
		t.Fatalf("reconnect log timestamp %s outside restart bound starting %s", loggedAt, readyAt)
	}
	for i := range 2 {
		body, code, err := request("POST", "/send", protocol.Envelope{To: "#restart@fixture", Body: "probe", Topic: "restart", Tag: fmt.Sprint(i)})
		if err != nil || code != 200 {
			t.Fatalf("send after restart: %d %v %s", code, err, body)
		}
	}
	for i := range 2 {
		body, code, err := request("GET", "/consume?wait=1s&topic=restart&tag="+url.QueryEscape(fmt.Sprint(i)), nil)
		var e protocol.Envelope
		if err != nil || code != 200 || json.Unmarshal(body, &e) != nil || e.Body != "answered" || e.From != "#restart@fixture" {
			t.Fatalf("answer after daemon restart: %d %v %s; run log:\n%s", code, err, body, log.String())
		}
	}
	for i := range 2 {
		body, code, err := request("GET", "/consume?wait=100ms&topic=restart&tag="+url.QueryEscape(fmt.Sprint(i)), nil)
		if err != nil || code != http.StatusNoContent {
			t.Fatalf("extra answer for probe %d: %d %v %s", i, code, err, body)
		}
	}
	wait("one idle reader after both answers", time.Second, reader)
	if _, err := os.Stat(overlap); !os.IsNotExist(err) {
		t.Fatal("run started overlapping script handlers")
	}
}

// A shared or stale reader can prevent a lookup from confirming one reader.
// Recovery must back off instead of making ten lookups a second all poll long.
func TestRecoveryReaderProbeBacksOff(t *testing.T) {
	var lookups atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lookup" || r.URL.Query().Get("name") != "#recover@fixture" {
			t.Errorf("unexpected recovery request: %s", r.URL)
		}
		lookups.Add(1)
		readers := 2
		json.NewEncoder(w).Encode(protocol.Record{Name: "#recover@fixture", Readers: &readers})
	}))
	defer server.Close()
	t.Setenv("AGENT_BUS_TOKEN", "fixture-token")
	saved := transport
	transport = func() (*http.Client, string) { return server.Client(), server.URL }
	defer func() { transport = saved }()
	l := &link{name: "#recover@fixture", say: io.Discard}
	// Keep the healthy watcher alive beyond its first probe delay: stopping it
	// immediately would also hide an unwanted watcher behind that delay.
	healthy, endHealthy := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer endHealthy()
	stopHealthy := l.watchReader(healthy)
	<-healthy.Done()
	stopHealthy()
	if n := lookups.Load(); n != 0 {
		t.Fatalf("healthy poll made %d lookups", n)
	}
	l.since = time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stop := l.watchReader(ctx)
	<-ctx.Done()
	stop()
	// 100, 200, 400, 800 ms: at most four requests before this deadline.
	if n := lookups.Load(); n < 1 || n > 4 {
		t.Fatalf("recovery probe made %d lookups in two seconds; want 1..4 with backoff", n)
	}
	if l.registered {
		t.Fatal("two readers were accepted as one")
	}
}

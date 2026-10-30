package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/sandbox"
)

// keptFixture keeps one process of body, a shell script, and returns the
// runner, the agent and its keeper; every process is killed at cleanup.
func keptFixture(t *testing.T, body string) (*runner, *served, *keeper, string) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "agent.sh")
	if err := os.WriteFile(script, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	r := &runner{ctx: context.Background(), say: io.Discard, served: map[string]*served{}}
	s := &served{bus: unconfiguredClient(), name: "#fixture@h", cmd: "/bin/sh " + strconv.Quote(script), work: dir, dir: dir, box: sandbox.Off{}, jail: &jail{},
		in: installed{Key: "fixture", Agent: agentFile{Deadline: "80ms"}, Span: span{Min: 1, Max: 1}}}
	r.installed = []installed{s.in}
	r.mu.Lock()
	r.served[s.name] = s
	k := r.keep(s, "")
	s.kept = k
	r.mu.Unlock()
	t.Cleanup(func() {
		k.cancel()
		k.mu.Lock()
		for cmd := range k.running {
			if cmd.Process != nil {
				syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		}
		k.mu.Unlock()
		k.wg.Wait()
	})
	return r, s, k, dir
}

// await waits for path to hold want, failing after within.
func await(t *testing.T, path, want string, within time.Duration) {
	t.Helper()
	end := time.Now().Add(within)
	for {
		raw, _ := os.ReadFile(path)
		if string(raw) == want {
			return
		}
		if time.Now().After(end) {
			t.Fatalf("%s holds %q, want %q", filepath.Base(path), raw, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

const keptLoop = "printf s >> starts\nwhile :; do sleep .02; done\n"

func TestKeptProcessHandlingSighupKeepsServing(t *testing.T) {
	_, _, k, dir := keptFixture(t, "trap 'printf h >> hups' HUP\n"+keptLoop)
	await(t, filepath.Join(dir, "starts"), "s", time.Second)
	k.hangup()
	await(t, filepath.Join(dir, "hups"), "h", time.Second)
	time.Sleep(150 * time.Millisecond)
	await(t, filepath.Join(dir, "starts"), "s", 0)
}

// The restart backoff starts at a second: well inside it is "at once".
func TestKeptProcessNotHandlingSighupIsStartedAgainAtOnce(t *testing.T) {
	_, _, k, dir := keptFixture(t, keptLoop)
	await(t, filepath.Join(dir, "starts"), "s", time.Second)
	k.hangup()
	await(t, filepath.Join(dir, "starts"), "ss", backoffFirst/2)
}

func TestKilledKeptProcessIsRestartedAndReported(t *testing.T) {
	r, _, k, dir := keptFixture(t, keptLoop)
	await(t, filepath.Join(dir, "starts"), "s", time.Second)
	k.signal(syscall.SIGKILL)
	await(t, filepath.Join(dir, "starts"), "ss", 3*backoffFirst)
	r.mu.Lock()
	exit := r.installed[0].LastExit
	r.mu.Unlock()
	if exit == nil || exit.Signal != "killed" {
		t.Fatalf("last exit %+v, want one ended by signal killed", exit)
	}
}

// A changed setting reaches the running process as SIGHUP at reload, which
// does not restart it; an unchanged reload sends nothing.
func TestReloadSendsSighupWhenAKeptAgentsSettingsChanged(t *testing.T) {
	r, _, _ := configurationFixture(t, "itself", "trap 'printf h >> \"$AGENT_BUS_WORK/hups\"' HUP\nprintf s >> \"$AGENT_BUS_WORK/starts\"\nwhile :; do sleep .02; done\n")
	r.mu.Lock()
	s := r.served["#fixture@h"]
	r.mu.Unlock()
	if s == nil {
		t.Fatal("fixture not served")
	}
	await(t, filepath.Join(s.work, "starts"), "s", 3*time.Second)
	r.reload()
	time.Sleep(150 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(s.work, "hups")); err == nil {
		t.Fatal("an unchanged reload sent SIGHUP")
	}
	// The host's own entry: agent.json is in the package digest, which restarts.
	if err := os.WriteFile(filepath.Join(r.host, "agents.json"), mustJSON(map[string]any{"fixture": map[string]any{"sandbox": "off", "tags": []string{"changed"}}}), 0600); err != nil {
		t.Fatal(err)
	}
	r.reload()
	await(t, filepath.Join(s.work, "hups"), "h", 2*time.Second)
	await(t, filepath.Join(s.work, "starts"), "s", 0)
}

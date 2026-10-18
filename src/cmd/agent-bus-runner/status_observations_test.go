package main

import (
	"encoding/json"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/sandbox"
)

// Status is a read of the admitted in-memory state: no host probing or metrics
// history is needed to describe how a served agent runs and its limits.
func TestStatusDescribesServingAndAppliedLimits(t *testing.T) {
	on := installed{Key: "limited", Agent: agentFile{Command: "true"}, Host: hostEntry{Limits: &limits{Memory: "2G", Processes: 48, Disk: "6G"}}, Span: span{0, 3}}
	off := installed{Key: "plain", Agent: agentFile{Command: "true"}, Span: span{0, 4}}
	self := installed{Key: "kept", Agent: agentFile{Command: "true", Serves: "itself"}, Span: span{2, 2}}
	disabled := false
	stopped := installed{Key: "disabled", Agent: agentFile{Command: "true"}, Host: hostEntry{Enabled: &disabled}}
	refused := installed{Key: "bad", Agent: agentFile{Command: "true"}, Refused: "not runnable"}
	r := &runner{startedAt: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC), name: "#runner/test@h", realm: "h", agents: "/fixture/agent.d", host: "/fixture/runner", say: io.Discard, installed: []installed{on, off, self, stopped, refused}, served: map[string]*served{}}
	r.served["#limited@h"] = &served{in: on, box: sandbox.Bwrap{}, jail: &jail{tmp: 2 << 30, processes: 48, disk: 6 << 30}}
	plain := &served{in: off, box: sandbox.Off{}, jail: &jail{}}
	plain.busy.Store(2)
	r.served["#plain@h"] = plain
	r.served["#kept@h"] = &served{in: self, box: sandbox.Off{}, jail: &jail{}, kept: &keeper{running: map[*exec.Cmd]keptProcess{new(exec.Cmd): {}, new(exec.Cmd): {}}}}
	data, err := json.Marshal(r.status())
	if err != nil {
		t.Fatal(err)
	}
	var status struct {
		StartedAt string `json:"started_at"`
		AgentDir  string `json:"agent_dir"`
		RunnerDir string `json:"runner_dir"`
		Agents    []struct {
			Name    string `json:"name"`
			Serves  string `json:"serves"`
			Sandbox string `json:"sandbox"`
			Limits  *struct {
				Memory    int64 `json:"memory_bytes"`
				Processes int   `json:"processes"`
				Disk      int64 `json:"disk_bytes"`
			} `json:"limits"`
			Processes *processes `json:"processes"`
		}
	}
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatal(err)
	}
	if status.StartedAt != "2026-10-18T12:00:00Z" {
		t.Errorf("runner start not serialized as RFC3339: %s", data)
	}
	if status.AgentDir != r.agents || status.RunnerDir != r.host {
		t.Errorf("runner paths not reported: %s", data)
	}
	for _, a := range status.Agents {
		want := "runner"
		if a.Name == "#kept@h" {
			want = "itself"
		}
		if a.Serves != want {
			t.Errorf("%s serves=%q, want %q", a.Name, a.Serves, want)
		}
		switch a.Name {
		case "#limited@h":
			if a.Sandbox != "on" || a.Limits == nil || a.Limits.Memory != 2<<30 || a.Limits.Processes != 48 || a.Limits.Disk != 6<<30 {
				t.Errorf("applied limits not reported: %s", data)
			}
		case "#plain@h", "#kept@h":
			if a.Sandbox != "off" || a.Limits != nil || a.Processes == nil || a.Processes.Running != 2 {
				t.Errorf("sandbox-off must report actual count and no fictitious limits: %s", data)
			}
		default:
			if a.Sandbox != "" || a.Limits != nil {
				t.Errorf("unserved agent claims applied settings: %s", data)
			}
		}
	}
}

// Older lifecycle checks need just the count from the same live snapshot.
func (k *keeper) count() int { n, _ := k.snapshot(); return n }

func TestScriptExitSnapshotReplacesOnlyOnActualFailures(t *testing.T) {
	r, s, _, dir := deadlineFixture(t, "printf 'old detail\nfirst failure\n' >&2\nexit 7\n", "1s")
	r.realm = "h"
	s.in.Key = "fixture"
	r.installed = []installed{s.in}
	r.served = map[string]*served{s.name: s}
	before := time.Now()
	run := func() { r.runOne(s, protocol.Envelope{ID: "fixture", From: "caller@h"}) }
	run()
	first := r.status().Agents[0].LastExit
	if first == nil || first.Code != 7 || first.Reason != "first failure" || first.At.Before(before) || first.At.After(time.Now()) {
		t.Fatalf("first script exit not captured: %+v", first)
	}
	if err := os.WriteFile(filepath.Join(dir, "agent.sh"), []byte("printf 'second failure' >&2\nexit 8\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run()
	second := r.status().Agents[0].LastExit
	if second == nil || second.Code != 8 || second.Reason != "second failure" || second.At.Before(first.At) {
		t.Fatalf("last exit not replaced: %+v then %+v", first, second)
	}
	if err := os.WriteFile(filepath.Join(dir, "agent.sh"), []byte("exit 0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run()
	if got := r.status().Agents[0].LastExit; got != second {
		t.Fatalf("successful script replaced failure snapshot: %+v", got)
	}
	s.work = filepath.Join(dir, "absent")
	run()
	if got := r.status().Agents[0].LastExit; got != second {
		t.Fatalf("local Start failure invented an exit: %+v", got)
	}
}

func TestKeptStartClearsAndExit78RetainsLastExit(t *testing.T) {
	r, _, _ := configurationFixture(t, "itself", "printf 'ready\n' >&2\necho ready > \"$AGENT_BUS_WORK/ready\"\nexec sleep 30\n")
	st := awaitConfigurationState(t, r, "running")
	if st.StartedAt == nil || st.StartedAt.After(time.Now()) || time.Since(*st.StartedAt) > time.Minute {
		t.Fatalf("current process start not captured: %+v", st)
	}
	until := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(r.host, "work", "fixture", "ready")); err == nil {
			break
		}
		if time.Now().After(until) {
			t.Fatal("kept fixture never initialized")
		}
		time.Sleep(2 * time.Millisecond)
	}
	r.mu.Lock()
	k := r.served["#fixture@h"].kept
	r.mu.Unlock()
	// Cancel the keeper so this test needs no backoff window to inspect.
	k.cancel()
	k.signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() { k.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("keeper did not finish")
	}
	st = r.status().Agents[0]
	if st.StartedAt != nil || st.Processes.Running != 0 || st.LastExit == nil || st.LastExit.Code != -1 || st.LastExit.Reason != "ready" {
		t.Fatalf("exited process still current, or exit missing: %+v", st)
	}
	r2, _, script := configurationFixture(t, "itself", "printf 'need token\n' >&2\nexit 78\n")
	st = awaitConfigurationState(t, r2, "unconfigured")
	if st.StartedAt != nil || st.LastExit == nil || st.LastExit.Code != 78 || st.LastExit.Reason != "need token" {
		t.Fatalf("unconfigured exit snapshot lost: %+v", st)
	}
	if err := os.WriteFile(script, []byte("exec sleep 30\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r2.reload()
	if st := r2.status().Agents[0]; st.LastExit != nil {
		t.Fatalf("fresh load retained historical exit: %+v", st.LastExit)
	}
}

func TestKeptSnapshotUsesEarliestStillLiveStart(t *testing.T) {
	old, newer := new(exec.Cmd), new(exec.Cmd)
	first := time.Date(2026, 10, 18, 1, 0, 0, 0, time.UTC)
	second := first.Add(time.Minute)
	k := &keeper{running: map[*exec.Cmd]keptProcess{old: {startedAt: first}, newer: {startedAt: second}}}
	n, at := k.snapshot()
	if n != 2 || at == nil || !at.Equal(first) {
		t.Fatalf("earliest live start: %d %v", n, at)
	}
	delete(k.running, old)
	n, at = k.snapshot()
	if n != 1 || at == nil || !at.Equal(second) {
		t.Fatalf("retired process kept its start: %d %v", n, at)
	}
	delete(k.running, newer)
	n, at = k.snapshot()
	if n != 0 || at != nil {
		t.Fatalf("empty keeper invented start: %d %v", n, at)
	}
}

func TestRetiredExitCannotOverwriteReplacement(t *testing.T) {
	r, s, _, _ := deadlineFixture(t, "exit 0\n", "1s")
	s.in.Key = "fixture"
	r.realm = "h"
	r.installed = []installed{s.in}
	r.served = map[string]*served{s.name: s}
	old, current := new(keeper), new(keeper)
	s.kept = current
	cmd := exec.Command("sh", "-c", "exit 9")
	cmd.Stderr = &stderrLine{out: io.Discard}
	if err := cmd.Run(); err == nil || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 9 {
		t.Fatalf("exit9 fixture: %v", err)
	}
	r.recordExit(s, old, cmd)
	if r.installed[0].LastExit != nil {
		t.Fatal("retired keeper overwrote current observation")
	}
	replacement := &served{in: s.in, name: s.name}
	r.served[s.name] = replacement
	r.recordExit(s, nil, cmd)
	if r.installed[0].LastExit != nil {
		t.Fatal("obsolete serving overwrote replacement")
	}
	r.served[s.name] = s
	r.recordExit(s, current, cmd)
	if got := r.installed[0].LastExit; got == nil || got.Code != 9 {
		t.Fatalf("current exit ignored: %+v", got)
	}
}

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestATakenOneShotKeepsItsCgroupUntilItAnswers(t *testing.T) {
	if os.Getenv("AB_TEST_TAKEN_CGROUP") != "1" {
		// A private delegated unit owns every group this proof creates. The child
		// test also strips launcher credentials even if the user manager has them.
		cmd := exec.Command("systemd-run", "--user", "--quiet", "--wait", "--pipe", "--collect",
			"--unit=ab-taken-cgroup-test-"+fmt.Sprint(os.Getpid()), "--property=Delegate=yes",
			"--setenv=AB_TEST_TAKEN_CGROUP=1", "/usr/bin/env", "-u", "AGENT_BUS_SESSION_FILE", "-u", "AGENT_BUS_TOKEN",
			os.Args[0], "-test.run=^TestATakenOneShotKeepsItsCgroupUntilItAnswers$", "-test.v")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("delegated taken-message fixture: %v\n%s", err, out)
		}
		return
	}
	for _, stop := range []string{"stop", "not-configured", "disk-limit"} {
		t.Run(stop, func(t *testing.T) {
			output := &badAnswerLog{}
			r, b, _ := configurationFixtureOutput(t, "", "printf answered", output, "on")
			r.mu.Lock()
			s := r.served["#fixture@h"]
			if s == nil || s.jail.group == nil {
				r.mu.Unlock()
				t.Fatal("sandboxed fixture has no cgroup")
			}
			// Join the normal reader before taking its next message explicitly. Keep
			// exactly its production hand-off: reserve a slot, take, then run/release.
			s.cancel()
			<-s.readDone
			s.in.Agent.Deadline = "2s"
			group := s.jail.group
			r.mu.Unlock()
			s.slots <- struct{}{}
			release := sync.OnceFunc(func() { <-s.slots })
			defer release()
			if _, err := b.Send(protocol.Envelope{From: "owner@h", To: s.name, Body: "already taken"}); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			e, got, err := s.bus.next(ctx, time.Second, "", false)
			if err != nil || !got {
				t.Fatalf("take before stop: got=%v err=%v", got, err)
			}
			switch stop {
			case "stop":
				r.mu.Lock()
				r.stop(s.name, s)
				r.mu.Unlock()
			case "not-configured":
				r.notConfigured(s, nil, "fixture configuration missing")
			case "disk-limit":
				if err := os.WriteFile(filepath.Join(s.work, "oversize"), []byte("xx"), 0600); err != nil {
					t.Fatal(err)
				}
				s.jail.disk = 1
				r.checkDisk(s, s.jail)
			}
			if group.FD() < 0 {
				t.Error("cgroup released before the taken message launched")
			}
			r.mu.Lock()
			retired := r.retiring[s.in.Key]
			r.mu.Unlock()
			r.runOne(s, e)
			release()
			if retired != nil {
				select {
				case <-retired:
				case <-time.After(2 * time.Second):
					t.Fatal("cgroup retirement did not finish after the answer")
				}
			}
			// The transport must have an answer, not just a log saying that Start
			// failed. Only the isolated fixture's explicitly addressed inbox is read.
			reply, err := b.Consume(ctx, "owner@h", "", "", false, false)
			if err != nil || reply.Body != "answered" {
				t.Errorf("taken message lost its answer: body=%q err=%v log=%s", reply.Body, err, output.String())
			}
			if strings.Contains(output.String(), "exited badly") {
				t.Errorf("taken message exited badly: %s", output.String())
			}
			if group.FD() != -1 {
				t.Error("cgroup descriptor remained open after the last run ended")
			}
		})
	}
}

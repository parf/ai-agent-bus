package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvReadNeverReturnsValues(t *testing.T) {
	home := t.TempDir()
	r := &runner{agents: filepath.Join(home, "agent.d"), host: filepath.Join(home, "runner")}
	writeFile(t, filepath.Join(r.agents, "debug", "agent.json"), `{"description":"test","command":"x"}`)
	value := "TOKEN=rd105-file-sentinel\nOTHER='multiline\nrd105-line-two'\nTOKEN=last\n"
	for _, name := range []string{"runner/debug/env", "runner/debug/work/env"} {
		writeFile(t, filepath.Join(home, name), value)
		got, err := r.readFile(name)
		if err != nil {
			t.Fatal(err)
		}
		wire, _ := json.Marshal(got)
		var body map[string]any
		json.Unmarshal(wire, &body)
		if strings.Contains(string(wire), "rd105-") || body["content"] != nil {
			t.Fatalf("env read exposed content: %s", wire)
		}
		if body["sha256"] != fmt.Sprintf("%x", sha256.Sum256([]byte(value))) || body["size"] != float64(len(value)) || fmt.Sprint(body["names"]) != "[TOKEN OTHER]" {
			t.Fatalf("wrong metadata: %s", wire)
		}
	}
}

func TestBundledDebugMasksPrivateEnvironment(t *testing.T) {
	cmd := exec.Command("python3", "../../agents/debug/debug.py", "env")
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=rd105-home-sentinel", "FILE_SECRET=rd105-file-sentinel", "BUS_SECRET=rd105-bus-sentinel", "AGENT_BUS_FROM=caller@h", "AB_ROLE_OWNER=1"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "rd105-") {
		t.Fatal("debug env exposed a private value")
	}
	for _, want := range []string{"FILE_SECRET=***", "BUS_SECRET=***", "HOME=***", "AGENT_BUS_FROM=caller@h", "AB_ROLE_OWNER=1"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("debug missing %s", want)
		}
	}
}

func TestChildStderrRedactsInjectedValues(t *testing.T) {
	for _, chunked := range []bool{false, true} {
		var journal strings.Builder
		stderr := secretStderr(&journal, []string{"FILE=rd105-file-sentinel", "BUS=rd105-bus-sentinel", "SHORT=abc"})
		if chunked {
			for _, b := range []byte("rd105-file-sentinel\nlast rd105-bus-sentinel") {
				stderr.Write([]byte{b})
			}
		} else {
			cmd := exec.Command("sh", "-c", `printf '%s\nlast %s' "$FILE" "$BUS" >&2; exit 78`)
			cmd.Env = []string{"FILE=rd105-file-sentinel", "BUS=rd105-bus-sentinel"}
			cmd.Stderr = stderr
			if err := cmd.Run(); err == nil {
				t.Fatal("expected configuration exit")
			}
		}
		stderr.flush()
		if journal.String() != "***\nlast ***" || stderr.reason() != "last ***" {
			t.Fatalf("private stderr reached journal or status (chunked=%v)", chunked)
		}
	}
}

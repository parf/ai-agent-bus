package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartRefusesRelativeScriptBeforeRegistration(t *testing.T) {
	for _, script := range []string{"./hi.sh", "../hi.sh"} {
		_, err := describe([]string{"sample-hello", script})
		if err == nil || !strings.Contains(err.Error(), "absolute script path") {
			t.Fatalf("%q was not refused before registration: %v", script, err)
		}
	}
}

func TestStopAcceptsPlainAgentName(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() { _ = cmd.Process.Kill(); <-done }()
	if err := (running{Name: "#sample-hello", PID: cmd.Process.Pid, Started: time.Now()}).note(); err != nil {
		t.Fatal(err)
	}
	if err := stopVerb([]string{"sample-hello"}); err != nil {
		t.Fatal(err)
	}
}

func TestLogsAcceptPlainAgentName(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(logPath("#sample-hello")), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath("#sample-hello"), []byte("sample answered\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := logsVerb([]string{"sample-hello"}); err != nil {
		t.Fatalf("plain name did not find agent log: %v", err)
	}
}

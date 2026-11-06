package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/authkeys"
)

// The daemon refuses, at start, a forced-command program no key line could be read back for.
func TestSSHTokenIsCheckedAtStart(t *testing.T) {
	for _, bad := range []string{"/x/other", "agent-bus-token", "/opt/my tools/agent-bus-token"} {
		if _, err := sshTokenProgram(bad); err == nil || !(errors.Is(err, authkeys.ErrProgramName) || errors.Is(err, authkeys.ErrProgram)) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	program := filepath.Join(t.TempDir(), "agent-bus-token")
	if err := os.WriteFile(program, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if got, err := sshTokenProgram(program); err != nil || got != program {
		t.Fatalf("explicit helper: %q %v", got, err)
	}
}

func TestSSHTokenExecutableFixture(t *testing.T) {
	if os.Getenv("AB_SSH_TOKEN_FIXTURE") != "1" {
		return
	}
	program, err := sshTokenProgram("")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(program)
	os.Exit(0)
}

func TestDefaultSSHTokenUsesTheRealExecutableNotTheWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	realDir, linkDir, cwd := filepath.Join(root, "real"), filepath.Join(root, "path"), filepath.Join(root, "cwd")
	for _, dir := range []string{realDir, linkDir, cwd} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	realBinary := filepath.Join(realDir, "agent-busd")
	if err := os.WriteFile(realBinary, data, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realBinary, filepath.Join(linkDir, "agent-busd")); err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(realDir, "agent-bus-token")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	// A plausible executable in cwd must not stand in for the daemon's sibling.
	if err := os.WriteFile(filepath.Join(cwd, "agent-bus-token"), []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", linkDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	cmd := exec.Command("agent-busd", "-test.run=^TestSSHTokenExecutableFixture$")
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "AB_SSH_TOKEN_FIXTURE=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("default helper lookup: %v: %s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != helper {
		t.Fatalf("default helper = %q, want real executable sibling %q", got, helper)
	}
}

func TestSSHTokenRefusesMissingAndNonExecutablePrograms(t *testing.T) {
	for _, kind := range []string{"missing", "directory", "not-executable"} {
		t.Run(kind, func(t *testing.T) {
			program := filepath.Join(t.TempDir(), "agent-bus-token")
			switch kind {
			case "directory":
				if err := os.Mkdir(program, 0700); err != nil {
					t.Fatal(err)
				}
			case "not-executable":
				if err := os.WriteFile(program, []byte("#!/bin/sh\nexit 0\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := sshTokenProgram(program); err == nil || !strings.Contains(err.Error(), program) {
				t.Fatalf("%s helper must be refused by path: %v", kind, err)
			}
		})
	}
}

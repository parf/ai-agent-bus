package main

import (
	"errors"
	"testing"

	"github.com/parf/ai-agent-bus/internal/authkeys"
)

// The daemon refuses, at start, a forced-command program no key line could be read back for.
func TestSSHTokenIsCheckedAtStart(t *testing.T) {
	for _, bad := range []string{"/x/other", "agent-bus-token", "/opt/my tools/agent-bus-token"} {
		if _, err := sshTokenProgram(bad, "/usr/local/bin/agent-busd"); err == nil || !(errors.Is(err, authkeys.ErrProgramName) || errors.Is(err, authkeys.ErrProgram)) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if program, err := sshTokenProgram("", "/usr/local/bin/agent-busd"); err != nil || program != "/usr/local/bin/agent-bus-token" {
		t.Errorf("the default beside the daemon: %q %v", program, err)
	}
	if _, err := sshTokenProgram("/usr/local/bin/agent-bus-token", "/x/agent-busd"); err != nil {
		t.Error(err)
	}
}

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A child's shell clears every AB_ROLE_* it starts with — the runner's or a
// user manager's alike — and then holds only the roles the daemon worked out.
func TestAChildHoldsOnlyTheRolesItWasHanded(t *testing.T) {
	script := filepath.Join(t.TempDir(), "roles.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"owner=${AB_ROLE_OWNER:-no} maint=${AB_ROLE_MAINTAINER:-no} evil=${AB_ROLE_EVIL:-none} arg=$1\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		algo  string
		roles []string
		want  string
	}{
		{algoArgs, []string{"owner"}, "owner=1 maint=no evil=none arg=hello"},
		{algoArgs, nil, "owner=no maint=no evil=none arg=hello"},
		{algoJSON, []string{"maintainer"}, "owner=no maint=1 evil=none arg="},
		// A name the daemon would never write is not quoted into the command.
		{algoArgs, []string{"x ab_role_evil=1", "owner"}, "owner=1 maint=no evil=none arg=hello"},
	} {
		argv := scriptArgv(script, c.algo, "hello", c.roles)
		cmd := exec.Command(argv[0], argv[1:]...)
		// Planted where a sandboxed child would find it: the starting environment.
		cmd.Env = []string{"PATH=/usr/bin:/bin", "AB_ROLE_EVIL=1", "AB_ROLE_OWNER=1"}
		if c.roles == nil || c.roles[0] == "maintainer" {
			cmd.Env = append(cmd.Env, "AB_ROLE_MAINTAINER=")
		}
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("%v: %v", c.roles, err)
		}
		if got := strings.TrimSpace(string(out)); got != c.want {
			t.Errorf("%s %v: %q, want %q", c.algo, c.roles, got, c.want)
		}
	}
}

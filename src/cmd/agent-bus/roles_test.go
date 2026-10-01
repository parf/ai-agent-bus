package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A child's shell clears every AB_ROLE_* it starts with — the runner's or a
// user manager's alike — and then holds only the roles the daemon worked out,
// leaving the rest of its environment as it was. The prelude runs under sh,
// and under dash too where one is found (AB_TEST_DASH names one elsewhere).
func TestAChildHoldsOnlyTheRolesItWasHanded(t *testing.T) {
	shells := []string{"sh"}
	if d := os.Getenv("AB_TEST_DASH"); d != "" {
		shells = append(shells, d)
	} else if d, err := exec.LookPath("dash"); err == nil {
		shells = append(shells, d)
	}
	for _, sh := range shells {
		t.Run(filepath.Base(sh), func(t *testing.T) { childRoles(t, sh) })
	}
}

func childRoles(t *testing.T, sh string) {
	script := filepath.Join(t.TempDir(), "roles.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"owner=${AB_ROLE_OWNER:-no} maint=${AB_ROLE_MAINTAINER:-no} evil=${AB_ROLE_EVIL:-none} arg=$1\"\necho \"v=$v tag=$AGENT_BUS_TAG lower=${AB_ROLE_lower:-none} loop=${AB_ROLE_:-none} z=${AB_ROLE_Z:-none}\"\n"), 0o700); err != nil {
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
		cmd := exec.Command(sh, argv[1:]...)
		// Planted where a sandboxed child would find it: the starting
		// environment. A sender's tag whose words look like assignments, and a
		// variable named like a loop's, must come through untouched.
		cmd.Env = []string{"PATH=/usr/bin:/bin", "AB_ROLE_EVIL=1", "AB_ROLE_OWNER=1", "AB_ROLE_lower=1", "AB_ROLE_=1", "AB_ROLE_Z=1", "v=keep me", "AGENT_BUS_TAG=ok AB_ROLE_BAD-NAME=x * AB_ROLE_EVIL=2 ok"}
		if c.roles == nil || c.roles[0] == "maintainer" {
			cmd.Env = append(cmd.Env, "AB_ROLE_MAINTAINER=")
		}
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("%v: %v", c.roles, err)
		}
		if got, want := strings.TrimSpace(string(out)), c.want+"\nv=keep me tag=ok AB_ROLE_BAD-NAME=x * AB_ROLE_EVIL=2 ok lower=none loop=none z=none"; got != want {
			t.Errorf("%s %v: %q, want %q", c.algo, c.roles, got, want)
		}
	}
}

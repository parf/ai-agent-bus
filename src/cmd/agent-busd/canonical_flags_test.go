package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestDaemonFlagFixture(t *testing.T) {
	arg := os.Getenv("DAEMON_FLAG_FIXTURE")
	if arg == "" {
		return
	}
	os.Args = []string{os.Args[0], arg}
	main()
}

func TestDaemonRequiresCanonicalFlags(t *testing.T) {
	for _, arg := range []string{"--web", "-h", "-help", "-version", "-owner=owner@h"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestDaemonFlagFixture$")
		cmd.Env = append(os.Environ(), "DAEMON_FLAG_FIXTURE="+arg)
		out, err := cmd.CombinedOutput()
		if err == nil || (!strings.Contains(string(out), "invalid option") && !strings.Contains(string(out), "flag provided but not defined")) {
			t.Errorf("%s: %v %s", arg, err, out)
		}
	}
	for _, arg := range []string{"--help", "--version"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestDaemonFlagFixture$")
		cmd.Env = append(os.Environ(), "DAEMON_FLAG_FIXTURE="+arg)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("%s: %v %s", arg, err, out)
		}
	}
}

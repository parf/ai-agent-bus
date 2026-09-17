package main

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func TestBusChildRetainsEnvironment(t *testing.T) {
	around := []string{"PATH=/usr/bin", "AGENT_BUS_TOKEN=bus-only"}
	bus := &child{what: "bus", env: []string{roleEnv + "=" + roleBus}}
	got := bus.environ(around)
	for _, want := range append(around, roleEnv+"="+roleBus) {
		if !slices.Contains(got, want) {
			t.Errorf("bus child lost %q", want)
		}
	}
}

// Exercise the actual factory, not a manually configured child fixture.
func TestWebWrapperInheritsNoEnvironment(t *testing.T) {
	exe, err := exeForTest(t)
	if err != nil {
		t.Fatal(err)
	}
	web := webChild(exe, "/run/agent-bus/bus.sock")
	if !web.limited {
		t.Fatal("the web child can start without its resource group")
	}
	if got := web.environ([]string{"AGENT_BUS_TOKEN=secret", "OTHER_SECRET=also-secret", "LD_PRELOAD=unsafe"}); len(got) != 0 {
		t.Errorf("the dashboard wrapper inherited host environment: %v", got)
	}
}

func TestWebSandboxKeepsOnlyExplicitInputs(t *testing.T) {
	t.Setenv("AGENT_BUS_WEB_ADDR", "127.0.0.1:8765")
	t.Setenv("AGENT_BUS_WEB_CERT", "/private/tls.crt")
	t.Setenv("AGENT_BUS_WEB_KEY", "/private/tls.key")
	args := webSandbox("/program/web", "/daemon/shared.sock", os.Getenv)
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--unshare-user --unshare-pid", "--new-session", "--cap-drop ALL",
		"--ro-bind /program/web /agent-bus-web", "--ro-bind /daemon/shared.sock /bus.sock",
		"--proc /proc", "--clearenv", "--setenv AGENT_BUS_ADDR /bus.sock",
		"--setenv AGENT_BUS_WEB_ADDR 127.0.0.1:8765",
		"--ro-bind /private/tls.crt /tls/cert", "--setenv AGENT_BUS_WEB_CERT /tls/cert",
		"--ro-bind /private/tls.key /tls/key", "--setenv AGENT_BUS_WEB_KEY /tls/key",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("web sandbox lost %q", want)
		}
	}
	if !slices.Equal(args[len(args)-2:], []string{"--", "/agent-bus-web"}) {
		t.Fatal("sandbox does not execute the bound web binary")
	}
}

// webChild refuses to exist without a binary beside it, so the test supplies
// one rather than skipping the assertion that matters.
func exeForTest(t *testing.T) (string, error) {
	t.Helper()
	path := t.TempDir() + "/agent-bus-web"
	return path, os.WriteFile(path, []byte("#!/bin/true\n"), 0o755)
}

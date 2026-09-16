package main

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// The dashboard must carry no credential of its own: on the owner's socket it
// would serve every page as the owner, and a child holding the owner's token is
// a credential mint (docs/05-discovery.md#signing-in). That was said in a
// comment and enforced by nothing — the child was started with the supervisor's
// whole environment, so starting the daemon from a shell that had exported
// AGENT_BUS_TOKEN handed the dashboard that token.
func TestTheWebChildIsGivenNoCredential(t *testing.T) {
	around := []string{
		"PATH=/usr/bin",
		"AGENT_BUS_TOKEN=089771cd830316a767b02722123e1153715fbc1cb83a62ca",
		"AGENT_BUS_WEB_CERT=/etc/ssl/web.pem",
		"AGENT_BUS_ADDR=/run/agent-bus/owner.sock",
	}
	web := &child{what: "web", env: []string{"AGENT_BUS_ADDR=/run/agent-bus/bus.sock"}, drop: []string{"AGENT_BUS_TOKEN"}}
	got := web.environ(around)

	for _, kv := range got {
		if strings.HasPrefix(kv, "AGENT_BUS_TOKEN=") {
			t.Error("the dashboard was handed a credential of its own")
		}
	}
	// Positive controls. Dropping everything would pass the check above, and
	// a dashboard with no PATH and no certificate is not a dashboard.
	for _, want := range []string{"PATH=/usr/bin", "AGENT_BUS_WEB_CERT=/etc/ssl/web.pem"} {
		if !slices.Contains(got, want) {
			t.Errorf("the dashboard lost %s, so the check above proves nothing", want)
		}
	}
	// And what the supervisor says wins over what was around it: the shared
	// socket, never the owner's.
	last := ""
	for _, kv := range got {
		if strings.HasPrefix(kv, "AGENT_BUS_ADDR=") {
			last = kv
		}
	}
	if last != "AGENT_BUS_ADDR=/run/agent-bus/bus.sock" {
		t.Errorf("the dashboard reaches the bus at %q, not the shared socket it was told", last)
	}

	// The bus child drops nothing, so this is about which child and not about
	// the mechanism refusing everybody.
	bus := &child{what: "bus", env: []string{roleEnv + "=" + roleBus}}
	if !slices.Contains(bus.environ(around), "AGENT_BUS_TOKEN=089771cd830316a767b02722123e1153715fbc1cb83a62ca") {
		t.Error("the bus child lost the environment it was started with")
	}
}

// And the dashboard the supervisor actually builds is the one that drops it,
// which is the half a hand-built fixture cannot check.
func TestTheRealWebChildDropsTheToken(t *testing.T) {
	exe, err := exeForTest(t)
	if err != nil {
		t.Skip(err)
	}
	web := webChild(exe, "/run/agent-bus/bus.sock")
	if !slices.Contains(web.drop, "AGENT_BUS_TOKEN") {
		t.Errorf("the dashboard child drops %v, which does not include its credential", web.drop)
	}
}

// webChild refuses to exist without a binary beside it, so the test supplies
// one rather than skipping the assertion that matters.
func exeForTest(t *testing.T) (string, error) {
	t.Helper()
	path := t.TempDir() + "/agent-bus-web"
	return path, os.WriteFile(path, []byte("#!/bin/true\n"), 0o755)
}

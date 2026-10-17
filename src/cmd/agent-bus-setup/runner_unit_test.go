package main

import (
	"strings"
	"testing"
)

// The runner's unit runs it as its own account, delegates it a cgroup
// subtree, writes only its home, holds no capability, and wants the bus
// without requiring it.
func TestTheRunnersUnitDelegatesItsCgroupsAndHoldsNothingElse(t *testing.T) {
	u := runnerUnit("/usr/local/bin/agent-bus-runner")
	for _, want := range []string{
		"User=agent-bus-runner\n",
		"ExecStart=/usr/local/bin/agent-bus-runner --home /var/lib/agent-bus\n",
		"Delegate=yes\n",
		"ReadWritePaths=/var/lib/agent-bus/runner\n",
		"Wants=agent-busd.service\n",
		"After=agent-busd.service\n",
		"NoNewPrivileges=yes\n",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("the unit lacks %q", strings.TrimSpace(want))
		}
	}
	for _, never := range []string{"Requires=", "AmbientCapabilities", "CapabilityBoundingSet=CAP"} {
		if strings.Contains(u, never) {
			t.Errorf("the unit has %q", never)
		}
	}
}

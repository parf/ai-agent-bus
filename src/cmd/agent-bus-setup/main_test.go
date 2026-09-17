package main

import (
	"strings"
	"testing"
)

func TestUnitDelegatesOnlyTheWebControllers(t *testing.T) {
	unit := unitFor("/program/agent-busd", "127.0.0.1:6767", "owner@example", nil)
	for _, want := range []string{
		"Delegate=cpu memory pids",
		"DelegateSubgroup=supervisor",
		"CapabilityBoundingSet=CAP_CHOWN",
	} {
		if strings.Count(unit, want) != 1 {
			t.Errorf("generated unit has %d copies of %q", strings.Count(unit, want), want)
		}
	}
}

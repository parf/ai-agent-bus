package main

import (
	"os/exec"
	"testing"
)

// Keep the same authored-help snapshots in the normal Go shard and the fast list.
func TestManifestHelpMatchesGoldenFixtures(t *testing.T) {
	command := exec.Command("python3", "-B", "-m", "unittest", "discover", "-s", "../../agents/shared", "-p", "test_help_golden.py", "-v")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("manifest help golden check: %v\n%s", err, output)
	}
}

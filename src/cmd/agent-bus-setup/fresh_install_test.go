package main

import (
	"os/exec"
	"testing"
)

func TestFreshInstallPortablePreflight(t *testing.T) {
	cmd := exec.Command("python3", "-B", "../../acceptance/test_fresh_install.py")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fresh-install portable preflight: %v\n%s", err, out)
	}
}

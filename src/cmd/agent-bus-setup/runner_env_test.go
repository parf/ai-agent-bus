package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Setup's token file is the canonical common subset that systemd also reads.
func TestRunnerTokenFileIsTheSharedParsersMachineWrittenSubset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runner.env")
	for _, src := range []string{"AGENT_BUS_TOKEN=abcdef012345\n", "AGENT_BUS_TOKEN=abcdef012345"} {
		if err := os.WriteFile(path, []byte(src), 0600); err != nil {
			t.Fatal(err)
		}
		if got := envToken(path); got != "abcdef012345" {
			t.Fatalf("canonical token was not read: %q", got)
		}
	}
	for _, src := range []string{"AGENT_BUS_TOKEN=abcdef012345\nBROKEN", "AGENT_BUS_TOKEN=not-hex\n", "OTHER=abcdef012345\n", "AGENT_BUS_TOKEN=\n", "export AGENT_BUS_TOKEN=abcdef012345\n", "AGENT_BUS_TOKEN=\"abcdef012345\"\n", "AGENT_BUS_TOKEN=abcdef012345\nEXTRA=1\n"} {
		if err := os.WriteFile(path, []byte(src), 0600); err != nil {
			t.Fatal(err)
		}
		if got := envToken(path); got != "" {
			t.Fatal("a noncanonical or invalid token file was trusted")
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got := envToken(path); got != "" {
		t.Fatal("a missing file held a token")
	}
}

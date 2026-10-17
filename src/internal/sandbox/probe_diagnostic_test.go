package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSandboxProbeNamesAnUnsupportedRequiredOption(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "systemd-run")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf 'unrecognized option --expand-environment=no\\n' >&2\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	if _, err := Pick("on"); err == nil || !strings.Contains(err.Error(), "unrecognized option --expand-environment=no") {
		t.Fatalf("missing actual sandbox refusal: %v", err)
	}
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := Pick("on"); err != nil {
		t.Fatalf("supported host refused: %v", err)
	}
}

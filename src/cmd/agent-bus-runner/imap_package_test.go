package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// The entrypoint owns its reader and cache: the actual exported package must
// select the kept-process path, never the runner's per-message stdin path.
func TestTheImapPackageOwnsItsReader(t *testing.T) {
	root := t.TempDir()
	agents := filepath.Join(root, "agents")
	if err := os.Mkdir(agents, 0755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", filepath.Join("..", "..", "agents", "imap", "package.sh"), filepath.Join(agents, "imap"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("package: %v: %s", err, out)
	}
	list, err := load(agents, filepath.Join(root, "host"))
	if err != nil || len(list) != 1 {
		t.Fatalf("load: %v, %+v", err, list)
	}
	in := list[0]
	if in.Refused != "" || in.Agent.Command != "./imap.py" || in.Agent.Serves != "itself" || in.Span.Min != 1 || in.Span.Max != 1 {
		t.Fatalf("IMAP reader mode: refused=%q command=%q serves=%q processes=%+v", in.Refused, in.Agent.Command, in.Agent.Serves, in.Span)
	}
	if in.Host.Sandbox == "off" {
		t.Fatal("IMAP must keep the default sandbox")
	}
}

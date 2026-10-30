package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Exercise the package script rather than only the author's source manifest:
// both bundled entry points must be flat and the runner must accept the result.
func TestTheTelegramPackageLoads(t *testing.T) {
	root := t.TempDir()
	agents := filepath.Join(root, "agents")
	if err := os.Mkdir(agents, 0755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(agents, "telegram")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", filepath.Join("..", "..", "agents", "telegram", "package.sh"), dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("package: %v\n%s", err, out)
	}
	for _, name := range []string{"telegram.js", "agent.json"} {
		if fi, err := os.Stat(filepath.Join(dest, name)); err != nil || fi.Size() == 0 {
			t.Fatalf("package lacks %s: %v", name, err)
		}
	}
	bundle, err := os.ReadFile(filepath.Join(dest, "telegram.js"))
	if err != nil || !strings.Contains(string(bundle), "class Subscriptions") || !strings.Contains(string(bundle), "filter_invalid") {
		t.Fatalf("package lacks the bundled shared subscription engine: %v", err)
	}
	list, err := load(agents, filepath.Join(root, "private"))
	if err != nil || len(list) != 1 {
		t.Fatalf("load: %v, %+v", err, list)
	}
	if in := list[0]; in.Refused != "" || in.Agent.Command != "./telegram.js" || in.Agent.Serves != "itself" {
		t.Fatalf("packaged manifest: %+v", in)
	}
	// A self-contained module loads with no source checkout imports needed.
	cmd = exec.CommandContext(ctx, "bun", filepath.Join(dest, "telegram.js"))
	cmd.Env = append(os.Environ(), "AGENT_BUS_NAME=")
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "AGENT_BUS_NAME is not set") {
		t.Fatalf("bundle did not reach its own startup validation: %v\n%s", err, out)
	}
}

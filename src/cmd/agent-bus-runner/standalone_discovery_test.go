package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStandaloneOnlyPackagesDoNotAppearAsRefusedRunnerAgents(t *testing.T) {
	root := t.TempDir()
	bundled, host := filepath.Join(root, "agents"), filepath.Join(root, "runner")
	for _, name := range []string{"git", "logwatch", "server-info", "systemd", "tmux-sessions"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "agents", "standalone", name, "agent.json"))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(bundled, "standalone", name, "agent.json"), string(data))
	}
	r := &runner{ctx: context.Background(), agents: filepath.Join(root, "shared"), bundled: bundled, host: host, say: io.Discard, served: map[string]*served{}}
	r.reload()
	rows := r.status().Agents
	if len(rows) != 1 || rows[0].Key != "tmux-sessions" || rows[0].State != "available" {
		t.Fatalf("standalone-only services must not be refused runner agents; got %+v", rows)
	}
	for _, name := range []string{"git", "logwatch", "server-info", "systemd"} {
		if _, err := r.packageRoot(name); err == nil {
			t.Errorf("standalone-only %s accepted by runner file controls", name)
		}
	}
}

func TestStandaloneScanDoesNotHideBrokenRunnerPackages(t *testing.T) {
	root, host := t.TempDir(), t.TempDir()
	shared, bundled := filepath.Join(root, "shared"), filepath.Join(root, "agents")
	writeFile(t, filepath.Join(shared, "broken-shared", "agent.json"), `{"description":"Missing command"}`)
	writeFile(t, filepath.Join(bundled, "broken-bundle", "agent.json"), `{"description":"Missing command"}`)
	writeFile(t, filepath.Join(bundled, "standalone", "empty-command", "agent.json"), `{"description":"Broken runner","command":""}`)
	writeFile(t, filepath.Join(bundled, "standalone", "upper-empty-command", "agent.json"), `{ "description":"Broken runner", "COMMAND":"" }`)
	writeFile(t, filepath.Join(bundled, "standalone", "malformed", "agent.json"), `{"description":`)
	r := &runner{agents: shared, bundled: bundled}
	rows, err := loadSources(r.sources(), host)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("broken packages disappeared: %+v", rows)
	}
	for _, in := range rows {
		if in.Refused == "" {
			t.Errorf("broken %s was not refused", in.Key)
		}
		if in.Key != "malformed" && !strings.Contains(in.Refused, "command is required") {
			t.Errorf("wrong refusal for %s: %s", in.Key, in.Refused)
		}
	}
}

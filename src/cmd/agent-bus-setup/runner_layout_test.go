package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunnerLayoutIsSharedCodeAndSeparateAccounts(t *testing.T) {
	a, _ := runnerFor("alice", "h")
	b, _ := runnerFor("bob", "h")
	if a.agents != b.agents || a.agents != stateRoot+"/agent.d" {
		t.Fatalf("code is not shared: %s, %s", a.agents, b.agents)
	}
	if a.private == b.private || a.private != stateRoot+"/alice-ab-runner" {
		t.Fatalf("private directories: %s, %s", a.private, b.private)
	}
	st := installedState(a, "fixture")
	st.others = 1
	for _, purge := range []bool{false, true} {
		// Execute the real remove action on disposable state, not just its text.
		root := t.TempDir()
		p := a
		p.private = filepath.Join(root, "alice-ab-runner")
		p.agents = filepath.Join(root, "agent.d")
		for _, dir := range []string{p.private, p.agents, filepath.Join(root, "bob-ab-runner")} {
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
		}
		sentinel := filepath.Join(p.agents, "kept")
		if err := os.WriteFile(sentinel, []byte("code"), 0600); err != nil {
			t.Fatal(err)
		}
		plan, err := runnerPlan("uninstall", p, st, "fixture", purge, runnerOps{})
		if err != nil {
			t.Fatal(err)
		}
		for _, step := range plan {
			if strings.HasPrefix(step.say, "remove "+p.private+" ") {
				if err := step.do(); err != nil {
					t.Fatal(err)
				}
			}
		}
		if _, err := os.Stat(p.private); !os.IsNotExist(err) {
			t.Fatalf("private directory not removed: %v", err)
		}
		if _, err := os.Stat(sentinel); err != nil {
			t.Fatalf("shared code removed: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, "bob-ab-runner")); err != nil {
			t.Fatalf("second runner removed: %v", err)
		}
		if strings.Contains(says(t, plan), runnerTemplatePath) {
			t.Fatal("second runner lost template")
		}
	}
}

func TestLegacyRunnerTreeRefusesWithoutTouchingIt(t *testing.T) {
	root := t.TempDir()
	if err := refuseLegacyRunners(root); err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(root, "runners", "alice")
	if err := os.MkdirAll(tree, 0700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(tree, "kept")
	if err := os.WriteFile(sentinel, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := refuseLegacyRunners(root); err == nil || !strings.Contains(err.Error(), filepath.Join(root, "runners")) {
		t.Fatalf("legacy tree accepted: %v", err)
	}
	if b, err := os.ReadFile(sentinel); err != nil || string(b) != "secret" {
		t.Fatal("refusal changed old state")
	}
}

func TestWebUsesItsCodeDirectly(t *testing.T) {
	template, err := os.ReadFile("../../web/agent-bus-web.service")
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"/usr/local/src/ai-agent-bus/src/web", "/usr/local/lib/agent-bus/current/web"} {
		unit := string(webUnit(template, dir))
		for _, line := range []string{"ExecStart=/usr/bin/bun run " + dir + "/server.ts", "WorkingDirectory=" + dir, "Environment=HOME=" + dir, "InaccessiblePaths=-/var/lib/agent-bus "} {
			if !strings.Contains(unit, line) {
				t.Errorf("unit lacks %q", line)
			}
		}
		if strings.Contains(unit, "@WEB_DIR@") || strings.Contains(unit, "/var/lib/agent-bus/web") {
			t.Fatal("unit still uses link or placeholder")
		}
		if strings.Contains(strings.Join(webSteps(dir), "\n"), "link ") {
			t.Fatal("installation plans a web link")
		}
	}
}

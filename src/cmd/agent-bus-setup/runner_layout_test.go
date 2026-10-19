package main

import (
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
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

// The entry point must honor the refusal before talking to the bus or planning.
func TestRunnerVerbRefusesLegacyTreeBeforeBusOrPlan(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "runners"), 0700); err != nil {
		t.Fatal(err)
	}
	oldRoot := runnerLayoutRoot
	runnerLayoutRoot = root
	t.Cleanup(func() { runnerLayoutRoot = oldRoot })
	socket := filepath.Join(t.TempDir(), "bus.sock")
	l, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); http.Error(w, "unexpected bus call", 503) })}
	go func() { _ = server.Serve(l) }()
	t.Cleanup(func() { _ = server.Close() })
	t.Setenv("AGENT_BUS_ADDR", socket)
	rd, wr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldOut := os.Stdout
	os.Stdout = wr
	defer func() { os.Stdout = oldOut; _ = rd.Close(); _ = wr.Close() }()
	err = runnerVerb("disable", "alice", false, true)
	_ = wr.Close()
	out, readErr := io.ReadAll(rd)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if err == nil || !strings.Contains(err.Error(), "legacy runner tree "+filepath.Join(root, "runners")) {
		t.Fatalf("entry point ignored legacy refusal: %v", err)
	}
	if calls.Load() != 0 || len(out) != 0 {
		t.Fatalf("legacy refusal reached bus/plan: %d calls, output %q", calls.Load(), out)
	}
}

// Run the installer's actual restart-and-health tail, with host commands mocked.
func TestWebDevInstallerKeepsOldLinkUntilHealthy(t *testing.T) {
	source, err := os.ReadFile("../../web/install-dev.sh")
	if err != nil {
		t.Fatal(err)
	}
	marker := "systemctl restart agent-bus-web.service\n"
	i := strings.Index(string(source), marker)
	if i < 0 {
		t.Fatal("no restart in installer")
	}
	tail := string(source)[i:]
	for _, healthy := range []bool{false, true} {
		t.Run(map[bool]string{false: "unhealthy", true: "healthy"}[healthy], func(t *testing.T) {
			dir := t.TempDir()
			link := filepath.Join(dir, "old-link")
			target := filepath.Join(dir, "code")
			if err := os.Mkdir(target, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			unit := filepath.Join(dir, "unit")
			if err := os.WriteFile(unit, []byte("Environment=AGENT_BUS_WEB_ADDR=127.0.0.1:1\n"), 0600); err != nil {
				t.Fatal(err)
			}
			rc := "1"
			if healthy {
				rc = "0"
			}
			prefix := `set -euo pipefail
systemctl(){ :; }
curl(){ return "$HEALTH_RC"; }
seq(){ printf '1\n'; }
sleep(){ :; }
link="$TEST_LINK"; unit="$TEST_UNIT"
`
			cmd := exec.Command("bash")
			cmd.Stdin = strings.NewReader(prefix + tail)
			cmd.Env = append(os.Environ(), "HEALTH_RC="+rc, "TEST_LINK="+link, "TEST_UNIT="+unit)
			out, err := cmd.CombinedOutput()
			if healthy && err != nil || !healthy && err == nil {
				t.Fatalf("health outcome: %v %s", err, out)
			}
			_, err = os.Lstat(link)
			if healthy && !os.IsNotExist(err) {
				t.Fatalf("healthy installer kept old link: %v", err)
			}
			if !healthy && err != nil {
				t.Fatalf("failed health check removed old link: %v", err)
			}
			if _, err := os.Stat(target); err != nil {
				t.Fatal("installer removed code directory")
			}
		})
	}
}

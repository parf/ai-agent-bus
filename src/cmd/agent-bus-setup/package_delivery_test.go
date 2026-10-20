package main

import (
	"archive/tar"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Run the real packager against a build fixture independent of bundleFiles:
// deriving both sides from that list would hide delivery disagreements.
func TestPackagerOutputIsAcceptedBySetup(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(filepath.Join(src, "internal/version"), 0700); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile("../../package.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(src, "package.sh"), script, 0700); err != nil {
		t.Fatal(err)
	}
	files := []string{"agent-bus", "agent-busd", "agent-bus-admin", "agent-bus-runner", "agent-bus-setup", "agent-bus-token", "web/server.ts", "web/agent-bus-web.service", ".claude-plugin/marketplace.json", "mcp/.claude-plugin/plugin.json", "mcp/channel.ts", "mcp/commands/ls.md", "mcp/commands/send.md",
		"mcp/server.js", "launchers/launcher.js", "launchers/ab-claude", "launchers/ab-codex", "launchers/ab-opencode", "launchers/ab-opencode1", "LICENSE.md", "NOTICE.md", "THIRD-PARTY-NOTICES.md", "INSTALL.md"}
	files = append(files, []string{
		"agents/slack/agent.json",
		"agents/slack/THIRD-PARTY-NOTICES.md",
		"agents/slack/env.dist",
		"agents/slack/slack.js",
		"agents/slack/filter-worker.js",
		"agents/telegram/agent.json",
		"agents/telegram/THIRD-PARTY-NOTICES.md",
		"agents/telegram/env.dist",
		"agents/telegram/telegram.js",
		"agents/telegram/filter-worker.js",
		"agents/ai/agent.json",
		"agents/ai/env.dist",
		"agents/ai/ai.py",
		"agents/ai/LICENSE.md",
		"agents/sms-telnyx/agent.json", "agents/sms-telnyx/env.dist", "agents/sms-telnyx/sms.py", "agents/sms-telnyx/LICENSE.md",
		"agents/debug/agent.json",
		"agents/debug/THIRD-PARTY-NOTICES.md",
		"agents/debug/env.dist",
		"agents/debug/debug.py",
		"agents/shared/metadata.py",
		"agents/LICENSE.md",
		"agents/NOTICE.md",
		"agents/THIRD-PARTY-NOTICES.md",
		"agents/standalone/install.py",
		"agents/standalone/systemd/agent.json",
		"agents/standalone/systemd/env.dist",
		"agents/standalone/systemd/systemd.py",
		"agents/standalone/systemd/install.py",
		"agents/standalone/server-info/agent.json",
		"agents/standalone/server-info/env.dist",
		"agents/standalone/server-info/server-info.py",
		"agents/standalone/server-info/install.sh",
		"agents/standalone/git/agent.json",
		"agents/standalone/git/env.dist",
		"agents/standalone/git/git_agent.py",
		"agents/standalone/git/install.py",
		"agents/standalone/git/autopull.py",
		"agents/standalone/git/timers.py",
		"agents/standalone/sessions/agent.json",
		"agents/standalone/sessions/sessions.py",
		"agents/standalone/sessions/install.py",
		"agents/standalone/logwatch/agent.json",
		"agents/standalone/logwatch/env.dist",
		"agents/standalone/logwatch/logwatch.py",
		"agents/standalone/logwatch/install.py",
		"agents/standalone/logwatch/cleaning.py",
		"agents/standalone/logwatch/filters.py",
		"agents/standalone/logwatch/watcher.py",
		"clients/LICENSE.md",
		"clients/python/agent_bus/__init__.py",
		"clients/python/agent_bus/_version.py",
		"clients/python/agent_bus/client.py",
		"clients/python/agent_bus/errors.py",
		"clients/python/agent_bus/message.py",
		"clients/python/agent_bus/storage.py",
		"clients/python/agent_bus/transport.py",
	}...)
	build := "#!/bin/bash\nset -eu\nroot=$1\nprintf 'packaging agents\\n'\n"
	for _, name := range files {
		build += "mkdir -p \"$root/" + filepath.Dir(name) + "\"\nprintf fixture > \"$root/" + name + "\"\nchmod 755 \"$root/" + name + "\"\n"
	}
	build += "mkdir -p \"$root/internal/version\"\nprintf '0.0.1\\n' > \"$root/internal/version/VERSION\"\n"
	if err = os.WriteFile(filepath.Join(src, "build.sh"), []byte(build), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(src, "internal/version/VERSION"), []byte("0.0.1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "package.sh")
	cmd.Dir = src
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("package: %v: %s", err, out)
	}
	archive := strings.Split(strings.TrimSpace(string(out)), "\n")[0]
	unpack := filepath.Join(root, "unpack")
	if err = os.Mkdir(unpack, 0700); err != nil {
		t.Fatal(err)
	}
	if out, err = exec.Command("tar", "-xzf", archive, "-C", unpack).CombinedOutput(); err != nil {
		t.Fatalf("unpack: %v: %s", err, out)
	}
	entries, err := os.ReadDir(unpack)
	if err != nil || len(entries) != 1 {
		t.Fatalf("unpacked: %v %v", entries, err)
	}
	if _, err = checkBundleVersion(filepath.Join(unpack, entries[0].Name()), "0.0.1"); err != nil {
		t.Fatalf("packager and installer disagree: %v", err)
	}
}

func TestReleaseArchiveIncludesTheBuildNotice(t *testing.T) {
	script, err := os.ReadFile("../../release.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(script), "\n") {
		if strings.Contains(line, "archive '$commit'") {
			// These are the arguments passed to git archive by release.sh.
			args := strings.Split(strings.Split(line, "archive '$commit' ")[1], " | tar")[0]
			root := t.TempDir()
			run := func(args ...string) []byte {
				t.Helper()
				cmd := exec.Command("git", args...)
				cmd.Dir = root
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %v: %s", args, err, out)
				}
				return out
			}
			run("init", "-q")
			if err := os.Mkdir(filepath.Join(root, "src"), 0700); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"src/build.sh", "LICENSE.md", "NOTICE.md"} {
				if err := os.WriteFile(filepath.Join(root, name), []byte("fixture"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			run("add", "src", "LICENSE.md", "NOTICE.md")
			run("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
			archiveArgs := append([]string{"archive", "HEAD"}, strings.Fields(args)...)
			reader := tar.NewReader(bytes.NewReader(run(archiveArgs...)))
			for {
				header, err := reader.Next()
				if err != nil {
					break
				}
				if header.Name == "NOTICE.md" {
					return
				}
			}
			t.Fatal("release archive omits NOTICE.md required by build.sh")
			return
		}
	}
	t.Fatal("release archive command not found")
}

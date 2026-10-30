package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/version"
)

var requiredBundleContract = []string{
	"agent-bus", "agent-busd", "agent-bus-admin", "agent-bus-runner", "agent-bus-setup", "agent-bus-token",
	"web/server.ts", "web/agent-bus-web.service",
	".claude-plugin/marketplace.json", "mcp/.claude-plugin/plugin.json", "mcp/channel.ts", "mcp/tags.ts", "mcp/commands/ls.md", "mcp/commands/send.md",
	"agent-bus-ts", // the compiled MCP face, launcher and web face (src/ts-main.ts)
	"launchers/ab-claude", "launchers/ab-codex", "launchers/ab-opencode", "launchers/ab-opencode1",
	"internal/version/VERSION", "LICENSE.md", "licenses/NOTICE.md", "INSTALL.md",
	"agents/slack/agent.json",
	"agents/slack/licenses/NOTICE.md",
	"agents/slack/env.dist",
	"agents/slack/slack.js",
	"agents/slack/filter-worker.js",
	"agents/telegram/agent.json",
	"agents/telegram/licenses/NOTICE.md",
	"agents/telegram/env.dist",
	"agents/telegram/telegram.js",
	"agents/telegram/filter-worker.js",
	"agents/ai/support/agents/shared/provider_http.py",
	"agents/shared/provider_http.py",
	"agents/voyage/agent.json",
	"agents/voyage/env.dist",
	"agents/voyage/voyage.py",
	"agents/voyage/LICENSE.md",
	"agents/voyage/support/agents/shared/metadata.py",
	"agents/voyage/support/agents/shared/provider_http.py",
	"agents/voyage/support/internal/version/VERSION",
	"agents/rag/rag.py",
	"agents/rag/rules.py",
	"agents/rag/core.py",
	"agents/rag/chunks.py",
	"agents/rag/prompts.py",
	"agents/rag/blocks.py",
	"agents/rag/agent.json",
	"agents/rag/env.dist",
	"agents/rag/CHANGELOG.md",
	"agents/rag/config.json.dist",
	"agents/rag/LICENSE.md",
	"agents/rag/licenses/NOTICE.md",
	"agents/rag/support/agents/shared/metadata.py",
	"agents/rag/support/clients/LICENSE.md",
	"agents/rag/support/internal/version/VERSION",
	"agents/rag/support/clients/python/agent_bus/__init__.py",
	"agents/rag/support/clients/python/agent_bus/_version.py",
	"agents/rag/support/clients/python/agent_bus/client.py",
	"agents/rag/support/clients/python/agent_bus/errors.py",
	"agents/rag/support/clients/python/agent_bus/message.py",
	"agents/rag/support/clients/python/agent_bus/storage.py",
	"agents/rag/support/clients/python/agent_bus/transport.py",
	"agents/shared/mail_oauth.py",
	"agents/shared/mail.py",
	"agents/fetch/LICENSE.md",
	"agents/fetch/agent.json",
	"agents/fetch/env.dist",
	"agents/fetch/fetch.py",
	"agents/fetch/worker.py",
	"agents/fetch/policy.py",
	"agents/fetch/transport.py",
	"agents/fetch/extract.py",
	"agents/fetch/support/agents/shared/metadata.py",
	"agents/fetch/support/internal/version/VERSION",
	"agents/webhook/LICENSE.md",
	"agents/webhook/licenses/NOTICE.md",
	"agents/webhook/agent.json",
	"agents/webhook/config.json.dist",
	"agents/webhook/env.dist",
	"agents/webhook/incoming.py",
	"agents/webhook/outgoing.py",
	"agents/webhook/rules.py",
	"agents/webhook/support/agents/shared/envfile.py",
	"agents/webhook/support/agents/shared/metadata.py",
	"agents/webhook/support/clients/LICENSE.md",
	"agents/webhook/support/clients/python/agent_bus/__init__.py",
	"agents/webhook/support/clients/python/agent_bus/_version.py",
	"agents/webhook/support/clients/python/agent_bus/client.py",
	"agents/webhook/support/clients/python/agent_bus/errors.py",
	"agents/webhook/support/clients/python/agent_bus/message.py",
	"agents/webhook/support/clients/python/agent_bus/storage.py",
	"agents/webhook/support/clients/python/agent_bus/transport.py",
	"agents/webhook/support/internal/version/VERSION",
	"agents/webhook/webhook.py",
	"agents/imap/imap.py",
	"agents/imap/cache.py",
	"agents/imap/wire.py",
	"agents/imap/budget.py",
	"agents/imap/matching.py",
	"agents/imap/progress.py",
	"agents/imap/ledger.py",
	"agents/imap/outbox.py",
	"agents/imap/publication.py",
	"agents/imap/delivery.py",
	"agents/imap/subscriptions.py",
	"agents/imap/forwarding.py",
	"agents/imap/mailbox_session.py",
	"agents/imap/agent.json",
	"agents/imap/env.dist",
	"agents/imap/LICENSE.md",
	"agents/imap/licenses/NOTICE.md",
	"agents/imap/support/agents/shared/metadata.py",
	"agents/imap/support/agents/shared/mail.py",
	"agents/imap/support/agents/shared/mail_oauth.py",
	"agents/imap/support/internal/version/VERSION",
	"agents/imap/support/clients/LICENSE.md",
	"agents/imap/support/clients/python/agent_bus/__init__.py",
	"agents/imap/support/clients/python/agent_bus/_version.py",
	"agents/imap/support/clients/python/agent_bus/client.py",
	"agents/imap/support/clients/python/agent_bus/errors.py",
	"agents/imap/support/clients/python/agent_bus/message.py",
	"agents/imap/support/clients/python/agent_bus/storage.py",
	"agents/imap/support/clients/python/agent_bus/transport.py",
	"agents/cron/cron.py",
	"agents/cron/agent.json",
	"agents/cron/LICENSE.md",
	"agents/cron/licenses/NOTICE.md",
	"agents/cron/support/agents/shared/metadata.py",
	"agents/cron/support/agents/shared/scheduler.py",
	"agents/cron/support/agents/shared/cronexpr.py",
	"agents/cron/support/internal/version/VERSION",
	"agents/cron/support/clients/LICENSE.md",
	"agents/cron/support/clients/python/agent_bus/__init__.py",
	"agents/cron/support/clients/python/agent_bus/_version.py",
	"agents/cron/support/clients/python/agent_bus/client.py",
	"agents/cron/support/clients/python/agent_bus/errors.py",
	"agents/cron/support/clients/python/agent_bus/message.py",
	"agents/cron/support/clients/python/agent_bus/storage.py",
	"agents/cron/support/clients/python/agent_bus/transport.py",
	"agents/at/at.py",
	"agents/at/agent.json",
	"agents/at/LICENSE.md",
	"agents/at/licenses/NOTICE.md",
	"agents/at/support/agents/shared/metadata.py",
	"agents/at/support/agents/shared/scheduler.py",
	"agents/at/support/agents/shared/cronexpr.py",
	"agents/at/support/internal/version/VERSION",
	"agents/at/support/clients/LICENSE.md",
	"agents/at/support/clients/python/agent_bus/__init__.py",
	"agents/at/support/clients/python/agent_bus/_version.py",
	"agents/at/support/clients/python/agent_bus/client.py",
	"agents/at/support/clients/python/agent_bus/errors.py",
	"agents/at/support/clients/python/agent_bus/message.py",
	"agents/at/support/clients/python/agent_bus/storage.py",
	"agents/at/support/clients/python/agent_bus/transport.py",
	"agents/shared/scheduler.py",
	"agents/shared/cronexpr.py",
	"agents/eventlog/agent.json",
	"agents/eventlog/eventlog.py",
	"agents/eventlog/env.dist",
	"agents/eventlog/config.json.dist",
	"agents/eventlog/LICENSE.md",
	"agents/eventlog/licenses/NOTICE.md",
	"agents/eventlog/support/agents/shared/metadata.py",
	"agents/eventlog/support/agents/shared/event_rings.py",
	"agents/eventlog/support/agents/shared/subscriptions.py",
	"agents/eventlog/support/agents/shared/subscription_filters.py",
	"agents/eventlog/support/clients/LICENSE.md",
	"agents/eventlog/support/internal/version/VERSION",
	"agents/eventlog/support/clients/python/agent_bus/__init__.py",
	"agents/eventlog/support/clients/python/agent_bus/_version.py",
	"agents/eventlog/support/clients/python/agent_bus/client.py",
	"agents/eventlog/support/clients/python/agent_bus/errors.py",
	"agents/eventlog/support/clients/python/agent_bus/message.py",
	"agents/eventlog/support/clients/python/agent_bus/storage.py",
	"agents/eventlog/support/clients/python/agent_bus/transport.py",
	"agents/shared/event_rings.py",
	"agents/shared/subscriptions.py",
	"agents/shared/subscription_filters.py",
	"agents/smtp/agent.json",
	"agents/smtp/env.dist",
	"agents/smtp/smtp.py",
	"agents/smtp/LICENSE.md",
	"agents/smtp/support/agents/shared/metadata.py",
	"agents/smtp/support/agents/shared/mail_oauth.py",
	"agents/smtp/support/agents/shared/mail.py",
	"agents/smtp/support/internal/version/VERSION",
	"agents/qdrant/agent.json",
	"agents/qdrant/env.dist",
	"agents/qdrant/qdrant.py",
	"agents/qdrant/LICENSE.md",
	"agents/qdrant/support/agents/shared/metadata.py",
	"agents/qdrant/support/agents/shared/provider_http.py",
	"agents/qdrant/support/internal/version/VERSION",
	"agents/ai/agent.json",
	"agents/ai/env.dist",
	"agents/ai/ai.py",
	"agents/ai/LICENSE.md",
	"agents/ai/support/agents/shared/metadata.py", "agents/ai/support/internal/version/VERSION",
	"agents/sms-telnyx/support/agents/shared/metadata.py", "agents/sms-telnyx/support/internal/version/VERSION",
	"agents/sms-telnyx/agent.json", "agents/sms-telnyx/env.dist", "agents/sms-telnyx/sms.py", "agents/sms-telnyx/LICENSE.md",
	"agents/debug/agent.json",
	"agents/debug/licenses/NOTICE.md",
	"agents/debug/env.dist",
	"agents/debug/debug.py",
	"agents/shared/metadata.py", "agents/shared/envfile.py",
	"agents/LICENSE.md",
	"agents/licenses/NOTICE.md",
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
	"agents/standalone/tmux-sessions/agent.json", "agents/standalone/tmux-sessions/config.json.dist",
	"agents/standalone/tmux-sessions/sessions.py",
	"agents/standalone/tmux-sessions/install.py", "agents/standalone/tmux-sessions/package.sh",
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
}

func fixtureBundle(t *testing.T, suffix string) string {
	return fixtureBundleVersion(t, version.String, suffix)
}

func fixtureBundleVersion(t *testing.T, bundleVersion, suffix string) string {
	t.Helper()
	root := t.TempDir()
	var lines []string
	for _, name := range bundleFiles {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		body := []byte("artifact " + name + " " + suffix + "\n")
		if name == "internal/version/VERSION" {
			body = []byte(bundleVersion + "\n")
		}
		mode := os.FileMode(0o644)
		if executableFiles[name] {
			mode = 0o755
		}
		if err := os.WriteFile(filepath.Join(root, name), body, mode); err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256(body)
		lines = append(lines, fmt.Sprintf("%x  %s", h, name))
	}
	sort.Strings(lines)
	if err := os.WriteFile(filepath.Join(root, "MANIFEST.sha256"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestInstalledRollbackBundleMayHaveThePreviousVersion(t *testing.T) {
	root := fixtureBundleVersion(t, "0.5.68", "old")
	if _, err := checkBundle(root); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("incoming old-version bundle: %v", err)
	}
	b, err := checkBundleVersion(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(b.releaseID, "0.5.68-") {
		t.Fatalf("rollback release id=%s", b.releaseID)
	}
}

func withInstallPaths(t *testing.T) (root, bin string) {
	t.Helper()
	oldRoot, oldBin := installRoot, installBin
	base := t.TempDir()
	installRoot, installBin = filepath.Join(base, "lib"), filepath.Join(base, "bin")
	t.Cleanup(func() { installRoot, installBin = oldRoot, oldBin })
	return installRoot, installBin
}

func TestBundleMustBeCompleteBeforeInstallation(t *testing.T) {
	if !slices.Equal(bundleFiles, requiredBundleContract) {
		t.Fatalf("bundle contract=%q, want %q", bundleFiles, requiredBundleContract)
	}
	root := fixtureBundle(t, "complete")
	if err := os.Remove(filepath.Join(root, "agent-bus-ts")); err != nil {
		t.Fatal(err)
	}
	if _, err := checkBundle(root); err == nil || !strings.Contains(err.Error(), "agent-bus-ts") {
		t.Fatalf("missing compiled MCP face: %v", err)
	}
}

func TestBundleRejectsDamagedAndNonExecutableArtifacts(t *testing.T) {
	root := fixtureBundle(t, "damage")
	if err := os.WriteFile(filepath.Join(root, "agent-bus-ts"), []byte("changed"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := checkBundle(root); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("damaged face: %v", err)
	}

	root = fixtureBundle(t, "mode")
	if err := os.Chmod(filepath.Join(root, "agent-busd"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := checkBundle(root); err == nil || !strings.Contains(err.Error(), "not executable") {
		t.Fatalf("non-executable daemon: %v", err)
	}
}

func TestBundleManifestIsAnExactAllowlist(t *testing.T) {
	root := fixtureBundle(t, "extra")
	extra := []byte("not installed\n")
	h := sha256.Sum256(extra)
	f, err := os.OpenFile(filepath.Join(root, "MANIFEST.sha256"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(f, "%x  extra-file\n", h); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "extra-file"), extra, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := checkBundle(root); err == nil || !strings.Contains(err.Error(), "not a release artifact") {
		t.Fatalf("extra manifest entry: %v", err)
	}
}

// The web face's tree rides along: every file under web/ the manifest names
// is checked and installed, and a changed one is refused.
func TestWebSourcesAreInstalledAndChecked(t *testing.T) {
	withInstallPaths(t)
	root := fixtureBundle(t, "web")
	src := []byte("export const x = 1;\n")
	if err := os.WriteFile(filepath.Join(root, "web", "config.ts"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(src)
	f, err := os.OpenFile(filepath.Join(root, "MANIFEST.sha256"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(f, "%x  web/config.ts\n", h)
	f.Close()
	b, err := checkBundle(root)
	if err != nil {
		t.Fatal(err)
	}
	installed, err := installBundle(b)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(installed, "web", "config.ts")); err != nil || string(got) != string(src) {
		t.Fatalf("web/config.ts was not installed: %q %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(root, "web", "config.ts"), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := checkBundle(root); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("a changed web source: %v", err)
	}
}

func TestInstallBundleSwitchesOneCompleteReleaseAndIsIdempotent(t *testing.T) {
	root, bin := withInstallPaths(t)
	b, err := checkBundle(fixtureBundle(t, "first"))
	if err != nil {
		t.Fatal(err)
	}
	current, err := installBundle(b)
	if err != nil {
		t.Fatal(err)
	}
	if current != filepath.Join(root, "current") {
		t.Fatalf("current=%s", current)
	}
	target, err := os.Readlink(current)
	if err != nil || target != filepath.Join("releases", b.releaseID) {
		t.Fatalf("current target=%q err=%v", target, err)
	}
	info, err := os.Stat(filepath.Join(root, "releases", b.releaseID))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("release mode=%v", info.Mode().Perm())
	}
	for name, want := range commandLinks() {
		got, err := os.Readlink(filepath.Join(bin, name))
		if err != nil || got != want {
			t.Errorf("%s target=%q want=%q err=%v", name, got, want, err)
		}
	}
	if _, err := installBundle(b); err != nil {
		t.Fatalf("identical reinstall: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "releases"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("release entries=%d err=%v", len(entries), err)
	}
}

func TestDifferentBundleRequiresExplicitUpgradeSelection(t *testing.T) {
	root, _ := withInstallPaths(t)
	first, err := checkBundle(fixtureBundle(t, "first"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := checkBundle(fixtureBundle(t, "second"))
	if err != nil {
		t.Fatal(err)
	}
	if first.releaseID == second.releaseID {
		t.Fatal("different bundle got the same release id")
	}
	if _, err := installBundle(first); err != nil {
		t.Fatal(err)
	}
	if _, err := installBundle(second); err == nil || !strings.Contains(err.Error(), "--upgrade") {
		t.Fatalf("ordinary setup selected a different release: %v", err)
	}
	target, err := os.Readlink(filepath.Join(root, "current"))
	if err != nil || target != filepath.Join("releases", first.releaseID) {
		t.Fatalf("current target=%q err=%v", target, err)
	}
	if _, err := stageBundle(second); err != nil {
		t.Fatal(err)
	}
	if _, err := selectBundle(second.releaseID); err != nil {
		t.Fatal(err)
	}
	target, err = os.Readlink(filepath.Join(root, "current"))
	if err != nil || target != filepath.Join("releases", second.releaseID) {
		t.Fatalf("upgrade target=%q err=%v", target, err)
	}
}

func TestDamagedExistingReleaseIsNeverSelectedAgain(t *testing.T) {
	root, _ := withInstallPaths(t)
	b, err := checkBundle(fixtureBundle(t, "installed"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := installBundle(b); err != nil {
		t.Fatal(err)
	}
	damaged := filepath.Join(root, "releases", b.releaseID, "agent-bus-ts")
	if err := os.WriteFile(damaged, []byte("damaged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := installBundle(b); err == nil || !strings.Contains(err.Error(), "damaged") {
		t.Fatalf("damaged installed release: %v", err)
	}
}

func TestCommandCollisionLeavesReleaseUnselected(t *testing.T) {
	root, bin := withInstallPaths(t)
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "agent-bus"), []byte("unrelated"), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := checkBundle(fixtureBundle(t, "collision"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := installBundle(b); err == nil || !strings.Contains(err.Error(), "unrelated command") {
		t.Fatalf("collision err=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "current")); !os.IsNotExist(err) {
		t.Fatalf("current exists after collision: %v", err)
	}
}

// An installed release from before 0.8.50 is a rollback target by its own
// manifest: the Go web binary and no web/ tree. It is still refused as an
// incoming package, and it still needs its daemon.
func TestOlderReleaseIsARollbackTargetByItsOwnManifest(t *testing.T) {
	old := func(t *testing.T, drop string) string {
		t.Helper()
		root := t.TempDir()
		var lines []string
		names := []string{"agent-bus", "agent-busd", "agent-bus-admin", "agent-bus-setup", "agent-bus-token", "agent-bus-web",
			".claude-plugin/marketplace.json", "mcp/.claude-plugin/plugin.json", "mcp/channel.ts", "mcp/commands/ls.md", "mcp/commands/send.md",
			"mcp/server.js", "launchers/launcher.js", "internal/version/VERSION", "LICENSE.md", "licenses/NOTICE.md", "INSTALL.md"}
		for _, name := range names {
			if name == drop {
				continue
			}
			body := []byte("old " + name + "\n")
			if name == "internal/version/VERSION" {
				body = []byte("0.8.27\n")
			}
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, name), body, 0o755); err != nil {
				t.Fatal(err)
			}
			lines = append(lines, fmt.Sprintf("%x  %s", sha256.Sum256(body), name))
		}
		if err := os.WriteFile(filepath.Join(root, "MANIFEST.sha256"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return root
	}
	root := old(t, "")
	if b, err := checkBundleVersion(root, ""); err != nil || !strings.HasPrefix(b.releaseID, "0.8.27-") {
		t.Fatalf("0.8.27-shaped rollback target: %v", err)
	}
	if _, err := checkBundle(root); err == nil || !strings.Contains(err.Error(), "agent-bus-runner") {
		t.Fatalf("0.8.27-shaped incoming package: %v", err)
	}
	if _, err := checkBundleVersion(old(t, "agent-busd"), ""); err == nil || !strings.Contains(err.Error(), "agent-busd") {
		t.Fatalf("rollback target without its daemon: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "agent-bus-web"), []byte("changed"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := checkBundleVersion(root, ""); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("changed legacy web binary: %v", err)
	}
}

// Every bundled runner package's command is installed executable: setup
// writes everything else 0644, and a runner cannot start a 0644 script.
func TestEveryBundledAgentCommandIsInstalledExecutable(t *testing.T) {
	checked := 0
	for _, name := range bundleFiles {
		parts := strings.Split(name, "/")
		if len(parts) != 3 || parts[0] != "agents" || parts[2] != "agent.json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("../..", name))
		if err != nil {
			t.Fatal(err)
		}
		var m struct{ Command string }
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.HasPrefix(m.Command, "./") {
			continue
		}
		script := "agents/" + parts[1] + "/" + strings.TrimPrefix(strings.Fields(m.Command)[0], "./")
		if !slices.Contains(bundleFiles, script) {
			continue // built into another file, such as slack.ts into slack.js
		}
		if !executableFiles[script] {
			t.Errorf("%s runs %s, which setup would install 0644", name, script)
		}
		checked++
	}
	if checked < 8 {
		t.Fatalf("checked only %d packages", checked)
	}
}

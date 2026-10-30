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
	files := []string{"agent-bus", "agent-busd", "agent-bus-admin", "agent-bus-runner", "agent-bus-setup", "agent-bus-token", "web/server.ts", "web/agent-bus-web.service", ".claude-plugin/marketplace.json", "mcp/.claude-plugin/plugin.json", "mcp/channel.ts", "mcp/tags.ts", "mcp/commands/ls.md", "mcp/commands/send.md",
		"agent-bus-ts", "launchers/ab-claude", "launchers/ab-codex", "launchers/ab-opencode", "launchers/ab-opencode1", "LICENSE.md", "NOTICE.md", "THIRD-PARTY-NOTICES.md", "INSTALL.md"}
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
		"agents/rag/NOTICE.md",
		"agents/rag/THIRD-PARTY-NOTICES.md",
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
		"agents/webhook/NOTICE.md",
		"agents/webhook/THIRD-PARTY-NOTICES.md",
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
		"agents/imap/NOTICE.md",
		"agents/imap/THIRD-PARTY-NOTICES.md",
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
		"agents/cron/NOTICE.md",
		"agents/cron/THIRD-PARTY-NOTICES.md",
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
		"agents/at/NOTICE.md",
		"agents/at/THIRD-PARTY-NOTICES.md",
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
		"agents/eventlog/NOTICE.md",
		"agents/eventlog/THIRD-PARTY-NOTICES.md",
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
		"agents/debug/THIRD-PARTY-NOTICES.md",
		"agents/debug/env.dist",
		"agents/debug/debug.py",
		"agents/shared/metadata.py", "agents/shared/envfile.py",
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

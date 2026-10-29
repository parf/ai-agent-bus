package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundledAgentsRunOutsideCheckout(t *testing.T) {
	root := t.TempDir()
	script, err := filepath.Abs("../../agents/package.sh")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", script, root)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("export runtime: %v: %s", err, out)
	}
	check, err := filepath.Abs("../../acceptance/agent-runtime.py")
	if err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("python3", check, root)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("outside-checkout runtime: %v: %s", err, out)
	}
	for _, name := range []string{"agents/shared/metadata.py", "agents/shared/envfile.py", "agents/shared/mail_oauth.py", "agents/smtp/support/agents/shared/mail_oauth.py", "agents/webhook/rules.py", "agents/webhook/incoming.py", "agents/webhook/outgoing.py", "agents/webhook/support/agents/shared/envfile.py", "agents/imap/cache.py", "agents/imap/wire.py", "agents/imap/support/agents/shared/mail.py", "agents/imap/support/agents/shared/mail_oauth.py", "agents/imap/support/clients/python/agent_bus/client.py", "agents/shared/mail.py", "agents/smtp/support/agents/shared/mail.py", "agents/shared/provider_http.py", "agents/ai/support/agents/shared/provider_http.py", "agents/voyage/support/agents/shared/provider_http.py", "agents/qdrant/support/agents/shared/provider_http.py", "agents/rag/core.py", "agents/rag/support/agents/shared/metadata.py", "agents/rag/support/clients/python/agent_bus/client.py", "agents/standalone/tmux-sessions/config.json.dist", "clients/python/agent_bus/client.py", "agents/standalone/logwatch/logwatch.py", "agents/cron/support/agents/shared/scheduler.py", "agents/at/support/agents/shared/cronexpr.py", "agents/at/support/clients/python/agent_bus/client.py"} {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
		cmd = exec.Command("python3", "-S", check, root)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("missing %s passed runtime check: %s", name, out)
		}
		if strings.Contains(string(out), "Traceback (most recent call last)") || !strings.Contains(string(out), "agent runtime check failed:") {
			t.Fatalf("missing %s must fail without an unhandled Python exception: %s", name, out)
		}
		if err := os.WriteFile(filepath.Join(root, name), content, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBuildShipsAgentRuntimeAndSetupKeepsItRunnable(t *testing.T) {
	root := t.TempDir()
	tools := t.TempDir()
	// Build the real runtime output without recompiling the Go programs inside
	// their own test. The TypeScript bundles and copied native files are real.
	if err := os.WriteFile(filepath.Join(tools, "go"), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs("../../build.sh")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", script, root)
	cmd.Env = append(os.Environ(), "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build runtime: %v: %s", err, out)
	}
	installed := t.TempDir()
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		return copyBundleFile(root, installed, name)
	})
	if err != nil {
		t.Fatal(err)
	}
	check, err := filepath.Abs("../../acceptance/agent-runtime.py")
	if err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("python3", check, installed)
	cmd.Dir = installed
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("installed runtime outside checkout: %v: %s", err, out)
	}
}

func TestMissingAgentHelperIsRefusedBeforeSetup(t *testing.T) {
	for _, name := range []string{"mcp/tags.ts", "agents/shared/metadata.py", "agents/shared/envfile.py", "agents/shared/mail_oauth.py", "agents/smtp/support/agents/shared/mail_oauth.py", "agents/webhook/rules.py", "agents/webhook/incoming.py", "agents/webhook/outgoing.py", "agents/webhook/support/agents/shared/envfile.py", "agents/imap/cache.py", "agents/imap/wire.py", "agents/imap/support/agents/shared/mail.py", "agents/imap/support/agents/shared/mail_oauth.py", "agents/imap/support/clients/python/agent_bus/client.py", "agents/shared/mail.py", "agents/smtp/support/agents/shared/mail.py", "agents/shared/provider_http.py", "agents/ai/support/agents/shared/provider_http.py", "agents/voyage/support/agents/shared/provider_http.py", "agents/qdrant/support/agents/shared/provider_http.py", "agents/rag/core.py", "agents/rag/support/agents/shared/metadata.py", "agents/rag/support/clients/python/agent_bus/client.py", "agents/standalone/tmux-sessions/config.json.dist", "agents/ai/support/agents/shared/metadata.py", "agents/ai/support/internal/version/VERSION", "agents/sms-telnyx/support/agents/shared/metadata.py", "agents/sms-telnyx/support/internal/version/VERSION", "agents/cron/cron.py", "agents/at/support/agents/shared/scheduler.py"} {
		t.Run(name, func(t *testing.T) {
			root := fixtureBundle(t, "agent-helper")
			if err := os.Remove(filepath.Join(root, name)); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "MANIFEST.sha256")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var kept []string
			for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
				if !strings.HasSuffix(line, "  "+name) {
					kept = append(kept, line)
				}
			}
			if err := os.WriteFile(path, []byte(strings.Join(kept, "\n")+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := checkBundle(root); err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("missing helper %s accepted or unnamed: %v", name, err)
			}
		})
	}
}

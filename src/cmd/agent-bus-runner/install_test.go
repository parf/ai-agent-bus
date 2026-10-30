package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestDeployReplacementIsExplicitAndReplacesTheWholeTree(t *testing.T) {
	r, b, _ := configurationFixture(t, "", "printf old")
	r.bundled = t.TempDir()
	directoryAgent(t, r.bundled, "fixture", "deployed")
	old := filepath.Join(r.agents, "fixture", "obsolete")
	writeFile(t, old, "stale")
	before, _ := os.ReadFile(filepath.Join(r.agents, "fixture", "agent.json"))
	if err := r.installPackage(context.Background(), "install fixture"); err == nil || !strings.Contains(err.Error(), "--replace") {
		t.Fatalf("implicit replacement accepted: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(r.agents, "fixture", "agent.json")); string(got) != string(before) {
		t.Fatal("refused install changed package")
	}
	// Use the real bus-delivered management role and answer path.
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: r.name, Body: "install --replace fixture"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	e, err := b.Consume(ctx, r.name, "", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	r.handle(e)
	if answer := configurationReply(t, b).Body; strings.HasPrefix(answer, "error:") {
		t.Fatal(answer)
	}
	if _, err := os.Lstat(old); !os.IsNotExist(err) {
		t.Fatalf("whole-tree replacement retained obsolete file: %v", err)
	}
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: "probe"}); err != nil {
		t.Fatal(err)
	}
	if got := configurationReply(t, b).Body; got != "deployed" {
		t.Fatalf("installed agent did not serve new code: %q", got)
	}
}

func TestDeployRemoveKeepsDataTokenAndBusRecord(t *testing.T) {
	r, b, _ := configurationFixture(t, "", "printf old")
	token := r.tokenFile("#fixture@h")
	before, err := os.ReadFile(token)
	if err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(r.host, "work", "fixture", "kept")
	writeFile(t, work, "work-data")
	writeFile(t, filepath.Join(r.host, "fixture", "env"), "SECRET=private-sentinel\n")
	writeFile(t, filepath.Join(r.host, "fixture", "agent.json"), `{"idle":"2s"}`)
	r.handle(protocol.Envelope{From: "owner@h", Body: "remove fixture", Roles: []string{"owner"}})
	if got := configurationReply(t, b).Body; strings.HasPrefix(got, "error:") || !strings.Contains(got, "kept work data, tokens and bus records") {
		t.Fatalf("remove did not report kept state: %s", got)
	}
	for _, p := range []string{filepath.Join(r.agents, "fixture"), filepath.Join(r.host, "fixture")} {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Fatalf("remove left configuration/package %s: %v", p, err)
		}
	}
	if got, err := os.ReadFile(token); err != nil || string(got) != string(before) {
		t.Fatalf("remove lost token: %q %v", got, err)
	}
	if got, err := os.ReadFile(work); err != nil || string(got) != "work-data" {
		t.Fatalf("remove lost data: %q %v", got, err)
	}
	if _, ok := b.Lookup("owner@h", "#fixture@h"); !ok {
		t.Fatal("remove lost bus record")
	}
	entries, err := readAgents(r.host)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := entries["fixture"]; ok {
		t.Fatal("remove kept inventory entry")
	}
}

func TestDeployRefusesBundledRemovalInsteadOfExposingItsFallback(t *testing.T) {
	r, _, _ := configurationFixture(t, "", "printf old")
	r.bundled = t.TempDir()
	directoryAgent(t, r.bundled, "fixture", "bundled")
	if err := r.removePackage("fixture"); err == nil || !strings.Contains(err.Error(), "fixture") || !strings.Contains(err.Error(), "disable it instead") {
		t.Fatalf("bundled-name remove accepted or unclear: %v", err)
	}
	if _, err := os.Stat(filepath.Join(r.agents, "fixture", "agent.json")); err != nil {
		t.Fatalf("refused remove changed shared tree: %v", err)
	}
}

func TestDeployRefusesReadOnlyAgentsDirectory(t *testing.T) {
	r, _, _ := configurationFixture(t, "", "printf old")
	r.bundled = t.TempDir()
	directoryAgent(t, r.bundled, "fixture", "bundled")
	if err := os.Chmod(r.agents, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(r.agents, 0700)
	if err := r.installPackage(context.Background(), "install --replace fixture"); err == nil || !strings.Contains(err.Error(), "writable --agents") {
		t.Fatalf("read-only deploy accepted or unclear: %v", err)
	}
}

func TestDeployRefusesSubdirectoryPackagesAndEscapingLinks(t *testing.T) {
	root := t.TempDir()
	directoryAgent(t, root, "nested", "nested")
	if err := validateDeployTree(root); err == nil || !strings.Contains(err.Error(), "agent.json at the repository root") {
		t.Fatalf("subdirectory package accepted: %v", err)
	}
	directoryAgent(t, filepath.Dir(root), filepath.Base(root), "root")
	outside := filepath.Join(t.TempDir(), "secret")
	writeFile(t, outside, "outside")
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := validateDeployTree(root); err == nil || !strings.Contains(err.Error(), "self-contained root") {
		t.Fatalf("escaping symlink accepted: %v", err)
	}
}

func TestDeployCallsRequireDirectManagementAndNeverReturnSecrets(t *testing.T) {
	r, b, _ := configurationFixture(t, "", "printf old")
	r.bundled = t.TempDir()
	directoryAgent(t, r.bundled, "fixture", "bundled")
	writeFile(t, filepath.Join(r.host, "fixture", "env"), "SECRET=private-sentinel\n")
	for _, body := range []string{"install --replace fixture", "remove fixture"} {
		for _, roles := range [][]string{nil, {"owner"}} {
			original := ""
			if len(roles) > 0 {
				original = "#forwarder@h"
			}
			r.handle(protocol.Envelope{From: "owner@h", Body: body, Roles: roles, OriginalTo: original})
			if got := configurationReply(t, b).Body; !strings.HasPrefix(got, "error:") || !strings.Contains(got, "Owner and Maintainers") {
				t.Fatalf("unauthorized deploy accepted: %s", got)
			}
		}
	}
	r.handle(protocol.Envelope{From: "owner@h", Body: "read runner/fixture/env", Roles: []string{"owner"}})
	if got := configurationReply(t, b).Body; strings.Contains(got, "private-sentinel") || !strings.Contains(got, "SECRET") {
		t.Fatalf("secret returned or names missing: %s", got)
	}
	for _, path := range []string{"runner/.deploy/id", "runner/.deploy/ssh_config", "runner/.deploy/known_hosts"} {
		if _, _, err := r.hostFile(path); err == nil {
			t.Fatalf("deploy credential reachable through file calls: %s", path)
		}
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeploySourcesAreBundledNamesOrFullPinnedGitCommits(t *testing.T) {
	for _, body := range []string{"install cron", "install --replace cron", "install helper git https://example.invalid/repo.git " + strings.Repeat("a", 40), "install --replace helper git ssh://host/repo " + strings.Repeat("B", 64)} {
		in, err := parseInstall(body)
		if err != nil || in.agent == "" || in.replace != strings.Contains(body, "--replace") {
			t.Fatalf("valid selector %q: %+v %v", body, in, err)
		}
		if in.remote != "" && (!deployCommit.MatchString(in.commit) || in.commit != strings.ToLower(in.commit)) {
			t.Fatalf("unusable commit pin: %+v", in)
		}
	}
	for _, revision := range []string{"main", "v1.0", "HEAD", "HEAD~1", "0123456", strings.Repeat("a", 39), strings.Repeat("a", 41), strings.Repeat("a", 63), strings.Repeat("z", 40)} {
		if _, err := parseInstall("install helper git https://example.invalid/repo " + revision); err == nil || !strings.Contains(err.Error(), "full 40- or 64-hex") {
			t.Errorf("non-pin %q accepted or wrong refusal: %v", revision, err)
		}
	}
	for _, body := range []string{"install", "install ../outside", "install agent/instance", "install /absolute", "install --unknown cron", "install cron archive body", "install cron\nPRIVATE_KEY=sent", "install --replace", "remove cron"} {
		if _, err := parseInstall(body); err == nil {
			t.Errorf("invalid deploy selector accepted: %q", body)
		}
	}
}

func TestDeployURLsNeverCarryCredentialsOrSelectOtherProtocols(t *testing.T) {
	for _, remote := range []string{"https://host/repo", "ssh://host/repo", "ssh://host:2222/repo"} {
		if err := checkDeployURL(remote); err != nil {
			t.Fatalf("valid URL refused: %v", err)
		}
	}
	for _, remote := range []string{"file:///host/tree", "/host/tree", "git@host:repo", "git://host/repo", "http://host/repo", "ext::command", "https://user:private-sentinel@host/repo", "ssh://user@host/repo", "https://host/repo?key=private-sentinel", "https://host/repo#private-sentinel", "https://host/repo?", "https://host", "ssh:///repo", "https://host/\narg", "https://host/repo%0Acommand", "ssh://-option/repo"} {
		err := checkDeployURL(remote)
		if err == nil {
			t.Errorf("unsafe deploy URL accepted: %q", remote)
		} else if strings.Contains(err.Error(), "private-sentinel") {
			t.Errorf("refusal exposed a credential: %v", err)
		}
	}
}

func TestBundledDeploySelectionNeverUsesSharedOrArbitraryHostCode(t *testing.T) {
	r := &runner{bundled: filepath.Join(t.TempDir(), "bundled"), agents: filepath.Join(t.TempDir(), "shared")}
	for _, path := range []string{filepath.Join(r.bundled, "cron"), filepath.Join(r.bundled, "standalone", "sessions"), filepath.Join(r.agents, "cron"), filepath.Join(r.agents, "host-only")} {
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(path, "agent.json"), `{"description":"fixture","command":"./script"}`)
	}
	for name, want := range map[string]string{"cron": filepath.Join(r.bundled, "cron"), "sessions": filepath.Join(r.bundled, "standalone", "sessions")} {
		got, err := r.bundledInstallSource(name)
		if err != nil || got != want {
			t.Fatalf("bundled %s: %s %v, want %s", name, got, err, want)
		}
	}
	if err := os.Symlink(filepath.Join(r.agents, "cron"), filepath.Join(r.bundled, "linked")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"host-only", "../shared/cron", "linked", "missing"} {
		if _, err := r.bundledInstallSource(name); err == nil {
			t.Errorf("non-bundled source %q accepted", name)
		}
	}
}

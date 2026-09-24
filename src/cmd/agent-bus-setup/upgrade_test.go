package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnitAddressRequiresExplicitManagedAddress(t *testing.T) {
	unit := []byte("[Service]\nExecStart=/usr/local/lib/agent-bus/current/agent-busd -addr 127.0.0.1:7777 -web\n")
	got, err := unitAddress(unit)
	if err != nil || got != "127.0.0.1:7777" {
		t.Fatalf("address=%q err=%v", got, err)
	}
	if _, err := unitAddress([]byte("[Service]\nExecStart=/bin/agent-busd -web\n")); err == nil || !strings.Contains(err.Error(), "-addr") {
		t.Fatalf("missing address: %v", err)
	}
}

func TestCurrentReleaseRejectsTargetsOutsideReleaseDirectory(t *testing.T) {
	root, _ := withInstallPaths(t)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../elsewhere", filepath.Join(root, "current")); err != nil {
		t.Fatal(err)
	}
	if _, err := currentReleaseID(); err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("unsafe current target: %v", err)
	}
}

func TestUpgradeJournalIsDurableAndStrict(t *testing.T) {
	root, _ := withInstallPaths(t)
	j := upgradeJournal{OldRelease: "0.5.68-old", NewRelease: "0.5.69-new", Backup: "/state/backup", UnitSHA256: "abc"}
	if err := writeUpgradeJournal(j); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, upgradeJournalName))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"old_release": "0.5.68-old"`, `"new_release": "0.5.69-new"`, `"backup": "/state/backup"`} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("journal missing %s: %s", want, b)
		}
	}
	info, err := os.Stat(filepath.Join(root, upgradeJournalName))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("journal mode=%v", info.Mode().Perm())
	}
}

func TestStateBackupAndRestoreReplaceTheWholeTree(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "daemon")
	backup := filepath.Join(base, "backups", "old")
	if err := os.MkdirAll(filepath.Join(source, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "dump.json"), []byte("old snapshot\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "token"), []byte("old credential\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".ssh", "authorized_keys"), []byte("old key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("dump.json", filepath.Join(source, "snapshot-link")); err != nil {
		t.Fatal(err)
	}
	if err := copyTreeAtomic(source, backup); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "dump.json"), []byte("migrated snapshot\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(source, "token")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "new-file"), []byte("must disappear\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := restoreTreeAtomic(backup, source); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"dump.json": "old snapshot\n", "token": "old credential\n", filepath.Join(".ssh", "authorized_keys"): "old key\n",
	} {
		got, err := os.ReadFile(filepath.Join(source, name))
		if err != nil || string(got) != want {
			t.Errorf("%s=%q err=%v", name, got, err)
		}
	}
	if _, err := os.Stat(filepath.Join(source, "new-file")); !os.IsNotExist(err) {
		t.Fatalf("new-release residue survived restore: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(backup, "dump.json")); err != nil || string(got) != "old snapshot\n" {
		t.Fatalf("backup was consumed: %q %v", got, err)
	}
	if target, err := os.Readlink(filepath.Join(source, "snapshot-link")); err != nil || target != "dump.json" {
		t.Fatalf("state symlink=%q err=%v", target, err)
	}
	if info, err := os.Stat(filepath.Join(source, "token")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("credential mode=%v err=%v", info.Mode().Perm(), err)
	}
}

func TestCgroupProcessInspectionDescendsDelegatedSubgroups(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"cgroup.procs": "101\n",
		filepath.Join("supervisor", "cgroup.procs"): "202\n",
		filepath.Join("web", "cgroup.procs"):        "303\n202\n",
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pids, err := cgroupPIDs(root)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, pid := range pids {
		seen[pid] = true
	}
	for _, want := range []string{"101", "202", "303"} {
		if !seen[want] {
			t.Errorf("missing delegated pid %s from %v", want, pids)
		}
	}
}

func TestAReinstallRefusesAnUnreadableCurrentRelease(t *testing.T) {
	root, _ := withInstallPaths(t)
	if id, err := priorReleaseForRollback(); err != nil || id != "" {
		t.Fatalf("no release yet: %q, %v; want a first install", id, err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if id, err := priorReleaseForRollback(); err != nil || id != "" {
		t.Fatalf("no release yet: %q, %v; want a first install", id, err)
	}
	if err := os.Symlink("/elsewhere", filepath.Join(root, "current")); err != nil {
		t.Fatal(err)
	}
	if _, err := priorReleaseForRollback(); err == nil || !strings.Contains(err.Error(), "could not be rolled back") {
		t.Fatalf("an unsafe current link was accepted for rollback: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "current")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("releases/0.8.24-abc", filepath.Join(root, "current")); err != nil {
		t.Fatal(err)
	}
	if id, err := priorReleaseForRollback(); err != nil || id != "0.8.24-abc" {
		t.Fatalf("a relative release link, as release.sh writes it: %q, %v", id, err)
	}
}

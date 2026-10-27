package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordTransferOptionsAreExplicit(t *testing.T) {
	for _, args := range [][]string{{"--info=x"}, {"--out=x", "--info=y"}, {"--replace=all"}, {"--out"}} {
		if _, e := parseTransfer("export", args); e == nil {
			t.Fatalf("accepted export %v", args)
		}
	}
	out, e := parseTransfer("export", []string{"--include=name:#x@h,tag:ops", "--include=tag:messaging", "--exclude=name:#y@h", "--out=-", "--info=backup.info"})
	if e != nil || len(out.selection.Include) != 2 || out.info != "backup.info" {
		t.Fatal("export union/stream options")
	}
	out, e = parseTransfer("import", []string{"archive.age", "--rename-realm==new", "--replace=tag:ops", "--replace-tokens=name:#x@new", "--dry-run"})
	if e != nil || out.choices.RenameRealm[""] != "new" || !out.choices.DryRun {
		t.Fatal("import options")
	}
	for _, args := range [][]string{{}, {"a", "b"}, {"a", "--rename-realm=old"}, {"a", "--rename-realm=a=b", "--rename-realm=a=c"}} {
		if _, e = parseTransfer("import", args); e == nil {
			t.Fatalf("accepted import %v", args)
		}
	}
}
func TestWrongImportKeyNeverReachesDaemon(t *testing.T) {
	// An unavailable socket would produce a different error if decryption were
	// skipped. No network or privilege handover occurs before local validation.
	t.Setenv("AGENT_BUS_ADDR", filepath.Join(t.TempDir(), "absent.sock"))
	file := filepath.Join(t.TempDir(), "bad.age")
	if e := os.WriteFile(file, []byte("not encrypted"), 0600); e != nil {
		t.Fatal(e)
	}
	e := recordTransfer("import", []string{file, "--key=" + filepath.Join(t.TempDir(), "missing-key")})
	if e == nil || strings.Contains(e.Error(), "login socket") {
		t.Fatal("invalid archive reached daemon")
	}
}

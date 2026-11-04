package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseSwitchRequiresCurrentLayout(t *testing.T) {
	source, err := os.ReadFile("../../release.sh")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start, end := strings.Index(text, "validate_release()"), strings.Index(text, "\nswitch()")
	if start < 0 || end < start {
		t.Fatal("release validator missing")
	}
	script := `progs=(agent-bus agent-busd agent-bus-admin agent-bus-runner agent-bus-setup)
scripts=(agent-bus-token)
launchers=(ab-claude ab-codex ab-opencode ab-opencode1)
` + text[start:end] + "\nvalidate_release \"$1\"\n"
	switchEnd := strings.Index(text, "\nif [ \"${1:-}\" = --rollback ]")
	if switchEnd < end {
		t.Fatal("release switch missing")
	}
	// Run the actual switch with host operations stubbed; invalid input must
	// leave every link unchanged even when the validator itself still exists.
	switchScript := "set -euo pipefail\n" + script[:strings.Index(script, "validate_release()")] + text[start:switchEnd] + `
root="$2"
bindir="$3"
unit=fixture
systemctl() { echo called > "$root/service-called"; return 1; }
restorecon() { :; }
getenforce() { echo Disabled; }
switch "$1"
`
	installed, bin := t.TempDir(), t.TempDir()
	if err := os.Symlink("releases/current-fixture", filepath.Join(installed, "current")); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	files := []string{"agent-bus", "agent-busd", "agent-bus-admin", "agent-bus-runner", "agent-bus-setup", "agent-bus-token", "agent-bus-ts", "launchers/ab-claude", "launchers/ab-codex", "launchers/ab-opencode", "launchers/ab-opencode1", "internal/version/VERSION", "web/agent-bus-web.service"}
	for _, name := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	check := func(want bool) {
		t.Helper()
		out, err := exec.Command("bash", "-c", script, "fixture", root).CombinedOutput()
		if (err == nil) != want {
			t.Fatalf("layout accepted=%v want=%v: %s", err == nil, want, out)
		}
		if !want {
			out, err = exec.Command("bash", "-c", switchScript, "fixture", root, installed, bin).CombinedOutput()
			if err == nil {
				t.Fatal("switch accepted unsupported layout")
			}
			current, readErr := os.Readlink(filepath.Join(installed, "current"))
			if readErr != nil || current != "releases/current-fixture" {
				t.Fatalf("refused switch changed current: %q %v", current, readErr)
			}
			entries, readErr := os.ReadDir(bin)
			if readErr != nil || len(entries) != 0 {
				t.Fatalf("refused switch installed command links: %v %v", entries, readErr)
			}
			if _, err := os.Stat(filepath.Join(installed, "service-called")); !os.IsNotExist(err) {
				t.Fatal("refused switch touched service")
			}
			if !strings.Contains(string(out), "unsupported release layout") {
				t.Fatalf("wrong switch refusal: %s", out)
			}
		}
	}
	check(true)
	for _, name := range []string{"agent-bus-web", "mcp/server.js", "launchers/launcher.js"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("obsolete"), 0700); err != nil {
			t.Fatal(err)
		}
		check(false)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(filepath.Join(root, "agent-bus-ts")); err != nil {
		t.Fatal(err)
	}
	check(false)
}

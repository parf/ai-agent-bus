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

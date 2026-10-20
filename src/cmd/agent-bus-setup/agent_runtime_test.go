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
	for _, name := range []string{"agents/shared/metadata.py", "clients/python/agent_bus/client.py", "agents/standalone/logwatch/logwatch.py"} {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
		cmd = exec.Command("python3", check, root)
		if out, err := cmd.CombinedOutput(); err == nil {
			t.Fatalf("missing %s passed runtime check: %s", name, out)
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
	root := fixtureBundle(t, "agent-helper")
	if err := os.Remove(filepath.Join(root, "agents/shared/metadata.py")); err != nil {
		t.Fatal(err)
	}
	if _, err := checkBundle(root); err == nil || !strings.Contains(err.Error(), "agents/shared/metadata.py") {
		t.Fatalf("missing helper accepted or unnamed: %v", err)
	}
}

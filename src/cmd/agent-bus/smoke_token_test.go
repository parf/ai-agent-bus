package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// The smoke credential cache is shell code, not the CLI's credential source.
// Pause eight first-use writers halfway through their output: no reader may
// observe that partial credential, and every caller must get the complete one.
func TestSmokeTokenCachePublishesCompleteCredentials(t *testing.T) {
	source, err := os.ReadFile("../../smoke.sh")
	if err != nil {
		t.Fatal(err)
	}
	helper := regexp.MustCompile(`(?m)^tok\(\) \{\n(?:.*\n)*?\}`).Find(source)
	if len(helper) == 0 {
		t.Fatal("smoke token helper not found")
	}
	dir := t.TempDir()
	fixture := `#!/bin/bash
touch "$D/started.$$"
for _ in $(seq 1 500); do
 [ "$(find "$D" -name 'started.*' | wc -l)" -eq 8 ] && break
 sleep 0.01
done
printf cred
touch "$D/partial.$$"
for _ in $(seq 1 500); do
 [ -e "$D/release" ] && { printf 'ential\n'; exit 0; }
 sleep 0.01
done
exit 1
`
	if err := os.WriteFile(filepath.Join(dir, "agent-bus-token"), []byte(fixture), 0700); err != nil {
		t.Fatal(err)
	}
	script := "export D=" + dir + "\nTOKEN=fixture\nOWNER=fixture\n" + string(helper) + "\n" + `for i in $(seq 1 8); do tok '#pool@srv1' >"$D/result.$i" & done
wait
`
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.WriteFile(filepath.Join(dir, "release"), nil, 0600); _ = cmd.Wait() }()
	until := time.Now().Add(5 * time.Second)
	for {
		writers, _ := filepath.Glob(filepath.Join(dir, "partial.*"))
		if len(writers) == 8 {
			break
		}
		if time.Now().After(until) {
			t.Fatalf("only %d concurrent token writers started", len(writers))
		}
		time.Sleep(time.Millisecond)
	}
	cache := filepath.Join(dir, "tok.#pool_srv1")
	if b, err := os.ReadFile(cache); err == nil && string(b) != "credential\n" {
		t.Fatalf("cache exposed a partial credential (%d bytes)", len(b))
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 8; i++ {
		b, err := os.ReadFile(filepath.Join(dir, "result."+string(rune('0'+i))))
		if err != nil || strings.TrimSpace(string(b)) != "credential" {
			t.Fatalf("caller %d did not receive a complete credential: %v", i, err)
		}
	}
	info, err := os.Stat(cache)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("cache permissions %o, want 600", info.Mode().Perm())
	}
}

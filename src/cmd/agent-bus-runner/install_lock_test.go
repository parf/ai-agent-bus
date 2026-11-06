package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var installPin = strings.Repeat("a", 40)

func stagingLeft(t *testing.T, r *runner) []string {
	t.Helper()
	entries, err := os.ReadDir(r.agents)
	if err != nil {
		t.Fatal(err)
	}
	var left []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".install-") {
			left = append(left, e.Name())
		}
	}
	return left
}

// A slow fetch is outside reloadMu: while it blocks, the runner's controls, stop included,
// answer at once, and a failed install leaves no staging behind.
func TestAnInstallFetchDoesNotHoldTheReloadLock(t *testing.T) {
	r, _, _ := configurationFixture(t, "", "printf old")
	entered, release := make(chan struct{}), make(chan struct{})
	r.fetch = func(ctx context.Context, in installRequest, dest string) error {
		close(entered)
		select {
		case <-release:
			return fmt.Errorf("fetch fixture failed")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	done := make(chan error, 1)
	go func() {
		done <- r.installPackage(context.Background(), "install newagent git https://fixture.invalid/repo "+installPin)
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the install never reached its fetch")
	}
	// The install is inside its fetch. The lock must be free, and a stop must be answered.
	stopped := make(chan error, 1)
	go func() { stopped <- r.control("stop", "fixture") }()
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("stop during an install fetch: %v", err)
		}
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("stop was not answered within 2 s while an install fetch was running: reloadMu is held across the fetch")
	}
	if !r.reloadMu.TryLock() {
		close(release)
		t.Fatal("reloadMu is held while the fetch runs")
	}
	r.reloadMu.Unlock()
	close(release)
	if err := <-done; err == nil || !strings.Contains(err.Error(), "fetch fixture failed") {
		t.Fatalf("the install's own failure was not returned: %v", err)
	}
	if left := stagingLeft(t, r); len(left) != 0 {
		t.Fatalf("a failed install left staging behind: %v", left)
	}
	if _, err := os.Lstat(filepath.Join(r.agents, "newagent")); !os.IsNotExist(err) {
		t.Fatalf("a failed install published a package: %v", err)
	}
}

// A plain mistake is refused before any fetch is made; and what changed while the fetch ran is
// decided again under the lock, with the staging removed.
func TestAnInstallChecksBeforeTheFetchAndAgainUnderTheLock(t *testing.T) {
	r, _, _ := configurationFixture(t, "", "printf old")
	fetches := 0
	r.fetch = func(ctx context.Context, in installRequest, dest string) error {
		fetches++
		// While the fetch runs, somebody else installs the same package.
		return os.MkdirAll(filepath.Join(r.agents, in.agent), 0700)
	}
	if err := r.installPackage(context.Background(), "install fixture git https://fixture.invalid/repo "+installPin); err == nil || !strings.Contains(err.Error(), "--replace") {
		t.Fatalf("an existing package without --replace: %v", err)
	}
	if fetches != 0 {
		t.Fatalf("the refusal came after %d fetches, want none", fetches)
	}
	err := r.installPackage(context.Background(), "install raced git https://fixture.invalid/repo "+installPin)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("a package that appeared during the fetch was not refused under the lock: %v", err)
	}
	if fetches != 1 {
		t.Fatalf("fetches = %d, want 1", fetches)
	}
	if left := stagingLeft(t, r); len(left) != 0 {
		t.Fatalf("staging left behind after the locked refusal: %v", left)
	}
}

package main

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// tree is every file under dir with its content: a checkout's whole state.
func tree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if d.IsDir() {
			out[rel+"/"] = ""
			return nil
		}
		b, err := os.ReadFile(path)
		out[rel] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func sameTree(t *testing.T, what string, before, after map[string]string) {
	t.Helper()
	if len(before) != len(after) {
		t.Fatalf("%s changed the checkout: %v -> %v", what, before, after)
	}
	for k, v := range before {
		if got, ok := after[k]; !ok || got != v {
			t.Fatalf("%s changed the checkout at %s", what, k)
		}
	}
}

// linkedFixture links agents/fixture to a checkout outside agents/.
func linkedFixture(t *testing.T) (*runner, string) {
	t.Helper()
	r, _, script := configurationFixture(t, "", "printf old")
	if err := os.RemoveAll(filepath.Dir(script)); err != nil {
		t.Fatal(err)
	}
	checkout := directoryAgent(t, t.TempDir(), "fixture", "linked")
	if err := os.Symlink(checkout, filepath.Join(r.agents, "fixture")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(r.host, "agents.json"), `{"fixture":{"sandbox":"off","enabled":true}}`)
	return r, checkout
}

// install --replace over a linked package swaps the link for a real directory and writes nothing
// into the checkout.
func TestInstallReplaceOverALinkedPackageReplacesTheLinkNotTheCheckout(t *testing.T) {
	r, checkout := linkedFixture(t)
	r.bundled = t.TempDir()
	directoryAgent(t, r.bundled, "fixture", "deployed")
	before := tree(t, checkout)
	if err := r.installPackage(context.Background(), "install fixture"); err == nil || !strings.Contains(err.Error(), "--replace") {
		t.Fatalf("a linked package is an existing package: %v", err)
	}
	if err := r.installPackage(context.Background(), "install --replace fixture"); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(filepath.Join(r.agents, "fixture"))
	if err != nil || fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
		t.Fatalf("agents/fixture after the replace: %v %v, want a real directory", fi, err)
	}
	sameTree(t, "install --replace", before, tree(t, checkout))
	if left := stagingLeft(t, r); len(left) != 0 {
		t.Fatalf("staging left: %v", left)
	}
	if got, _ := os.ReadFile(filepath.Join(r.agents, "fixture", "env.dist")); string(got) != "FLAVOUR=deployed\n" {
		t.Fatalf("the replacement is not the deployed package: %q", got)
	}
}

// remove of a linked package unlinks it; the checkout it pointed at stays whole.
func TestRemoveOfALinkedPackageUnlinksItAndKeepsTheCheckout(t *testing.T) {
	r, checkout := linkedFixture(t)
	before := tree(t, checkout)
	if err := r.removePackage("fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(r.agents, "fixture")); !os.IsNotExist(err) {
		t.Fatalf("the link is still there: %v", err)
	}
	sameTree(t, "remove", before, tree(t, checkout))
	// A dangling link is removable too.
	dangling := filepath.Join(r.agents, "gone")
	if err := os.Symlink(filepath.Join(t.TempDir(), "absent"), dangling); err != nil {
		t.Fatal(err)
	}
	if err := r.removePackage("gone"); err != nil {
		t.Fatalf("a dangling link: %v", err)
	}
	if _, err := os.Lstat(dangling); !os.IsNotExist(err) {
		t.Fatal("the dangling link is still there")
	}
}

// A link out of agents/ is read through for the package's own files, and written through never:
// the runner's file calls refuse the author's files, and nothing lands in the checkout.
func TestALinkedPackageIsNeverWrittenThrough(t *testing.T) {
	r, checkout := linkedFixture(t)
	r.reload()
	before := tree(t, checkout)
	for _, name := range []string{"agent.d/fixture/env.dist", "agent.d/fixture/agent.json"} {
		if err := r.writeHostFile(name, "FLAVOUR=changed\n"); err == nil || !strings.Contains(err.Error(), "read only") {
			t.Fatalf("write %s through a linked package: %v", name, err)
		}
	}
	sameTree(t, "a refused write", before, tree(t, checkout))
}

// A pull in the checkout is picked up by a reload: new code runs, nothing is copied.
func TestReloadPicksUpAPullInTheLinkedCheckout(t *testing.T) {
	r, b, _ := configurationFixture(t, "", "printf old")
	if err := os.RemoveAll(filepath.Join(r.agents, "fixture")); err != nil {
		t.Fatal(err)
	}
	checkout := directoryAgent(t, t.TempDir(), "fixture", "before")
	if err := os.Symlink(checkout, filepath.Join(r.agents, "fixture")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(r.host, "agents.json"), `{"fixture":{"sandbox":"off","enabled":true}}`)
	r.reload()
	ask := func() string {
		if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: "hello"}); err != nil {
			t.Fatal(err)
		}
		return configurationReply(t, b).Body
	}
	if got := ask(); got != "before" {
		t.Fatalf("before the pull: %q", got)
	}
	// What a `git pull` does: the files change in place.
	if err := os.WriteFile(filepath.Join(checkout, "env.dist"), []byte("FLAVOUR=pulled\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r.reload()
	if got := ask(); got != "pulled" {
		t.Fatalf("after the pull and a reload: %q", got)
	}
}

// A runner that starts finds an install's cut-off staging in agents/ and removes it, but never
// follows a link there, and keeps a staging directory that holds the only copy of a package.
func TestStartSweepsLeftoverInstallStagingSafely(t *testing.T) {
	r, _, _ := configurationFixture(t, "", "printf old")
	var said bytes.Buffer
	r.say = &said
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "precious"), "keep")
	stale := filepath.Join(r.agents, ".install-stale")
	writeFile(t, filepath.Join(stale, "agent", "agent.json"), "{}")
	kept := filepath.Join(r.agents, ".install-kept")
	writeFile(t, filepath.Join(kept, "previous", "agent.json"), "{}")
	if err := os.Symlink(outside, filepath.Join(r.agents, ".install-link")); err != nil {
		t.Fatal(err)
	}
	removeStale := filepath.Join(r.agents, ".remove-stale")
	writeFile(t, filepath.Join(removeStale, "scratch"), "x")
	removeKept := filepath.Join(r.agents, ".remove-kept")
	writeFile(t, filepath.Join(removeKept, "package", "agent.json"), "{}")
	hostStale := filepath.Join(r.host, ".remove-stale")
	writeFile(t, filepath.Join(hostStale, "scratch"), "x")
	hostKept := filepath.Join(r.host, ".remove-kept")
	writeFile(t, filepath.Join(hostKept, "settings", "env"), "A=1")
	if err := os.Symlink(outside, filepath.Join(r.agents, ".remove-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(r.host, ".remove-link")); err != nil {
		t.Fatal(err)
	}
	// Abandoned means old enough that nobody is about to take its lease.
	old := time.Now().Add(-time.Hour)
	for _, dir := range []string{stale, kept, removeStale, removeKept, hostStale, hostKept} {
		if err := os.Chtimes(dir, old, old); err != nil {
			t.Fatal(err)
		}
	}
	r.sweepStaging()
	for _, gone := range []string{removeStale, hostStale, filepath.Join(r.agents, ".remove-link"), filepath.Join(r.host, ".remove-link")} {
		if _, err := os.Lstat(gone); !os.IsNotExist(err) {
			t.Fatalf("%s survived the sweep: %v", gone, err)
		}
	}
	for _, kept := range []string{filepath.Join(removeKept, "package", "agent.json"), filepath.Join(hostKept, "settings", "env")} {
		if _, err := os.Lstat(kept); err != nil {
			t.Fatalf("the sweep removed a retained tree %s: %v", kept, err)
		}
	}
	for _, gone := range []string{stale, filepath.Join(r.agents, ".install-link")} {
		if _, err := os.Lstat(gone); !os.IsNotExist(err) {
			t.Fatalf("%s survived the sweep: %v", gone, err)
		}
	}
	if got, err := os.ReadFile(filepath.Join(outside, "precious")); err != nil || string(got) != "keep" {
		t.Fatalf("the sweep followed a link and touched its target: %q %v", got, err)
	}
	if _, err := os.Lstat(filepath.Join(kept, "previous", "agent.json")); err != nil {
		t.Fatalf("the sweep removed a retained previous tree: %v", err)
	}
	for _, name := range []string{".install-kept", ".remove-kept"} {
		if !strings.Contains(said.String(), name) {
			t.Fatalf("the kept tree %s was not named: %q", name, said.String())
		}
	}
	if _, err := os.Lstat(filepath.Join(r.agents, "fixture")); err != nil {
		t.Fatalf("the sweep touched a package: %v", err)
	}
}

// Two runners may share one agents/ directory (a root-owned shared agent.d): a runner that starts and
// sweeps leftovers must not remove the staging another one is working in, and a remove's too.
func TestStartSweepKeepsAnotherRunnersLiveStaging(t *testing.T) {
	a, _, _ := configurationFixture(t, "", "printf old")
	b := &runner{name: "#runner2@h", agents: a.agents, host: a.host, say: &bytes.Buffer{}}
	entered, release := make(chan struct{}), make(chan struct{})
	a.fetch = func(ctx context.Context, in installRequest, dest string) error {
		writeFile(t, filepath.Join(dest, "agent.json"), "{}")
		close(entered)
		<-release
		return os.ErrInvalid
	}
	done := make(chan error, 1)
	go func() {
		done <- a.installPackage(context.Background(), "install newagent git https://fixture.invalid/repo "+installPin)
	}()
	<-entered
	staged := stagingLeft(t, a)
	if len(staged) != 1 {
		close(release)
		t.Fatalf("staging while the fetch runs: %v", staged)
	}
	stagedFile := filepath.Join(a.agents, staged[0], "newagent", "agent.json")
	// Old enough that only the lease, not its age, protects it.
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(a.agents, staged[0]), old, old); err != nil {
		t.Fatal(err)
	}
	b.sweepStaging()
	_, statErr := os.Lstat(stagedFile)
	close(release)
	<-done
	if statErr != nil {
		t.Fatalf("another runner's sweep removed a live install's staging: %v", statErr)
	}
	// Once its install is over, nothing of it is left, and a sweep finds nothing to do.
	if left := stagingLeft(t, a); len(left) != 0 {
		t.Fatalf("staging left after the install: %v", left)
	}
	// An abandoned staging directory (no lease held, not new) is removed.
	dead := filepath.Join(a.agents, ".install-dead")
	writeFile(t, filepath.Join(dead, "newagent", "agent.json"), "{}")
	if err := os.Chtimes(dead, old, old); err != nil {
		t.Fatal(err)
	}
	b.sweepStaging()
	if _, err := os.Lstat(dead); !os.IsNotExist(err) {
		t.Fatalf("an abandoned staging directory survived: %v", err)
	}
}

// A directory is never visible under its final name before its lease is locked: claimDir builds it
// under a claim name, locks, then renames. So the sweep needs no age test: a final-named directory
// whose lease nobody holds is dead however new it is, and a creator stalled before its lock loses
// only an empty claim directory and gets an error, never a staged package.
func TestClaimDirIsLockedFromTheFirstMomentItHasItsName(t *testing.T) {
	r, _, _ := configurationFixture(t, "", "printf old")
	dir, lease, err := claimDir(r.agents, ".install-")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if !strings.HasPrefix(filepath.Base(dir), ".install-") {
		t.Fatalf("claimed directory %s", dir)
	}
	if !leased(dir) {
		t.Fatal("a freshly claimed directory is not protected by its lease")
	}
	if left, _ := filepath.Glob(filepath.Join(r.agents, claimPrefix+"*")); len(left) != 0 {
		t.Fatalf("claim directory left behind: %v", left)
	}
	writeFile(t, filepath.Join(dir, "newagent", "agent.json"), "{}")
	b := &runner{name: "#runner2@h", agents: r.agents, host: r.host, say: &bytes.Buffer{}}
	b.sweepStaging()
	if _, err := os.Lstat(filepath.Join(dir, "newagent", "agent.json")); err != nil {
		t.Fatalf("a sweep removed a claimed directory held by a live creator: %v", err)
	}
	lease.Close()
	b.sweepStaging()
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatalf("a directory whose lease is no longer held, however new, was kept: %v", err)
	}
}

// A creator stalled between making its claim directory and locking it: the sweep takes the
// claim, and the creator's rename fails loudly. Nothing was staged under a name that outlives it.
func TestAStalledClaimIsSweptAndTheCreatorFailsWithoutLoss(t *testing.T) {
	r, _, _ := configurationFixture(t, "", "printf old")
	claim, err := os.MkdirTemp(r.agents, claimPrefix)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(claim, leaseFile), os.O_CREATE|os.O_RDWR, 0600) // opened, not yet locked
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b := &runner{name: "#runner2@h", agents: r.agents, host: r.host, say: &bytes.Buffer{}}
	b.sweepStaging()
	if _, err := os.Lstat(claim); !os.IsNotExist(err) {
		t.Fatalf("an unlocked claim directory was kept: %v", err)
	}
	if err := os.Rename(claim, filepath.Join(r.agents, ".install-late")); err == nil {
		t.Fatal("the stalled creator's rename succeeded over a swept claim")
	}
}

// A final-named directory with an unlocked lease, or none, is abandoned at any age.
func TestStartSweepTakesAnUnlockedLeaseAtAnyAge(t *testing.T) {
	r, _, _ := configurationFixture(t, "", "printf old")
	b := &runner{name: "#runner2@h", agents: r.agents, host: r.host, say: &bytes.Buffer{}}
	fresh := filepath.Join(r.agents, ".install-fresh")
	writeFile(t, filepath.Join(fresh, ".lease"), "")
	writeFile(t, filepath.Join(fresh, "newagent", "agent.json"), "{}")
	none := filepath.Join(r.agents, ".install-none")
	writeFile(t, filepath.Join(none, "newagent", "agent.json"), "{}")
	b.sweepStaging()
	for _, d := range []string{fresh, none} {
		if _, err := os.Lstat(d); !os.IsNotExist(err) {
			t.Fatalf("%s: an unlocked directory survived the sweep: %v", d, err)
		}
	}
}

// A failed install closes its lease: nothing is left open however many fail.
func TestAFailedInstallClosesItsLease(t *testing.T) {
	r, _, _ := configurationFixture(t, "", "printf old")
	r.fetch = func(ctx context.Context, in installRequest, dest string) error { return os.ErrInvalid }
	open := func() int {
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			t.Skip("no /proc/self/fd")
		}
		return len(entries)
	}
	_ = r.installPackage(context.Background(), "install newagent git https://fixture.invalid/repo "+installPin) // warm up
	// A leaked *os.File is closed by its finalizer at the next collection: hold the collector off so
	// the count shows what the code itself closed.
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	before := open()
	for range 5 {
		if err := r.installPackage(context.Background(), "install newagent git https://fixture.invalid/repo "+installPin); err == nil {
			t.Fatal("the stub fetch should fail")
		}
	}
	if after := open(); after > before {
		t.Fatalf("%d descriptors left open by 5 failed installs", after-before)
	}
}

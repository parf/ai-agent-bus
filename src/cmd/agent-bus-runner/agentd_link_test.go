package main

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

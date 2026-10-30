package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/sandbox"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

// The agent reads its shipped file where it always did and finds the host's
// layers merged into it: the agent's override, then the instance's; nested
// objects merge, null removes, a list replaces. Run in the real sandbox, so
// the mount is what is read, and a sibling file the host left alone is the
// author's.
func TestSandboxedAgentReadsItsSettingsFileWithTheHostsOverridesMerged(t *testing.T) {
	if err := sandbox.ProbeBwrap(); err != nil {
		t.Skipf("the mount proof needs bubblewrap: %v", err)
	}
	root := t.TempDir()
	agents, host := filepath.Join(root, "agent.d"), filepath.Join(root, "runner")
	writeTree(t, agents, map[string]string{
		"hello/agent.json":    `{"command":"x","description":"d"}`,
		"hello/settings.json": `{"keep":1,"drop":2,"nest":{"a":1,"b":2},"list":[1,2]}`,
		"hello/other.json":    `{"author":true}`,
	})
	writeTree(t, host, map[string]string{
		"hello/settings.json":     `{"drop":null,"nest":{"b":3},"list":[9]}`,
		"hello/one/settings.json": `{"nest":{"c":4}}`,
	})
	over, sum, err := shippedOverlays(host, agents, "hello", "hello/one")
	if err != nil {
		t.Fatal(err)
	}
	if len(over) != 1 || sum == "" || over[0].Dst != filepath.Join(agents, "hello", "settings.json") {
		t.Fatalf("overlays %+v digest %q, want only settings.json", over, sum)
	}
	work := filepath.Join(root, "work")
	if err := os.Mkdir(work, 0700); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(agents, "hello")
	script := filepath.Join(pkg, "agent.sh")
	probe := "cat " + strconv.Quote(filepath.Join(pkg, "settings.json")) + " > " + strconv.Quote(filepath.Join(work, "seen")) +
		"\ncat " + strconv.Quote(filepath.Join(pkg, "other.json")) + " > " + strconv.Quote(filepath.Join(work, "other")) + "\n"
	writeTree(t, pkg, map[string]string{"agent.sh": probe})
	r := &runner{ctx: context.Background(), say: io.Discard}
	s := &served{bus: unconfiguredClient(), name: "#hello@h", cmd: "/bin/sh " + strconv.Quote(script), work: work, dir: pkg, box: sandbox.Bwrap{}, jail: &jail{},
		in: installed{Span: span{Min: 1, Max: 1}, Over: over}}
	k := &keeper{r: r, s: s, ctx: context.Background()}
	cmd, _, err := k.command()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Run(); err != nil {
		t.Fatalf("the sandboxed probe failed: %v", err)
	}
	seen, err := os.ReadFile(filepath.Join(work, "seen"))
	if err != nil {
		t.Fatal(err)
	}
	var got, want any
	if err := json.Unmarshal(seen, &got); err != nil {
		t.Fatalf("the agent read %q: %v", seen, err)
	}
	json.Unmarshal([]byte(`{"keep":1,"nest":{"a":1,"b":3,"c":4},"list":[9]}`), &want)
	if g, w := mustJSON(got), mustJSON(want); string(g) != string(w) {
		t.Fatalf("the agent read %s, want %s", g, w)
	}
	other, err := os.ReadFile(filepath.Join(work, "other"))
	if err != nil || string(other) != `{"author":true}` {
		t.Fatalf("a file the host did not override read %q (%v), want the author's", other, err)
	}
	if b, _ := os.ReadFile(filepath.Join(pkg, "settings.json")); !strings.Contains(string(b), `"drop":2`) {
		t.Fatalf("the shipped file on the host was changed: %s", b)
	}
}

// A changed override changes what the runner compares at reload, so a kept
// agent that mounted the old file starts again; a file the agent does not ship
// has nothing to merge into and is refused by name.
func TestOverrideChangeIsNoticedAndAnUnshippedFileIsRefused(t *testing.T) {
	root := t.TempDir()
	agents, host := filepath.Join(root, "agent.d"), filepath.Join(root, "runner")
	writeTree(t, agents, map[string]string{"hello/agent.json": `{"command":"/bin/true","description":"d"}`, "hello/settings.json": `{"a":1}`})
	digest := func() installed {
		t.Helper()
		list, err := load(agents, host)
		if err != nil || len(list) != 1 {
			t.Fatalf("load: %v %+v", err, list)
		}
		return list[0]
	}
	writeTree(t, host, map[string]string{"hello/settings.json": `{"a":2}`})
	first := digest()
	if first.Refused != "" || len(first.Over) != 1 {
		t.Fatalf("loaded %+v, want one overlay and no refusal", first)
	}
	writeTree(t, host, map[string]string{"hello/settings.json": `{"a":3}`})
	if second := digest(); first.Digest == second.Digest {
		t.Fatal("a changed override left the digest as it was, so a kept agent would not restart")
	}
	writeTree(t, host, map[string]string{"hello/missing.json": `{}`})
	if refused := digest().Refused; !strings.Contains(refused, "missing.json") {
		t.Fatalf("an override of a file the agent does not ship: refused %q, want it named", refused)
	}
}

// Off has no namespace to mount into: serving such an agent on the author's
// file would be the override silently not applied, so it is refused.
func TestOverrideOfAShippedFileIsRefusedWithoutTheSandbox(t *testing.T) {
	r, _, _ := configurationFixture(t, "itself", "while :; do sleep .02; done\n")
	writeTree(t, r.agents, map[string]string{"fixture/settings.json": `{"a":1}`})
	writeTree(t, r.host, map[string]string{"fixture/settings.json": `{"a":2}`})
	r.reload()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.installed) != 1 || !strings.Contains(r.installed[0].Refused, "bubblewrap") {
		t.Fatalf("installed %+v, want the agent refused naming the sandbox it needs", r.installed)
	}
}

// The host's files call takes the override of a settings file as it takes
// agent.json's: checked, written, applied. Reload merges it into the shipped
// file the agent is mounted (the mount itself is proved above, in the real
// sandbox); a file it does not ship or a body that is not an object is refused
// by name and writes nothing. Sandbox off keeps the fixture runnable here: the
// agent is then refused for lack of a mount, which says the override was seen.
func TestWritingAnOverrideOfASettingsFileOverTheBusReachesTheMountedFile(t *testing.T) {
	r, _, _ := configurationFixture(t, "itself", "while :; do sleep .02; done\n")
	writeTree(t, r.agents, map[string]string{"fixture/settings.json": `{"a":1,"b":1}`})
	for _, bad := range []struct{ file, body, want string }{
		{"runner/fixture/nothere.json", `{}`, "nothere.json"},
		{"runner/fixture/settings.json", `[1]`, "runner/fixture/settings.json"},
		{"runner/fixture/settings.json", `null`, "runner/fixture/settings.json"},
		{"runner/fixture/.merged.json", `{}`, "not a file the runner"},
	} {
		if err := r.writeHostFile(bad.file, bad.body); err == nil || !strings.Contains(err.Error(), bad.want) {
			t.Fatalf("write %s %s: %v, want a refusal naming %q", bad.file, bad.body, err, bad.want)
		}
	}
	if _, err := os.Stat(filepath.Join(r.host, "fixture", "settings.json")); err == nil {
		t.Fatal("a refused write left the file")
	}
	if err := r.writeHostFile("runner/fixture/settings.json", `{"b":null,"c":3}`); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.installed) != 1 || len(r.installed[0].Over) != 1 {
		t.Fatalf("installed %+v, want one overlay", r.installed)
	}
	raw, err := os.ReadFile(r.installed[0].Over[0].Src)
	if err != nil {
		t.Fatal(err)
	}
	var got any
	json.Unmarshal(raw, &got)
	if g := string(mustJSON(got)); g != `{"a":1,"c":3}` {
		t.Fatalf("the file the agent is given holds %s, want {\"a\":1,\"c\":3}", g)
	}
}

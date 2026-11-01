package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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
	over, sum, files, err := shippedOverlays(host, agents, "hello", "hello/one")
	if err != nil {
		t.Fatal(err)
	}
	if len(over) != 1 || sum == "" || over[0].Dst != filepath.Join(agents, "hello", "settings.json") {
		t.Fatalf("overlays %+v digest %q, want only settings.json", over, sum)
	}
	if err := publishOverlays(host, "hello/one", files); err != nil {
		t.Fatal(err)
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

func TestStagedInstallCannotChangeLiveMergedOverrides(t *testing.T) {
	r, _, _ := configurationFixture(t, "", "printf old")
	writeTree(t, r.agents, map[string]string{"fixture/settings.json": `{"value":"live"}`})
	writeTree(t, r.host, map[string]string{"fixture/settings.json": `{"host":true}`})
	r.reload()
	if len(r.installed[0].Over) != 1 {
		t.Fatal("live overlay was not published")
	}
	src := r.installed[0].Over[0].Src
	before, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	beforeHash := sha256.Sum256(before)
	// A valid staging load computes a different digest, but writes nothing.
	staged := t.TempDir()
	directoryAgent(t, staged, "fixture", "new")
	writeTree(t, staged, map[string]string{"fixture/settings.json": `{"value":"staged"}`})
	list, err := loadSources([]agentSource{{path: staged}}, r.host)
	if err != nil || len(list) != 1 || list[0].Refused != "" {
		t.Fatalf("staging load: %+v %v", list, err)
	}
	after, err := os.ReadFile(src)
	if err != nil || sha256.Sum256(after) != beforeHash {
		t.Fatalf("valid staging changed live merge: %s %v", after, err)
	}
	if list[0].Digest == r.installed[0].Digest {
		t.Fatal("staging did not compute its own digest")
	}
	// The real install staging path rejects a package dropping the shipped file.
	r.bundled = t.TempDir()
	directoryAgent(t, r.bundled, "fixture", "new")
	if err := r.installPackage(context.Background(), "install --replace fixture"); err == nil || !strings.Contains(err.Error(), "does not ship") {
		t.Fatalf("package dropping an overridden file was not refused: %v", err)
	}
	after, err = os.ReadFile(src)
	if err != nil || sha256.Sum256(after) != beforeHash {
		t.Fatalf("refused install destroyed live merged file or changed digest: %s %v", after, err)
	}
}

func TestOverlayValidationWritesNothingAndApplyReplacesCompleteMerge(t *testing.T) {
	r, _, _ := configurationFixture(t, "", "printf old")
	writeTree(t, r.agents, map[string]string{"fixture/settings.json": `{"value":1}`, "fixture/old.json": `{}`})
	writeTree(t, r.host, map[string]string{"fixture/settings.json": `{"host":true}`, "fixture/old.json": `{}`})
	list, err := load(r.agents, r.host)
	if err != nil || len(list) != 1 || len(list[0].Over) != 2 {
		t.Fatalf("load: %+v %v", list, err)
	}
	if _, err := os.Lstat(filepath.Join(r.host, ".merged")); !os.IsNotExist(err) {
		t.Fatalf("validation wrote .merged: %v", err)
	}
	r.reload()
	src := r.installed[0].Over[1].Src
	if filepath.Base(src) != "settings.json" {
		t.Fatalf("unexpected overlay order: %s", src)
	}
	writeTree(t, r.agents, map[string]string{"fixture/settings.json": `{"value":2}`})
	if err := os.Remove(filepath.Join(r.host, "fixture", "old.json")); err != nil {
		t.Fatal(err)
	}
	r.reload()
	data, err := os.ReadFile(src)
	if err != nil || !strings.Contains(string(data), `"value": 2`) {
		t.Fatalf("replacement not published: %s %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(src), "old.json")); !os.IsNotExist(err) {
		t.Fatalf("replacement retained obsolete merged file: %v", err)
	}
	if err := os.Remove(filepath.Join(r.host, "fixture", "settings.json")); err != nil {
		t.Fatal(err)
	}
	r.reload()
	if len(r.installed[0].Over) != 0 {
		t.Fatal("removed overrides are still mounted")
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("removing overrides deleted the prior source before retirement: %v", err)
	}
}

func TestOverlayApplyDiscardsInterruptedOutputAndRecoversPreviousMerge(t *testing.T) {
	r, _, _ := configurationFixture(t, "", "printf old")
	writeTree(t, r.agents, map[string]string{"fixture/settings.json": `{"value":1}`})
	writeTree(t, r.host, map[string]string{"fixture/settings.json": `{}`})
	r.reload()
	src := r.installed[0].Over[0].Src
	target := filepath.Dir(src)
	before, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Dir(target)
	stage := filepath.Join(parent, ".files.merge-interrupted")
	writeTree(t, stage, map[string]string{"settings.json": `{"partial":true}`})
	if err := os.Rename(target, filepath.Join(parent, ".files.previous")); err != nil {
		t.Fatal(err)
	}
	// Even a newly refused key must recover its old files and discard stale output.
	writeTree(t, r.host, map[string]string{"fixture/missing.json": `{}`})
	r.reload()
	if !strings.Contains(r.installed[0].Refused, "does not ship") {
		t.Fatalf("expected refused merge: %+v", r.installed[0])
	}
	got, err := os.ReadFile(src)
	if err != nil || string(got) != string(before) {
		t.Fatalf("interrupted publication lost previous merge: %s %v", got, err)
	}
	if _, err := os.Lstat(stage); !os.IsNotExist(err) {
		t.Fatalf("stale merge output retained: %v", err)
	}
}

func TestFailedOverlayBuildLeavesPreviousMergeUntouched(t *testing.T) {
	host := t.TempDir()
	if err := publishOverlays(host, "hello", []overlayFile{{name: "settings.json", body: []byte("old")}}); err != nil {
		t.Fatal(err)
	}
	if err := publishOverlays(host, "hello", []overlayFile{{name: "settings.json", body: []byte("new")}, {name: "absent/file.json", body: []byte("fail")}}); err == nil {
		t.Fatal("failed merge unexpectedly published")
	}
	data, err := os.ReadFile(filepath.Join(host, ".merged", "hello", ".files", "settings.json"))
	if err != nil || string(data) != "old" {
		t.Fatalf("failed merge changed previous files: %s %v", data, err)
	}
	entries, err := os.ReadDir(filepath.Join(host, ".merged", "hello"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != ".files" {
		t.Fatalf("failed build left temporary output: %v", entries)
	}
}

func TestAgentAndInstanceOverlayPublicationStayIndependent(t *testing.T) {
	host := t.TempDir()
	for _, key := range []string{"hello/files", "hello"} {
		if err := publishOverlays(host, key, []overlayFile{{name: "settings.json", body: []byte(key)}}); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range []string{"hello", "hello/files"} {
		data, err := os.ReadFile(filepath.Join(host, ".merged", filepath.FromSlash(key), ".files", "settings.json"))
		if err != nil || string(data) != key {
			t.Fatalf("publishing another key changed %s: %s %v", key, data, err)
		}
	}
}

func TestFailedOverlayPublicationRestoresPreviousMerge(t *testing.T) {
	host := t.TempDir()
	if err := publishOverlays(host, "hello", []overlayFile{{name: "settings.json", body: []byte("old")}}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(host, ".merged", "hello", ".files")
	err := publishOverlayTree(target, []overlayFile{{name: "settings.json", body: []byte("new")}}, func(string, string) error { return syscall.EINVAL }, func(from, to string) error {
		if strings.HasPrefix(filepath.Base(from), ".files.merge-") {
			return errors.New("injected publication refusal")
		}
		return os.Rename(from, to)
	})
	if err == nil || !strings.Contains(err.Error(), "injected publication refusal") {
		t.Fatalf("expected publication refusal: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(target, "settings.json"))
	if err != nil || string(data) != "old" {
		t.Fatalf("failed publication did not restore previous merge: %s %v", data, err)
	}
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != ".files" {
		t.Fatalf("failed publication retained temporary/backup output: %v", entries)
	}
}

func TestOverlayRecoveryDoesNotSweepPublishedFileNames(t *testing.T) {
	host := t.TempDir()
	name := ".files.merge-author.json"
	if err := publishOverlays(host, "hello", []overlayFile{{name: name, body: []byte("kept")}}); err != nil {
		t.Fatal(err)
	}
	if errs := recoverOverlays(host); len(errs) != 0 {
		t.Fatal(errs)
	}
	data, err := os.ReadFile(filepath.Join(host, ".merged", "hello", ".files", name))
	if err != nil || string(data) != "kept" {
		t.Fatalf("recovery swept published settings: %s %v", data, err)
	}
}

func TestOverlayRecoveryRefusesOnlyTheUnreadableKeyAndContinues(t *testing.T) {
	r, _, _ := configurationFixture(t, "", "printf old")
	directoryAgent(t, r.agents, "healthy", "healthy")
	writeTree(t, r.agents, map[string]string{"fixture/settings.json": `{"value":1}`, "healthy/settings.json": `{"value":1}`})
	writeTree(t, r.host, map[string]string{"agents.json": `{"fixture":{"enabled":false},"healthy":{"enabled":false}}`, "fixture/settings.json": `{}`, "healthy/settings.json": `{}`})
	r.reload()
	bad := filepath.Join(r.host, ".merged", "fixture")
	if err := os.Chmod(bad, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(bad, 0700) })
	stage := filepath.Join(r.host, ".merged", "healthy", ".files.merge-interrupted")
	writeTree(t, stage, map[string]string{"settings.json": "partial"})
	writeTree(t, r.agents, map[string]string{"healthy/settings.json": `{"value":2}`})
	r.reload()
	if r.loadErr != "" || len(r.installed) != 2 {
		t.Fatalf("one recovery error stopped healthy agents: loadErr=%q installed=%+v", r.loadErr, r.installed)
	}
	for _, in := range r.installed {
		if in.Key == "fixture" {
			if !strings.Contains(in.Refused, "recover merged overrides:") {
				t.Fatalf("unreadable key not refused by name: %+v", in)
			}
		} else if in.Key == "healthy" {
			if in.Refused != "" {
				t.Fatalf("healthy agent refused after another key's recovery error: %+v", in)
			}
			data, err := os.ReadFile(in.Over[0].Src)
			if err != nil || !strings.Contains(string(data), `"value": 2`) {
				t.Fatalf("healthy merge not published: %s %v", data, err)
			}
		}
	}
	if _, err := os.Stat(stage); !os.IsNotExist(err) {
		t.Fatalf("recovery stopped before the healthy key's temporary output: %v", err)
	}
}

func TestOverlayPublicationFallsBackOnlyWhenExchangeIsUnsupported(t *testing.T) {
	for _, tc := range []struct {
		name        string
		exchangeErr error
		wantRenames int
	}{
		{"EINVAL", syscall.EINVAL, 2}, {"EIO", syscall.EIO, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host := t.TempDir()
			if err := publishOverlays(host, "hello", []overlayFile{{name: "settings.json", body: []byte("old")}}); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(host, ".merged", "hello", ".files")
			exchanges, renames := 0, 0
			err := publishOverlayTree(target, []overlayFile{{name: "settings.json", body: []byte("new")}}, func(string, string) error { exchanges++; return tc.exchangeErr }, func(a, b string) error { renames++; return os.Rename(a, b) })
			want := "old"
			if tc.exchangeErr == syscall.EINVAL {
				want = "new"
				if err != nil {
					t.Fatalf("EINVAL exchange did not fall back: %v", err)
				}
			} else if !errors.Is(err, syscall.EIO) {
				t.Fatalf("exchange failure was not refused: %v", err)
			}
			if exchanges != 1 || renames != tc.wantRenames {
				t.Fatalf("exchange/fallback calls=%d/%d want1/%d", exchanges, renames, tc.wantRenames)
			}
			data, readErr := os.ReadFile(filepath.Join(target, "settings.json"))
			if readErr != nil || string(data) != want {
				t.Fatalf("exchange/fallback contents: %s %v want%s", data, readErr, want)
			}
			if tc.exchangeErr == syscall.EINVAL {
				old, err := os.ReadFile(filepath.Join(filepath.Dir(target), ".files.previous", "settings.json"))
				if err != nil || string(old) != "old" {
					t.Fatalf("fallback deleted old tree before retirement: %s %v", old, err)
				}
			}
			if errs := recoverOverlays(host); len(errs) != 0 {
				t.Fatal(errs)
			}
			entries, err := os.ReadDir(filepath.Dir(target))
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries[0].Name() != ".files" {
				t.Fatalf("exchange/fallback left temporary output: %v", entries)
			}
		})
	}
}

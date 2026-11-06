package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/agentmeta"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/sandbox"
)

func dirAgent() agentFile {
	return agentFile{Needs: &agentmeta.Needs{Dir: true}}
}

func realTemp(t *testing.T) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func mustRefuse(t *testing.T, raw string, state []string, home, want string) {
	t.Helper()
	got, err := resolveBindDir(raw, state, home)
	if err == nil {
		t.Fatalf("resolveBindDir(%q) = %q, want a refusal containing %q", raw, got, want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("resolveBindDir(%q) refusal = %q, want it to contain %q", raw, err, want)
	}
}

func TestResolveBindDirAcceptsADirectoryAndBindsItsRealPath(t *testing.T) {
	base := realTemp(t)
	vault := filepath.Join(base, "vault")
	if err := os.Mkdir(vault, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(vault, link); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{vault, vault + "/", filepath.Join(base, "x", "..", "vault"), link} {
		if got, err := resolveBindDir(raw, nil, ""); err != nil || got != vault {
			t.Fatalf("resolveBindDir(%q) = %q, %v; want %q", raw, got, err, vault)
		}
	}
	// A system path outside any home is allowed (Q308).
	if _, err := resolveBindDir("/usr/share", nil, ""); err != nil {
		t.Fatalf("a system path is refused: %v", err)
	}
}

func TestResolveBindDirRefusesWhatIsNotOneExistingAbsoluteDirectory(t *testing.T) {
	base := realTemp(t)
	file := filepath.Join(base, "f")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	mustRefuse(t, "", nil, "", "not an absolute path")
	mustRefuse(t, "vault", nil, "", "not an absolute path")
	mustRefuse(t, "./vault", nil, "", "not an absolute path")
	mustRefuse(t, base+"/a\x00b", nil, "", "not an absolute path")
	mustRefuse(t, filepath.Join(base, "missing"), nil, "", "cannot be resolved")
	mustRefuse(t, file, nil, "", "not a directory")
}

func TestResolveBindDirChecksTheResolvedPathSoALinkCannotLaunderAForbiddenOne(t *testing.T) {
	base := realTemp(t)
	home := filepath.Join(base, "home", "me")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	toRoot, toHome, toHomes := filepath.Join(base, "to-root"), filepath.Join(base, "to-home"), filepath.Join(base, "to-homes")
	for link, target := range map[string]string{toRoot: "/", toHome: home, toHomes: filepath.Dir(home)} {
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}
	mustRefuse(t, "/", nil, home, "the root directory")
	mustRefuse(t, toRoot, nil, home, "the root directory")
	mustRefuse(t, home, nil, home, "home root")
	mustRefuse(t, toHome, nil, home, "home root")
	mustRefuse(t, filepath.Dir(home), nil, home, "home root")
	mustRefuse(t, toHomes, nil, home, "home root")
	mustRefuse(t, "/home", nil, "", "home root")
	// A folder inside a home is fine: only the root of homes is refused.
	inside := filepath.Join(home, "notes")
	if err := os.Mkdir(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := resolveBindDir(inside, nil, home); err != nil || got != inside {
		t.Fatalf("a folder inside a home = %q, %v", got, err)
	}
}

func TestResolveBindDirRefusesTheRunnersOwnStateInEveryDirection(t *testing.T) {
	base := realTemp(t)
	state := filepath.Join(base, "runner")
	below := filepath.Join(state, "work", "filescan")
	outside := filepath.Join(base, "elsewhere")
	for _, d := range []string{below, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	viaLink := filepath.Join(base, "state-link")
	if err := os.Symlink(state, viaLink); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{state, below, base, viaLink} {
		mustRefuse(t, raw, []string{state}, "", "runner's own state")
	}
	if got, err := resolveBindDir(outside, []string{state, ""}, ""); err != nil || got != outside {
		t.Fatalf("a sibling of the runner state = %q, %v", got, err)
	}
}

func TestBindDirSetsOnlyTheCheckedPathAndOnlyForAnAgentThatAsked(t *testing.T) {
	base := realTemp(t)
	vault := filepath.Join(base, "vault")
	if err := os.Mkdir(vault, 0o755); err != nil {
		t.Fatal(err)
	}
	r := &runner{host: filepath.Join(base, "runner"), agents: filepath.Join(base, "agents")}
	forged := "AGENT_BUS_DIR=/etc"

	// An agent without needs.dir never carries the variable, forged or not.
	env, why := r.bindDir(agentFile{}, []string{"A=1", forged, `AGENT_BUS_CONFIG={"dir":"` + vault + `"}`})
	if why != "" || slices.ContainsFunc(env, func(kv string) bool { return strings.HasPrefix(kv, "AGENT_BUS_DIR=") }) {
		t.Fatalf("agent without needs.dir: env %v, why %q", env, why)
	}
	if got := dirReads(agentFile{}, []string{forged}); got != nil {
		t.Fatalf("dirReads for an agent that did not ask = %v", got)
	}

	// One that asked gets the resolved path, and the forged value is replaced.
	env, why = r.bindDir(dirAgent(), []string{"A=1", forged, `AGENT_BUS_CONFIG={"dir":"` + vault + `/"}`})
	if why != "" {
		t.Fatal(why)
	}
	if n := countPrefix(env, "AGENT_BUS_DIR="); n != 1 || !slices.Contains(env, "AGENT_BUS_DIR="+vault) {
		t.Fatalf("env %v", env)
	}
	if got := dirReads(dirAgent(), env); !slices.Equal(got, []string{vault}) {
		t.Fatalf("dirReads = %v, want [%s]", got, vault)
	}

	for name, env := range map[string][]string{
		"no configuration":  {"A=1"},
		"no dir key":        {`AGENT_BUS_CONFIG={"mode":"both"}`},
		"blank dir":         {`AGENT_BUS_CONFIG={"dir":"  "}`},
		"dir not a string":  {`AGENT_BUS_CONFIG={"dir":7}`},
		"configuration bad": {`AGENT_BUS_CONFIG=[1`},
		"forbidden dir":     {`AGENT_BUS_CONFIG={"dir":"/"}`},
		"runner state":      {`AGENT_BUS_CONFIG={"dir":"` + base + `"}`},
	} {
		if env, why := r.bindDir(dirAgent(), env); why == "" || env != nil || !strings.HasPrefix(why, "needs.dir: ") {
			t.Errorf("%s: env %v, why %q; want a needs.dir refusal", name, env, why)
		}
	}
}

func countPrefix(env []string, prefix string) int {
	n := 0
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			n++
		}
	}
	return n
}

func TestTheCheckedDirectoryIsBoundReadOnlyAndNothingElseIsWritable(t *testing.T) {
	argv := sandbox.Bwrap{}.Wrap(ports.Job{Argv: []string{"/bin/true"}, Work: "/w", Read: dirReads(dirAgent(), []string{"AGENT_BUS_DIR=/data/vault"})})
	joined := strings.Join(argv, " ")
	if !strings.Contains(joined, "--ro-bind /data/vault /data/vault") {
		t.Fatalf("no read-only bind: %s", joined)
	}
	if strings.Contains(joined, "--bind /data/vault") {
		t.Fatalf("the directory is writable: %s", joined)
	}
}

func TestAnAgentThatAsksForADirGetsItBoundFromItsConfigurationAndIsRefusedOtherwise(t *testing.T) {
	r, b, _ := configurationFixture(t, "itself", "true")
	s := r.served["#fixture@h"]
	s.in.Agent.Needs = &agentmeta.Needs{Dir: true}
	vault := realTemp(t)

	if _, why := r.environment(s.in, s.bus); !strings.HasPrefix(why, "its configuration: needs.dir: ") {
		t.Fatalf("unconfigured: why %q", why)
	}
	if _, err := b.Configure("#fixture@h", "owner@h", []byte(`{"dir":"`+vault+`"}`)); err != nil {
		t.Fatal(err)
	}
	env, why := r.environment(s.in, s.bus)
	if why != "" || !slices.Contains(env, "AGENT_BUS_DIR="+vault) {
		t.Fatalf("configured: env %v, why %q", env, why)
	}
	s.env = env
	if read := r.launchSettings(s).read; !slices.Contains(read, vault) || !slices.Contains(read, s.dir) {
		t.Fatalf("launch reads %v, want the package and %s", read, vault)
	}
	for _, bad := range []string{"/", "relative", filepath.Join(vault, "missing"), filepath.Dir(r.host)} {
		if _, err := b.Configure("#fixture@h", "owner@h", []byte(`{"dir":"`+bad+`"}`)); err != nil {
			t.Fatal(err)
		}
		if env, why := r.environment(s.in, s.bus); env != nil || !strings.Contains(why, "needs.dir") {
			t.Fatalf("dir %q: env %v, why %q; want a refusal", bad, env, why)
		}
	}
}

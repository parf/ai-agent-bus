package main

import (
	"github.com/parf/ai-agent-bus/internal/protocol"
	"net"
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

// everywhere lets a test reach the check it names without a root list in the way.
var everywhere = []string{"/"}

func mustRefuse(t *testing.T, raw string, state []string, home, want string) {
	t.Helper()
	got, err := resolveBindDir(raw, everywhere, state, home, "")
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
		if got, err := resolveBindDir(raw, everywhere, nil, "", ""); err != nil || got != vault {
			t.Fatalf("resolveBindDir(%q) = %q, %v; want %q", raw, got, err, vault)
		}
	}
	// A system path outside any home is allowed (Q308).
	if _, err := resolveBindDir("/usr/share", everywhere, nil, "", ""); err != nil {
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
	if got, err := resolveBindDir(inside, everywhere, nil, home, ""); err != nil || got != inside {
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
	if got, err := resolveBindDir(outside, everywhere, []string{state, ""}, "", ""); err != nil || got != outside {
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
	env, reads, why := r.bindDir(agentFile{}, []string{base}, []string{"A=1", forged, `AGENT_BUS_CONFIG={"dir":"` + vault + `"}`})
	if why != "" || reads != nil || slices.ContainsFunc(env, func(kv string) bool { return strings.HasPrefix(kv, "AGENT_BUS_DIR=") }) {
		t.Fatalf("agent without needs.dir: env %v, reads %v, why %q", env, reads, why)
	}

	// One that asked gets the resolved path, and the forged value is replaced.
	env, reads, why = r.bindDir(dirAgent(), []string{base}, []string{"A=1", forged, `AGENT_BUS_CONFIG={"dir":"` + vault + `/"}`})
	if why != "" {
		t.Fatal(why)
	}
	if n := countPrefix(env, "AGENT_BUS_DIR="); n != 1 || !slices.Contains(env, "AGENT_BUS_DIR="+vault) {
		t.Fatalf("env %v", env)
	}
	if !slices.Equal(reads, []string{vault}) {
		t.Fatalf("reads = %v, want [%s]", reads, vault)
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
		if env, reads, why := r.bindDir(dirAgent(), []string{base}, env); why == "" || env != nil || reads != nil || !strings.HasPrefix(why, "needs.dir: ") {
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
	argv := sandbox.Bwrap{}.Wrap(ports.Job{Argv: []string{"/bin/true"}, Work: "/w", Read: []string{"/data/vault"}})
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
	s.in.Host.Roots = []string{filepath.Dir(vault)}

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
	launch := r.launchSettings(s)
	if !slices.Equal(launch.roots, s.in.Host.Roots) {
		t.Fatalf("launch roots %v", launch.roots)
	}
	// The launch asks again on the Configuration it just read.
	_, reads, why := r.bindDir(launch.agent, launch.roots, env)
	if why != "" || !slices.Equal(reads, []string{vault}) {
		t.Fatalf("launch bind %v, %q", reads, why)
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

func TestOnlyTheNodeOwnersRootsAreWatchable(t *testing.T) {
	base := realTemp(t)
	root := filepath.Join(base, "srv")
	vault := filepath.Join(root, "vault")
	other := filepath.Join(base, "other")
	for _, d := range []string{vault, other} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A link inside the root that leads out of it.
	escape := filepath.Join(root, "escape")
	if err := os.Symlink(other, escape); err != nil {
		t.Fatal(err)
	}
	if got, err := resolveBindDir(vault, []string{root}, nil, "", ""); err != nil || got != vault {
		t.Fatalf("inside the root: %q, %v", got, err)
	}
	if got, err := resolveBindDir(root, []string{root}, nil, "", ""); err != nil || got != root {
		t.Fatalf("the root itself: %q, %v", got, err)
	}
	// The empty list is the default: nothing is watchable until the owner opts a root in.
	for _, roots := range [][]string{nil, {}} {
		if got, err := resolveBindDir(vault, roots, nil, "", ""); err == nil || !strings.Contains(err.Error(), "allows no watch root") || !strings.Contains(err.Error(), "roots") {
			t.Fatalf("empty list: %q, %v", got, err)
		}
	}
	if _, err := resolveBindDir(other, []string{root}, nil, "", ""); err == nil || !strings.Contains(err.Error(), "outside this node's watch roots") {
		t.Fatalf("outside every root: %v", err)
	}
	if _, err := resolveBindDir(escape, []string{root}, nil, "", ""); err == nil || !strings.Contains(err.Error(), "outside this node's watch roots") {
		t.Fatalf("a link out of the root: %v", err)
	}
	if _, err := resolveBindDir(vault, []string{"srv"}, nil, "", ""); err == nil || !strings.Contains(err.Error(), "not an absolute path") {
		t.Fatalf("a relative root: %v", err)
	}
	// A root named through a link still bounds the real path.
	rootLink := filepath.Join(base, "root-link")
	if err := os.Symlink(root, rootLink); err != nil {
		t.Fatal(err)
	}
	if got, err := resolveBindDir(vault, []string{rootLink}, nil, "", ""); err != nil || got != vault {
		t.Fatalf("a root given by link: %q, %v", got, err)
	}
	// Any one of several roots will do.
	if got, err := resolveBindDir(other, []string{root, other}, nil, "", ""); err != nil || got != other {
		t.Fatalf("second root: %q, %v", got, err)
	}
}

func TestHostExposingDirectoriesAreRefusedInBothDirections(t *testing.T) {
	runtime := realTemp(t)
	for _, p := range []string{"/proc", "/proc/self", "/sys", "/sys/kernel", "/dev", "/dev/shm", "/run", "/var/run"} {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		mustRefuse(t, p, nil, "", "exposes the host")
	}
	// A directory that holds the runtime directory is refused as well.
	parent := filepath.Dir(runtime)
	if _, err := resolveBindDir(parent, everywhere, nil, "", runtime); err == nil || !strings.Contains(err.Error(), "exposes the host") {
		t.Fatalf("a parent of the runtime directory: %v", err)
	}
	if _, err := resolveBindDir(runtime, everywhere, nil, "", runtime); err == nil || !strings.Contains(err.Error(), "exposes the host") {
		t.Fatalf("the runtime directory: %v", err)
	}
	// Reached through a link, it is still refused.
	link := filepath.Join(realTemp(t), "to-proc")
	if err := os.Symlink("/proc", link); err != nil {
		t.Fatal(err)
	}
	mustRefuse(t, link, nil, "", "exposes the host")
}

func TestATreeHoldingASocketIsRefused(t *testing.T) {
	base := realTemp(t)
	deep := filepath.Join(base, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := resolveBindDir(base, []string{base}, nil, "", ""); err != nil || got != base {
		t.Fatalf("a tree without a socket: %q, %v", got, err)
	}
	l, err := net.Listen("unix", filepath.Join(deep, "s"))
	if err != nil {
		t.Skipf("no unix sockets here: %v", err)
	}
	defer l.Close()
	if _, err := resolveBindDir(base, []string{base}, nil, "", ""); err == nil || !strings.Contains(err.Error(), "unix socket") || !strings.Contains(err.Error(), filepath.Join(deep, "s")) {
		t.Fatalf("a socket two levels down: %v", err)
	}
}

func TestVarHomeLayoutRefusesVarThroughTheStateRule(t *testing.T) {
	base := realTemp(t)
	state := filepath.Join(base, "var", "lib", "agent-bus", "runner")
	home := filepath.Join(base, "var", "home", "me")
	for _, d := range []string{state, home} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// /var holds the runner state on a /var/home system: refused as a container of it, not as a home root.
	mustRefuse(t, filepath.Join(base, "var"), []string{state}, home, "runner's own state")
	mustRefuse(t, filepath.Join(base, "var", "home"), nil, home, "home root")
	mustRefuse(t, home, nil, home, "home root")
	if got, err := resolveBindDir(filepath.Join(home, "notes"), everywhere, nil, home, ""); err == nil {
		t.Fatalf("a missing folder passed: %q", got)
	}
}

func TestACheckedPathSwappedForALinkIsRefusedAtTheNextLaunch(t *testing.T) {
	base := realTemp(t)
	root := filepath.Join(base, "srv")
	vault := filepath.Join(root, "vault")
	outside := filepath.Join(base, "outside")
	for _, d := range []string{vault, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	r := &runner{host: filepath.Join(base, "runner")}
	env := []string{`AGENT_BUS_CONFIG={"dir":"` + vault + `"}`}
	if _, reads, why := r.bindDir(dirAgent(), []string{root}, env); why != "" || !slices.Equal(reads, []string{vault}) {
		t.Fatalf("first check: %v %q", reads, why)
	}
	if err := os.Remove(vault); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, vault); err != nil {
		t.Fatal(err)
	}
	if _, reads, why := r.bindDir(dirAgent(), []string{root}, env); why == "" || reads != nil || !strings.Contains(why, "outside this node's watch roots") {
		t.Fatalf("after the swap: %v %q", reads, why)
	}
}

func TestAPerMessageLaunchBindsTheCheckedDirAndRefusesAChangedOne(t *testing.T) {
	r, b, _ := configurationFixture(t, "", `printf '%s' "${AGENT_BUS_DIR-unset}"`)
	s := r.served["#fixture@h"]
	vault := realTemp(t)
	s.in.Agent.Needs = &agentmeta.Needs{Dir: true}
	s.in.Host.Roots = []string{filepath.Dir(vault)}
	ask := func() string {
		t.Helper()
		r.runOne(s, protocol.Envelope{ID: "dir", From: "owner@h", To: s.name, Body: "go"})
		return configurationReply(t, b).Body
	}
	if _, err := b.Configure("#fixture@h", "owner@h", []byte(`{"dir":"`+vault+`"}`)); err != nil {
		t.Fatal(err)
	}
	if got := ask(); got != vault {
		t.Fatalf("script saw AGENT_BUS_DIR %q, want %q", got, vault)
	}
	if _, err := b.Configure("#fixture@h", "owner@h", []byte(`{"dir":"/"}`)); err != nil {
		t.Fatal(err)
	}
	if got := ask(); !strings.HasPrefix(got, "error: needs.dir: ") {
		t.Fatalf("a changed dir was not refused at launch: %q", got)
	}
}

func TestAKeptLaunchChecksTheDirOnTheConfigurationItJustRead(t *testing.T) {
	r, b, _ := configurationFixture(t, "itself", `while :; do sleep .02; done`)
	s := r.served["#fixture@h"]
	vault := realTemp(t)
	s.in.Agent.Needs = &agentmeta.Needs{Dir: true}
	s.in.Host.Roots = []string{filepath.Dir(vault)}
	for _, tc := range []struct {
		dir, want string
	}{{vault, ""}, {"/", "needs.dir"}, {"relative", "needs.dir"}, {filepath.Join(vault, "missing"), "needs.dir"}} {
		if _, err := b.Configure("#fixture@h", "owner@h", []byte(`{"dir":"`+tc.dir+`"}`)); err != nil {
			t.Fatal(err)
		}
		cmd, _, err := s.kept.command()
		switch {
		case tc.want == "" && (err != nil || !slices.Contains(cmd.Env, "AGENT_BUS_DIR="+vault)):
			t.Fatalf("dir %q: err %v, env %v", tc.dir, err, cmd)
		case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
			t.Fatalf("dir %q: err %v, want %q", tc.dir, err, tc.want)
		}
	}
}

func TestAPooledLaunchRefusesAnUnconfiguredDir(t *testing.T) {
	st, _ := streamFixture(t, `cat`, 1)
	st.s.in.Agent.Needs = &agentmeta.Needs{Dir: true}
	if _, err := st.start(); err == nil || !strings.Contains(err.Error(), "needs.dir") {
		t.Fatalf("start with no dir: %v", err)
	}
}

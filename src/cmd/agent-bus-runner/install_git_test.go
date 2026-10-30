package main

import (
	"context"
	"encoding/json"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeployFetchChecksHeadAndIsolatesCredentials(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	dest := filepath.Join(root, "stage")
	host := filepath.Join(root, "host")
	log := filepath.Join(root, "environment")
	wrong := filepath.Join(root, "wrong")
	for _, p := range []string{bin, dest, host} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	literal := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	pin := strings.Repeat("a", 40)
	script := "#!/usr/bin/python3\nimport os,sys,json\njson.dump(dict(os.environ),open(" + literal(log) + ",'w'))\nif 'rev-parse' in sys.argv: print(('f'*40) if os.path.exists(" + literal(wrong) + ") else " + literal(pin) + ")\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, key := range []string{"GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS", "SSH_AUTH_SOCK", "GIT_SSH_COMMAND", "HOME", "GIT_ASKPASS"} {
		t.Setenv(key, "private-sentinel")
	}
	r := &runner{host: host}
	in := installRequest{agent: "fixture", remote: "https://fixture.invalid/repo", commit: pin}
	if err := r.fetchDeploy(context.Background(), in, dest); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]string
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "private-sentinel") || env["GIT_ALLOW_PROTOCOL"] != "https:ssh" || !strings.Contains(env["GIT_SSH_COMMAND"], filepath.Join(host, ".deploy", "id")) || !strings.Contains(env["GIT_SSH_COMMAND"], "StrictHostKeyChecking=yes") {
		t.Fatalf("ambient credentials or missing host policy: %s", b)
	}
	writeFile(t, wrong, "wrong")
	if err := r.fetchDeploy(context.Background(), in, dest); err == nil || !strings.Contains(err.Error(), "HEAD does not equal") {
		t.Fatalf("wrong fetched HEAD accepted: %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.fetchDeploy(canceled, in, dest); err == nil {
		t.Fatal("canceled deploy succeeded")
	}
}

// Use real local Git objects and a test-only executable that maps one checked
// https URL to the fixture. No production file-protocol option is introduced.
func TestDeployFromGitInstallsOnlyTheReviewedRootPackage(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	r, b, _ := configurationFixture(t, "", "printf old")
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	bin := filepath.Join(root, "bin")
	for _, p := range []string{repo, bin} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	command := func(args ...string) string {
		t.Helper()
		c := exec.Command(git, append([]string{"-C", repo}, args...)...)
		c.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + root, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}
		out, e := c.CombinedOutput()
		if e != nil {
			t.Fatalf("fixture git %v: %s %v", args, out, e)
		}
		return strings.TrimSpace(string(out))
	}
	command("init", "--quiet")
	command("config", "user.name", "fixture")
	command("config", "user.email", "fixture@example.invalid")
	directoryAgent(t, root, "repo", "reviewed")
	command("add", ".")
	command("commit", "--quiet", "-m", "reviewed")
	pin := command("rev-parse", "HEAD")
	writeFile(t, filepath.Join(repo, "env.dist"), "FLAVOUR=floating\n")
	command("commit", "--quiet", "-am", "unreviewed")
	literal := func(s string) string { out, _ := json.Marshal(s); return string(out) }
	script := "#!/usr/bin/python3\nimport os,sys,subprocess\na=[" + literal(repo) + " if x=='https://fixture.invalid/repo' else x for x in sys.argv[1:]]\ne=dict(os.environ);e['GIT_ALLOW_PROTOCOL']='file'\nsys.exit(subprocess.run([" + literal(git) + "]+a,env=e).returncode)\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := r.installPackage(context.Background(), "install --replace fixture git https://fixture.invalid/repo "+pin); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: "probe"}); err != nil {
		t.Fatal(err)
	}
	if got := configurationReply(t, b).Body; got != "reviewed" {
		t.Fatalf("fetched unreviewed HEAD instead of full pin: %q", got)
	}
	if _, err := os.Lstat(filepath.Join(r.agents, "fixture", ".git")); !os.IsNotExist(err) {
		t.Fatalf("Git metadata published: %v", err)
	}
	// A repository that only packages a subdirectory is refused and leaves the
	// installed code and inventory intact.
	if err := os.Mkdir(filepath.Join(repo, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	command("mv", "agent.json", "nested/agent.json")
	command("commit", "--quiet", "-m", "subdirectory layout")
	nested := command("rev-parse", "HEAD")
	before, err := packageDigest(filepath.Join(r.agents, "fixture"))
	if err != nil {
		t.Fatal(err)
	}
	if err := r.installPackage(context.Background(), "install --replace fixture git https://fixture.invalid/repo "+nested); err == nil || !strings.Contains(err.Error(), "agent.json at the repository root") {
		t.Fatalf("subdirectory deploy accepted or wrong refusal: %v", err)
	}
	after, err := packageDigest(filepath.Join(r.agents, "fixture"))
	if err != nil || before != after {
		t.Fatalf("failed layout check changed live tree: %s %s %v", before, after, err)
	}
	entries, err := readAgents(r.host)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(entries["fixture"]["origin"]), pin) {
		t.Fatal("failed layout check changed origin pin")
	}

}

// Arm the private staging repository after init, as a compromised local Git
// configuration could. Real fetch/checkout would run these hooks without the
// command-line core.hooksPath override. The URL shim permits only local objects;
// this check cannot contact a provider or use host Git configuration.
func TestDeployFetchNeverRunsRepositoryHooks(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	dest := filepath.Join(root, "stage")
	bin := filepath.Join(root, "bin")
	hooks := filepath.Join(root, "hooks")
	for _, p := range []string{repo, dest, bin, hooks} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	gitEnv := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + root,
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}
	command := func(args ...string) string {
		t.Helper()
		c := exec.Command(git, append([]string{"-c", "core.hooksPath=/dev/null", "-c", "init.templateDir=", "-C", repo}, args...)...)
		c.Env = gitEnv
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("fixture git %v: %s %v", args, out, err)
		}
		return strings.TrimSpace(string(out))
	}
	command("init", "--quiet")
	writeFile(t, filepath.Join(repo, "reviewed"), "pinned code")
	command("add", "reviewed")
	command("-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "--quiet", "-m", "reviewed")
	pin := command("rev-parse", "HEAD")
	literal := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	for _, name := range []string{"reference-transaction", "post-checkout"} {
		marker := filepath.Join(root, name+".ran")
		script := "#!/usr/bin/python3\nopen(" + literal(marker) + ", 'a').write('hook ran\\n')\n"
		if err := os.WriteFile(filepath.Join(hooks, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	armed := filepath.Join(root, "armed")
	script := "#!/usr/bin/python3\nimport os,sys,subprocess\n" +
		"a=sys.argv[1:]; commands=('init','fetch','checkout','rev-parse')\n" +
		"verb=next(x for x in a if x in commands)\n" +
		"if verb=='fetch':\n" +
		"    assert a[-2]=='https://fixture.invalid/repo'\n" +
		"    a[-2]=" + literal(repo) + "\n" +
		"e=dict(os.environ); e['GIT_ALLOW_PROTOCOL']='file'\n" +
		"rc=subprocess.run([" + literal(git) + "]+a,env=e).returncode\n" +
		"if verb=='init' and rc==0:\n" +
		"    subprocess.run([" + literal(git) + ",'--git-dir'," + literal(filepath.Join(dest, ".git")) + ",'config','core.hooksPath'," + literal(hooks) + "],env=e,check=True)\n" +
		"    open(" + literal(armed) + ",'w').write('armed')\n" +
		"sys.exit(rc)\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	r := &runner{host: filepath.Join(root, "host")}
	if err := r.fetchDeploy(context.Background(), installRequest{remote: "https://fixture.invalid/repo", commit: pin}, dest); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(armed); err != nil || string(got) != "armed" {
		t.Fatalf("fixture never armed the repository hooks: %q %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(dest, "reviewed")); err != nil || string(got) != "pinned code" {
		t.Fatalf("fetch did not publish the pinned fixture: %q %v", got, err)
	}
	for _, name := range []string{"reference-transaction", "post-checkout"} {
		if _, err := os.Stat(filepath.Join(root, name+".ran")); !os.IsNotExist(err) {
			t.Errorf("deploy ran repository hook %s: %v", name, err)
		}
	}
}

package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/sandbox"
)

func checkoutCronTree(t *testing.T) (string, string) {
	t.Helper()
	// Outside /usr: that whole tree is already readable in bubblewrap and
	// would hide a missing support mount on the development node.
	root := t.TempDir()
	if strings.HasPrefix(root, "/usr/") {
		t.Fatal("checkout import proof must be outside /usr")
	}
	src := filepath.Join(root, "src")
	writeFile(t, filepath.Join(root, ".git"), "gitdir: fixture\n")
	writeFile(t, filepath.Join(src, "go.mod"), "module fixture\n")
	for _, name := range []string{"agents/cron", "agents/shared", "clients/python", "internal/version"} {
		if err := os.CopyFS(filepath.Join(src, name), os.DirFS(filepath.Join("..", "..", name))); err != nil {
			t.Fatal(err)
		}
	}
	host := filepath.Join(root, "runner")
	writeFile(t, filepath.Join(host, "agents.json"), `{"cron":{"enabled":true}}`)
	return filepath.Join(src, "agents"), host
}

func TestGitBundledCronStartsWithTheDefaultSandbox(t *testing.T) {
	if err := sandbox.ProbeBwrap(); err != nil {
		t.Skipf("checkout proof requires bubblewrap: %v", err)
	}
	bundled, host := checkoutCronTree(t)
	r := &runner{agents: filepath.Join(host, "agent.d"), bundled: bundled, host: host, say: io.Discard, ctx: context.Background()}
	list, err := loadSources(r.sources(), host)
	if err != nil || len(list) != 1 || list[0].Refused != "" || !list[0].enabled() {
		t.Fatalf("enabled checkout cron: %+v %v", list, err)
	}
	in := list[0]
	box, why := r.sandboxFor(in)
	if why != "" || box.Name() != "bwrap" {
		t.Fatalf("default sandbox: %v %s", box, why)
	}
	work := filepath.Join(host, "work", "cron")
	if err := os.MkdirAll(work, 0700); err != nil {
		t.Fatal(err)
	}
	s := &served{in: in, name: "#cron@fixture.test", bus: unconfiguredClient(), dir: r.codeDir(in), work: work, cmd: command(r.codeDir(in), in.Agent.Command), box: box, jail: &jail{}, env: []string{"AGENT_BUS_ASK=help", "PYTHONDONTWRITEBYTECODE=1"}}
	k := &keeper{r: r, s: s}
	cmd, _, err := k.command()
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("checkout cron failed in default sandbox: %v: %s", err, stderr.String())
		}
	case <-time.After(5 * time.Second):
		k.signalOne(cmd, s.jail, 9)
		<-done
		t.Fatal("checkout cron startup did not complete")
	}
	var manifest struct {
		Description string `json:"description"`
	}
	raw, err := os.ReadFile(filepath.Join(bundled, "cron", "agent.json"))
	if err != nil || json.Unmarshal(raw, &manifest) != nil {
		t.Fatalf("manifest: %v", err)
	}
	if !strings.Contains(out.String(), manifest.Description) || stderr.String() != "" {
		t.Fatalf("real cron help after imports: stdout=%q stderr=%q", out.String(), stderr.String())
	}
}

func TestCheckoutSupportReadsNeverApplyToSharedOrReleasePackages(t *testing.T) {
	bundled, host := checkoutCronTree(t)
	// Include the fourth allowed root, even though cron only uses Python.
	if err := os.MkdirAll(filepath.Join(filepath.Dir(bundled), "clients", "typescript"), 0700); err != nil {
		t.Fatal(err)
	}
	r := &runner{agents: filepath.Join(host, "agent.d"), bundled: bundled}
	dir := filepath.Join(bundled, "cron")
	want := []string{dir, filepath.Join(bundled, "shared"), filepath.Join(filepath.Dir(bundled), "clients", "python"), filepath.Join(filepath.Dir(bundled), "clients", "typescript"), filepath.Join(filepath.Dir(bundled), "internal", "version")}
	if got := r.codeReads(installed{Bundled: true}, dir); !reflect.DeepEqual(got, want) {
		t.Fatalf("checkout read roots: %v, want %v", got, want)
	}
	if got := r.codeReads(installed{}, dir); !reflect.DeepEqual(got, []string{dir}) {
		t.Fatalf("shared agent inherited checkout reads: %v", got)
	}
	r.bundled = filepath.Join(t.TempDir(), "agents")
	if err := os.MkdirAll(filepath.Join(r.bundled, "shared"), 0700); err != nil {
		t.Fatal(err)
	}
	if got := r.codeReads(installed{Bundled: true}, dir); !reflect.DeepEqual(got, []string{dir}) {
		t.Fatalf("release package inherited checkout reads: %v", got)
	}
}

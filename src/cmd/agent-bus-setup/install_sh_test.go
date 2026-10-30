package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// install.sh, driven the way a person runs it, against a local release
// signed with a throwaway key: the shipped script carries neither, so the
// test renders a copy with this key and this server (and plain HTTP).
type installFixture struct {
	t       *testing.T
	dir     string // releases served: <dir>/download/v<version>/...
	home    string
	key     string
	script  string
	server  *httptest.Server
	release string
}

func newInstallFixture(t *testing.T) *installFixture {
	t.Helper()
	for _, tool := range []string{"ssh-keygen", "sha256sum", "tar", "curl"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("needs %s", tool)
		}
	}
	f := &installFixture{t: t, dir: t.TempDir(), home: t.TempDir()}
	keys := t.TempDir()
	f.key = filepath.Join(keys, "release")
	if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "test", "-f", f.key).CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen: %v %s", err, out)
	}
	f.server = httptest.NewServer(http.FileServer(http.Dir(f.dir)))
	t.Cleanup(f.server.Close)
	pub, err := os.ReadFile(f.key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	f.script = f.render("agent-bus-release " + strings.TrimSpace(string(pub)))
	return f
}

// render is install.sh with the test's key and server, and HTTPS not required.
func (f *installFixture) render(keys string) string {
	src, err := os.ReadFile("../../../install.sh")
	if err != nil {
		f.t.Fatal(err)
	}
	s := string(src)
	for old, new := range map[string]string{
		`RELEASES="https://github.com/parf/ai-agent-bus/releases"`: `RELEASES="` + f.server.URL + `"`,
		`RELEASE_KEYS=""`:             `RELEASE_KEYS="` + keys + `"`,
		` --proto '=https' --tlsv1.2`: "",
	} {
		if strings.Count(s, old) != 1 {
			f.t.Fatalf("install.sh no longer holds %q once", old)
		}
		s = strings.Replace(s, old, new, 1)
	}
	path := filepath.Join(f.t.TempDir(), "install.sh")
	if err := os.WriteFile(path, []byte(s), 0o755); err != nil {
		f.t.Fatal(err)
	}
	return path
}

// publish writes one client release, signed unless sign is false.
func (f *installFixture) publish(version string, sign bool) string {
	f.t.Helper()
	name := "agent-bus-client-" + version + "-linux-" + map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[goarchFromUname(f.t)]
	files := map[string]string{"agent-bus": "#!/bin/sh\necho agent-bus " + version + "\n", "agent-bus-runner": "#!/bin/sh\n", "agent-bus-ts": "#!/bin/sh\necho ts \"$@\"\n",
		"launchers/ab-claude": "#!/bin/sh\n", "launchers/ab-codex": "#!/bin/sh\n", "launchers/ab-opencode": "#!/bin/sh\n", "launchers/ab-opencode1": "#!/bin/sh\n",
		"internal/version/VERSION": version + "\n"}
	var manifest strings.Builder
	for _, p := range sortedKeys(files) {
		sum := sha256.Sum256([]byte(files[p]))
		manifest.WriteString(hex.EncodeToString(sum[:]) + "  " + p + "\n")
	}
	files["MANIFEST.sha256"] = manifest.String()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, p := range sortedKeys(files) {
		body := files[p]
		if err := tw.WriteHeader(&tar.Header{Name: name + "/" + p, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			f.t.Fatal(err)
		}
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	rel := filepath.Join(f.dir, "download", "v"+version)
	if err := os.MkdirAll(rel, 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rel, name+".tar.gz"), buf.Bytes(), 0o644); err != nil {
		f.t.Fatal(err)
	}
	if sign {
		if out, err := exec.Command("../../release-sign.sh", rel, f.key).CombinedOutput(); err != nil {
			f.t.Fatalf("release-sign.sh: %v %s", err, out)
		}
	}
	return rel
}

func goarchFromUname(t *testing.T) string {
	out, err := exec.Command("uname", "-m").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) == "aarch64" {
		return "arm64"
	}
	return "amd64"
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (f *installFixture) run(args ...string) (string, error) {
	cmd := exec.Command("sh", append([]string{f.script}, args...)...)
	cmd.Env = []string{"HOME=" + f.home, "PATH=" + os.Getenv("PATH")}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (f *installFixture) current() string {
	target, err := os.Readlink(filepath.Join(f.home, ".local/lib/agent-bus/current"))
	if err != nil {
		return ""
	}
	return filepath.Base(target)
}

func TestInstallShClientInstallsUpdatesAndRollsBack(t *testing.T) {
	f := newInstallFixture(t)
	f.publish("1.0.0", true)
	f.publish("1.0.1", true)
	f.publish("1.0.2", true)
	if out, err := f.run("--client", "--version", "1.0.0"); err != nil || f.current() != "1.0.0" {
		t.Fatalf("first install: %v %s current=%s", err, out, f.current())
	}
	for _, link := range []string{"agent-bus", "agent-bus-runner", "agent-bus-mcp", "ab-claude", "ab-opencode1"} {
		if _, err := os.Stat(filepath.Join(f.home, ".local/bin", link)); err != nil {
			t.Fatalf("command %s is not linked: %v", link, err)
		}
	}
	if target, _ := os.Readlink(filepath.Join(f.home, ".local/bin/agent-bus-mcp")); !strings.HasSuffix(target, "/current/agent-bus-ts") {
		t.Fatalf("agent-bus-mcp names %s", target)
	}
	f.run("--client", "--version", "1.0.1")
	if out, err := f.run("--client", "--version", "1.0.2"); err != nil || f.current() != "1.0.2" {
		t.Fatalf("update: %v %s", err, out)
	}
	kept, _ := os.ReadDir(filepath.Join(f.home, ".local/lib/agent-bus/releases"))
	if len(kept) != 2 || kept[0].Name() != "1.0.1" || kept[1].Name() != "1.0.2" {
		t.Fatalf("kept releases: %v, want the current and the one it replaced", kept)
	}
	if out, err := f.run("--client", "--rollback"); err != nil || f.current() != "1.0.1" {
		t.Fatalf("rollback: %v %s current=%s", err, out, f.current())
	}
}

func TestInstallShRefusesBeforeWritingAnything(t *testing.T) {
	cases := map[string]func(f *installFixture, rel string){
		"tampered archive": func(f *installFixture, rel string) {
			matches, _ := filepath.Glob(filepath.Join(rel, "*.tar.gz"))
			body, _ := os.ReadFile(matches[0])
			body[len(body)/2] ^= 0xff
			os.WriteFile(matches[0], body, 0o644)
		},
		"tampered SHA256SUMS": func(f *installFixture, rel string) {
			sums := filepath.Join(rel, "SHA256SUMS")
			body, _ := os.ReadFile(sums)
			os.WriteFile(sums, append([]byte(strings.Repeat("0", 64)+"  agent-bus-client-9.9.9-linux-x86_64.tar.gz\n"), body...), 0o644)
		},
		"missing signature": func(f *installFixture, rel string) { os.Remove(filepath.Join(rel, "SHA256SUMS.sig")) },
		"another key": func(f *installFixture, rel string) {
			other := filepath.Join(f.t.TempDir(), "other")
			exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", other).Run()
			os.Remove(filepath.Join(rel, "SHA256SUMS.sig"))
			exec.Command("ssh-keygen", "-Y", "sign", "-q", "-f", other, "-n", "agent-bus-release", filepath.Join(rel, "SHA256SUMS")).Run()
		},
		"another namespace": func(f *installFixture, rel string) {
			os.Remove(filepath.Join(rel, "SHA256SUMS.sig"))
			exec.Command("ssh-keygen", "-Y", "sign", "-q", "-f", f.key, "-n", "agent-bus", filepath.Join(rel, "SHA256SUMS")).Run()
		},
	}
	for name, damage := range cases {
		t.Run(name, func(t *testing.T) {
			f := newInstallFixture(t)
			rel := f.publish("1.0.0", true)
			damage(f, rel)
			out, err := f.run("--client", "--version", "1.0.0")
			if err == nil {
				t.Fatalf("installed anyway: %s", out)
			}
			if !strings.Contains(out, "nothing installed") && !strings.Contains(out, "cannot download the signature") {
				t.Fatalf("refusal does not say nothing was installed: %s", out)
			}
			if entries, _ := os.ReadDir(f.home); len(entries) != 0 {
				t.Fatalf("the refused install wrote into HOME: %v", entries)
			}
		})
	}
}

func TestInstallShWithoutAKeyInstallsNothing(t *testing.T) {
	f := newInstallFixture(t)
	f.publish("1.0.0", true)
	f.script = f.render("")
	if out, err := f.run("--client", "--version", "1.0.0"); err == nil || !strings.Contains(out, "carries no release key") {
		t.Fatalf("an unkeyed script installed or said otherwise: %v %s", err, out)
	}
}

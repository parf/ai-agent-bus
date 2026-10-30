package main

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/tlsdir"
)

// J59: setup restarted the web unit on every run, so a quick series of runs
// (the fresh-install acceptance, a script, an upgrade) hit systemd's start
// limit and left the face failed. The face is written first and started once,
// only when something it reads changed or it is not running.
type webRig struct {
	t      *testing.T
	web    string
	log    string
	state  string
	moved  bool
	daemon string
}

func newWebRig(t *testing.T) *webRig {
	t.Helper()
	account := tlsTestAccount(t)
	dir := t.TempDir()
	r := &webRig{t: t, web: filepath.Join(dir, "web"), log: filepath.Join(dir, "systemctl.log"), state: filepath.Join(dir, "state"), daemon: filepath.Join(dir, "daemon-tls")}
	if err := os.Mkdir(r.web, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := "#!/bin/sh\necho \"$1\" >> \"$SYSTEMCTL_TEST_LOG\"\ncase \"$1\" in\n  restart|start) echo active > \"$SYSTEMCTL_TEST_STATE\" ;;\n  stop) echo inactive > \"$SYSTEMCTL_TEST_STATE\" ;;\n  is-active) [ \"$(cat \"$SYSTEMCTL_TEST_STATE\" 2>/dev/null)\" = active ] ;;\nesac\n"
	if err := os.WriteFile(filepath.Join(dir, "systemctl"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SYSTEMCTL_TEST_LOG", r.log)
	t.Setenv("SYSTEMCTL_TEST_STATE", r.state)
	set := func(p *string, v string) { old := *p; *p = v; t.Cleanup(func() { *p = old }) }
	set(&webUnitPath, filepath.Join(dir, "agent-bus-web.service"))
	set(&bunPath, "/bin/sh")
	set(&webTLSDir, filepath.Join(dir, "web-tls"))
	set(&webTLSDropIn, filepath.Join(dir, "web.service.d", "tls.conf"))
	set(&webTLSAccount, account)
	set(&webAppliedPath, filepath.Join(dir, "web.applied"))
	old := ensureWebAccount
	ensureWebAccount = func(string) (bool, error) { return r.moved, nil }
	t.Cleanup(func() { ensureWebAccount = old })
	choice := &tlsChoice{mode: "self-signed", dir: r.daemon}
	if _, err := choice.install(account, io.Discard); err != nil {
		t.Fatal(err)
	}
	return r
}

// unit writes the release's unit template naming the address the face answers on.
func (r *webRig) unit(addr, comment string) {
	r.t.Helper()
	text := "# " + comment + "\n[Service]\nExecStart=/usr/bin/bun run @WEB_DIR@/server.ts\nEnvironment=AGENT_BUS_WEB_ADDR=" + addr + "\nExecPaths=/usr/bin/bun\n"
	if err := os.WriteFile(filepath.Join(r.web, webUnitName), []byte(text), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

// run is one setup run's web step, and answers how many restarts it made.
func (r *webRig) run(tlsOn bool) int {
	r.t.Helper()
	if err := os.WriteFile(r.log, nil, 0o600); err != nil {
		r.t.Fatal(err)
	}
	if err := installWeb(r.web, tlsOn, r.daemon); err != nil {
		r.t.Fatal(err)
	}
	got, err := os.ReadFile(r.log)
	if err != nil {
		r.t.Fatal(err)
	}
	return strings.Count(string(got), "restart\n")
}

func healthServer(t *testing.T, cert *tls.Certificate) string {
	t.Helper()
	s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	if cert != nil {
		s.TLS = &tls.Config{Certificates: []tls.Certificate{*cert}}
		s.StartTLS()
	} else {
		s.Start()
	}
	t.Cleanup(s.Close)
	return strings.TrimPrefix(strings.TrimPrefix(s.URL, "https://"), "http://")
}

func TestSetupStartsTheWebFaceOnceAndOnlyWhenSomethingChanged(t *testing.T) {
	r := newWebRig(t)
	plain := healthServer(t, nil)
	cert, err := tlsdir.Load(r.daemon)
	if err != nil {
		t.Fatal(err)
	}
	secure := healthServer(t, &cert)

	r.unit(plain, "first")
	if n := r.run(false); n != 1 {
		t.Fatalf("a first install restarted the face %d times, want exactly 1", n)
	}
	// A burst of runs that change nothing must not touch a healthy face: this
	// is what systemd's start limit counts.
	total := 0
	for i := 0; i < 6; i++ {
		total += r.run(false)
	}
	if total != 0 {
		t.Fatalf("six identical setup runs restarted a healthy face %d times, want 0", total)
	}
	// One run that changes the unit, turns TLS on and moves the account is still one start.
	r.unit(secure, "second")
	r.moved = true
	if n := r.run(true); n != 1 {
		t.Fatalf("a run changing the unit, TLS and account restarted %d times, want exactly 1", n)
	}
	r.moved = false
	if n := r.run(true); n != 0 {
		t.Fatalf("an identical run over TLS restarted %d times, want 0", n)
	}
	// A face that is not running is started even when nothing changed.
	if err := os.WriteFile(r.state, []byte("inactive\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if n := r.run(true); n != 1 {
		t.Fatalf("a stopped face was restarted %d times, want exactly 1", n)
	}
	if got, _ := os.ReadFile(r.state); strings.TrimSpace(string(got)) != "active" {
		t.Fatalf("setup left the face %q", got)
	}
}

func TestSetupChangingOnlyTheWebTLSCertificateRestartsOnce(t *testing.T) {
	r := newWebRig(t)
	cert, err := tlsdir.Load(r.daemon)
	if err != nil {
		t.Fatal(err)
	}
	secure := healthServer(t, &cert)
	r.unit(secure, "tls")
	if n := r.run(true); n != 1 {
		t.Fatalf("first TLS install: %d restarts, want 1", n)
	}
	// The drop-in goes with TLS off: the face needs its restart, and one is enough.
	plain := healthServer(t, nil)
	r.unit(plain, "tls")
	if n := r.run(false); n != 1 {
		t.Fatalf("TLS turned off: %d restarts, want 1", n)
	}
}

// A release switch, forward or back, changes the code behind an unchanged
// unit through the stable `current` link: the running face is the old release
// and must be restarted (found in review of the first J59 fix).
func TestSetupRestartsTheWebFaceWhenCurrentSwitchesToAnotherRelease(t *testing.T) {
	r := newWebRig(t)
	root := t.TempDir()
	plain := healthServer(t, nil)
	sh, err := os.ReadFile("/bin/sh")
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{"release-1", "release-2"} {
		dir := filepath.Join(root, "releases", id)
		if err := os.MkdirAll(filepath.Join(dir, "web"), 0o755); err != nil {
			t.Fatal(err)
		}
		// Distinct executables, one byte apart: only what the link resolves to differs.
		if err := os.WriteFile(filepath.Join(dir, "agent-bus-ts"), append(append([]byte{}, sh...), byte(i)), 0o755); err != nil {
			t.Fatal(err)
		}
		unit := "[Service]\nExecStart=@ROOT@/agent-bus-ts web\nEnvironment=AGENT_BUS_WEB_ADDR=" + plain + "\nExecPaths=/usr/bin/bun\n"
		if err := os.WriteFile(filepath.Join(dir, "web", webUnitName), []byte(unit), 0o644); err != nil {
			t.Fatal(err)
		}
		// The same modification times everywhere: only the executable's bytes differ.
		same := time.Unix(1_700_000_000, 0)
		for _, p := range []string{dir, filepath.Join(dir, "web"), filepath.Join(dir, "web", webUnitName), filepath.Join(dir, "agent-bus-ts")} {
			if err := os.Chtimes(p, same, same); err != nil {
				t.Fatal(err)
			}
		}
	}
	current := filepath.Join(root, "current")
	switchTo := func(id string) {
		t.Helper()
		_ = os.Remove(current)
		if err := os.Symlink(filepath.Join("releases", id), current); err != nil {
			t.Fatal(err)
		}
	}
	r.web = filepath.Join(current, "web")
	for _, step := range []struct {
		release string
		want    int
		why     string
	}{
		{"release-1", 1, "first install"},
		{"release-1", 0, "the same release again"},
		{"release-2", 1, "an upgrade: current now resolves to other bytes under an identical unit"},
		{"release-2", 0, "the upgraded release again"},
		{"release-1", 1, "a rollback"},
	} {
		switchTo(step.release)
		if n := r.run(false); n != step.want {
			t.Fatalf("%s: %d restarts, want %d", step.why, n, step.want)
		}
	}
}

// A checkout runs its sources: changing a file under the web directory is a
// change, though the unit and the path are the same.
func TestSetupRestartsTheWebFaceWhenItsSourcesChange(t *testing.T) {
	r := newWebRig(t)
	plain := healthServer(t, nil)
	r.unit(plain, "checkout")
	if err := os.WriteFile(filepath.Join(r.web, "server.ts"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if n := r.run(false); n != 1 {
		t.Fatalf("first install: %d restarts, want 1", n)
	}
	if n := r.run(false); n != 0 {
		t.Fatalf("unchanged sources: %d restarts, want 0", n)
	}
	if err := os.WriteFile(filepath.Join(r.web, "server.ts"), []byte("two, longer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if n := r.run(false); n != 1 {
		t.Fatalf("changed sources: %d restarts, want 1", n)
	}
}

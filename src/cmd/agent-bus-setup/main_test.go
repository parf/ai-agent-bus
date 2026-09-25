package main

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The daemon's unit runs no web face since 0.8.50: that is its own unit, so
// the daemon keeps no cgroup delegation and no -web.
func TestTheDaemonUnitRunsNoWebFace(t *testing.T) {
	unit := unitFor("/program/agent-busd", "127.0.0.1:6767", "owner@example", nil)
	if strings.Count(unit, "CapabilityBoundingSet=CAP_CHOWN") != 1 {
		t.Error("the daemon unit lost its one capability")
	}
	for _, gone := range []string{"Delegate", " -web"} {
		if strings.Contains(unit, gone) {
			t.Errorf("the daemon unit still has %q", gone)
		}
	}
}

// The web unit setup installs is the one the release ships: its account, no
// capabilities, loopback only, and the shared socket.
func TestTheWebUnitIsLockedDown(t *testing.T) {
	unit, err := os.ReadFile("../../web/agent-bus-web.service")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"User=agent-bus-web", "\nCapabilityBoundingSet=\n", "NoNewPrivileges=yes", "ProtectSystem=strict",
		"IPAddressDeny=any", "IPAddressAllow=localhost", "Environment=AGENT_BUS_ADDR=/run/agent-bus/bus.sock",
		"Environment=AGENT_BUS_WEB_ADDR=127.0.0.1:6780", "ExecStart=/usr/bin/bun run /var/lib/agent-bus/web/server.ts",
		"MemoryMax=256M", "TasksMax=64", "NoExecPaths=/",
	} {
		if !strings.Contains(string(unit), want) {
			t.Errorf("the web unit lacks %q", want)
		}
	}
	if m := webAddr.FindSubmatch(unit); m == nil || string(m[1]) != "127.0.0.1:6780" {
		t.Errorf("setup reads the web address as %q", m)
	}
}

func TestWebDirFollowsTheCurrentRelease(t *testing.T) {
	withInstallPaths(t)
	if got := webDirFor(filepath.Join(installRoot, "releases", "0.8.50-abc", "agent-busd")); got != filepath.Join(installRoot, "current", "web") {
		t.Errorf("a packaged release's web: %s", got)
	}
	if got := webDirFor("/usr/local/src/x/src/agent-busd"); got != "/usr/local/src/x/src/web" {
		t.Errorf("an explicit build's web: %s", got)
	}
}

func TestFirstUserWaitsForTheCredentialSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "account.sock")
	ready := make(chan net.Listener, 1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		l, err := net.Listen("unix", path)
		if err != nil {
			ready <- nil
			return
		}
		ready <- l
	}()
	if err := waitForSocket(path, time.Second); err != nil {
		t.Fatal(err)
	}
	l := <-ready
	if l == nil {
		t.Fatal("fixture listener failed")
	}
	l.Close()

	regular := filepath.Join(t.TempDir(), "not-a-socket")
	if err := os.WriteFile(regular, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := waitForSocket(regular, 25*time.Millisecond); err == nil {
		t.Fatal("regular file satisfied the credential-socket wait")
	}
}

// ldd's lines name each library by path; the unit may execute those alone.
func TestExecPathsComeFromTheLibrariesBunLinks(t *testing.T) {
	out := []byte("\tlinux-vdso.so.1 (0x00007ffd)\n\tlibc.so.6 => /lib64/libc.so.6 (0x00007f)\n\t/lib64/ld-linux-x86-64.so.2 (0x00007f)\n")
	var got []string
	for _, m := range lddPath.FindAllSubmatch(out, -1) {
		got = append(got, string(m[1]))
	}
	if strings.Join(got, " ") != "/lib64/libc.so.6 /lib64/ld-linux-x86-64.so.2" {
		t.Fatalf("libraries read from ldd: %v", got)
	}
	if _, err := os.Stat(bunPath); err == nil {
		p, err := hostExecPaths()
		if err != nil || !strings.HasPrefix(p, "ExecPaths=/usr/bin/bun ") || !strings.Contains(p, "libc.so") {
			t.Fatalf("this host's exec paths: %q %v", p, err)
		}
	}
}

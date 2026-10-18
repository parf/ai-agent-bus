package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/tlsdir"
)

func tlsTestAccount(t *testing.T) string {
	t.Helper()
	me, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	group, err := user.LookupGroupId(me.Gid)
	if err != nil {
		t.Fatal(err)
	}
	old := tlsOwner
	tlsOwner = os.Getuid()
	t.Cleanup(func() { tlsOwner = old })
	return group.Name
}

func TestTLSWritesRestartAnOtherwiseUnchangedDaemon(t *testing.T) {
	account := tlsTestAccount(t)
	dir := t.TempDir()
	log := filepath.Join(dir, "systemctl.log")
	fake := filepath.Join(dir, "systemctl")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s %s\\n' \"$1\" \"$2\" >> \"$SYSTEMCTL_TEST_LOG\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SYSTEMCTL_TEST_LOG", log)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(protocol.NodeIdentity{Version: "fixture-version", Build: "fixture-build"})
	}))
	defer server.Close()
	addr := strings.TrimPrefix(server.URL, "http://")
	choice := &tlsChoice{mode: "self-signed", dir: filepath.Join(dir, "tls")}
	for _, tc := range []struct {
		mode string
		want string
	}{
		{"self-signed", "restart agent-busd\n"},
		{"self-signed", "restart agent-busd\n"}, // same unit and version, new certificate
		{"off", "restart agent-busd\n"},
		{"", "start agent-busd\n"}, // install resets its per-run write flag
	} {
		choice.mode = tc.mode
		if _, err := choice.install(account, io.Discard); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(log, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := choice.startDaemon(true, false, addr, "fixture-version", "fixture-build"); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(log)
		if err != nil || string(got) != tc.want {
			t.Fatalf("TLS mode %q started with %q, want %q: %v", tc.mode, got, tc.want, err)
		}
	}
}

func TestReinstallRestoresExternalTLSAndItsPermissions(t *testing.T) {
	account := tlsTestAccount(t)
	base, home, dropins := fixtureNode(t)
	tls := filepath.Join(base, "etc", "tls") // outside the daemon state home
	choice := &tlsChoice{mode: "self-signed", dir: tls}
	oldFingerprint, err := choice.install(account, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	before := map[string]os.FileInfo{}
	for _, name := range []string{"", tlsdir.CertFile, tlsdir.KeyFile} {
		before[name], err = os.Stat(filepath.Join(tls, name))
		if err != nil {
			t.Fatal(err)
		}
	}
	aside := filepath.Join(base, "set-aside")
	if err := moveAside(aside, home, dropins, tls); err != nil {
		t.Fatal(err)
	}
	// No explicit TLS change keeps using the existing certificate on reinstall.
	if _, err := tlsdir.Load(tls); err != nil {
		t.Fatalf("setting aside made unchanged TLS unavailable: %v", err)
	}
	backup, err := tlsdir.Load(filepath.Join(aside, "tls"))
	if err != nil || tlsdir.Fingerprint(backup) != oldFingerprint {
		t.Fatalf("old TLS was not backed up: %v", err)
	}
	newFingerprint, err := choice.install(account, io.Discard)
	if err != nil || newFingerprint == oldFingerprint {
		t.Fatalf("fixture did not rotate TLS: %v", err)
	}
	// Rollback must undo changed permissions as well as changed bytes.
	if err := os.Chmod(tls, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(tls, tlsdir.KeyFile), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := restoreAside(aside, home, dropins, tls); err != nil {
		t.Fatal(err)
	}
	restored, err := tlsdir.Load(tls)
	if err != nil || tlsdir.Fingerprint(restored) != oldFingerprint {
		t.Fatalf("rollback did not restore the old certificate/key: %v", err)
	}
	for name, old := range before {
		got, err := os.Stat(filepath.Join(tls, name))
		if err != nil {
			t.Fatal(err)
		}
		oldOwner, owner := old.Sys().(*syscall.Stat_t), got.Sys().(*syscall.Stat_t)
		if got.Mode().Perm() != old.Mode().Perm() || oldOwner.Uid != owner.Uid || oldOwner.Gid != owner.Gid {
			t.Errorf("TLS %q ownership or mode changed on rollback", name)
		}
	}
}

func TestReinstallRollbackRestoresTLSAbsence(t *testing.T) {
	account := tlsTestAccount(t)
	base, home, dropins := fixtureNode(t)
	tls := filepath.Join(base, "tls")
	aside := filepath.Join(base, "set-aside")
	if err := moveAside(aside, home, dropins, tls); err != nil {
		t.Fatal(err)
	}
	if _, err := (&tlsChoice{mode: "self-signed", dir: tls}).install(account, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := restoreAside(aside, home, dropins, tls); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tls); !os.IsNotExist(err) {
		t.Fatalf("rollback left newly installed TLS enabled: %v", err)
	}
	if _, err := tlsdir.Load(filepath.Join(aside, "failed-tls")); err != nil {
		t.Fatalf("failed new TLS was not retained root-only: %v", err)
	}
}

func TestTLSRestoreFailureKeepsItsBackup(t *testing.T) {
	account := tlsTestAccount(t)
	base, home, dropins := fixtureNode(t)
	tls := filepath.Join(base, "tls")
	choice := &tlsChoice{mode: "self-signed", dir: tls}
	fingerprint, err := choice.install(account, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	aside := filepath.Join(base, "set-aside")
	if err := moveAside(aside, home, dropins, tls); err != nil {
		t.Fatal(err)
	}
	// It is a readable directory, but renaming its '/.' entry is refused.
	// This forces the real atomic restorer to fail after its source was read.
	if err := restoreAside(aside, home, dropins, tls+"/."); err == nil {
		t.Fatal("a failed TLS restore was reported as successful")
	}
	backup, err := tlsdir.Load(filepath.Join(aside, "tls"))
	if err != nil || tlsdir.Fingerprint(backup) != fingerprint {
		t.Fatalf("failed TLS restore discarded its recovery copy: %v", err)
	}
}

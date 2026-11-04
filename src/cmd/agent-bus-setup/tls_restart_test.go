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
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
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

package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestInstalledAdminDefaultsAreForThisOS(t *testing.T) {
	account, socket := "agent-busd", "/run/agent-bus/bus.sock"
	if runtime.GOOS == "darwin" {
		account, socket = "_agentbusd", "/usr/local/var/run/agent-bus/bus.sock"
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("release platforms only")
	}
	if svcAccount != account || installedSocket != socket {
		t.Fatalf("installed admin defaults %q %q, want %q %q", svcAccount, installedSocket, account, socket)
	}
}

func TestAdminHomeDefaultsMatchReleasePlatforms(t *testing.T) {
	for _, tc := range []struct{ platform, account, home string }{
		{"linux", "agent-busd", "/var/lib/agent-bus/daemon"},
		{"darwin", "_agentbusd", "/usr/local/var/agent-bus/daemon"},
	} {
		account, home := adminAccountDefaults(tc.platform)
		if account != tc.account || home != tc.home {
			t.Fatalf("%s admin defaults: %s %s", tc.platform, account, home)
		}
	}
}

func TestAdminCredentialHintMatchesThisPlatform(t *testing.T) {
	got := credentialHint("fixture@localhost")
	want := "ssh agent-busd@<host>"
	if runtime.GOOS == "darwin" {
		want = "sudo -u _agentbusd agent-bus token fixture@localhost"
		if strings.Contains(got, "ssh") {
			t.Fatalf("macOS hint still recommends SSH: %s", got)
		}
	}
	if !strings.Contains(got, want) {
		t.Fatalf("credential hint %q lacks %q", got, want)
	}
}

package main

import (
	"runtime"
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

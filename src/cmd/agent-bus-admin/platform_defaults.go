package main

import (
	"fmt"
	"runtime"
)

var svcAccount, defaultAccountHome = adminAccountDefaults(runtime.GOOS)

func adminAccountDefaults(goos string) (string, string) {
	if goos == "darwin" {
		return "_agentbusd", "/usr/local/var/agent-bus/daemon"
	}
	return "agent-busd", "/var/lib/agent-bus/daemon"
}

func credentialHint(name string) string {
	if runtime.GOOS == "darwin" {
		return fmt.Sprintf("%s may get its credential with sudo -u %s agent-bus token %s", name, svcAccount, name)
	}
	return fmt.Sprintf("%s may now ask for its credential over ssh %s@<host>", name, svcAccount)
}

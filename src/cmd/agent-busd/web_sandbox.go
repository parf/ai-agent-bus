package main

// webSandbox exposes individual inputs, never the daemon's directories. A
// private PID namespace and proc mount prevent reaching those directories via
// another process's root, cwd or descriptors. Host networking remains available
// for the HTTP listener; it grants no authenticated bus identity.
// See docs/11-processes.md#web-authority-boundary.
func webSandbox(exe, shared string, getenv func(string) string) []string {
	args := []string{
		"--unshare-user", "--unshare-pid", "--unshare-ipc", "--unshare-uts",
		"--die-with-parent", "--new-session", "--cap-drop", "ALL",
		"--ro-bind", exe, "/agent-bus-web",
		"--ro-bind", shared, "/bus.sock",
		"--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp", "--chdir", "/",
		// Only resolver inputs are needed if the configured listen address is
		// a hostname. No host directory (or its other files) is exposed.
		"--ro-bind-try", "/etc/hosts", "/etc/hosts",
		"--ro-bind-try", "/etc/resolv.conf", "/etc/resolv.conf",
		"--clearenv", "--setenv", "AGENT_BUS_ADDR", "/bus.sock",
	}
	if addr := getenv("AGENT_BUS_WEB_ADDR"); addr != "" {
		args = append(args, "--setenv", "AGENT_BUS_WEB_ADDR", addr)
	}
	for _, file := range []struct{ env, path string }{
		{"AGENT_BUS_WEB_CERT", "/tls/cert"},
		{"AGENT_BUS_WEB_KEY", "/tls/key"},
	} {
		if src := getenv(file.env); src != "" {
			// Missing requested files fail the mount: never downgrade to HTTP.
			args = append(args, "--ro-bind", src, file.path, "--setenv", file.env, file.path)
		}
	}
	return append(args, "--", "/agent-bus-web")
}

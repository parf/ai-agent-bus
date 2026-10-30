//go:build !linux

package api

// SystemRuntimeDir is where a daemon installed for the whole host keeps its
// sockets, outside anyone's home. macOS has no /run, and launchd makes no
// runtime directory for a job, so setup makes this one and it outlives a
// reboot; the daemon clears a stale socket before it listens.
// See docs/07-setup.md#macos.
const SystemRuntimeDir = "/usr/local/var/run/agent-bus"

package api

// SystemRuntimeDir is where a daemon installed for the whole host keeps its
// sockets: outside anyone's home, and cleared by a reboot.
// See docs/02-access.md#local-socket.
const SystemRuntimeDir = "/run/agent-bus"

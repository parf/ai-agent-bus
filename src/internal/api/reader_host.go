package api

import (
	"os"
	"strings"

	"github.com/parf/ai-agent-bus/internal/core"
)

// HostEnv overrides the hostname a reader states, for a host whose own name
// says nothing about it (docs/05-discovery.md#where-a-pool-member-says-it-is).
const HostEnv = "AGENT_BUS_HOST"

// ReaderHost is the hostname a runner or the CLI states on its reads: HostEnv
// when set, otherwise the operating system's hostname. The daemon refuses a
// value that is not 1 to 64 characters of letters, digits, '.' and '-', and a
// refused read would stop the reader, so one that would be refused is not
// sent: the second answer says why, and the read goes without a host.
func ReaderHost() (host, why string) {
	host = strings.TrimSpace(os.Getenv(HostEnv))
	source := HostEnv
	if host == "" {
		h, err := os.Hostname()
		if err != nil || h == "" {
			return "", ""
		}
		host, source = h, "the hostname"
	}
	if err := core.CheckHost(host); err != nil {
		return "", source + " " + strings.TrimPrefix(err.Error(), "bad name: ") + "; reads state no host"
	}
	return host, ""
}

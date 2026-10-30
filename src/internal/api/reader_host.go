package api

import (
	"os"

	"github.com/parf/ai-agent-bus/internal/core"
)

// hostname is the machine's name; a variable so a check can set it.
var hostname = os.Hostname

// ReaderHost is the hostname a runner or the CLI states on its reads: the value
// of AGENT_BUS_READER_HOST when the unit sets one, else the machine's own, as
// the operating system reports it
// (docs/05-discovery.md#where-a-pool-member-says-it-is). The daemon refuses a
// value that is not 1 to 64 characters of letters, digits, '.' and '-', and a
// refused read would stop the reader, so one that would be refused is not
// sent: the second answer says why, and the read goes without a host.
func ReaderHost() (host, why string) {
	h, err := hostname()
	if o := os.Getenv("AGENT_BUS_READER_HOST"); o != "" {
		h, err = o, nil
	}
	if err != nil || h == "" {
		return "", ""
	}
	if err := core.CheckHost(h); err != nil {
		return "", "the hostname " + h + " is not a valid host name; reads state no host"
	}
	return h, ""
}

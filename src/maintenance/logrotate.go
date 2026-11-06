// Package maintenance carries the node's tracked maintenance files that a
// program installs, so the program and the file on the shelf are one value.
package maintenance

import _ "embed"

// Logrotate is the rule for the daemon's logs, src/maintenance/agent-bus.logrotate:
// agent-bus-setup writes exactly this to /etc/logrotate.d/agent-bus.
//
//go:embed agent-bus.logrotate
var Logrotate string

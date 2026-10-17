// Package dashboard supplies the daemon's default browser redirect address.
// The web face owns its listener address; -dashboard or AGENT_BUS_DASHBOARD
// overrides the daemon's redirect independently of that listener.
// See docs/05-discovery.md#where-it-listens.
package dashboard

// The default web address is loopback HTTP. Configure the redirect override
// when the web face listens elsewhere or serves HTTPS.
const (
	Addr = "127.0.0.1:6780"
	URL  = "http://" + Addr + "/"
)

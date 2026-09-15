// Package dashboard owns where the dashboard is reached, because two programs
// need the same answer: the dashboard, which binds it, and the daemon, which
// sends a browser there from an API root that has no page of its own. A second
// copy of the address is a copy that drifts.
// See docs/05-discovery.md#where-it-listens.
package dashboard

// Loopback, and plain HTTP unless somebody supplies a certificate. The bus
// does not listen off this machine (docs/09-setup.md#storage), so the
// dashboard is a page for the person at it, reached by the address it binds
// rather than by a name anybody has to make resolve.
const (
	Addr = "127.0.0.1:6780"
	URL  = "http://" + Addr + "/"
)

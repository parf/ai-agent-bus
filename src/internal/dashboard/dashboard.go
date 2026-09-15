// Package dashboard owns where the dashboard is meant to be reached, because
// two programs need the same answer: the dashboard, which binds it, and the
// daemon, which sends a browser there from an API root that has no page of
// its own. A second copy of the hostname is a copy that drifts.
// See docs/05-discovery.md#where-it-listens.
package dashboard

// `*.localhost.direct` resolves to 127.0.0.1 in public DNS, so a browser gets
// a real hostname and a real certificate without an /etc/hosts line and
// without a warning — and nothing leaves the machine.
const (
	Host      = "agent-bus.localhost.direct"
	HTTPSPort = "443"
	AltPort   = "8443" // where it lands when 443 is not ours to bind

	// URL is the address to hand a person, which is the certificate's name
	// on its default port. A dashboard told to listen elsewhere is told
	// about elsewhere: the daemon takes this as a default, not as a fact.
	URL = "https://" + Host + "/"
)

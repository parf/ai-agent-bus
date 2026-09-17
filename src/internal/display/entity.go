// Package display owns human-facing labels that are deliberately not protocol
// values. JSON, URLs, CLI arguments and editable access expressions keep the
// plain machine vocabulary from internal/protocol.
package display

// Entity labels an identity from a kind the daemon stated. Other stated kinds
// pass through unchanged; callers without a kind fact must not invent one.
func Entity(kind string) string {
	switch kind {
	case "user", "person":
		return "👤 User"
	case "agent":
		return "🤖 Agent"
	case "generic":
		return "⚙️ Service"
	default:
		return kind
	}
}

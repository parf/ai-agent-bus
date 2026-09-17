// Package display owns human-facing labels that are deliberately not protocol
// values. JSON, URLs, CLI arguments and editable access expressions keep the
// plain machine vocabulary from internal/protocol.
package display

const GroupGlyph = "👥"

// EntityGlyph returns the glyph for a daemon-stated entity kind. Unknown and
// absent kinds stay unmarked; callers must not infer a kind from the name.
func EntityGlyph(kind string) string {
	switch kind {
	case "user", "person":
		return "👤"
	case "agent":
		return "🤖"
	case "generic":
		return "⚙️"
	default:
		return ""
	}
}

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

// Package display owns human-facing labels that are deliberately not protocol
// values. JSON, URLs, CLI arguments and editable access expressions keep the
// plain machine vocabulary from internal/protocol.
package display

const (
	GroupGlyph = "👥"
	// DaemonOwnerGlyph and MaintainerGlyph are the two authorities that carry
	// one. Every other role stays a word: the node has exactly one daemon
	// owner and a record names its maintainers once, so neither mark can
	// spread far enough to become a column heading — which is the whole
	// reason the default is no glyph (Plans/MVP/web/glyphs.md#the-rule-that-matters-most).
	DaemonOwnerGlyph = "🔱"
	MaintainerGlyph  = "👮"
)

// Authority labels what a person is on this node. Only the daemon owner is
// marked; a daemon administrator and an ordinary user are words, because the
// mark is for the one authority that cannot be delegated.
func Authority(daemonOwner, administrator bool) string {
	switch {
	case daemonOwner:
		return DaemonOwnerGlyph + " Daemon owner"
	case administrator:
		return "Daemon administrator"
	default:
		return "User"
	}
}

// EntityGlyph returns the glyph for a daemon-stated entity kind. Unknown and
// absent kinds stay unmarked; callers must not infer a kind from the name.
func EntityGlyph(kind string) string {
	switch kind {
	case "user", "person":
		return "👤"
	case "agent":
		return "👾"
	case "queue":
		return "📮"
	case "pubsub":
		return "📣"
	case "service":
		return "📡"
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
		return "👾 Agent"
	case "queue":
		return "📮 Queue"
	case "pubsub":
		return "📣 PubSub"
	case "service":
		return "📡 Service"
	default:
		return kind
	}
}

// Identity labels a row whose subject is a person or a record, preferring the
// one authority that outranks an entity type. A node has exactly one daemon
// owner, so the mark cannot spread far enough to become a column heading, which
// is the rule the quiet default exists for
// (Plans/MVP/web/glyphs.md#the-rule-that-matters-most). The cell still carries
// one glyph and one word.
func Identity(kind string, daemonOwner bool) string {
	if daemonOwner {
		return DaemonOwnerGlyph + " Daemon owner"
	}
	return Entity(kind)
}

// IdentityGlyph is Identity for a compact row, where the word moves into the
// accessible label. An unmarked kind stays unmarked whoever owns the node.
func IdentityGlyph(kind string, daemonOwner bool) string {
	if daemonOwner {
		return DaemonOwnerGlyph
	}
	return EntityGlyph(kind)
}

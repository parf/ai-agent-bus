package display

import "testing"

func TestEntityUsesDisplayLabelsOnlyForStatedKinds(t *testing.T) {
	for kind, want := range map[string]string{
		"user": "👤 User", "person": "👤 User", "agent": "👾 Agent",
		"queue": "📮 Queue", "pubsub": "📣 PubSub", "service": "📡 Service",
		// Retired spellings are not kinds any more, so they pass through as
		// stated rather than being labelled as something they no longer are.
		"generic": "generic", "topic": "topic", "": "", "mystery": "mystery",
	} {
		if got := Entity(kind); got != want {
			t.Errorf("Entity(%q) = %q, want %q", kind, got, want)
		}
	}
}

func TestEntityGlyphUsesOnlyStatedKinds(t *testing.T) {
	for kind, want := range map[string]string{
		"user": "👤", "person": "👤", "agent": "👾",
		"queue": "📮", "pubsub": "📣", "service": "📡",
		"generic": "", "topic": "", "": "", "mystery": "",
	} {
		if got := EntityGlyph(kind); got != want {
			t.Errorf("EntityGlyph(%q) = %q, want %q", kind, got, want)
		}
	}
	if GroupGlyph != "👥" {
		t.Errorf("GroupGlyph = %q", GroupGlyph)
	}
}

// Two authorities carry a glyph and the rest stay words. The daemon owner is
// the one that cannot be delegated, so it is the one that is marked; a daemon
// administrator is deliberately not, or the mark would be on most of the
// people who can act.
func TestOnlyTheDaemonOwnerCarriesAnAuthorityGlyph(t *testing.T) {
	for _, c := range []struct {
		owner, admin bool
		want         string
	}{
		{true, false, "🔱 Daemon owner"},
		{true, true, "🔱 Daemon owner"},
		{false, true, "Daemon administrator"},
		{false, false, "User"},
	} {
		if got := Authority(c.owner, c.admin); got != c.want {
			t.Errorf("Authority(%v, %v) = %q, want %q", c.owner, c.admin, got, c.want)
		}
	}
	if MaintainerGlyph != "👮" || DaemonOwnerGlyph != "🔱" {
		t.Errorf("the authority glyphs moved: %q and %q", DaemonOwnerGlyph, MaintainerGlyph)
	}
}

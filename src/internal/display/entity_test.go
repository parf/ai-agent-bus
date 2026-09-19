package display

import "testing"

func TestEntityUsesDisplayLabelsOnlyForStatedKinds(t *testing.T) {
	for kind, want := range map[string]string{
		"user": "👤 User", "person": "👤 User", "agent": "📥 Inbox",
		"generic": "⚙️ Service", "topic": "topic", "": "", "mystery": "mystery",
	} {
		if got := Entity(kind); got != want {
			t.Errorf("Entity(%q) = %q, want %q", kind, got, want)
		}
	}
}

func TestEntityGlyphUsesOnlyStatedKinds(t *testing.T) {
	for kind, want := range map[string]string{
		"user": "👤", "person": "👤", "agent": "📥", "generic": "⚙️",
		"topic": "", "": "", "mystery": "",
	} {
		if got := EntityGlyph(kind); got != want {
			t.Errorf("EntityGlyph(%q) = %q, want %q", kind, got, want)
		}
	}
	if GroupGlyph != "👥" {
		t.Errorf("GroupGlyph = %q", GroupGlyph)
	}
}

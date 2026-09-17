package display

import "testing"

func TestEntityUsesDisplayLabelsOnlyForStatedKinds(t *testing.T) {
	for kind, want := range map[string]string{
		"user": "👤 User", "person": "👤 User", "agent": "🤖 Agent",
		"generic": "⚙️ Service", "topic": "topic", "": "", "mystery": "mystery",
	} {
		if got := Entity(kind); got != want {
			t.Errorf("Entity(%q) = %q, want %q", kind, got, want)
		}
	}
}

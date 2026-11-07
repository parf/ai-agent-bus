package agentmeta

import (
	"strings"
	"testing"
)

func TestReleasedIsADateOrAbsent(t *testing.T) {
	for _, value := range []string{"2026-11-07", "2028-02-29"} {
		m, err := Decode([]byte(`{"description":"Fixture","version":"1.0.0","released":"` + value + `"}`))
		if err != nil || m.Released != value {
			t.Fatalf("released %q: %v, got %q", value, err, m.Released)
		}
	}
	if m, err := Decode([]byte(`{"description":"Fixture"}`)); err != nil || m.Released != "" {
		t.Fatalf("an absent released is fine at this layer: %v, %q", err, m.Released)
	}
	for _, value := range []string{"yesterday", "2026-13-01", "2026-02-30", "2027-02-29", "2026-1-7", "2026-11-07T00:00:00Z", "20261107"} {
		_, err := Decode([]byte(`{"description":"Fixture","released":"` + value + `"}`))
		if err == nil || !strings.Contains(err.Error(), "released must be a YYYY-MM-DD date") {
			t.Fatalf("released %q must be refused by name, got %v", value, err)
		}
	}
}

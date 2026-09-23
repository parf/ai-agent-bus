package display

import (
	"fmt"
	"time"
)

// Ago says how long before now t was, coarse on purpose: a listing is read to
// tell a session in use from one left behind, not to audit it. The MCP face's
// ago() in mcp/catalogue.ts says the same words.
func Ago(t, now time.Time) string {
	s := int(now.Sub(t).Round(time.Second) / time.Second)
	if s < 60 {
		return "just now"
	}
	m := (s + 30) / 60
	if m < 60 {
		return fmt.Sprintf("%dm ago", m)
	}
	h := (m + 30) / 60
	if h < 48 {
		return fmt.Sprintf("%dh ago", h)
	}
	return fmt.Sprintf("%dd ago", (h+12)/24)
}

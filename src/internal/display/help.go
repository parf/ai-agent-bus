package display

import (
	"strings"
	"unicode/utf8"
)

// ManText uses one description column for the page, capped at column 34.
// A longer label stands above its description. Prose and synopses stay intact.
func ManText(text string) string {
	lines := strings.Split(text, "\n")
	type row struct {
		index       int
		label, desc string
	}
	var rows []row
	width := 0
	for i, line := range lines {
		if !strings.HasPrefix(line, "  ") {
			continue
		}
		line = strings.TrimSpace(line)
		label, desc, ok := strings.Cut(line, " — ")
		if !ok && (strings.HasPrefix(line, "--") || strings.HasPrefix(line, "-N ")) {
			label, desc, ok = strings.Cut(line, "  ")
		}
		if !ok {
			continue
		}
		if strings.HasPrefix(label, "--") {
			if key, value, found := strings.Cut(label, "="); found && !strings.HasPrefix(value, "<") {
				label = key + "=<" + value + ">"
			}
		}
		width = max(width, utf8.RuneCountInString(label))
		rows = append(rows, row{i, label, strings.TrimSpace(desc)})
	}
	width = min(width, 30)
	for _, r := range rows {
		n := utf8.RuneCountInString(r.label)
		if n > width {
			lines[r.index] = "  " + r.label + "\n" + strings.Repeat(" ", width+4) + r.desc
		} else {
			lines[r.index] = "  " + r.label + strings.Repeat(" ", width-n+2) + r.desc
		}
	}
	return strings.Join(lines, "\n")
}

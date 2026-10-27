package display

import (
	"strings"
	"unicode/utf8"
)

// ManText uses one description column for the page, capped at column 34.
// A longer label keeps its description after one space; rows never wrap.
func ManText(text string) string {
	lines := wrapDescriptions(strings.Split(text, "\n"))
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
		if !ok && (strings.HasPrefix(line, "-")) {
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
			lines[r.index] = "  " + r.label + " " + r.desc
		} else {
			lines[r.index] = "  " + r.label + strings.Repeat(" ", width-n+2) + r.desc
		}
	}
	return strings.Join(lines, "\n")
}

// Only DESCRIPTION is prose: command and option rows keep their no-wrap rule.
func wrapDescriptions(input []string) []string {
	var out []string
	for i := 0; i < len(input); {
		out = append(out, input[i])
		if input[i] != "DESCRIPTION" {
			i++
			continue
		}
		i++
		var words []string
		flush := func() {
			if len(words) == 0 {
				return
			}
			line := "  "
			for _, word := range words {
				if line != "  " && utf8.RuneCountInString(line)+1+utf8.RuneCountInString(word) > 80 {
					out = append(out, line)
					line = "  "
				}
				if line != "  " {
					line += " "
				}
				line += word
			}
			out = append(out, line)
			words = nil
		}
		for i < len(input) && (strings.TrimSpace(input[i]) == "" || strings.HasPrefix(input[i], "  ")) {
			if strings.TrimSpace(input[i]) == "" {
				flush()
				out = append(out, "")
			} else {
				words = append(words, strings.Fields(input[i])...)
			}
			i++
		}
		flush()
	}
	return out
}

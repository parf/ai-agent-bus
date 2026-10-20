package main

import (
	"strings"
	"unicode"
)

// Agent-provided human output keeps line breaks and table tabs, not terminal controls.
func humanText(s string) string {
	return strings.Map(func(r rune) rune {
		if r != '\n' && r != '\t' && unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

// Reuse the CLI's own usage rather than maintaining a second flag list.
func verbUsage(verb string) string {
	if verb == "enroll" {
		verb = "enrol"
	}
	var out []string
	keep := false
	for _, line := range strings.Split(usageText, "\n") {
		if strings.HasPrefix(line, "  agent-bus ") {
			command := strings.TrimPrefix(line, "  agent-bus ")
			keep = command == verb || strings.HasPrefix(command, verb+" ")
		}
		if keep {
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

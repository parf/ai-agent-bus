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

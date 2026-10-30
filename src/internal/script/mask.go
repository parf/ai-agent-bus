package script

import (
	"sort"
	"strings"
)

// minMasked is the shortest value masked in a log: a shorter one would mangle
// ordinary text, so a key holding one is warned about instead. It is a floor
// on purpose: a short common line of a longer value (a PEM header) is masked
// too, and over-masking is the safe side, so nobody should raise it.
const minMasked = 4

// MaskValues is every text a log must hide for these `KEY=value` secrets: each
// value, and each line of a value of several lines (a key is masked by its
// lines, since a log may break, prefix or escape them), of at least four
// bytes, once each, the longest first so a value holding another is hidden
// whole and not left half showing.
func MaskValues(env []string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(v string) {
		if len(v) >= minMasked && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	for _, kv := range env {
		_, value, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		add(value)
		for _, line := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
			add(line)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

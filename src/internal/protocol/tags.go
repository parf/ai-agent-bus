package protocol

import (
	"slices"
	"strings"
)

// NormalizeTags produces stable lowercase labels. no-tags is synthetic only.
func NormalizeTags(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, raw := range in {
		for _, word := range strings.Split(strings.ToLower(raw), ",") {
			word = strings.TrimSpace(word)
			if word == "no-tags" {
				continue
			}
			word = strings.Map(func(r rune) rune {
				if r == '\t' || r == '\r' || r == '\n' || r == '\f' || r == '\v' {
					return ' '
				}
				if r >= 'a' && r <= 'z' || r == '_' || r == ' ' {
					return r
				}
				return -1
			}, word)
			word = strings.Join(strings.Fields(word), " ")
			if word != "" && !seen[word] {
				seen[word] = true
				out = append(out, word)
			}
		}
	}
	slices.Sort(out)
	return out
}
func HasTag(tags []string, tag string) bool { return slices.Contains(tags, tag) }

// SyncTags mirrors the legacy Personal field after the caller authorizes it.
func (r *Record) SyncTags() {
	r.Tags = NormalizeTags(r.Tags)
	if r.Personal {
		if !HasTag(r.Tags, "personal") {
			r.Tags = append(r.Tags, "personal")
			slices.Sort(r.Tags)
		}
	} else if HasTag(r.Tags, "personal") {
		out := make([]string, 0, len(r.Tags)-1)
		for _, t := range r.Tags {
			if t != "personal" {
				out = append(out, t)
			}
		}
		r.Tags = out
	}
}

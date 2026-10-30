package protocol

import (
	"fmt"
	"slices"
	"strings"
)

// NormalizeTags lowercases, deduplicates and sorts labels, refusing bad input.
// Commas separate labels; no-tags is reserved for the empty-tag filter, never a record label.
func NormalizeTags(in []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, raw := range in {
		for _, word := range strings.Split(strings.ToLower(raw), ",") {
			if word == "" || word == "no-tags" {
				continue
			}
			for _, r := range word {
				if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
					return nil, fmt.Errorf("invalid tag %q: use a-z, 0-9, - or _", word)
				}
			}
			if !seen[word] {
				seen[word] = true
				out = append(out, word)
			}
		}
	}
	slices.Sort(out)
	return out, nil
}
func HasTag(tags []string, tag string) bool { return slices.Contains(tags, tag) }

// TagFilter reads a listing's comma-delimited tag query. It selects the
// records carrying every label listed; "no-tags" alone selects the records
// with none, and combined with a real label it is refused, since a record has
// no tags or has that one.
func TagFilter(raw string) (labels []string, none bool, err error) {
	for _, word := range strings.Split(strings.ToLower(raw), ",") {
		if word == "no-tags" {
			none = true
		}
	}
	labels, err = NormalizeTags([]string{raw})
	if err != nil {
		return nil, false, err
	}
	if none && len(labels) > 0 {
		return nil, false, fmt.Errorf("no-tags cannot be combined with another tag")
	}
	return labels, none, nil
}

// HasEveryTag reports whether tags holds every label.
func HasEveryTag(tags, labels []string) bool {
	for _, label := range labels {
		if !HasTag(tags, label) {
			return false
		}
	}
	return true
}

// SyncTags mirrors the legacy Personal field after the caller validates tags.
func (r *Record) SyncTags() {
	r.Tags, _ = NormalizeTags(r.Tags)
	if r.Personal {
		if !HasTag(r.Tags, "personal") {
			r.Tags = append(r.Tags, "personal")
			slices.Sort(r.Tags)
		}
	} else if HasTag(r.Tags, "personal") {
		r.Tags = slices.DeleteFunc(r.Tags, func(t string) bool { return t == "personal" })
	}
}

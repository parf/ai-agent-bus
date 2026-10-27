package core

import (
	"fmt"
	"sort"
	"strings"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

type RecordSelection struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
}
type ImportChoices struct {
	KeepExisting  []string          `json:"keep_existing"`
	Replace       []string          `json:"replace"`
	ReplaceUsers  []string          `json:"replace_users"`
	ReplaceTokens []string          `json:"replace_tokens"`
	RenameRealm   map[string]string `json:"rename_realm"`
	DryRun        bool              `json:"dry_run"`
}

// Selectors are resolved before exclusion/changes. An ambiguous bare label is
// never guessed to be a name or tag; explicit prefixes remove the ambiguity.
func resolveSelectors(rules []string, records map[string]protocol.Record, all bool) (map[string]bool, error) {
	out := map[string]bool{}
	for _, rule := range rules {
		for _, raw := range strings.Split(rule, ",") {
			if raw == "all" && all {
				for name := range records {
					out[name] = true
				}
				continue
			}
			mode, value, explicit := strings.Cut(raw, ":")
			if !explicit {
				value = raw
				mode = ""
			} else if mode != "name" && mode != "tag" {
				return nil, fmt.Errorf("unknown selector prefix; use name: or tag:")
			}
			if value == "" {
				return nil, fmt.Errorf("an empty selector is not allowed")
			}
			name := value
			if n, e := canon(value); e == nil {
				name = n
			}
			_, isName := records[name]
			tag := strings.ToLower(value)
			tagValid := tag == "no-tags"
			if tags, e := protocol.NormalizeTags([]string{tag}); e == nil && len(tags) == 1 && tags[0] == tag {
				tagValid = true
			}
			var tagged []string
			if tagValid {
				for n, r := range records {
					if protocol.HasTag(r.Tags, tag) || tag == "no-tags" && len(r.Tags) == 0 {
						tagged = append(tagged, n)
					}
				}
			}
			if mode == "" && isName && len(tagged) > 0 {
				return nil, fmt.Errorf("selector %q is both a name and a tag; use name: or tag:", raw)
			}
			matched := false
			if mode != "tag" && isName {
				out[name] = true
				matched = true
			}
			if mode != "name" {
				for _, n := range tagged {
					out[n] = true
					matched = true
				}
			}
			if !matched {
				return nil, fmt.Errorf("selector %q matches nothing", raw)
			}
		}
	}
	return out, nil
}
func sortedNames(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for n, yes := range m {
		if yes {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

func rewriteName(name string, realms map[string]string) (string, error) {
	if name == "" || name == "*" || name == OwnerGroup || name == AgentTerm {
		return name, nil
	}
	prefix := ""
	if strings.HasPrefix(name, "@") {
		prefix = "@"
		name = name[1:]
		if owner, local, ok := strings.Cut(name, "/"); ok {
			a, e := rewriteName(owner, realms)
			if e != nil {
				return "", e
			}
			b, e := rewriteName(local, realms)
			return "@" + a + "/" + b, e
		}
	}
	n, e := protocol.ParseName(name)
	if e != nil {
		return "", fmt.Errorf("invalid typed identity in backup")
	}
	if realm, ok := realms[n.Realm]; ok {
		n.Realm = realm
	}
	next := prefix + n.String()
	if _, e = canon(next); e != nil {
		return "", fmt.Errorf("realm rewrite produces an invalid name")
	}
	return next, nil
}
func rewriteTerm(term string, realms map[string]string) (string, error) {
	a, roles, e := protocol.ParseACLTerm(term)
	if e != nil {
		return "", fmt.Errorf("invalid ACL in backup")
	}
	a, e = rewriteName(a, realms)
	if len(roles) > 0 {
		a += "(" + strings.Join(roles, ",") + ")"
	}
	return a, e
}

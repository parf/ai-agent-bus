package protocol

import (
	"fmt"
	"strings"
)

// ValidRole is shared by the daemon and runner. The AB_ROLE_ prefix makes a
// digit-leading role a valid shell variable too. Roles have no vocabulary or
// length limit of their own; request size is bounded at the HTTP edge.
func ValidRole(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}

type RoleGrant struct {
	Actor string
	Roles []string
}

// ParsedACL is derived immutable state, never wire or stored data. Nil on a
// plain ACL, so existing access checks need no parsing or allocation.
type ParsedACL struct {
	Actors []string
	Grants []RoleGrant
}

// ParseACLTerm parses one term, not a whole list. Actor canonicalization and
// record-specific authority remain the daemon's responsibility.
func ParseACLTerm(raw string) (actor string, roles []string, err error) {
	s := strings.TrimSpace(raw)
	i := strings.IndexByte(s, '(')
	if i < 0 {
		if strings.ContainsRune(s, ')') {
			return "", nil, fmt.Errorf("unmatched role parentheses in %q", raw)
		}
		return s, nil, nil
	}
	if !strings.HasSuffix(s, ")") || strings.ContainsAny(s[i+1:len(s)-1], "()") {
		return "", nil, fmt.Errorf("invalid role parentheses in %q", raw)
	}
	actor = strings.TrimSpace(s[:i])
	if actor == "" {
		return "", nil, fmt.Errorf("a role needs an actor")
	}
	seen := map[string]bool{}
	for _, part := range strings.Split(s[i+1:len(s)-1], ",") {
		role := strings.TrimSpace(part)
		if !ValidRole(role) {
			return "", nil, fmt.Errorf("a role uses lowercase letters, digits and _: %q", role)
		}
		if role == "owner" || role == "maintainer" {
			return "", nil, fmt.Errorf("%s is generated, not assignable", role)
		}
		if !seen[role] {
			roles = append(roles, role)
			seen[role] = true
		}
	}
	return actor, roles, nil
}

func CompileACL(allow []string) (*ParsedACL, error) {
	var out *ParsedACL
	for i, raw := range allow {
		actor, roles, err := ParseACLTerm(raw)
		if err != nil {
			return nil, err
		}
		if len(roles) != 0 && out == nil {
			out = &ParsedACL{Actors: make([]string, len(allow))}
			copy(out.Actors, allow[:i])
		}
		if out != nil {
			out.Actors[i] = actor
			if len(roles) != 0 {
				out.Grants = append(out.Grants, RoleGrant{Actor: actor, Roles: roles})
			}
		}
	}
	return out, nil
}

func (r Record) ACLActors() []string {
	if r.ParsedAllow != nil {
		return r.ParsedAllow.Actors
	}
	return r.Allow
}

// SplitACLText preserves separators inside parentheses, including malformed
// terms, so the daemon can reject the whole save instead of a face repairing it.
func SplitACLText(s string) []string {
	out := []string{}
	start, depth := 0, 0
	for i, c := range s {
		switch c {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && (c == ',' || c == ' ' || c == '\t' || c == '\n' || c == '\r') {
				if term := strings.TrimSpace(s[start:i]); term != "" {
					out = append(out, term)
				}
				start = i + 1
			}
		}
	}
	if term := strings.TrimSpace(s[start:]); term != "" {
		out = append(out, term)
	}
	return out
}

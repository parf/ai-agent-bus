package core

import (
	"strings"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// assignedRoles collects all matching grants, never on the authorization
// walk: may() can return early for management or the first matching grant.
// Caller holds b.mu; plain ACLs return without allocation or graph work.
func (b *Bus) assignedRoles(from string, rec protocol.Record, roles []string) []string {
	if rec.ParsedAllow == nil {
		return roles
	}
	grants := rec.ParsedAllow.Grants
	groups := b.roleGroups(from, grants)
	var seen map[string]bool
	for _, g := range grants {
		if !(g.Actor == "*" || g.Actor == from || b.runtimeTerm(from, g.Actor, rec) || groups[g.Actor]) {
			continue
		}
		for _, role := range g.Roles {
			if seen != nil {
				if seen[role] {
					continue
				}
			} else {
				duplicate := false
				for _, held := range roles {
					if held == role {
						duplicate = true
						break
					}
				}
				if duplicate {
					continue
				}
				// At most eight cheap comparisons for small unions; beyond
				// that a set bounds total deduplication work linearly.
				if len(roles) >= 8 {
					seen = make(map[string]bool, len(roles)+len(g.Roles))
					for _, held := range roles {
						seen[held] = true
					}
				}
			}
			roles = append(roles, role)
			if seen != nil {
				seen[role] = true
			}
		}
	}
	return roles
}

// roleGroups explores the union of reachable groups once, then propagates
// matches backwards. Unlike caching false from an interrupted recursive walk,
// this is correct even with cycles and overlapping roots. O(V+E) per send;
// nothing survives edits. Runtime @owner/@agent are not stored groups.
func (b *Bus) roleGroups(from string, grants []protocol.RoleGrant) map[string]bool {
	var todo []string
	for _, g := range grants {
		if groupName(g.Actor) && !reservedTerm(g.Actor) {
			todo = append(todo, g.Actor)
		}
	}
	if len(todo) == 0 {
		return nil
	}
	visited := map[string]bool{}
	parents := map[string][]string{}
	matches := map[string]bool{}
	var seeds []string
	for len(todo) > 0 {
		group := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if visited[group] {
			continue
		}
		visited[group] = true
		members, _ := b.groupMembers(group)
		for _, member := range members {
			if member == from && !matches[group] {
				matches[group] = true
				seeds = append(seeds, group)
			}
			if group != AdministratorsGroup && groupName(member) {
				parents[member] = append(parents[member], group)
				todo = append(todo, member)
			}
		}
	}
	for len(seeds) > 0 {
		group := seeds[len(seeds)-1]
		seeds = seeds[:len(seeds)-1]
		for _, parent := range parents[group] {
			if !matches[parent] {
				matches[parent] = true
				seeds = append(seeds, parent)
			}
		}
	}
	return matches
}

// Writes store canonical actor(role,role); trim also handles restored text.
// dropACL removes every decorated or plain grant to the old name, rather than
// accidentally handing its roles to a future record that reuses the name.
func dropACL(list []string, name string) []string {
	out := list[:0:0]
	for _, term := range list {
		actor, _, _ := strings.Cut(term, "(")
		if strings.TrimSpace(actor) != name {
			out = append(out, term)
		}
	}
	return out
}

// Removing a bare actor revokes every grant to it. A decorated removal names
// one role set, irrespective of the order used to spell that set.
func removeACLGrant(list []string, term string) []string {
	actor, roles, _ := protocol.ParseACLTerm(term)
	if len(roles) == 0 {
		return dropACL(list, actor)
	}
	want := make(map[string]bool, len(roles))
	for _, role := range roles {
		want[role] = true
	}
	out := list[:0:0]
	for _, stored := range list {
		base, _, _ := strings.Cut(stored, "(")
		match := strings.TrimSpace(base) == actor
		if match {
			_, held, err := protocol.ParseACLTerm(stored)
			match = err == nil && len(held) == len(want)
			for _, role := range held {
				if !want[role] {
					match = false
					break
				}
			}
		}
		if !match {
			out = append(out, stored)
		}
	}
	return out
}

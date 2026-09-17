package core

import (
	"fmt"
	"maps"
	"slices"

	"github.com/parf/ai-agent-bus/internal/ports"
)

const legacyAdministratorsGroup = "@maintainers"

// migrateAdministrators renames the old administrative group and its explicit
// record grants. An existing ordinary group at the new name must not acquire
// administrative authority. Preserve it under an unused name instead. The old
// name cannot be created through SetGroup, so a migrated snapshot stays migrated.
func migrateAdministrators(s ports.Snapshot) ports.Snapshot {
	admins, legacy := s.Groups[legacyAdministratorsGroup]
	if !legacy {
		return s
	}
	s.Groups = maps.Clone(s.Groups)
	renames := map[string]string{legacyAdministratorsGroup: AdministratorsGroup}
	if members, collision := s.Groups[AdministratorsGroup]; collision {
		// Avoid existing groups and every dangling reference: creating a group
		// under an unresolved ACL or nested-group term would grant new access.
		used := map[string]bool{}
		for name, groupMembers := range s.Groups {
			used[name] = true
			for _, member := range groupMembers {
				if groupName(member) {
					used[member] = true
				}
			}
		}
		for _, r := range s.Records {
			for _, name := range r.Maintainers {
				used[name] = true
			}
			for _, name := range r.Allow {
				used[name] = true
			}
		}
		name := AdministratorsGroup + "-legacy"
		for suffix := 1; used[name]; suffix++ {
			name = fmt.Sprintf("%s-legacy-%d", AdministratorsGroup, suffix)
		}
		s.Groups[name] = members
		renames[AdministratorsGroup] = name
	}
	s.Groups[AdministratorsGroup] = admins
	delete(s.Groups, legacyAdministratorsGroup)
	// Group members may themselves be group names. Rewrite those edges in the
	// same direction as record grants so migration cannot leave a nested grant
	// pointing at the retired or colliding name.
	for group, members := range s.Groups {
		members = slices.Clone(members)
		for i, member := range members {
			if renamed, ok := renames[member]; ok {
				members[i] = renamed
			}
		}
		s.Groups[group] = members
	}
	s.Records = slices.Clone(s.Records)
	for i := range s.Records {
		r := &s.Records[i]
		r.Maintainers = slices.Clone(r.Maintainers)
		for j, name := range r.Maintainers {
			if renamed, ok := renames[name]; ok {
				r.Maintainers[j] = renamed
			}
		}
		r.Allow = slices.Clone(r.Allow)
		for j, name := range r.Allow {
			if renamed, ok := renames[name]; ok {
				r.Allow[j] = renamed
			}
		}
	}
	return s
}

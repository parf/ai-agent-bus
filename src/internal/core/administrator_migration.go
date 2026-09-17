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
		// Avoid both existing groups and dangling record references: creating a
		// group under a previously unresolved ACL term would grant new access.
		used := map[string]bool{}
		for name := range s.Groups {
			used[name] = true
		}
		for _, r := range s.Records {
			used[r.Maintainers] = true
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
	s.Records = slices.Clone(s.Records)
	for i := range s.Records {
		r := &s.Records[i]
		if renamed, ok := renames[r.Maintainers]; ok {
			r.Maintainers = renamed
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

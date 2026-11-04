package core

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// ValidateSnapshot refuses unsupported durable state before it changes a bus.
// Only an empty store may establish its initial owner and account map.
func ValidateSnapshot(s ports.Snapshot) error {
	if s.Empty() {
		return nil
	}
	if !s.OwnerEstablished {
		return fmt.Errorf("snapshot owner establishment marker is missing")
	}
	if !s.AccountsEstablished {
		return fmt.Errorf("snapshot account establishment marker is missing")
	}
	owner, err := canon(s.Owner)
	if err != nil || owner != s.Owner {
		return fmt.Errorf("snapshot daemon owner is invalid: %q", s.Owner)
	}
	check := New()
	check.admin = owner
	userIDs, recordIDs := map[uint32]bool{}, map[uint32]bool{}
	identities := map[string]string{}
	for _, u := range s.Users {
		n, err := canon(u.Name)
		if err != nil || n != u.Name || (u.ID == 0 || u.ID >= s.NextUserID) {
			return fmt.Errorf("snapshot user %s has invalid name or missing ID", u.Name)
		}
		if _, duplicate := check.users[u.Name]; duplicate || userIDs[u.ID] {
			return fmt.Errorf("snapshot repeats user name or ID: %s", u.Name)
		}
		for _, key := range []string{"email:" + u.Email, "github:" + u.GithubUser, "twitter:" + strings.ToLower(u.GithubTwitterUsername)} {
			if strings.HasSuffix(key, ":") {
				continue
			}
			if other, ok := identities[key]; ok {
				return fmt.Errorf("snapshot users %s and %s share an identity", other, u.Name)
			}
			identities[key] = u.Name
		}
		check.users[u.Name] = u
		userIDs[u.ID] = true
	}
	if u, ok := check.users[owner]; !ok || u.Status != "active" {
		return fmt.Errorf("snapshot daemon owner %s is not an active registered user", owner)
	}
	for _, r := range s.Records {
		n, err := canon(r.Name)
		if err != nil || n != r.Name || r.ID == 0 || r.ID >= s.NextRecordID || r.OwnerID == 0 {
			return fmt.Errorf("snapshot record %s has invalid name or missing record/owner ID", r.Name)
		}
		if _, duplicate := check.records[r.Name]; duplicate || recordIDs[r.ID] {
			return fmt.Errorf("snapshot repeats record name or ID: %s", r.Name)
		}
		if err := validateRestored(r); err != nil {
			return fmt.Errorf("snapshot record %s: %w", r.Name, err)
		}
		check.records[r.Name] = r
		recordIDs[r.ID] = true
	}
	for _, r := range s.Records {
		if why := check.incorrect(r); why != "" {
			return fmt.Errorf("snapshot record %s: %s", r.Name, why)
		}
		if r.Name == DaemonAgent {
			if len(r.Config) > 0 {
				if err := validBusConfig(r.Config); err != nil {
					return fmt.Errorf("snapshot bus configuration: %w", err)
				}
			}
			clearLiveRecord(&r)
			expected := daemonAgentRecord(r, owner)
			if !reflect.DeepEqual(r, expected) {
				return fmt.Errorf("snapshot #bus metadata conflicts with daemon-owned identity")
			}
		}
		if r.Name == AdministratorsGroup {
			if r.Owner != owner {
				return fmt.Errorf("snapshot administrators group is not owned by daemon owner")
			}
			for _, m := range r.Allow {
				if _, ok := check.users[m]; !ok {
					return fmt.Errorf("snapshot administrators member %s is not a user", m)
				}
			}
		}
	}
	for _, u := range s.Users {
		if r, ok := check.records[u.Name]; !ok || r.Kind != protocol.KindUser {
			return fmt.Errorf("snapshot user %s has no user record", u.Name)
		}
	}
	accounts := map[string]bool{}
	for _, m := range s.Accounts {
		n, err := canon(m.Principal)
		if m.Account == "" || accounts[m.Account] || err != nil || n != m.Principal {
			return fmt.Errorf("snapshot account mapping %q is invalid", m.Account)
		}
		if _, err := check.pairFor(m.Principal); err != nil {
			return fmt.Errorf("snapshot account %s: %w", m.Account, err)
		}
		accounts[m.Account] = true
	}
	queues := map[string]bool{}
	for _, q := range s.Queues {
		r, known := check.records[q.Name]
		if !known || !onBus(r) || queues[q.Name] {
			return fmt.Errorf("snapshot queue %s has no unique receiving record", q.Name)
		}
		queues[q.Name] = true
	}
	return nil
}

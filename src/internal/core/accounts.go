package core

import (
	"fmt"
	"sort"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// EstablishAccounts applies setup's mappings only to a first or legacy
// snapshot. active is also retained separately: a persisted change cannot be
// called active until the supervisor has restarted and opened those sockets.
func (b *Bus) EstablishAccounts(active map[string]string) error {
	normalized := map[string]string{}
	for account, raw := range active {
		principal, err := canon(raw)
		if account == "" || err != nil {
			return fmt.Errorf("local account mapping %q=%q is invalid", account, raw)
		}
		normalized[account] = principal
	}
	b.mu.Lock()
	defer b.unlock()
	if b.accountRestoreErr != nil {
		return b.accountRestoreErr
	}
	b.activeAccounts = cloneAccounts(normalized)
	if !b.accountsRestored {
		b.setAccounts(cloneAccounts(normalized))
		b.accountsRestored = true
	}
	return b.commit()
}

func cloneAccounts(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for account, principal := range in {
		out[account] = principal
	}
	return out
}

func sameAccounts(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for account, principal := range a {
		if b[account] != principal {
			return false
		}
	}
	return true
}

func (b *Bus) accountView() protocol.AccountMappings {
	// An ignored mapping's socket is open but not served, so it is not a
	// difference a restart would apply.
	active := cloneAccounts(b.activeAccounts)
	for account := range b.ignoredAccounts {
		delete(active, account)
	}
	out := protocol.AccountMappings{RestartRequired: !sameAccounts(b.accounts, active)}
	for account, principal := range b.accounts {
		out.Mappings = append(out.Mappings, protocol.AccountMapping{Account: account, Principal: principal})
	}
	sort.Slice(out.Mappings, func(i, j int) bool { return out.Mappings[i].Account < out.Mappings[j].Account })
	return out
}

// Accounts returns daemon configuration only to daemon administration.
func (b *Bus) Accounts(caller string) (protocol.AccountMappings, error) {
	who, err := canon(caller)
	if err != nil {
		return protocol.AccountMappings{}, err
	}
	b.mu.Lock()
	defer b.unlock()
	if err := b.acting(who); err != nil {
		return protocol.AccountMappings{}, err
	}
	if !b.isAdministrator(who) {
		return protocol.AccountMappings{}, ErrNotOwner
	}
	return b.accountView(), nil
}

// SetAccount changes the durable desired map. The API adapter validates the
// OS account before reaching core; core owns identity and authority checks.
// Listeners remain the supervisor's and apply this state on its next start.
func (b *Bus) SetAccount(caller, account, principal string, remove bool) (protocol.AccountMappings, error) {
	who, err := canon(caller)
	if err != nil {
		return protocol.AccountMappings{}, err
	}
	if account == "" {
		return protocol.AccountMappings{}, fmt.Errorf("%w: local account is empty", ErrBadName)
	}
	var mapped string
	if !remove {
		mapped, err = canon(principal)
		if err != nil {
			return protocol.AccountMappings{}, err
		}
	}
	b.mu.Lock()
	defer b.unlock()
	if err := b.acting(who); err != nil {
		return protocol.AccountMappings{}, err
	}
	if !b.isAdministrator(who) {
		return protocol.AccountMappings{}, ErrNotOwner
	}
	if remove {
		_, ok := b.accounts[account]
		if _, ignored := b.ignoredAccounts[account]; !ok && !ignored {
			return protocol.AccountMappings{}, ErrUnknown
		}
		next := cloneAccounts(b.accounts)
		delete(next, account)
		b.setAccounts(next)
	} else {
		if err := b.acting(mapped); err != nil {
			return protocol.AccountMappings{}, fmt.Errorf("mapped principal: %w", err)
		}
		// Only a User or an Agent speaks, so only one is a socket's principal.
		if !b.actor(mapped) {
			return protocol.AccountMappings{}, fmt.Errorf("%w: a local account maps to a user or an agent, and %s is neither", ErrKind, mapped)
		}
		next := cloneAccounts(b.accounts)
		next[account] = mapped
		b.setAccounts(next)
	}
	if err := b.commit(); err != nil {
		return protocol.AccountMappings{}, err
	}
	// The map just written is the database's whole map, so an ignored row
	// under this account is gone with it.
	delete(b.ignoredAccounts, account)
	return b.accountView(), nil
}

// Unserved says whether a socket for principal belongs to a stored mapping
// this start ignored, and so is not to be served (docs/02-access.md#local-socket).
func (b *Bus) Unserved(principal string) bool {
	b.mu.Lock()
	defer b.unlock()
	for _, p := range b.ignoredAccounts {
		if p == principal {
			return true
		}
	}
	return false
}

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
	defer b.mu.Unlock()
	if b.accountRestoreErr != nil {
		return b.accountRestoreErr
	}
	b.activeAccounts = cloneAccounts(normalized)
	if !b.accountsRestored {
		b.accounts = cloneAccounts(normalized)
		b.accountsRestored = true
	}
	return nil
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
	out := protocol.AccountMappings{RestartRequired: !sameAccounts(b.accounts, b.activeAccounts)}
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
	defer b.mu.Unlock()
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
	defer b.mu.Unlock()
	if err := b.acting(who); err != nil {
		return protocol.AccountMappings{}, err
	}
	if !b.isAdministrator(who) {
		return protocol.AccountMappings{}, ErrNotOwner
	}
	if remove {
		if _, ok := b.accounts[account]; !ok {
			return protocol.AccountMappings{}, ErrUnknown
		}
		delete(b.accounts, account)
	} else {
		if err := b.acting(mapped); err != nil {
			return protocol.AccountMappings{}, fmt.Errorf("mapped principal: %w", err)
		}
		b.accounts[account] = mapped
	}
	if err := b.checkpoint(false); err != nil {
		return protocol.AccountMappings{}, err
	}
	return b.accountView(), nil
}

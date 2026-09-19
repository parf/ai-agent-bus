package core

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Manage changes only explicitly supplied properties, under the ownership lock.
// Unlike registration it cannot erase a concurrently changed configuration.
type Management struct {
	Name        string                   `json:"name"`
	Descr       *string                  `json:"descr,omitempty"`
	Addr        *string                  `json:"addr,omitempty"`
	Proto       *string                  `json:"protocol,omitempty"`
	Allow       *[]string                `json:"allow,omitempty"`
	Disabled    *bool                    `json:"disabled,omitempty"`
	Maintainers *protocol.MaintainerList `json:"maintainers,omitempty"`
	Personal    *bool                    `json:"personal,omitempty"`
	Owner       *string                  `json:"owner,omitempty"`
	TTL         *string                  `json:"ttl,omitempty"`
	Bound       *int                     `json:"bound,omitempty"`
	Full        *string                  `json:"overflow,omitempty"`
}

// SetDaemonOwner establishes an owner in an in-memory or embedded bus. The
// installed daemon uses EstablishDaemonOwner so a startup seed cannot replace
// durable transferred authority.
func (b *Bus) SetDaemonOwner(owner string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ownerRestored = false
	b.ownerRestoreErr = nil
	b.setDaemonOwner(owner)
}

// setDaemonOwner updates the role nesting while caller holds b.mu.
func (b *Bus) setDaemonOwner(owner string) {
	b.admin = owner
	if !b.member(owner, AdministratorsGroup) {
		b.groups[AdministratorsGroup] = append(b.groups[AdministratorsGroup], owner)
		sort.Strings(b.groups[AdministratorsGroup])
	}
	b.administratorsAreUsers()
}

// EstablishDaemonOwner applies setup's owner only to a first or legacy
// snapshot. A current snapshot is authoritative: damage is an error rather
// than an excuse to resurrect the setup seed after a transfer.
func (b *Bus) EstablishDaemonOwner(seed string) error {
	owner, err := canon(seed)
	if err != nil {
		return fmt.Errorf("daemon owner: %w", err)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.recordRestoreErr != nil {
		return b.recordRestoreErr
	}
	if b.ownerRestoreErr != nil {
		return b.ownerRestoreErr
	}
	if b.ownerRestored {
		return nil
	}
	if b.admin == "" {
		b.setDaemonOwner(owner)
	}
	return nil
}

func (b *Bus) DaemonOwner() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.admin
}

// administratorsAreUsers keeps daemon roles nested: an owner is an Administrator
// and an Administrator is a user (docs/01-identity-and-roles.md#groups). Somebody
// given authority over users who was not one themselves would be a principal
// the user administration cannot see, and a credential the ownerless sweep
// would take (docs/02-access.md#ownerless-credentials). Caller holds b.mu.
//
// Being taken out of the group does not take the profile away again: a user is
// never deleted, only made inactive (docs/01-identity-and-roles.md#user-states).
func (b *Bus) administratorsAreUsers() {
	for _, name := range b.groups[AdministratorsGroup] {
		// Administrative authority is direct-only. A damaged snapshot is
		// refused at startup; do not manufacture a user for its group name on
		// the way to that refusal.
		if groupName(name) {
			continue
		}
		if _, known := b.users[name]; !known {
			b.users[name] = protocol.User{Name: name, State: "active"}
		}
	}
}

func (b *Bus) resourceManages(caller string, r protocol.Record) bool {
	return b.acting(caller) == nil && (caller == r.Owner || caller == r.Name || b.maintains(caller, r.Maintainers))
}

func (b *Bus) maintains(caller string, terms protocol.MaintainerList) bool {
	for _, term := range terms {
		if term == caller || b.member(caller, term) {
			return true
		}
	}
	return false
}

// manages includes the daemon Owner's accepted node-wide management override.
// It is deliberately separate from resourceManages: root management does not
// turn an empty ACL into message access.
func (b *Bus) manages(caller string, r protocol.Record) bool {
	return (caller == b.admin && b.acting(caller) == nil) || b.resourceManages(caller, r)
}

// canSee gives management enough discovery to act without widening message
// access. Ordinary callers continue to see exactly what may() permits.
func (b *Bus) canSee(caller string, r protocol.Record) bool {
	return b.manages(caller, r) || b.may(caller, r)
}

// Personal is a grouping tag, not an access mode. Its assignment limits are
// checked when the final record is written: later removal of a named service
// makes that ACL entry inert rather than retroactively changing the tag.
// Caller holds b.mu.
func (b *Bus) validatePersonal(r protocol.Record) error {
	if !r.Personal {
		return nil
	}
	if r.Kind != protocol.KindAgent {
		return fmt.Errorf("%w: only an agent may be personal", ErrPersonal)
	}
	// An agent record may also be a user's own name, and a person is not
	// somebody's personal agent.
	if _, user := b.users[r.Name]; user {
		return fmt.Errorf("%w: a user identity is not a personal agent", ErrPersonal)
	}
	if len(r.Maintainers) != 0 {
		return fmt.Errorf("%w: remove maintainers first", ErrPersonal)
	}
	for _, name := range r.Allow {
		if name == "*" || strings.HasPrefix(name, "@") || name == r.Name {
			return fmt.Errorf("%w: %s is not another agent", ErrPersonal, name)
		}
		other, known := b.records[name]
		_, user := b.users[name]
		if !known || user || other.Kind != protocol.KindAgent {
			return fmt.Errorf("%w: %s is not a registered agent", ErrPersonal, name)
		}
	}
	return nil
}
func (b *Bus) member(caller, group string) bool {
	return b.memberThrough(caller, group, map[string]bool{})
}

// memberThrough resolves ordinary group membership as finite graph
// reachability. Cycles and unknown groups are inert unless another edge reaches
// the caller. The protected Administrator group is always a direct membership
// boundary, including when an ordinary group names it for an access grant.
func (b *Bus) memberThrough(caller, group string, seen map[string]bool) bool {
	if !groupName(group) || seen[group] {
		return false
	}
	seen[group] = true
	for _, member := range b.groups[group] {
		if member == caller {
			return true
		}
		if group != AdministratorsGroup && groupName(member) && b.memberThrough(caller, member, seen) {
			return true
		}
	}
	return false
}
func groupName(n string) bool {
	if len(n) < 2 || len(n) > 64 || n[0] != '@' {
		return false
	}
	for _, c := range n[1:] {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

// normalizeMaintainers validates the whole replacement before Manage stores
// any of it. Named users, services and agents may be direct Maintainers;
// ordinary groups inherit their nested membership. Topics, credential-only
// names, duplicates and wildcard authority are deliberately refused.
// Caller holds b.mu.
func (b *Bus) normalizeMaintainers(in protocol.MaintainerList) (protocol.MaintainerList, error) {
	out := make(protocol.MaintainerList, 0, len(in))
	seen := map[string]bool{}
	for _, raw := range in {
		var term string
		if strings.HasPrefix(strings.TrimSpace(raw), "@") {
			term = strings.TrimSpace(raw)
			if term == OwnerGroup {
				return nil, fmt.Errorf("%w: %s is runtime ACL access, not a Maintainer group", ErrBadName, OwnerGroup)
			}
			if !groupName(term) {
				return nil, fmt.Errorf("%w: invalid maintainers group %q", ErrBadName, raw)
			}
			if _, ok := b.groups[term]; !ok {
				return nil, fmt.Errorf("%w: maintainer %s", ErrUnknown, term)
			}
		} else {
			var err error
			term, err = canon(raw)
			if err != nil {
				return nil, err
			}
			_, user := b.users[term]
			record, registered := b.records[term]
			if !user && !registered {
				return nil, fmt.Errorf("%w: maintainer %s", ErrUnknown, term)
			}
			if !user && record.Kind != protocol.KindAgent {
				return nil, fmt.Errorf("%w: maintainer %s must be a user, agent or group", ErrBadName, term)
			}
		}
		if seen[term] {
			return nil, fmt.Errorf("%w: duplicate maintainer %s", ErrBadName, term)
		}
		seen[term] = true
		out = append(out, term)
	}
	return out, nil
}

// Groups are daemon-local sets of principals and ordinary groups.
// Administrators edit ordinary groups; only the daemon owner changes direct
// administrative membership. Record ownership does not grant daemon
// administration.
func (b *Bus) SetGroup(caller, name string, members []string) error {
	who, err := canon(caller)
	if err != nil {
		return err
	}
	if name == OwnerGroup {
		return fmt.Errorf("%w: %s is a runtime ACL term and cannot be created", ErrBadName, OwnerGroup)
	}
	if !groupName(name) {
		return fmt.Errorf("%w: invalid group", ErrBadName)
	}
	normalized := []string{}
	for _, m := range members {
		m = strings.TrimSpace(m)
		if m == OwnerGroup {
			return fmt.Errorf("%w: %s is direct ACL syntax and cannot be nested in a group", ErrBadName, OwnerGroup)
		}
		n := m
		if !groupName(m) {
			n, err = canon(m)
			if err != nil {
				return err
			}
		}
		normalized = append(normalized, n)
	}
	sort.Strings(normalized)
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.acting(who); err != nil {
		return err
	}
	if !b.isAdministrator(who) {
		return ErrNotOwner
	}
	if name == AdministratorsGroup {
		if who != b.admin {
			return ErrNotOwner
		}
		includesOwner := false
		for _, member := range normalized {
			if groupName(member) {
				return fmt.Errorf("%w: %s accepts direct user identities only", ErrBadName, AdministratorsGroup)
			}
			if member == b.admin {
				includesOwner = true
			}
		}
		if !includesOwner {
			return fmt.Errorf("%w: owner must remain an administrator", ErrNotOwner)
		}
	}
	b.groups[name] = normalized
	if name == AdministratorsGroup {
		b.administratorsAreUsers()
	}
	b.recheckReaders()
	return b.checkpoint(false)
}

func (b *Bus) Groups(caller string) map[string][]string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := map[string][]string{}
	if b.acting(caller) != nil {
		return out
	}
	for group, members := range b.groups {
		// Names are available for assignment. Membership lists are administrative.
		out[group] = []string{}
		if b.isAdministrator(caller) {
			out[group] = append(out[group], members...)
		}
	}
	return out
}

// TransferDaemonOwner moves node authority while preserving the former owner
// as an Administrator. Removing that standing is a separate explicit act by
// the new owner. The target must already be an active registered User.
func (b *Bus) TransferDaemonOwner(caller, next string) (protocol.User, error) {
	who, err := canon(caller)
	if err != nil {
		return protocol.User{}, err
	}
	owner, err := canon(next)
	if err != nil {
		return protocol.User{}, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.acting(who); err != nil {
		return protocol.User{}, err
	}
	if who != b.admin {
		return protocol.User{}, ErrNotOwner
	}
	if owner == b.admin {
		return protocol.User{}, fmt.Errorf("%w: already the daemon owner", ErrBadName)
	}
	_, exists := b.users[owner]
	if !exists {
		return protocol.User{}, fmt.Errorf("%w: daemon owner must be a registered user", ErrBadName)
	}
	if err := b.acting(owner); err != nil {
		return protocol.User{}, err
	}
	b.setDaemonOwner(owner)
	if err := b.checkpoint(false); err != nil {
		return protocol.User{}, err
	}
	return b.userView(owner, owner), nil
}

func (b *Bus) Manage(caller string, change Management) (protocol.Record, error) {
	who, err := canon(caller)
	if err != nil {
		return protocol.Record{}, err
	}
	name, err := canon(change.Name)
	if err != nil {
		return protocol.Record{}, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.acting(who); err != nil {
		return protocol.Record{}, err
	}
	r, ok := b.records[name]
	if !ok {
		return protocol.Record{}, ErrUnknown
	}
	if !b.manages(who, r) {
		return protocol.Record{}, ErrNotOwner
	}
	if (change.Owner != nil || change.Maintainers != nil || change.Personal != nil) && who != r.Owner && who != b.admin {
		return protocol.Record{}, ErrNotOwner
	}
	if change.Owner != nil {
		owner, err := canon(*change.Owner)
		if err != nil {
			return protocol.Record{}, err
		}
		// A transfer cannot manufacture an identity or turn a directory-enrolled
		// person into somebody else's service.
		target, known := b.records[owner]
		if !known || target.Owner != owner {
			return protocol.Record{}, fmt.Errorf("%w: new owner must be a registered self-owned principal", ErrBadName)
		}
		// And one who may act. Handing a record to a paused or banned name
		// leaves it owned by somebody who cannot answer for it, which is the
		// orphan by another route.
		if err := b.acting(owner); err != nil {
			return protocol.Record{}, err
		}
		if r.Name == r.Owner {
			return protocol.Record{}, fmt.Errorf("%w: a self-owned identity cannot be transferred", ErrNotOwner)
		}
		r.Owner = owner
	}
	if change.Maintainers != nil {
		maintainers, err := b.normalizeMaintainers(*change.Maintainers)
		if err != nil {
			return protocol.Record{}, err
		}
		r.Maintainers = maintainers
	}
	if change.Personal != nil {
		r.Personal = *change.Personal
	}
	if change.Allow != nil {
		allow := make([]string, 0, len(*change.Allow))
		for _, a := range *change.Allow {
			if a != "*" {
				if strings.HasPrefix(a, "@") {
					if a != OwnerGroup {
						if _, ok := b.groups[a]; !ok {
							return protocol.Record{}, fmt.Errorf("%w: ACL group", ErrUnknown)
						}
					}
				} else {
					var err error
					a, err = canon(a)
					if err != nil {
						return protocol.Record{}, err
					}
				}
			}
			allow = append(allow, a)
		}
		r.Allow = allow
	}
	if change.Descr != nil {
		r.Descr = *change.Descr
	}
	if change.Addr != nil {
		r.Addr = *change.Addr
	}
	if change.Proto != nil {
		r.Proto = *change.Proto
	}
	if change.Disabled != nil {
		r.Disabled = *change.Disabled
	}
	if change.TTL != nil {
		if *change.TTL != "" {
			d, err := time.ParseDuration(*change.TTL)
			if err != nil || d <= 0 {
				return protocol.Record{}, ErrTTL
			}
		}
		r.TTL = *change.TTL
	}
	if change.Bound != nil {
		if *change.Bound < 0 {
			return protocol.Record{}, ErrBound
		}
		r.Bound = *change.Bound
	}
	if change.Full != nil {
		if *change.Full != protocol.OverflowStrict && *change.Full != protocol.OverflowRing {
			return protocol.Record{}, ErrOverflow
		}
		r.Full = *change.Full
	}
	// The whole record after the change, not the fields the change named: a
	// settings edit must not be able to leave behind a record that could not
	// have been registered in that shape.
	if err := validateKind(r); err != nil {
		return protocol.Record{}, err
	}
	if err := b.validatePersonal(r); err != nil {
		return protocol.Record{}, err
	}
	r.At = time.Now()
	b.records[name] = r
	b.recheckInbox(name)
	if err := b.checkpoint(false); err != nil {
		return protocol.Record{}, err
	}
	return b.withLiveness(name, r.Public()), nil
}

// A blocked read cannot retain access removed by a policy change. Dequeued
// messages have already left the broker and cannot be recalled.
func (b *Bus) recheckReaders() {
	for name := range b.inboxes {
		b.recheckInbox(name)
	}
}

func (b *Bus) recheckInbox(name string) {
	if in := b.inboxes[name]; in != nil {
		r := b.records[name]
		suspended := b.ownerSuspension(name)
		for i := len(in.waiters) - 1; i >= 0; i-- {
			w := in.waiters[i]
			if r.Disabled || suspended != nil || !b.may(w.caller, r) {
				err := ErrNotAllow
				if e := b.acting(w.caller); e != nil {
					err = e
				}
				if r.Disabled {
					err = ErrDisabled
				}
				// Last, because a reader blocked on a service whose owner has
				// just been suspended is being told about the suspension, not
				// about the ACL it still satisfies. SetUserState rechecks
				// readers, so this fires at the moment of the pause.
				if suspended != nil {
					err = suspended
				}
				w.stopped <- err
				in.waiters = drop(in.waiters, i)
			}
		}
	}
}

// visible adds daemon-derived control permissions, never caller claims.
func (b *Bus) visible(caller string, r protocol.Record) protocol.Record {
	r = b.withLiveness(r.Name, r.Public())
	r.Disabled = r.Disabled || !b.active(r.Name)
	r.CanManage = b.manages(caller, r)
	r.CanTransfer = caller == r.Owner || caller == b.admin
	return r
}

// A channel manager may remove a subscriber; adding one remains the
// subscriber's own opt-in operation.
func (b *Bus) RemoveSubscriber(caller, topic, subscriber string) (protocol.Record, error) {
	who, err := canon(caller)
	if err != nil {
		return protocol.Record{}, err
	}
	name, err := canon(topic)
	if err != nil {
		return protocol.Record{}, err
	}
	sub, err := canon(subscriber)
	if err != nil {
		return protocol.Record{}, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.acting(who); err != nil {
		return protocol.Record{}, err
	}
	r, ok := b.records[name]
	if !ok {
		return protocol.Record{}, ErrUnknown
	}
	if !b.manages(who, r) {
		return protocol.Record{}, ErrNotOwner
	}
	if r.Kind != protocol.KindPubSub {
		return protocol.Record{}, ErrKind
	}
	r.Subs = drop1(r.Subs, sub)
	b.records[name] = r
	if err := b.checkpoint(false); err != nil {
		return protocol.Record{}, err
	}
	return r.Public(), nil
}

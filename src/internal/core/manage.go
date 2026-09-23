package core

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Manage changes only explicitly supplied properties, under the ownership lock.
// Unlike registration it cannot erase a concurrently changed configuration.
type Management struct {
	Name  string    `json:"name"`
	Descr *string   `json:"descr,omitempty"`
	Addr  *string   `json:"addr,omitempty"`
	Proto *string   `json:"protocol,omitempty"`
	Allow *[]string `json:"allow,omitempty"`
	// Subs is the 📣 Deliver-To list: who receives a copy, which the ACL
	// above no longer decides. See docs/04-messaging.md#subscribers.
	Subs *[]string `json:"subs,omitempty"`
	// Add, AddToSet and Remove are deltas on the lists, applied to the record
	// as it is when the write holds the registry, so concurrent writers never
	// lose one another's terms (docs/constitution.md#persistence-and-loading).
	Add      *ListDelta `json:"add,omitempty"`
	AddToSet *ListDelta `json:"add_to_set,omitempty"`
	Remove   *ListDelta `json:"remove,omitempty"`
	// Status is active or inactive; the one edit an inactive record takes.
	Status      *string                  `json:"status,omitempty"`
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
	defer b.unlock()
	b.ownerRestored = false
	b.ownerRestoreErr = nil
	b.setDaemonOwner(owner)
	if err := b.commit(); err != nil {
		panic(err) // an embedded bus has no store; a failure here is a bug
	}
}

// setDaemonOwner updates the role nesting while caller holds b.mu.
func (b *Bus) setDaemonOwner(owner string) {
	b.setOwner(owner)
	b.administratorsAreUsers()
	// The protected group is the daemon Owner's and follows daemon ownership
	// in this same write: it is never assigned on its own
	// (docs/constitution.md#-group).
	members := []string{owner}
	if r, ok := b.records[AdministratorsGroup]; ok {
		members = append([]string{}, r.Allow...)
		if !slices.Contains(members, owner) {
			members = append(members, owner)
			sort.Strings(members)
		}
	}
	b.setGroup(AdministratorsGroup, owner, members)
	if r := b.records[AdministratorsGroup]; r.Owner != owner {
		r.Owner = owner
		b.setRecord(AdministratorsGroup, r)
	}
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
	defer b.unlock()
	if b.ownerRestoreErr != nil {
		return b.ownerRestoreErr
	}
	if b.ownerRestored {
		return nil
	}
	if b.admin == "" {
		b.setDaemonOwner(owner)
	}
	return b.commit()
}

func (b *Bus) DaemonOwner() string {
	b.mu.Lock()
	defer b.unlock()
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
	members := append([]string{}, b.records[AdministratorsGroup].Allow...)
	if b.admin != "" && !slices.Contains(members, b.admin) {
		members = append(members, b.admin)
	}
	for _, name := range members {
		// Administrative authority is direct-only. A damaged snapshot is
		// refused at startup; do not manufacture a user for its group name on
		// the way to that refusal.
		if groupName(name) {
			continue
		}
		if _, known := b.users[name]; !known {
			b.setUser(name, protocol.User{Name: name, Status: protocol.StatusActive})
		}
		// Every User has exactly one user record (docs/constitution.md#-user).
		if _, known := b.records[name]; !known {
			b.setRecord(name, protocol.Record{Name: name, Owner: name, Kind: protocol.KindUser, Personal: true, Full: protocol.OverflowStrict, At: time.Now()})
			b.ensure(name)
		}
	}
}

func (b *Bus) resourceManages(caller string, r protocol.Record) bool {
	return b.acting(caller) == nil && (caller == r.Owner || caller == r.Name || b.maintains(caller, r))
}

func (b *Bus) maintains(caller string, r protocol.Record) bool {
	for _, term := range r.Maintainers {
		if term == caller || b.member(caller, term) || b.runtimeTerm(caller, term, r) {
			return true
		}
	}
	return false
}

// runtimeTerm answers the two terms that name nobody stored: @owner, the
// record's Owner and the Agents that Owner directly owns, and @agent, an
// Agent record's own principal. Both resolve at each check.
// See docs/constitution.md#actors-and-ascii-textarea-syntax. Caller holds b.mu.
func (b *Bus) runtimeTerm(caller, term string, r protocol.Record) bool {
	switch term {
	case OwnerGroup:
		return b.sameResourceOwner(caller, r.Owner)
	case AgentTerm:
		return r.Kind == protocol.KindAgent && caller == r.Name
	}
	return false
}

// normalizeAllow validates an ACL as a whole before anything stores it: one
// bad term refuses the lot. Caller holds b.mu.
func (b *Bus) normalizeAllow(in []string, r protocol.Record) ([]string, error) {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, raw := range in {
		a := strings.TrimSpace(raw)
		switch {
		case r.Kind == protocol.KindGroup && (a == "*" || reservedTerm(a)):
			// A Group's allow is its membership: actors, never the wildcard
			// or a runtime term, which name nobody in particular.
			return nil, fmt.Errorf("%w: %s cannot be a member of group %s", ErrBadName, a, r.Name)
		case a == "*" || a == OwnerGroup:
		case a == AgentTerm:
			if r.Kind != protocol.KindAgent {
				return nil, fmt.Errorf("%w: %s names an agent record's own principal, and %s is a %s", ErrBadName, AgentTerm, r.Name, r.Kind)
			}
		case strings.HasPrefix(a, "@"):
			a = strings.ToLower(a)
			if !groupName(a) {
				return nil, fmt.Errorf("%w: invalid ACL group %q", ErrBadName, raw)
			}
			if _, ok := b.groupMembers(a); !ok {
				return nil, fmt.Errorf("%w: ACL group %s", ErrUnknown, a)
			}
		default:
			var err error
			if a, err = canon(a); err != nil {
				return nil, err
			}
			if err := b.unmarkedAgent(a); err != nil {
				return nil, err
			}
			if r.Kind == protocol.KindGroup {
				if err := b.groupMember(a); err != nil {
					return nil, err
				}
			}
		}
		if seen[a] {
			continue
		}
		seen[a] = true
		out = append(out, a)
	}
	return out, nil
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
	// A user's own record is always Personal, and who reaches it follows the
	// user delivery rule rather than its lists (docs/constitution.md#-channels).
	if r.Kind == protocol.KindUser {
		return nil
	}
	// Personal is an audience: the Owner and the Agents that Owner owns. Its
	// lists may name that cohort, directly or by @owner and @agent, and
	// nothing wider; every save is checked against the whole record.
	// See docs/03-records.md#personal-and-shared.
	cohort := func(term string) bool {
		switch {
		case term == r.Owner, term == OwnerGroup:
			return true
		case term == AgentTerm:
			return r.Kind == protocol.KindAgent
		}
		other, known := b.records[term]
		return known && other.Kind == protocol.KindAgent && other.Owner == r.Owner
	}
	for _, term := range r.Allow {
		if !cohort(term) {
			return fmt.Errorf("%w: %s reaches outside the owner's own agents", ErrPersonal, term)
		}
	}
	for _, term := range r.Maintainers {
		if !cohort(term) {
			return fmt.Errorf("%w: %s reaches outside the owner's own agents", ErrPersonal, term)
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
	members, _ := b.groupMembers(group)
	for _, member := range members {
		if member == caller {
			return true
		}
		if group != AdministratorsGroup && groupName(member) && b.memberThrough(caller, member, seen) {
			return true
		}
	}
	return false
}

// groupName says whether n is a Group's name: "@" and then a name by the
// ordinary rules, realm included (docs/constitution.md#common-record-fields),
// lowercase as written and within the one bound on every name.
func groupName(n string) bool {
	if len(n) < 2 || len(n) > protocol.MaxName || n[0] != '@' {
		return false
	}
	// A prefixed group is its owner's: everything before the first "/" is a
	// User's whole name, realm and all, and the rest is the group's own name
	// with a realm of its choosing — "@alice@srv1/friends@batch1"
	// (docs/constitution.md#-group).
	if owner, rest, cut := strings.Cut(n[1:], "/"); cut {
		return plainName(owner) && plainName(rest)
	}
	return plainName(n[1:])
}

// plainName is a canonical name with neither an agent's "#" nor a "/".
func plainName(s string) bool {
	parsed, err := protocol.ParseName(s)
	return err == nil && !parsed.Agent && parsed.Template == "" && parsed.String() == s
}

// groupOwner is the User a prefixed group's name is reserved for, and false
// for a group in the shared namespace.
func groupOwner(n string) (string, bool) {
	if !strings.HasPrefix(n, "@") {
		return "", false
	}
	owner, _, cut := strings.Cut(n[1:], "/")
	return owner, cut
}

// normalizeMaintainers validates the whole replacement before Manage stores
// any of it. Named users and agents may be direct Maintainers; ordinary groups
// inherit their nested membership. Every other record, credential-only names,
// duplicates and wildcard authority are deliberately refused.
// Caller holds b.mu.
func (b *Bus) normalizeMaintainers(in protocol.MaintainerList, r protocol.Record) (protocol.MaintainerList, error) {
	out := make(protocol.MaintainerList, 0, len(in))
	seen := map[string]bool{}
	for _, raw := range in {
		var term string
		if strings.HasPrefix(strings.TrimSpace(raw), "@") {
			term = strings.ToLower(strings.TrimSpace(raw))
			switch {
			case term == OwnerGroup:
			case term == AgentTerm:
				if r.Kind != protocol.KindAgent {
					return nil, fmt.Errorf("%w: %s names an agent record's own principal, and %s is a %s", ErrBadName, AgentTerm, r.Name, r.Kind)
				}
			case !groupName(term):
				return nil, fmt.Errorf("%w: invalid maintainers group %q", ErrBadName, raw)
			default:
				if _, ok := b.groupMembers(term); !ok {
					return nil, fmt.Errorf("%w: maintainer %s", ErrUnknown, term)
				}
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
				if err := b.unmarkedAgent(term); err != nil {
					return nil, err
				}
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

// SetGroup writes a Group's whole membership. A Group is an ordinary record
// (docs/constitution.md#-group): whoever may own a record creates one, owned
// by their User, and its Owner, its Maintainers and the daemon's
// Administrators change its membership. The protected @administrators is the
// daemon Owner's alone, takes Users only, and always holds the Owner.
func (b *Bus) SetGroup(caller, name string, members []string) error {
	who, err := canon(caller)
	if err != nil {
		return err
	}
	name = strings.ToLower(strings.TrimSpace(name))
	if reservedTerm(name) {
		return fmt.Errorf("%w: %s is a runtime ACL term and cannot be created", ErrBadName, name)
	}
	if !groupName(name) {
		return fmt.Errorf("%w: invalid group", ErrBadName)
	}
	normalized := []string{}
	for _, m := range members {
		m = strings.TrimSpace(m)
		if reservedTerm(strings.ToLower(m)) {
			return fmt.Errorf("%w: %s is direct ACL syntax and cannot be a group member", ErrBadName, m)
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
	defer b.unlock()
	if err := b.acting(who); err != nil {
		return err
	}
	if name != AdministratorsGroup {
		for _, m := range normalized {
			if err := b.groupMember(m); err != nil {
				return err
			}
		}
	}
	old, exists := b.records[name]
	owner := ""
	switch {
	case name == AdministratorsGroup:
		if who != b.admin {
			return ErrNotOwner
		}
		includesOwner := false
		for _, member := range normalized {
			// A User's name only: a group, an agent or any record that is not
			// a User's own would become a User under a name no User may have
			// (docs/constitution.md#-group).
			if _, user := b.users[member]; groupName(member) || protocol.IsAgentName(member) || !user {
				return fmt.Errorf("%w: %s accepts direct user identities only", ErrBadName, AdministratorsGroup)
			}
			if member == b.admin {
				includesOwner = true
			}
		}
		if !includesOwner {
			return fmt.Errorf("%w: owner must remain an administrator", ErrNotOwner)
		}
		owner = b.admin
	case !exists:
		if owner, err = b.ownerFor(who); err != nil {
			return err
		}
		if err := b.mayOwn(owner, name); err != nil {
			return err
		}
	case old.Kind != protocol.KindGroup:
		return fmt.Errorf("%w: %s is a %s, not a group", ErrKind, name, old.Kind)
	case !b.live(old):
		return fmt.Errorf("%w: %s", ErrUnknown, name)
	case !b.manages(who, old) && !b.isAdministrator(who):
		return ErrNotOwner
	default:
		owner = old.Owner
	}
	b.setGroup(name, owner, normalized)
	r := b.records[name]
	if err := validateKind(r); err != nil {
		return err
	}
	if err := b.validatePersonal(r); err != nil {
		return err
	}
	if name == AdministratorsGroup {
		b.administratorsAreUsers()
	}
	b.recheckReaders()
	return b.commit()
}

// Groups names every live Group the caller may name in a list, and the
// membership of those whose membership the caller may read: Administrators
// read all, and a Group's own ACL — its membership — and managers read it.
func (b *Bus) Groups(caller string) map[string][]string {
	b.mu.Lock()
	defer b.unlock()
	out := map[string][]string{}
	if b.acting(caller) != nil {
		return out
	}
	for name, r := range b.records {
		if r.Kind != protocol.KindGroup || !b.live(r) {
			continue
		}
		out[name] = []string{}
		if b.isAdministrator(caller) || b.may(caller, r) {
			out[name] = append(out[name], r.Allow...)
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
	defer b.unlock()
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
	if err := b.commit(); err != nil {
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
	defer b.unlock()
	if err := b.acting(who); err != nil {
		return protocol.Record{}, err
	}
	r, ok := b.records[name]
	if !ok {
		return protocol.Record{}, ErrUnknown
	}
	// An inactive record takes one edit, its reactivation, and nothing else:
	// to every other change it is no such record. A record inactive because
	// its User is comes back with its User, not by this edit
	// (docs/constitution.md#common-record-fields).
	// Its existence is disclosed to nobody who could not reactivate it.
	if !b.live(r) {
		if change.Status == nil || !statusOnly(change) || !b.userActive(r.Owner) || !b.manages(who, r) {
			return protocol.Record{}, ErrUnknown
		}
	}
	// Deltas are resolved here, against the record as the write finds it,
	// and then checked exactly as a whole-list write is.
	if err := applyDeltas(r, &change); err != nil {
		return protocol.Record{}, err
	}
	// The protected group changes only through its own rule: its Owner
	// follows daemon ownership, it has no Maintainers, and its membership is
	// the daemon Owner's to set with SetGroup (docs/constitution.md#-group).
	if name == AdministratorsGroup && (change.Owner != nil || change.Maintainers != nil || change.Allow != nil || change.Personal != nil || change.Status != nil) {
		return protocol.Record{}, fmt.Errorf("%w: %s changes only with daemon ownership and its own membership rule", ErrNotOwner, AdministratorsGroup)
	}
	// A Group's membership is also the Administrators', through either door
	// (docs/01-identity-and-roles.md#groups).
	if !b.manages(who, r) && !(r.Kind == protocol.KindGroup && b.isAdministrator(who)) {
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
		// Only a User owns records (docs/constitution.md#-registry-record).
		if _, user := b.users[owner]; !user {
			return protocol.Record{}, fmt.Errorf("%w: new owner must be a registered user", ErrBadName)
		}
		// And one who may act. Handing a record to a paused or banned name
		// leaves it owned by somebody who cannot answer for it, which is the
		// orphan by another route.
		if err := b.acting(owner); err != nil {
			return protocol.Record{}, err
		}
		if r.Kind == protocol.KindUser {
			return protocol.Record{}, fmt.Errorf("%w: a user's own record cannot be transferred", ErrNotOwner)
		}
		// Its name says whose it is, and a name never changes: moving one is
		// an administrator editing the database and restarting the daemon.
		if prefix, ok := groupOwner(name); ok && owner != prefix {
			return protocol.Record{}, fmt.Errorf("%w: %s is named for %s and cannot be transferred; create @%s/… instead", ErrKind, name, prefix, owner)
		}
		// An agent's credentials go with it, in this same commit: its pair
		// names its Owner, and a transfer that committed without them would
		// leave every one naming the old Owner (docs/02-access.md#token-lifetime).
		if r.Kind == protocol.KindAgent && owner != r.Owner {
			b.stageCredential(name, &ports.CredentialPair{UserID: b.users[owner].ID, AgentID: r.ID})
			// And it leaves its old Owner's cohort: every Personal record of
			// that Owner loses its grants to it in this same commit, or the
			// record would admit an agent outside the cohort it names
			// (docs/03-records.md#personal-and-shared).
			for other, o := range b.records {
				if other == name || o.Owner != r.Owner || !o.Personal {
					continue
				}
				allow, maint := drop1(o.Allow, name), drop1(o.Maintainers, name)
				if len(allow) != len(o.Allow) || len(maint) != len(o.Maintainers) {
					o.Allow, o.Maintainers = allow, maint
					b.setRecord(other, o)
				}
			}
		}
		r.Owner = owner
	}
	if change.Maintainers != nil {
		maintainers, err := b.normalizeMaintainers(*change.Maintainers, r)
		if err != nil {
			return protocol.Record{}, err
		}
		r.Maintainers = maintainers
	}
	if change.Personal != nil {
		// Everything not Personal is shared, and a User is not
		// (docs/constitution.md#common-record-fields).
		if r.Kind == protocol.KindUser && !*change.Personal {
			return protocol.Record{}, fmt.Errorf("%w: a user's own record is always personal", ErrPersonal)
		}
		r.Personal = *change.Personal
	}
	if change.Subs != nil {
		// Not asked here whether the kind has a list at all: validateKind
		// below asks that of the record this edit would leave behind, which
		// is the one question every path that stores one asks.
		list, err := b.normalizeDeliverTo(*change.Subs, r)
		if err != nil {
			return protocol.Record{}, err
		}
		r.Subs = list
	}
	if change.Allow != nil {
		allow, err := b.normalizeAllow(*change.Allow, r)
		if err != nil {
			return protocol.Record{}, err
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
	if change.Status != nil {
		if err := validStatus(*change.Status); err != nil {
			return protocol.Record{}, err
		}
		// A User's own record lives exactly as its User does; the User's
		// status is changed where Users are (docs/constitution.md#-user).
		if r.Kind == protocol.KindUser {
			return protocol.Record{}, fmt.Errorf("%w: a user's own record takes its status from the user", ErrBadName)
		}
		if *change.Status != "" {
			r.Status = *change.Status
		}
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
	b.setRecord(name, r)
	// A Group grants on other records' lists, so its edit can take authority
	// from readers blocked anywhere.
	// A Group grants on other records' lists, a status or owner change can
	// take a reader's own standing, and a transfer rewrites the old Owner's
	// Personal lists: each can take authority from readers blocked anywhere.
	if r.Kind == protocol.KindGroup || change.Status != nil || change.Owner != nil {
		b.recheckReaders()
	} else {
		b.recheckInbox(name)
	}
	if err := b.commit(); err != nil {
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
		r, live := b.entity(name)
		for i := len(in.waiters) - 1; i >= 0; i-- {
			w := in.waiters[i]
			// A record that stopped being an entity releases every reader:
			// what it read is no such inbox now. A caller that stopped
			// acting is told so, and one the ACL dropped is told that.
			if !live || !b.may(w.caller, r) {
				err := ErrNotAllow
				if !live {
					err = fmt.Errorf("%w: %s is inactive", ErrUnknown, name)
				}
				if e := b.acting(w.caller); e != nil {
					err = e
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
	if !b.live(r) {
		r.Status = protocol.StatusInactive
	} else {
		r.Status = protocol.StatusActive
	}
	if (r.Kind == protocol.KindAgent || r.Kind == protocol.KindQueue) && len(r.Subs) == 1 {
		dst, ok := b.entity(r.Subs[0])
		allowed := ok && b.admits(dst, r.Name)
		r.RouteAllowed = &allowed
	}
	r.CanManage = b.manages(caller, r)
	r.CanTransfer = caller == r.Owner || caller == b.admin
	return r
}

// A channel manager may remove a subscriber; adding one remains the
// subscriber's own opt-in operation.
func (b *Bus) RemoveSubscriber(caller, channel, subscriber string) (protocol.Record, error) {
	who, err := canon(caller)
	if err != nil {
		return protocol.Record{}, err
	}
	name, err := canon(channel)
	if err != nil {
		return protocol.Record{}, err
	}
	sub, err := canon(subscriber)
	if err != nil {
		return protocol.Record{}, err
	}
	b.mu.Lock()
	defer b.unlock()
	if err := b.acting(who); err != nil {
		return protocol.Record{}, err
	}
	r, ok := b.entity(name)
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
	b.setRecord(name, r)
	if err := b.commit(); err != nil {
		return protocol.Record{}, err
	}
	return r.Public(), nil
}

// groupMember says whether a term may be a Group's member: an actor — a User
// or a live Agent — or a live Group (docs/constitution.md#-group). A queue, a
// service, a pubsub or a name nothing holds is refused, a group name included:
// a grant made to a name before it exists would pass to whoever created it.
// Caller holds b.mu.
func (b *Bus) groupMember(term string) error {
	if groupName(term) {
		if _, ok := b.groupMembers(term); !ok {
			return fmt.Errorf("%w: group member %s", ErrUnknown, term)
		}
		return nil
	}
	if err := b.unmarkedAgent(term); err != nil {
		return err
	}
	if _, user := b.users[term]; user {
		return nil
	}
	if r, ok := b.entity(term); ok && r.Kind == protocol.KindAgent {
		return nil
	}
	if r, ok := b.records[term]; ok && b.live(r) {
		return fmt.Errorf("%w: a group's members are users, agents and groups, and %s is a %s", ErrBadName, term, r.Kind)
	}
	return fmt.Errorf("%w: group member %s", ErrUnknown, term)
}

// unmarkedAgent refuses an unprefixed term that names no User while an Agent
// holds the same name with its "#": an unprefixed term names a User, and the
// agent it was probably meant for is named, never guessed or retyped
// (docs/constitution.md#actors-and-ascii-textarea-syntax). Caller holds b.mu.
func (b *Bus) unmarkedAgent(term string) error {
	if _, user := b.users[term]; user || protocol.IsAgentName(term) {
		return nil
	}
	if r, ok := b.records["#"+term]; ok && r.Kind == protocol.KindAgent {
		return fmt.Errorf("%w: %s names no user; an agent's name begins with #, so the agent is #%s", ErrBadName, term, term)
	}
	return nil
}

// ListDelta names terms to add to or remove from each list: the ACL — a
// Group's membership — the Maintainers, and deliver_to.
type ListDelta struct {
	Allow       []string `json:"allow,omitempty"`
	Maintainers []string `json:"maintainers,omitempty"`
	Subs        []string `json:"subs,omitempty"`
}

func (d *ListDelta) empty() bool {
	return d == nil || len(d.Allow) == 0 && len(d.Maintainers) == 0 && len(d.Subs) == 0
}

// listTerm is a term as its list stores it, so a delta compares like with
// like: a name canonical, a group or runtime term lower-cased, the wildcard
// itself. Whether it is valid there is the list's own normalizer's question.
func listTerm(raw string) (string, error) {
	t := strings.TrimSpace(raw)
	if t == "*" {
		return t, nil
	}
	if strings.HasPrefix(t, "@") {
		return strings.ToLower(t), nil
	}
	return canon(t)
}

// applyDeltas turns the change's deltas into whole-list writes against r as
// it is now, then leaves every check to the path a whole-list write takes.
// add refuses a term already there, and a one-slot deliver_to that is
// occupied; add_to_set adds what is absent and succeeds on what is present;
// remove takes what is there and is a no-op for what is not. A list may not
// be both written whole and changed by a delta in one write, and an invalid
// delta changes nothing (docs/01-identity-and-roles.md#record-authority).
func applyDeltas(r protocol.Record, change *Management) error {
	if change.Add.empty() && change.AddToSet.empty() && change.Remove.empty() {
		return nil
	}
	lists := []struct {
		field   string
		current []string
		whole   bool
		add     func(*ListDelta) []string
		set     func([]string)
	}{
		{"allow", r.Allow, change.Allow != nil, func(d *ListDelta) []string { return d.Allow }, func(l []string) { change.Allow = &l }},
		{"maintainers", r.Maintainers, change.Maintainers != nil, func(d *ListDelta) []string { return d.Maintainers }, func(l []string) { m := protocol.MaintainerList(l); change.Maintainers = &m }},
		{"deliver_to", r.Subs, change.Subs != nil, func(d *ListDelta) []string { return d.Subs }, func(l []string) { change.Subs = &l }},
	}
	oneSlot := r.Kind == protocol.KindAgent || r.Kind == protocol.KindQueue
	for _, l := range lists {
		pick := func(d *ListDelta) []string {
			if d == nil {
				return nil
			}
			return l.add(d)
		}
		add, toSet, remove := pick(change.Add), pick(change.AddToSet), pick(change.Remove)
		if len(add)+len(toSet)+len(remove) == 0 {
			continue
		}
		if l.whole {
			return fmt.Errorf("%w: %s is both written whole and changed by a delta", ErrBadName, l.field)
		}
		next := append([]string{}, l.current...)
		has := func(t string) bool { return slices.Contains(next, t) }
		for _, raw := range remove {
			t, err := listTerm(raw)
			if err != nil {
				return err
			}
			next = slices.DeleteFunc(next, func(x string) bool { return x == t })
		}
		for _, raw := range add {
			t, err := listTerm(raw)
			if err != nil {
				return err
			}
			if has(t) {
				return fmt.Errorf("%w: %s already names %s", ErrBusy, l.field, t)
			}
			if l.field == "deliver_to" && oneSlot && len(next) > 0 {
				return fmt.Errorf("%w: %s is occupied by %s; remove it or write the field whole", ErrBusy, l.field, next[0])
			}
			next = append(next, t)
		}
		for _, raw := range toSet {
			t, err := listTerm(raw)
			if err != nil {
				return err
			}
			if has(t) {
				continue
			}
			if l.field == "deliver_to" && oneSlot && len(next) > 0 {
				return fmt.Errorf("%w: %s is occupied by %s; remove it or write the field whole", ErrBusy, l.field, next[0])
			}
			next = append(next, t)
		}
		l.set(next)
	}
	change.Add, change.AddToSet, change.Remove = nil, nil, nil
	return nil
}

// statusOnly says whether a change edits nothing but the status.
func statusOnly(c Management) bool {
	return c.Descr == nil && c.Addr == nil && c.Proto == nil && c.Allow == nil && c.Subs == nil &&
		c.Add.empty() && c.AddToSet.empty() && c.Remove.empty() &&
		c.Maintainers == nil && c.Personal == nil && c.Owner == nil && c.TTL == nil && c.Bound == nil && c.Full == nil
}

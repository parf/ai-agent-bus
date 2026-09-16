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
	Name        string    `json:"name"`
	Descr       *string   `json:"descr,omitempty"`
	Addr        *string   `json:"addr,omitempty"`
	Proto       *string   `json:"protocol,omitempty"`
	Allow       *[]string `json:"allow,omitempty"`
	NoMaster    *bool     `json:"no_master,omitempty"`
	Disabled    *bool     `json:"disabled,omitempty"`
	Maintainers *string   `json:"maintainers,omitempty"`
	Owner       *string   `json:"owner,omitempty"`
	TTL         *string   `json:"ttl,omitempty"`
	Bound       *int      `json:"bound,omitempty"`
	Full        *string   `json:"overflow,omitempty"`
}

func (b *Bus) Administrator(owner string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.admin = owner
	if !b.member(owner, MaintainersGroup) {
		b.groups[MaintainersGroup] = append(b.groups[MaintainersGroup], owner)
	}
	b.maintainersAreUsers()
}

// maintainersAreUsers keeps the levels nested: an owner is a maintainer and a
// maintainer is a user (docs/01-identity.md#groups-and-maintainers). Somebody
// given authority over users who was not one themselves would be a principal
// the user administration cannot see, and a credential the ownerless sweep
// would take (docs/02-access.md#ownerless-credentials). Caller holds b.mu.
//
// Being taken out of the group does not take the profile away again: a user is
// never deleted, only made inactive (docs/01-identity.md#user-lifecycle).
func (b *Bus) maintainersAreUsers() {
	for _, name := range b.groups[MaintainersGroup] {
		if _, known := b.users[name]; !known {
			b.users[name] = protocol.User{Name: name, State: "active"}
		}
	}
}

func (b *Bus) manages(caller string, r protocol.Record) bool {
	return b.acting(caller) == nil && (caller == r.Owner || caller == r.Name || b.member(caller, r.Maintainers))
}
func (b *Bus) member(caller, group string) bool {
	for _, n := range b.groups[group] {
		if n == caller {
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

// Groups are daemon-local flat sets. Only the daemon owner changes membership;
// record ownership never confers organization administration.
func (b *Bus) SetGroup(caller, name string, members []string) error {
	who, err := canon(caller)
	if err != nil {
		return err
	}
	if !groupName(name) {
		return fmt.Errorf("%w: invalid group", ErrBadName)
	}
	normalized := []string{}
	for _, m := range members {
		n, err := canon(m)
		if err != nil {
			return err
		}
		normalized = append(normalized, n)
	}
	sort.Strings(normalized)
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.acting(who); err != nil {
		return err
	}
	if !b.isMaintainer(who) {
		return ErrNotOwner
	}
	if name == MaintainersGroup {
		if who != b.admin {
			return ErrNotOwner
		}
		includesOwner := false
		for _, member := range normalized {
			if member == b.admin {
				includesOwner = true
			}
		}
		if !includesOwner {
			return fmt.Errorf("%w: owner must remain a maintainer", ErrNotOwner)
		}
	}
	b.groups[name] = normalized
	if name == MaintainersGroup {
		b.maintainersAreUsers()
	}
	b.recheckReaders()
	return nil
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
		if b.isMaintainer(caller) {
			out[group] = append(out[group], members...)
		}
	}
	return out
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
	if (change.Owner != nil || change.Maintainers != nil) && who != r.Owner {
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
		if *change.Maintainers != "" {
			if _, ok := b.groups[*change.Maintainers]; !ok {
				return protocol.Record{}, fmt.Errorf("%w: maintainers group", ErrUnknown)
			}
		}
		r.Maintainers = *change.Maintainers
	}
	if change.Allow != nil {
		allow := make([]string, 0, len(*change.Allow))
		for _, a := range *change.Allow {
			if a != "*" {
				if strings.HasPrefix(a, "@") {
					if _, ok := b.groups[a]; !ok {
						return protocol.Record{}, fmt.Errorf("%w: ACL group", ErrUnknown)
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
	if change.NoMaster != nil {
		r.NoMaster = *change.NoMaster
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
	r.At = time.Now()
	b.records[name] = r
	b.recheckInbox(name)
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
	r.CanTransfer = caller == r.Owner
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
	if r.Kind != protocol.KindTopic || r.Mode != protocol.ModePubSub {
		return protocol.Record{}, ErrMode
	}
	r.Subs = drop1(r.Subs, sub)
	b.records[name] = r
	return r.Public(), nil
}

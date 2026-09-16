package core

import (
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

const MaintainersGroup = "@maintainers"

func (b *Bus) active(name string) bool {
	state := b.users[name].State
	return state == "" || state == "active"
}
func (b *Bus) CanAuthenticate(name string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.active(name)
}
func (b *Bus) isMaintainer(name string) bool {
	return name == b.admin || b.member(name, MaintainersGroup)
}
func (b *Bus) IsMaintainer(name string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.active(name) && b.isMaintainer(name)
}

// IsPerson says whether a name is somebody's identity rather than a service
// they registered. A person keeps their credential when an address of theirs
// is removed; a service does not (docs/01-identity.md#unregistering).
func (b *Bus) IsPerson(name string) bool {
	n, err := canon(name)
	if err != nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	_, known := b.users[n]
	return known
}

// Ownerless picks out, of the credential names given, those that answer for
// nothing: no record of their own and no registered user. Those are what the
// daemon drops at start (docs/02-access.md#ownerless-credentials) — a sweep at
// a known moment, never expiry, because a credential does not retire for being
// old or idle.
//
// The two tests are the daemon's own: a profile it holds and a record it
// holds. Never how a name is spelled — a name that looks like a test fixture
// and belongs to somebody is a person, and a tidy-looking name with nothing
// behind it is not (docs/01-identity.md#person-records).
//
// The daemon owner's credential is minted by the store rather than by a
// record, and survives because starting the daemon writes the owner a profile.
// That is why this must run after the administrator is set, and why it asks
// about users at all: a sweep that looked only for a record would take it.
func (b *Bus) Ownerless(names []string) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := []string{}
	for _, raw := range names {
		name, err := canon(raw)
		if err != nil {
			continue
		}
		if b.ownerless(name) {
			out = append(out, raw)
		}
	}
	sort.Strings(out)
	return out
}

// identityKind and ownerless share the facts behind directory labels and
// cleanup. Caller holds b.mu; empty profile fields never change a user's kind.
func (b *Bus) identityKind(name string) string {
	if _, person := b.users[name]; person {
		return protocol.DirectoryUser
	}
	if _, record := b.records[name]; record {
		return protocol.DirectoryRecord
	}
	return protocol.DirectoryCredential
}

func (b *Bus) ownerless(name string) bool {
	// Interim until H.5.5: keep the credential behind existing services.
	// Remove this guard with orphan-service deletion, not before it.
	return b.identityKind(name) == protocol.DirectoryCredential && !b.owns(name)
}

// RemoveOwnerless holds the same lock used to create profiles and records
// until the credential is removed. A stale directory row grants no authority.
// forget must only touch the credential store, never call back into Bus.
func (b *Bus) RemoveOwnerless(caller, name string, forget func(string) error) error {
	who, err := canon(caller)
	if err != nil {
		return err
	}
	n, err := canon(name)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.active(who) || !b.isMaintainer(who) {
		return ErrNotOwner
	}
	if !b.ownerless(n) {
		return fmt.Errorf("%w: credential is now backed by a user, record or owned service; refresh the directory", ErrBusy)
	}
	return forget(n)
}

// owns says whether this name is somebody else's owner. A record it owns that
// is itself needs no clause here: Ownerless has already kept it for holding a
// record. Caller holds b.mu.
func (b *Bus) owns(name string) bool {
	for _, r := range b.records {
		if r.Owner == name {
			return true
		}
	}
	return false
}

func (b *Bus) mayEditUser(caller, name string) bool {
	return b.active(caller) && (caller == b.admin || b.isMaintainer(caller) && caller != name && !b.isMaintainer(name))
}

func normalizedProfile(in protocol.User) (protocol.User, error) {
	name, err := canon(in.Name)
	if err != nil {
		return protocol.User{}, err
	}
	in.Name = name
	in.PersonName = strings.TrimSpace(in.PersonName)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.GithubUser = strings.ToLower(strings.TrimSpace(in.GithubUser))
	parsedName, _ := protocol.ParseName(in.Name)
	if parsedName.Realm == "github" {
		if in.GithubUser != "" && in.GithubUser != parsedName.Local {
			return protocol.User{}, fmt.Errorf("%w: GitHub identity must keep its login", ErrProfile)
		}
		in.GithubUser = parsedName.Local
	}
	if len(in.PersonName) > 200 {
		return protocol.User{}, fmt.Errorf("%w: person name is too long", ErrProfile)
	}
	if in.Email != "" {
		parsed, err := mail.ParseAddress(in.Email)
		if err != nil || parsed.Address != in.Email || parsed.Name != "" || len(in.Email) > 254 {
			return protocol.User{}, fmt.Errorf("%w: invalid email", ErrProfile)
		}
		for _, c := range in.Email {
			if c > 127 {
				return protocol.User{}, fmt.Errorf("%w: email must use ASCII spelling", ErrProfile)
			}
		}
	}
	if in.GithubUser != "" {
		if len(in.GithubUser) > 39 || in.GithubUser[0] == '-' || in.GithubUser[len(in.GithubUser)-1] == '-' {
			return protocol.User{}, fmt.Errorf("%w: invalid GitHub login", ErrProfile)
		}
		for _, c := range in.GithubUser {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return protocol.User{}, fmt.Errorf("%w: invalid GitHub login", ErrProfile)
			}
		}
	}
	in.Kind = ""
	in.Maintainer, in.DaemonOwner, in.CanEdit, in.CanActivate, in.CanRemove = false, false, false, false, false
	in.Groups, in.Services = nil, nil
	return in, nil
}

// SetUser is the sole profile/lifecycle write path. Registration cannot vouch
// for a person's fields, change their state or promote their authority.
func (b *Bus) SetUser(caller string, in protocol.User, create bool) (protocol.User, error) {
	who, err := canon(caller)
	if err != nil {
		return protocol.User{}, err
	}
	in, err = normalizedProfile(in)
	if err != nil {
		return protocol.User{}, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.mayEditUser(who, in.Name) {
		return protocol.User{}, ErrNotOwner
	}
	old, exists := b.users[in.Name]
	if create && (exists || b.records[in.Name].Name != "") {
		return protocol.User{}, ErrExists
	}

	if r, ok := b.records[in.Name]; ok && r.Owner != r.Name {
		return protocol.User{}, fmt.Errorf("%w: that name is a service", ErrProfile)
	}
	if in.State == "" {
		in.State = old.State
		if in.State == "" {
			in.State = "active"
		}
	}
	if in.State != "active" && in.State != "paused" && in.State != "banned" {
		return protocol.User{}, fmt.Errorf("%w: invalid user state", ErrProfile)
	}
	if in.Name == b.admin && in.State != "active" {
		return protocol.User{}, fmt.Errorf("%w: the daemon owner must remain active", ErrNotOwner)
	}
	if old.State == "banned" && in.State != "banned" && who != b.admin {
		return protocol.User{}, ErrNotOwner
	}
	for name, user := range b.users {
		if name == in.Name {
			continue
		}
		if in.Email != "" && user.Email == in.Email || in.GithubUser != "" && user.GithubUser == in.GithubUser {
			return protocol.User{}, fmt.Errorf("%w: identifying field already belongs to another user", ErrProfile)
		}
	}
	// A vouched realm still requires key-possession proof before its name can
	// be created. Editing an existing enrolled profile does not repeat proof.
	if _, known := b.records[in.Name]; !known {
		parsed, _ := protocol.ParseName(in.Name)
		if _, backed := b.dirs[parsed.Realm]; backed {
			return protocol.User{}, ErrEnrol
		}
		b.records[in.Name] = protocol.Record{Name: in.Name, Owner: in.Name, Kind: "agent", Full: protocol.OverflowStrict, At: time.Now()}
		b.ensure(in.Name)
	}
	b.users[in.Name] = in
	b.recheckReaders()
	return b.userView(who, in.Name), nil
}

func (b *Bus) userView(caller, name string) protocol.User {
	u := b.users[name]
	u.Name = name
	u.Kind = b.identityKind(name)
	if u.Kind != protocol.DirectoryUser {
		u.State = ""
	} else if u.State == "" {
		u.State = "active"
	}
	u.DaemonOwner = name == b.admin
	u.Maintainer = b.isMaintainer(name)
	u.CanEdit = u.Kind == protocol.DirectoryUser && b.mayEditUser(caller, name)
	u.CanActivate = u.CanEdit && (u.State != "banned" || caller == b.admin)
	u.CanRemove = b.active(caller) && b.isMaintainer(caller) && b.ownerless(name)
	for group, members := range b.groups {
		for _, member := range members {
			if member == name {
				u.Groups = append(u.Groups, group)
				break
			}
		}
	}
	for _, r := range b.records {
		if r.Owner == name && r.Name != name {
			u.Services = append(u.Services, r.Name)
		}
	}
	sort.Strings(u.Groups)
	sort.Strings(u.Services)
	return u
}

func (b *Bus) Users(caller string, credentialNames []string) []protocol.User {
	b.mu.Lock()
	defer b.mu.Unlock()
	names := map[string]bool{}
	for name := range b.users {
		names[name] = true
	}
	for name, r := range b.records {
		if name == r.Owner {
			names[name] = true
		}
	}
	for _, name := range credentialNames {
		if r, known := b.record(name); !known || r.Owner == name {
			names[name] = true
		}
	}
	out := []protocol.User{}
	for name := range names {
		if caller == name || b.active(caller) && b.isMaintainer(caller) {
			out = append(out, b.userView(caller, name))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (b *Bus) SetUserState(caller, name, state string) (protocol.User, error) {
	who, err := canon(caller)
	if err != nil {
		return protocol.User{}, err
	}
	name, err = canon(name)
	if err != nil {
		return protocol.User{}, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.mayEditUser(who, name) {
		return protocol.User{}, ErrNotOwner
	}
	if state != "active" && state != "paused" && state != "banned" {
		return protocol.User{}, ErrProfile
	}
	u, known := b.users[name]
	if !known {
		if r, ok := b.records[name]; !ok || r.Owner != name {
			return protocol.User{}, ErrUnknown
		}
	}
	if name == b.admin && state != "active" || u.State == "banned" && state != "banned" && who != b.admin {
		return protocol.User{}, ErrNotOwner
	}
	u.Name, u.State = name, state
	b.users[name] = u
	b.recheckReaders()
	return b.userView(who, name), nil
}

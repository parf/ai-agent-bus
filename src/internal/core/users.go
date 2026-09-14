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
	in.Maintainer, in.DaemonOwner, in.CanEdit, in.CanActivate = false, false, false, false
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
		if reserved, ok := b.retired[in.Name]; ok && reserved != in.Name {
			return protocol.User{}, ErrNotOwner
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
	if u.State == "" {
		u.State = "active"
	}
	u.DaemonOwner = name == b.admin
	u.Maintainer = b.isMaintainer(name)
	u.CanEdit = b.mayEditUser(caller, name)
	u.CanActivate = u.CanEdit && (u.State != "banned" || caller == b.admin)
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
		if r, known := b.recordOrReservation(name); !known || r.Owner == name {
			names[name] = true
		}
	}
	out := []protocol.User{}
	for name := range names {
		if caller == name || b.isMaintainer(caller) {
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

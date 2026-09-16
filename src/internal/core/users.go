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

// Authenticate says whether this name may act at all, and says why not in two
// different ways, because they are two different answers to give a caller.
//
// A name the daemon holds nothing for but a credential is **not somebody it
// knows**: no profile, no record of its own. Holding a token for it is not
// access, and the answer is *who are you* rather than *you may not* — nothing
// it could be granted would help, because there is nobody to grant it to
// ([refusals](docs/05-discovery.md#refusals) separates those two codes).
// Unknown had been reading as active, since a name with no profile has no
// state and no state passes for the ordinary case.
//
// This asks who the daemon knows, which is the first half of what the
// [ownerless sweep](docs/02-access.md#ownerless-credentials) asks at start. The
// sweep is the more forgiving of the two while its interim guard stands: it
// keeps a credential whose name owns records, so that it does not strand them
// before orphan deletion exists. Kept is not accepted — a credential the sweep
// spares on that ground still answers for nobody here, and grants nothing.
func (b *Bus) Authenticate(name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.knows(name); err != nil {
		return err
	}
	if !b.active(name) {
		return ErrInactive
	}
	return nil
}

// Known is the same test asked before a credential is handed out, because a
// credential is issued **to somebody**. Minting one for a name the daemon holds
// nothing for is what filled the directory with names that answered for
// nothing: the credential was their only trace, and on its own it let them
// call. See docs/02-access.md#getting-a-token.
func (b *Bus) Known(name string) error {
	n, err := canon(name)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.knows(n)
}

// IssueFor mints under the registry lock, so that the name cannot stop being
// somebody between the check and the credential. Checking and then issuing
// left a window an unregistration fitted through, and the credential it
// produced outlived the name it was issued for.
//
// mint must only touch the credential store, never call back into Bus. Same
// shape and same hold as RemoveOwnerless, which is the other half of this:
// one place decides, and it is still deciding when the store is written.
func (b *Bus) IssueFor(name string, mint func(string) (string, error)) (string, error) {
	n, err := canon(name)
	if err != nil {
		return "", err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.knows(n); err != nil {
		return "", err
	}
	return mint(n)
}

// knows says whether the daemon holds anything for this name beyond a
// credential. Caller holds b.mu.
func (b *Bus) knows(name string) error {
	if b.identityKind(name) == protocol.DirectoryCredential {
		return fmt.Errorf("%w: %s has no profile and no record of its own", ErrNoPrincipal, name)
	}
	return nil
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
// ownedBy lists the other records this name owns, for a refusal that says what
// is in the way rather than only that something is. Caller holds b.mu.
func (b *Bus) ownedBy(name string) []string {
	out := []string{}
	for _, r := range b.records {
		if r.Owner == name && r.Name != name {
			out = append(out, r.Name)
		}
	}
	sort.Strings(out)
	return out
}

func (b *Bus) owns(name string) bool {
	for _, r := range b.records {
		if r.Owner == name {
			return true
		}
	}
	return false
}

// vouchedFor refuses to let a name in a realm somebody vouches for be brought
// into existence by asking. In such a realm you become the name by proving a
// key the directory publishes ([proving possession](docs/01-identity.md#proving-possession)),
// and every path that creates a name has to say so — registration and
// configuration alike, because both of them create. Caller holds b.mu.
func (b *Bus) vouchedFor(name string) error {
	n, err := protocol.ParseName(name)
	if err != nil {
		return nil
	}
	if _, backed := b.dirs[n.Realm]; backed {
		return fmt.Errorf("%w: %s is vouched for, so it is enrolled, not registered", ErrEnrol, n.Realm)
	}
	return nil
}

// mayOwn says whether owner may end up owning the record called name.
//
// Registering a record for an owner the daemon knows nothing about would leave
// it owned by nobody — the wreckage the
// [deletion rule](docs/01-identity.md#when-the-owner-is-gone) exists to clean
// up — so it is refused, and refusing it is what makes that rule's premise true
// rather than aspirational.
//
// A name not yet known may still be created **owned by itself**, because that
// is one call creating a principal rather than stranding a record. Reaching it
// from outside takes a caller whose record is removed while its own request is
// in flight, which is the gate-to-mutation window [H.5.8] closes for every verb
// at once rather than for this one clause; restricting it to enrolment here
// would narrow one route into that window and leave the window. What does
// reach it legitimately is
// [enrolment](docs/01-identity.md#proving-possession), which writes a
// self-owned record for a newcomer a realm vouched for.
// Caller holds b.mu.
func (b *Bus) mayOwn(owner, name string) error {
	if b.identityKind(owner) == protocol.DirectoryCredential && owner != name {
		return fmt.Errorf("%w: register %s before owning anything else", ErrNoPrincipal, owner)
	}
	if !b.active(owner) {
		return ErrInactive
	}
	return nil
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

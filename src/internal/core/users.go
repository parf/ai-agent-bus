package core

import (
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

const (
	AdministratorsGroup = "@administrators"
	// OwnerGroup is a contextual ACL term, never a stored group. On a record,
	// it means that record's direct Owner and the Service or Agent principals
	// the same Owner directly owns.
	OwnerGroup = "@owner"
)

func (b *Bus) active(name string) bool {
	state := b.users[name].State
	return state == "" || state == "active"
}

// carriesUserState says whether a record can have a person or an agent behind
// it to suspend. Only a user's queue and an agent's do: a queue, a pub/sub
// topic and an external service are not somebody, so a user record that
// happens to share a name says nothing about them and must not pause them.
// See docs/03-services-and-topics.md#five-record-kinds.
func (b *Bus) carriesUserState(name string) bool {
	r, known := b.records[name]
	if !known {
		return true // a bare name is asked about as a person
	}
	return r.Kind == protocol.KindUser || r.Kind == protocol.KindAgent
}

// activeName is active(), asked only of a name that could be suspended.
func (b *Bus) activeName(name string) bool {
	return !b.carriesUserState(name) || b.active(name)
}

// suspension says why calls involving name are refused, or nil. There are two
// ways for somebody to be behind a name — be it, or own it — and a suspension
// on either side refuses the same way: `403 suspended`, one suspension seen
// from either side, with nothing the caller can do about it in either case
// (docs/01-identity-and-roles.md#user-states).
//
// Deliberately **not** folded into active(). active asks about a name's own
// user state and is asked at a dozen places about people, where a record
// lookup would mean nothing; this asks about somebody else's state and only of
// a record.
//
// It is also kept out of visible(), which already merges: it reports
// r.Disabled OR !active(r.Name) (manage.go), so the bit says *delivery is off*
// without saying which of two reasons it is. A third input would merge a third
// distinct fact into a field that cannot carry the two it has
// (Plans/MVP/web/data-dictionary.md#fields). Owner suspension stays a separate
// question so a face can answer it separately, or not at all, rather than
// answering it wrongly.
//
// The message names the side because the code cannot: an operator reading a
// log should not have to guess which of two people is suspended, while the
// caller is told no more than it was already entitled to know.
// Caller holds b.mu.
func (b *Bus) suspension(name string) error {
	if !b.activeName(name) {
		return ErrInactive
	}
	return b.ownerSuspension(name)
}

// ownerSuspension is the half that is about **somebody else**: a service, and
// the person who owns it. It is separate from the name's own state, and the
// separation is load-bearing rather than tidy.
//
// Q63 permits an active, authorized caller to drain an inactive name's own
// inbox while Send refuses new deliveries (docs/01-identity-and-roles.md#user-states).
// This asks only about a separate owner and returns nil for a self-owned
// record, where there is no separate owner to ask about.
//
// It is also **not transitive**. If a service owns a service, suspending the
// person at the top does not reach the bottom one: the contract is *every
// service they own* (docs/01-identity-and-roles.md#user-states),
// and ownership is the direct relation the record states. Following the chain
// would be a different and larger rule.
// Caller holds b.mu.
func (b *Bus) ownerSuspension(name string) error {
	r, ok := b.records[name]
	if !ok || r.Owner == name || b.active(r.Owner) {
		return nil
	}
	return fmt.Errorf("%w: %s is owned by %s, whose access is suspended", ErrInactive, name, r.Owner)
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
	return b.acting(name)
}

// acting is the same two questions asked again where the answer is used.
//
// A gate that answers before the operation starts answers about a moment that
// has passed: the lock it took is released before core takes its own, and in
// between the caller can stop being a principal, be paused or banned, or lose
// the authority the operation is about. Asking once at the edge made every
// verb a check-then-act, and the act ran on the check's stale word.
//
// So this runs under the hold the operation writes under, and every predicate
// that decides authority — may, manages, mayEditUser — asks it rather than
// asking only whether a name is active. The gate stays, because refusing at
// the edge is cheaper and says the same thing, but it is no longer what the
// refusal rests on. See docs/02-access.md#what-a-call-carries.
// Caller holds b.mu.
func (b *Bus) acting(name string) error {
	if err := b.knows(name); err != nil {
		return err
	}
	// Both sides, at the edge and again here: a suspended person's service
	// holds a credential that is kept rather than revoked, and kept is not
	// accepted — while the state lasts it grants no access, theirs or their
	// services' (docs/01-identity-and-roles.md#user-states).
	return b.suspension(name)
}

// IssueFor answers the whole of who may have name's credential, and mints it,
// under one hold: that the caller is somebody, that name is somebody, and that
// the caller is entitled to name's credential.
//
// Ownership used to be established outside this hold, and that was worse than
// a stale read. A record can change hands, so between the ownership answer and
// the mint the target could be transferred away — and the credential handed
// over was the *current* owner's, issued to the former one. Asking here, where
// it is used, is the only way that cannot happen.
//
// mint must only touch the credential store, never call back into Bus. Same
// shape and same hold as RemoveOwnerless, which is the other half of this:
// one place decides, and it is still deciding when the store is written.
// See docs/02-access.md#getting-a-token.
func (b *Bus) IssueFor(caller, name string, mint func(string) (string, error)) (string, error) {
	who, err := canon(caller)
	if err != nil {
		return "", err
	}
	n, err := canon(name)
	if err != nil {
		return "", err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.acting(who); err != nil {
		return "", err
	}
	if err := b.knows(n); err != nil {
		return "", err
	}
	if who != b.admin && who != n {
		r, known := b.record(n)
		if !known || r.Owner != who {
			return "", fmt.Errorf("%w: %s does not own %s", ErrNotOwner, who, n)
		}
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
func (b *Bus) isAdministrator(name string) bool {
	return name == b.admin || b.member(name, AdministratorsGroup)
}
func (b *Bus) IsAdministrator(name string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.acting(name) == nil && b.isAdministrator(name)
}

// IsPerson says whether a name is somebody's identity rather than a service
// they registered. A person keeps their credential when an address of theirs
// is removed; a service does not (docs/01-identity-and-roles.md#unregistering).
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
// behind it is not (docs/01-identity-and-roles.md#users-and-profiles).
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
	// No interim guard any more. It kept the credential behind a name that
	// still owned services, so as not to strand them before there was
	// anything to delete them; Orphans runs first now, and a name with no
	// record and no profile owns nothing by the time this is asked — every
	// record it owned was wreckage and went with it.
	return b.identityKind(name) == protocol.DirectoryCredential
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
	if err := b.acting(who); err != nil {
		return err
	}
	if !b.isAdministrator(who) {
		return ErrNotOwner
	}
	if !b.ownerless(n) {
		return fmt.Errorf("%w: credential is now backed by a user or a record; refresh the directory", ErrBusy)
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
// key the directory publishes ([proving possession](docs/02-access.md#proving-possession)),
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
// [deletion rule](docs/01-identity-and-roles.md#orphaned-records) exists to clean
// up — so it is refused, and refusing it is what makes that rule's premise true
// rather than aspirational.
//
// There is no longer a clause letting an unknown name create itself. It was
// here because one caller legitimately needs it — a newcomer a realm vouched
// for, whose [enrolment](docs/02-access.md#proving-possession) writes it a
// self-owned record — and separating that caller from an ordinary one asking
// for the same thing was not possible while the gate's answer was stale by the
// time this ran. Now that it is not, enrolment says so for itself (the
// enrolled argument to register) and this asks for nothing but a principal.
// Caller holds b.mu.
func (b *Bus) mayOwn(owner, name string) error {
	if err := b.acting(owner); err != nil {
		if errors.Is(err, ErrNoPrincipal) {
			return fmt.Errorf("%w: register %s before owning anything", ErrNoPrincipal, owner)
		}
		return err
	}
	return nil
}

func (b *Bus) mayEditUser(caller, name string) bool {
	return b.acting(caller) == nil && (caller == b.admin || b.isAdministrator(caller) && caller != name && !b.isAdministrator(name))
}

func normalizedEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" {
		return "", nil
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || parsed.Name != "" || len(email) > 254 {
		return "", fmt.Errorf("%w: invalid email", ErrProfile)
	}
	for _, c := range email {
		if c > 127 {
			return "", fmt.Errorf("%w: email must use ASCII spelling", ErrProfile)
		}
	}
	return email, nil
}

func normalizedProfile(in protocol.User) (protocol.User, error) {
	name, err := canon(in.Name)
	if err != nil {
		return protocol.User{}, err
	}
	in.Name = name
	in.PersonName = strings.TrimSpace(in.PersonName)
	in.Email, err = normalizedEmail(in.Email)
	if err != nil {
		return protocol.User{}, err
	}
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
	in.GithubCompany = strings.TrimSpace(in.GithubCompany)
	in.GithubLocation = strings.TrimSpace(in.GithubLocation)
	in.GithubTwitterUsername = strings.TrimPrefix(strings.TrimSpace(in.GithubTwitterUsername), "@")
	if len(in.GithubCompany) > 200 || len(in.GithubLocation) > 200 || len(in.GithubTwitterUsername) > 64 {
		return protocol.User{}, fmt.Errorf("%w: profile field is too long", ErrProfile)
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
	// Company, location and Twitter/X are ordinary editable profile fields that
	// GitHub may populate. Provider provenance and image data remain derived: a
	// caller can never claim what GitHub answered or supply image bytes.
	in.GithubProfileAt = time.Time{}
	in.GithubAvatarURL, in.GithubGravatarID = "", ""
	in.PhotoPNG, in.PhotoSource, in.PhotoFetchedAt = nil, "", time.Time{}
	clearDerivedUser(&in)
	return in, nil
}

// clearDerivedUser keeps caller-specific and computed answers out of stored
// profiles. userView reconstructs them for the current caller. Durable profile,
// lifecycle and trusted-provider fields survive unchanged.
func clearDerivedUser(u *protocol.User) {
	u.Kind = ""
	u.Administrator, u.DaemonOwner = false, false
	u.CanEdit, u.CanSetEmail, u.CanActivate, u.CanRemove = false, false, false, false
	u.Groups, u.Services = nil, nil
}

func copyGithubDecoration(dst *protocol.User, src protocol.User) {
	dst.GithubProfileAt = src.GithubProfileAt
	dst.GithubAvatarURL = src.GithubAvatarURL
	dst.GithubGravatarID = src.GithubGravatarID
	dst.PhotoPNG = append([]byte(nil), src.PhotoPNG...)
	dst.PhotoSource = src.PhotoSource
	dst.PhotoFetchedAt = src.PhotoFetchedAt
}

func clearGithubDecoration(u *protocol.User) {
	u.GithubProfileAt = time.Time{}
	u.GithubAvatarURL, u.GithubGravatarID = "", ""
	u.PhotoPNG, u.PhotoSource, u.PhotoFetchedAt = nil, "", time.Time{}
}

// applyGithubProfile imports one trusted provider answer. Caller holds b.mu.
// PersonName and Email fill blanks and then become ordinary AgentBus fields;
// company, location and Twitter/X are editable profile fields. A successful
// explicit refresh replaces them; a login setup fills them unless the same
// administrative write supplied an explicit nonblank value.
func (b *Bus) applyGithubProfile(u *protocol.User, p ports.DirectoryProfile, name string) error {
	if p.Login == "" && p.FetchedAt.IsZero() {
		return nil
	}
	if p.Login != "" && !strings.EqualFold(p.Login, u.GithubUser) {
		return fmt.Errorf("%w: GitHub profile answered for another login", ErrProfile)
	}
	personName := strings.TrimSpace(p.PersonName)
	if len(personName) > 200 {
		return fmt.Errorf("%w: person name is too long", ErrProfile)
	}
	company, location := strings.TrimSpace(p.Company), strings.TrimSpace(p.Location)
	twitter := strings.TrimPrefix(strings.TrimSpace(p.TwitterUsername), "@")
	if len(company) > 200 || len(location) > 200 || len(twitter) > 64 || len(p.AvatarURL) > 2048 || len(p.GravatarID) > 200 {
		return fmt.Errorf("%w: GitHub profile field is too long", ErrProfile)
	}
	if u.PersonName == "" {
		u.PersonName = personName
	}
	if u.Email == "" && p.Email != "" {
		if email, err := normalizedEmail(p.Email); err == nil {
			available := true
			for other, profile := range b.users {
				if other != name && profile.Email == email {
					available = false
					break
				}
			}
			if available {
				u.Email = email
			}
		}
	}
	u.GithubProfileAt = p.FetchedAt
	u.GithubCompany, u.GithubLocation, u.GithubTwitterUsername = company, location, twitter
	u.GithubAvatarURL, u.GithubGravatarID = strings.TrimSpace(p.AvatarURL), strings.TrimSpace(p.GravatarID)
	switch {
	case len(p.PhotoPNG) != 0:
		u.PhotoPNG = append([]byte(nil), p.PhotoPNG...)
		u.PhotoSource, u.PhotoFetchedAt = p.PhotoSource, p.PhotoFetchedAt
	case p.ClearPhoto:
		u.PhotoPNG, u.PhotoSource, u.PhotoFetchedAt = nil, "", time.Time{}
	}
	return nil
}

// githubChange performs the optional network half of a GitHub-login change
// without holding the bus mutex. It first checks authority so an untrusted
// caller cannot turn the daemon into a provider request proxy. Provider
// metadata is decoration: an unavailable lookup leaves it unobserved but does
// not refuse a valid, unique login. The observed login is compared again at
// commit to refuse stale overwrites.
func (b *Bus) githubChange(caller, name, login string) (ports.DirectoryProfile, string, bool, error) {
	b.mu.Lock()
	if err := b.acting(caller); err != nil {
		b.mu.Unlock()
		return ports.DirectoryProfile{}, "", false, err
	}
	if !b.mayEditUser(caller, name) {
		b.mu.Unlock()
		return ports.DirectoryProfile{}, "", false, ErrNotOwner
	}
	oldLogin := b.users[name].GithubUser
	provider := b.github
	b.mu.Unlock()
	if oldLogin == login {
		return ports.DirectoryProfile{}, oldLogin, false, nil
	}
	if login == "" {
		return ports.DirectoryProfile{}, oldLogin, true, nil
	}
	if provider == nil {
		return ports.DirectoryProfile{}, oldLogin, true, nil
	}
	p, err := provider.Profile(login)
	if err != nil {
		return ports.DirectoryProfile{}, oldLogin, true, nil
	}
	return p, oldLogin, true, nil
}

// EditOwnEmail is deliberately narrower than SetUser. The credential supplies
// the identity, and the operation carries only the one profile field a user
// may vouch for themselves. Person name and GitHub identity keep their trusted
// sources (docs/01-identity-and-roles.md#users-and-profiles).
func (b *Bus) EditOwnEmail(caller, raw string) (protocol.User, error) {
	who, err := canon(caller)
	if err != nil {
		return protocol.User{}, err
	}
	email, err := normalizedEmail(raw)
	if err != nil {
		return protocol.User{}, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.acting(who); err != nil {
		return protocol.User{}, err
	}
	u, exists := b.users[who]
	if !exists {
		return protocol.User{}, ErrUnknown
	}
	for name, other := range b.users {
		if name != who && email != "" && other.Email == email {
			return protocol.User{}, fmt.Errorf("%w: identifying field already belongs to another user", ErrProfile)
		}
	}
	u.Email = email
	b.users[who] = u
	if err := b.checkpoint(false); err != nil {
		return protocol.User{}, err
	}
	return b.userView(who, who), nil
}

// SetUser is the administrative profile/lifecycle write path. Registration
// cannot vouch for a person's fields, change their state or promote their
// authority; EditOwnEmail is the separate narrow self-service path.
func (b *Bus) SetUser(caller string, in protocol.User, create bool) (protocol.User, error) {
	return b.SetUserWithProfileDetails(caller, in, create, true)
}

// SetUserWithProfileDetails preserves company, location and Twitter/X for
// older writers that do not carry those editable fields. A writer setting
// profileDetails explicitly replaces the complete three-field set, including
// clearing it. SetUser is the in-process full-profile form.
func (b *Bus) SetUserWithProfileDetails(caller string, in protocol.User, create, profileDetails bool) (protocol.User, error) {
	who, err := canon(caller)
	if err != nil {
		return protocol.User{}, err
	}
	in, err = normalizedProfile(in)
	if err != nil {
		return protocol.User{}, err
	}
	providerProfile, observedGithub, githubChanged, err := b.githubChange(who, in.Name, in.GithubUser)
	if err != nil {
		return protocol.User{}, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	// Before the authority question, because "you are nobody" and "you are
	// suspended" are not "that is not yours": they are different codes to a
	// caller (docs/05-discovery.md#refusals), and a predicate that answers
	// true or false cannot tell them apart.
	if err := b.acting(who); err != nil {
		return protocol.User{}, err
	}
	if !b.mayEditUser(who, in.Name) {
		return protocol.User{}, ErrNotOwner
	}
	old, exists := b.users[in.Name]
	if exists && !profileDetails {
		in.GithubCompany, in.GithubLocation, in.GithubTwitterUsername = old.GithubCompany, old.GithubLocation, old.GithubTwitterUsername
	}
	if githubChanged && old.GithubUser != observedGithub {
		return protocol.User{}, fmt.Errorf("%w: GitHub login changed while its profile was being fetched; retry", ErrBusy)
	}
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
	if old.State == "banned" && in.State != "banned" && who != b.admin && !b.isAdministrator(who) {
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
	if !githubChanged {
		copyGithubDecoration(&in, old)
	} else if in.GithubUser == "" {
		clearGithubDecoration(&in)
	} else {
		// Photo import is optional even when the login changes. Seed only the
		// last normalized thumbnail so a failed fetch retains that fallback;
		// applyGithubProfile replaces or explicitly clears it from the complete
		// new provider answer. No other old provider fact crosses the change.
		in.PhotoPNG = append([]byte(nil), old.PhotoPNG...)
		in.PhotoSource, in.PhotoFetchedAt = old.PhotoSource, old.PhotoFetchedAt
		company, location, twitter := in.GithubCompany, in.GithubLocation, in.GithubTwitterUsername
		if err := b.applyGithubProfile(&in, providerProfile, in.Name); err != nil {
			return protocol.User{}, err
		}
		if company != "" {
			in.GithubCompany = company
		}
		if location != "" {
			in.GithubLocation = location
		}
		if twitter != "" {
			in.GithubTwitterUsername = twitter
		}
	}
	// A vouched realm still requires key-possession proof before its name can
	// be created. Editing an existing enrolled profile does not repeat proof.
	if _, known := b.records[in.Name]; !known {
		parsed, _ := protocol.ParseName(in.Name)
		if _, backed := b.dirs[parsed.Realm]; backed {
			return protocol.User{}, ErrEnrol
		}
		b.records[in.Name] = protocol.Record{Name: in.Name, Owner: in.Name, Kind: protocol.KindUser, Full: protocol.OverflowStrict, At: time.Now()}
		b.ensure(in.Name)
	}
	b.users[in.Name] = in
	b.recheckReaders()
	if err := b.checkpoint(false); err != nil {
		return protocol.User{}, err
	}
	return b.userView(who, in.Name), nil
}

// RefreshGithub re-reads an existing GitHub login without treating an
// ordinary profile save as provider I/O. Required profile failure changes
// nothing; optional photo failure retains the previous normalized thumbnail.
func (b *Bus) RefreshGithub(caller, name string) (protocol.User, error) {
	who, err := canon(caller)
	if err != nil {
		return protocol.User{}, err
	}
	name, err = canon(name)
	if err != nil {
		return protocol.User{}, err
	}
	b.mu.Lock()
	if err := b.acting(who); err != nil {
		b.mu.Unlock()
		return protocol.User{}, err
	}
	if !b.mayEditUser(who, name) {
		b.mu.Unlock()
		return protocol.User{}, ErrNotOwner
	}
	old, exists := b.users[name]
	provider := b.github
	b.mu.Unlock()
	if !exists {
		return protocol.User{}, ErrUnknown
	}
	if old.GithubUser == "" {
		return protocol.User{}, fmt.Errorf("%w: user has no GitHub login", ErrProfile)
	}
	if provider == nil {
		return protocol.User{}, fmt.Errorf("%w: GitHub profile lookup is unavailable", ErrProfile)
	}
	p, err := provider.Profile(old.GithubUser)
	if err != nil {
		return protocol.User{}, fmt.Errorf("%w: GitHub profile: %s", ErrProfile, err)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.acting(who); err != nil {
		return protocol.User{}, err
	}
	if !b.mayEditUser(who, name) {
		return protocol.User{}, ErrNotOwner
	}
	current, exists := b.users[name]
	if !exists {
		return protocol.User{}, ErrUnknown
	}
	if current.GithubUser != old.GithubUser {
		return protocol.User{}, fmt.Errorf("%w: GitHub login changed while its profile was being fetched; retry", ErrBusy)
	}
	if err := b.applyGithubProfile(&current, p, name); err != nil {
		return protocol.User{}, err
	}
	b.users[name] = current
	if err := b.checkpoint(false); err != nil {
		return protocol.User{}, err
	}
	return b.userView(who, name), nil
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
	u.Administrator = b.isAdministrator(name)
	u.CanEdit = u.Kind == protocol.DirectoryUser && b.mayEditUser(caller, name)
	u.CanSetEmail = u.Kind == protocol.DirectoryUser && caller == name && b.acting(caller) == nil
	u.CanActivate = u.CanEdit
	u.CanRemove = b.acting(caller) == nil && b.isAdministrator(caller) && b.ownerless(name)
	for group := range b.groups {
		if b.member(name, group) {
			u.Groups = append(u.Groups, group)
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
	// Including the caller's own row: a name that may not act is shown
	// nothing, and one of the rows this can produce is a junk credential,
	// which must not be able to look itself up.
	if b.acting(caller) != nil {
		return []protocol.User{}
	}
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
		if caller == name || b.acting(caller) == nil && b.isAdministrator(caller) {
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
	if err := b.acting(who); err != nil {
		return protocol.User{}, err
	}
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
	if name == b.admin && state != "active" || u.State == "banned" && state != "banned" && who != b.admin && !b.isAdministrator(who) {
		return protocol.User{}, ErrNotOwner
	}
	u.Name, u.State = name, state
	b.users[name] = u
	b.recheckReaders()
	if err := b.checkpoint(false); err != nil {
		return protocol.User{}, err
	}
	return b.userView(who, name), nil
}

// Package core is the domain: the registry and the inboxes. It touches
// nothing outside itself — no HTTP, no files, no clock beyond time.Now.
// See docs/10-modules.md.
package core

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/parf/ai-agent-bus/internal/activity"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Bounds. A queue that grows without limit is a memory leak with a friendly
// name. What happens at the bound is the receiver's choice, and it
// refuses unless it asked for a ring — see docs/04-messaging.md#overflow.
const maxQueue = 1000

var (
	ErrProfile  = errors.New("invalid user profile")
	ErrInactive = errors.New("user access is suspended")
	// Held a credential, and the daemon holds nothing else for the name: no
	// profile and no record of its own. That is not a state to recover from,
	// it is nobody (docs/02-access.md#what-a-call-carries).
	ErrNoPrincipal = errors.New("that credential answers for nobody")
	// ErrStaleCredential is a credential whose User/Agent pair is not who
	// the name is now: refused as if it did not exist.
	ErrStaleCredential = errors.New("that credential no longer answers for that name")
	ErrUnknown         = errors.New("no such name")
	ErrTwoReads        = errors.New("inbox already has a reader, and neither asked to share it")
	ErrBadName         = errors.New("bad name")
	ErrReceipt         = errors.New(`a receipt is "ack" or "done"`)
	ErrFull            = errors.New("the receiver's queue is full")
	ErrOverflow        = errors.New("overflow is strict or ring")
	ErrKind            = errors.New("unknown record kind")
	ErrConfig          = errors.New("a configuration is JSON")
	ErrTTL             = errors.New("a ttl is a duration, like 30s")
	ErrWait            = errors.New("a wait is a duration, like 30s")
	ErrBound           = errors.New("a bound is a positive number of messages")
	ErrNotOwner        = errors.New("that record belongs to someone else")
	ErrExists          = errors.New("that name is already registered")
	ErrBusy            = errors.New("cannot unregister a busy inbox")
	ErrPrivate         = errors.New("a configuration is private to the record it belongs to")
	ErrSecret          = errors.New("a secret is stored on an agent, a service or a group and on no other kind")
	ErrNoSecret        = errors.New("that service holds no secret")
	ErrNotAllow        = errors.New("not on that record's allow list")
	ErrPersonal        = errors.New("a personal record's allow and maintainers name only its owner and the owner's agents")
	ErrEnrol           = errors.New("enrolment")
	// A group is retired by emptying its membership
	// (docs/01-identity-and-roles.md#groups), so there is no removal to
	// ask for. A request that asks anyway is refused rather than read as a
	// membership change: emptying a group leaves every record that names it
	// alone, and unmapping the name would not have.
	ErrNoRemoval = errors.New("a group is retired by emptying its membership; there is no removal")
	// Every internal ID has been handed out once; none is ever reused.
	ErrExhausted = errors.New("no internal IDs are left to give a new entity")
)

// canon normalises a name so that "  x@y " and "x@y" are the same inbox.
// It lives here, not in a face, so every face gets the same answer.
func canon(s string) (string, error) {
	// A Group's name is "@" and a name by the same rules, trimmed and
	// lower-cased like any other (docs/constitution.md#-group).
	if t := strings.ToLower(strings.TrimSpace(s)); strings.HasPrefix(t, "@") {
		if !groupName(t) {
			return "", fmt.Errorf("%w: invalid group name %q", ErrBadName, s)
		}
		return t, nil
	}
	n, err := protocol.ParseName(s)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrBadName, err)
	}
	// Already canonical is the common case, and costs nothing to answer.
	if n.Is(s) {
		return s, nil
	}
	return n.String(), nil
}

// waiter is one blocked consume. A filtered waiter takes only the message it
// is waiting for; the unfiltered reader takes anything else.
//
// share is how the daemon tells a worker pool from an accidental second
// reader: a pool says so. See docs/04-messaging.md#one-reader-per-inbox.
type waiter struct {
	topic, tag string
	filtered   bool
	share      bool
	ch         chan protocol.Envelope
	caller     string
	stopped    chan error
}

type inbox struct {
	// What has been through it, since the daemon started. Observed, like
	// the rest of the live state: counted here and attached on the way out,
	// never taken from a caller. A queue that is drained and one nobody
	// ever wrote to both read as empty, and these tell them apart.
	in, out int
	refused int
	// Its own loss, not the daemon's, for the reason protocol.Record.Dropped
	// gives.
	dropped, expired int

	queue   []protocol.Envelope
	waiters []*waiter
	// act is its last day of traffic, differenced from the counters above at
	// each ten-minute boundary (activity.go).
	act activity.Ring
}

type Bus struct {
	store ports.Store
	// journal is where conceptual errors and warnings are reported
	// (docs/constitution.md#errors-and-alerts).
	journal ports.Journal
	// staging is the undo log of the management write in progress (commit.go).
	staging *staged
	// creds is the face's credential index, bound at start (credentials.go).
	creds ports.CredentialIndex
	// ignored names the stored records this start ignored (snapshot.go).
	ignored map[string]bool
	// flushed is each queue's counters as last saved, so a queue flush writes
	// only what moved since (snapshot.go).
	flushed map[string]queueMark
	// nextRecordID and nextUserID are the next internal IDs; they only grow,
	// so an ID is never reused (docs/constitution.md#common-record-fields).
	// recordByID and userByID are the derived indexes, rebuilt on load.
	nextRecordID, nextUserID uint32
	recordByID               map[uint32]string
	userByID                 map[uint32]string
	idsExhausted             bool
	accounts                 map[string]string
	// activeAccounts is what the supervisor actually opened for this run.
	// accounts may move ahead after a durable administrative edit; the
	// difference is the precise "restart required" fact returned to callers.
	activeAccounts    map[string]string
	accountsRestored  bool
	accountRestoreErr error
	users             map[string]protocol.User
	mu                sync.Mutex
	// node is the node-wide refusals' day, the daemon Owner's Refused series
	// (activity.go). clock is the time as the bus reads it; a test moves it.
	node    activity.Ring
	clock   func() time.Time
	records map[string]protocol.Record
	admin   string
	// ownerRestored means a current snapshot, rather than setup, supplied
	// admin. ownerRestoreErr is retained until startup validates that durable
	// authority before serving anything.
	ownerRestored   bool
	ownerRestoreErr error

	// How many calls were refused, and for what. Counted because a bus that
	// is quiet and one that is refusing everything look identical from
	// outside — see docs/05-discovery.md#what-it-shows.
	refused map[string]int
	// Whether the snapshot this run read back was written by a graceful
	// stop. Logged once at start is not enough: whoever comes to look at a
	// gap in the work arrives long after the log line scrolled away.
	unclean bool
	inboxes map[string]*inbox
	started time.Time
	// What the bus has seen lately, bodies struck out — the dashboard's
	// only source. See docs/05-discovery.md#dashboard.
	recent []protocol.Envelope
	// Which realms are backed by a directory, what verifies a signature,
	// and the challenges outstanding. See docs/01-identity-and-roles.md#registration.
	dirs    map[string]ports.Directory
	github  ports.ProfileDirectory
	sigs    ports.Signatures
	pending map[string]challenge
}

func New() *Bus {
	return &Bus{
		accounts:       map[string]string{},
		activeAccounts: map[string]string{},
		records:        map[string]protocol.Record{},
		ignored:        map[string]bool{},
		users:          map[string]protocol.User{},
		refused:        map[string]int{},
		inboxes:        map[string]*inbox{},
		flushed:        map[string]queueMark{},
		nextRecordID:   1,
		nextUserID:     1,
		recordByID:     map[uint32]string{},
		userByID:       map[uint32]string{},
		started:        time.Now(),
		clock:          time.Now,
	}
}

// Register states a record. A name in a realm a directory backs cannot be
// created this way — it has to be enrolled, or the first caller to ask for a
// name would become it. See docs/01-identity-and-roles.md#registration.
// onBus reports whether a record has a queue here. Four of the five kinds are
// names something on this bus reads; a service is reached at its own address by
// whoever wants it, so nothing is delivered to it, nothing is consumed from it,
// and it never has a queue to hold, bound or expire.
// See docs/03-records.md#record-kinds.
func onBus(r protocol.Record) bool {
	return r.Kind != protocol.KindService && r.Kind != protocol.KindGroup
}

// validateKind is what every stored record must satisfy, whichever path
// stores it. A service describes something this bus does not run, so it has to
// say where that thing is and how to reach it: without both, the record is a
// name with nothing behind it and no way to find out. For the same reason it
// carries no queue settings and no delivery switch — there is no queue for
// them to be about.
// See docs/03-records.md#record-kinds.
func validateKind(r protocol.Record) error {
	if !protocol.ValidKind(r.Kind) {
		return fmt.Errorf("%w: %q is not one of %s", ErrKind, r.Kind, protocol.KindNames())
	}
	if err := validStatus(r.Status); err != nil {
		return err
	}
	// An Agent's name begins with "#" and nothing else's does, so the name
	// alone says the kind (docs/constitution.md#actors-and-ascii-textarea-syntax).
	// Neither is completed, guessed or tolerated.
	if agent := protocol.IsAgentName(r.Name); r.Kind == protocol.KindAgent && !agent {
		return fmt.Errorf("%w: an agent's name begins with #, and %s does not", ErrKind, r.Name)
	} else if r.Kind != protocol.KindAgent && agent {
		return fmt.Errorf("%w: only an agent's name begins with #, and %s is a %s", ErrKind, r.Name, r.Kind)
	}
	// A Group's name begins with "@" and nothing else's does. A Group is a
	// list of actors and nothing more: no queue, no address, no delivery of
	// its own (docs/constitution.md#-group).
	if group := strings.HasPrefix(r.Name, "@"); r.Kind == protocol.KindGroup && !group {
		return fmt.Errorf("%w: a group's name begins with @, and %s does not", ErrKind, r.Name)
	} else if r.Kind != protocol.KindGroup && group {
		return fmt.Errorf("%w: only a group's name begins with @, and %s is a %s", ErrKind, r.Name, r.Kind)
	}
	// A User's own record is its User's, and carries neither allow nor
	// maintainers: who reaches it follows the User delivery rule
	// (docs/constitution.md#common-record-fields).
	if r.Kind == protocol.KindUser {
		if r.Owner != "" && r.Owner != r.Name {
			return fmt.Errorf("%w: a user record belongs to its own user", ErrKind)
		}
		if len(r.Allow) != 0 || len(r.Maintainers) != 0 {
			return fmt.Errorf("%w: a user record has no allow list and no maintainers", ErrKind)
		}
	}
	if r.Kind == protocol.KindGroup {
		if reservedTerm(r.Name) {
			return fmt.Errorf("%w: %s is a runtime term, never a stored group", ErrBadName, r.Name)
		}
		if r.Addr != "" || r.Proto != "" || r.TTL != "" || r.Bound != 0 || r.Full != "" {
			return fmt.Errorf("%w: a group holds no queue and no address", ErrKind)
		}
		// The protected group has no Maintainers and is never Personal:
		// its membership is the daemon Owner's alone (docs/constitution.md#-group).
		if r.Name == AdministratorsGroup && (len(r.Maintainers) != 0 || r.Personal) {
			return fmt.Errorf("%w: %s has no maintainers and is never personal", ErrNotOwner, AdministratorsGroup)
		}
		// A prefix reserves the name for its User, and a Personal group
		// must carry its owner's (docs/constitution.md#-group).
		prefix, prefixed := groupOwner(r.Name)
		if prefixed && r.Owner != "" && prefix != r.Owner {
			return fmt.Errorf("%w: %s is reserved for %s", ErrNotOwner, r.Name, prefix)
		}
		if r.Personal && !prefixed {
			return fmt.Errorf("%w: a personal group is named @<owner>/<name>, and %s is not", ErrKind, r.Name)
		}
	}
	// Deliver-To is what a 📣 does instead of holding a queue, so it is the
	// one kind that has one: the other three receive rather than fan out.
	// See docs/04-messaging.md#subscribers.
	// deliver_to: a list on a 📣, one slot on an 👾 or 📮, and nothing on any
	// other kind (docs/constitution.md#common-record-fields).
	switch r.Kind {
	case protocol.KindPubSub:
	case protocol.KindAgent, protocol.KindQueue:
		if len(r.Subs) > 1 {
			return fmt.Errorf("%w: a %s's deliver_to holds one destination, not %d", ErrKind, r.Kind, len(r.Subs))
		}
	default:
		if len(r.Subs) != 0 {
			return fmt.Errorf("%w: a %s delivers to nobody, so it carries no deliver_to", ErrKind, r.Kind)
		}
	}
	// config and secret are private values of the kinds the field table gives
	// them, and of no other (docs/constitution.md#-private-values).
	if !holdsPrivate(r.Kind) {
		if r.Secret != "" || r.SecretSHA != "" {
			return fmt.Errorf("%w: a %s holds no secret", ErrKind, r.Kind)
		}
		if len(r.Config) > 0 || r.ConfigSHA != "" {
			return fmt.Errorf("%w: a %s holds no configuration", ErrKind, r.Kind)
		}
	}
	// The target state is checked as the write was: a stored secret that is
	// not an env file, or a configuration that is not compact JSON, is one no
	// write of this version produced, and no legacy exception is loaded.
	if r.Secret != "" {
		if err := validEnv(r.Secret); err != nil {
			return err
		}
	}
	if len(r.Config) > 0 {
		var compact bytes.Buffer
		if err := json.Compact(&compact, r.Config); err != nil || !bytes.Equal(compact.Bytes(), r.Config) {
			return fmt.Errorf("%w: a stored configuration must be compact JSON", ErrConfig)
		}
	}
	if r.Kind != protocol.KindService {
		return nil
	}
	if r.Addr == "" || r.Proto == "" {
		return fmt.Errorf("%w: a service needs an address and a protocol", ErrKind)
	}
	if r.TTL != "" || r.Bound != 0 || r.Full != "" {
		return fmt.Errorf("%w: a service has no queue here, so it takes no TTL, capacity or overflow policy", ErrKind)
	}
	return nil
}

// holdsPrivate says whether a kind carries config and secret: an Agent, a
// Service and a Group do, and nothing else (docs/constitution.md#common-record-fields).
func holdsPrivate(kind string) bool {
	return kind == protocol.KindAgent || kind == protocol.KindService || kind == protocol.KindGroup
}

// mayReadPrivate is who reads a record's private values: its own principal
// where it has one, and otherwise the actors its allow list admits.
// Caller holds b.mu.
func (b *Bus) mayReadPrivate(caller string, r protocol.Record) bool {
	if r.Kind == protocol.KindAgent {
		return caller == r.Name && b.acting(caller) == nil
	}
	return b.may(caller, r)
}

func (b *Bus) Register(r protocol.Record) (protocol.Record, error) {
	return b.register(r, false, false, ports.DirectoryProfile{})
}

// RegisterNew claims a name without replacing even the caller's own record.
func (b *Bus) RegisterNew(r protocol.Record) (protocol.Record, error) {
	return b.register(r, false, true, ports.DirectoryProfile{})
}

func (b *Bus) register(r protocol.Record, enrolled, createOnly bool, profile ports.DirectoryProfile) (protocol.Record, error) {
	name, err := canon(r.Name)
	if err != nil {
		return protocol.Record{}, err
	}
	r.Name = name
	if owner, err := canon(r.Owner); err == nil {
		r.Owner = owner
	}
	// A caller that states nothing gets the external case: registering by
	// hand is how a thing that is not on this bus gets described, and every
	// caller that means one of the other four says so.
	// See docs/03-records.md#record-kinds.
	if r.Kind == "" {
		r.Kind = protocol.KindService
		if protocol.IsAgentName(name) {
			r.Kind = protocol.KindAgent
		}
	}
	// The face puts the caller in Owner. What is stored is the User the
	// caller is or acts for: every record is owned by a User, and an Agent
	// that creates one gains no authority over it by doing so
	// (docs/constitution.md#-registry-record). A registration that names
	// nobody is a User's own name registering itself.
	caller := r.Owner
	if caller == "" {
		caller = name
	}
	// An overflow policy is about a queue, so only a record that has one gets
	// the default. A service stating one is a caller error, which validateKind
	// below is where it is told.
	if r.Full == "" && onBus(r) {
		r.Full = protocol.OverflowStrict
	}
	if r.Full != "" && r.Full != protocol.OverflowStrict && r.Full != protocol.OverflowRing {
		return protocol.Record{}, fmt.Errorf("%w, not %q", ErrOverflow, r.Full)
	}
	// The shape alone here, before the lock: whose it is, and so whether a
	// user record is its own, is known only once the owner is resolved, and
	// a name somebody else holds is refused for that first.
	shape := r
	if shape.Kind == protocol.KindUser {
		shape.Owner = shape.Name
	}
	if err := validateKind(shape); err != nil {
		return protocol.Record{}, err
	}
	b.mu.Lock()
	defer b.unlock()
	// Enrolment is the one caller that legitimately registers a name the
	// daemon does not yet know — it has just proved the realm's key for it
	// (docs/02-access.md#proving-possession) — and it says so here rather
	// than through a clause in mayOwn that every other caller could reach.
	if enrolled {
		r.Owner = name
	} else {
		owner, err := b.ownerFor(caller)
		if err != nil {
			return protocol.Record{}, err
		}
		if err := b.mayOwn(owner, name); err != nil {
			return protocol.Record{}, err
		}
		r.Owner = owner
	}
	if _, exists := b.records[name]; createOnly && exists {
		return protocol.Record{}, ErrExists
	}
	// Refusing a *new* name in a vouched-for realm is what turns "the owner
	// of a record may have its credential" from a hole into a rule: you
	// become the name by proving you hold its key, not by asking first.
	// See docs/01-identity-and-roles.md#registration.
	if _, known := b.records[name]; !known && !enrolled {
		if err := b.vouchedFor(name); err != nil {
			return protocol.Record{}, err
		}
	}
	// Registering refreshes a description. It is not a way to take a record
	// over or to throw its configuration away: a service registers itself on
	// every start, and that must not destroy what it was configured with.
	// See docs/03-records.md#configuring-a-template.
	//
	// A registration also never carries either half of a configuration:
	// the bytes have one write path and this is not it, and the digest is
	// derived from them (protocol.Record.Public), so accepting one from a
	// caller would let anyone claim any setup — which is what the digest
	// exists to detect.
	// The same applies to the live fields: they are what the daemon
	// observes, attached to an answer on the way out, so a caller stating
	// them would be claiming a reader it does not have.
	if r.TTL != "" {
		d, err := time.ParseDuration(r.TTL)
		if err != nil || d <= 0 {
			return protocol.Record{}, fmt.Errorf("%w, not %q", ErrTTL, r.TTL)
		}
	}
	if r.Bound < 0 {
		return protocol.Record{}, fmt.Errorf("%w, not %d", ErrBound, r.Bound)
	}
	// A registration carries neither half of either private field: not the
	// bytes, and not the digest, which is derived from them and would
	// otherwise let anyone claim any setup or any credential. Each has one
	// verb that writes it. See docs/06-services.md#secrets.
	// Deliver-To may come with a registration: creating a 📣 and saying who
	// it delivers to is one act, and whoever registers the name owns it.
	// Checked here rather than in validateKind because the list is checked
	// against the registry, which needs the lock.
	// See docs/04-messaging.md#subscribers.
	if len(r.Subs) > 0 {
		list, err := b.normalizeDeliverTo(r.Subs, r)
		if err != nil {
			return protocol.Record{}, err
		}
		r.Subs = list
	}
	if r.Allow != nil {
		allow, err := b.normalizeAllow(r.Allow, r)
		if err != nil {
			return protocol.Record{}, err
		}
		r.Allow = allow
	}
	r.Config, r.ConfigSHA = nil, ""
	r.Secret, r.SecretSHA = "", ""
	// A registration states no status: a new record is active, and an
	// existing one keeps its own.
	r.Maintainers, r.Status = nil, ""
	clearLiveRecord(&r)
	// Publishing a name is open to anyone; changing one that exists belongs
	// to its owner, and to the record itself — a service registering on
	// every start is not a stranger to its own name, and it is the only
	// other principal that could hold that name's credential.
	// See docs/01-identity-and-roles.md#ownership.
	if old, known := b.record(name); known {
		// An inactive record keeps its name reserved: nothing registers over
		// it, and only a status edit brings it back
		// (docs/constitution.md#common-record-fields).
		if !b.live(old) {
			return protocol.Record{}, fmt.Errorf("%w: %s is reserved by an inactive record", ErrExists, name)
		}
		// A record's kind is what it is for good: a registration cannot turn
		// a user's own record, or anything else, into another kind
		// (docs/constitution.md#authority-rules).
		if old.Kind != r.Kind {
			return protocol.Record{}, fmt.Errorf("%w: %s is a %s, and a registration does not change a kind", ErrExists, name, old.Kind)
		}
		if old.Owner != "" && !b.manages(caller, old) {
			return protocol.Record{}, fmt.Errorf("%w: %s is %s's", ErrNotOwner, name, old.Owner)
		}
		r.Config = old.Config
		// Kept for the same reason a configuration is: a service refreshing
		// its metadata on every start must not lose its credential.
		r.Secret = old.Secret
		r.Owner = old.Owner
		// Deliver-To follows the ACL's rule rather than the configuration's:
		// a re-registration that states no list keeps the one the record has,
		// and one that states a list replaces it. Only a manager reaches this
		// branch at all, so an explicit replacement is a manager's act.
		if r.Subs == nil {
			r.Subs = old.Subs
		}
		r.Maintainers, r.Status = append(protocol.MaintainerList(nil), old.Maintainers...), old.Status
		// Personal is the owner's classification. A service refreshes its own
		// metadata on every start and cannot clear or set that owner choice.
		r.Personal = old.Personal
		// Metadata refreshes must not erase grants. Explicit ACLs still replace
		// the policy; Manage can clear it deliberately.
		if r.Allow == nil {
			r.Allow = old.Allow
		}
	}
	if err := validateKind(r); err != nil {
		return protocol.Record{}, err
	}
	if err := b.validatePersonal(r); err != nil {
		return protocol.Record{}, err
	}
	r.At = time.Now()

	if enrolled {
		user := b.users[name]
		user.Name = name
		parsed, _ := protocol.ParseName(name)
		if parsed.Realm == "github" {
			for other, existing := range b.users {
				if other != name && existing.GithubUser == parsed.Local {
					return protocol.Record{}, fmt.Errorf("%w: GitHub login already belongs to another profile", ErrProfile)
				}
			}
			user.GithubUser = parsed.Local
		}
		// The directory is a trusted source, but it does not outrank an
		// Administrator's explicit profile edit. Re-enrolment fills a blank and
		// never overwrites a non-empty person name.
		if err := b.applyGithubProfile(&user, profile, name); err != nil {
			return protocol.Record{}, err
		}
		if user.Status == "" {
			user.Status = protocol.StatusActive
		}
		b.setUser(name, user)
	}
	b.setRecord(name, r)
	b.recheckInbox(name)
	if onBus(r) {
		b.ensure(name)
	}
	if err := b.commit(); err != nil {
		return protocol.Record{}, err
	}
	// Public here, not in the face: the configuration is core's to guard,
	// and an answer that forgot to redact has already got out twice.
	return r.Public(), nil
}

// Configure attaches a configuration to a record, creating it if it does not
// exist yet: configuring an agent template is what produces a configured
// name. What it creates is an agent, the kind that has a queue from the moment
// it exists — a service could not be created here, having no address to be
// registered with. See docs/03-records.md#record-kinds.
//
// The configuration is opaque. The only thing checked is that it is JSON —
// the same "stored raw, shape-checked only" rule the MCP method info follows
// — so nothing here looks for a server, a user, a mailbox or a credential.
//
// A configuration is the one field that can hold a secret, so unlike a
// registration it is not something any caller may overwrite: an existing
// record belongs to its owner. See
// docs/03-records.md#configuring-a-template.
func (b *Bus) Configure(name, caller string, cfg json.RawMessage) (protocol.Record, error) {
	n, err := canon(name)
	if err != nil {
		return protocol.Record{}, err
	}
	who, err := canon(caller)
	if err != nil {
		return protocol.Record{}, err
	}
	if !json.Valid(cfg) {
		return protocol.Record{}, fmt.Errorf("%w, and this is not", ErrConfig)
	}
	if string(bytes.TrimSpace(cfg)) == "null" {
		// null is JSON, but it reads back identically to never-configured,
		// which would make "is it configured?" unanswerable.
		return protocol.Record{}, fmt.Errorf("%w, and null is the absence of one", ErrConfig)
	}
	// Store one spelling. The digest a query gets is over what is stored
	// (protocol.Record.Public), so reformatting a configuration file must not
	// look like changing it.
	var canonical bytes.Buffer
	if err := json.Compact(&canonical, cfg); err != nil {
		return protocol.Record{}, fmt.Errorf("%w: %s", ErrConfig, err)
	}
	cfg = canonical.Bytes()
	b.mu.Lock()
	defer b.unlock()
	if err := b.acting(who); err != nil {
		return protocol.Record{}, err
	}
	r, known := b.record(n)
	// An inactive record is no such name to configure, and its name stays
	// reserved, so this is no way to create over it either.
	if known && !b.live(r) {
		return protocol.Record{}, fmt.Errorf("%w: %s", ErrUnknown, n)
	}
	if !known {
		// Creating here obeys what creating anywhere else obeys. This is a
		// creation path as much as registration is, and a rule only one of
		// them applied was not a rule: a name /register refused for being in
		// a vouched realm could be taken by configuring it instead, and then
		// issued a credential, with no key ever proved.
		// See docs/01-identity-and-roles.md#registration.
		if err := b.vouchedFor(n); err != nil {
			return protocol.Record{}, err
		}
		// Configuring is not a second way to describe a record, only a way
		// to give one config, so this takes a bare registration's defaults.
		owner, err := b.ownerFor(who)
		if err != nil {
			return protocol.Record{}, err
		}
		r = protocol.Record{Name: n, Kind: protocol.KindAgent, Owner: owner, Full: protocol.OverflowStrict}
		if err := validateKind(r); err != nil {
			return protocol.Record{}, err
		}
	} else if !b.manages(who, r) {
		// Writing is the owner's, and the service's own. Reading is neither:
		// see Config.
		return protocol.Record{}, fmt.Errorf("%w: %s is %s's", ErrNotOwner, n, r.Owner)
	}
	r.Config = cfg
	r.At = time.Now()
	if err := validateKind(r); err != nil {
		return protocol.Record{}, err
	}
	b.setRecord(n, r)
	if onBus(r) {
		b.ensure(n)
	}
	if err := b.commit(); err != nil {
		return protocol.Record{}, err
	}
	return r.Public(), nil
}

// Config reads one back — for an agent, the agent itself and nobody else, its
// owner included; for a service, which has no principal, whoever its ACL
// admits. Setup data goes in and is used, not read back by whoever wrote it.
//
// It is in no listing either, so there is no other way to one.
func (b *Bus) Config(name, caller string) (json.RawMessage, error) {
	n, err := canon(name)
	if err != nil {
		return nil, err
	}
	who, err := canon(caller)
	if err != nil {
		return nil, err
	}
	b.mu.Lock()
	defer b.unlock()
	if err := b.acting(who); err != nil {
		return nil, err
	}
	r, known := b.entity(n)
	if !known || !b.may(who, r) && !b.mayReadPrivate(who, r) {
		return nil, fmt.Errorf("%w: %s", ErrUnknown, n)
	}
	if !holdsPrivate(r.Kind) {
		return nil, fmt.Errorf("%w: a %s holds no configuration", ErrConfig, r.Kind)
	}
	// Read by the record's own principal where it has one — an agent, and not
	// its owner — and otherwise by whoever its ACL admits
	// (docs/constitution.md#-private-values).
	if !b.mayReadPrivate(who, r) {
		return nil, fmt.Errorf("%w: only %s may read it", ErrPrivate, n)
	}
	return r.Config, nil
}

// SetSecret stores the credential for reaching a 📡. It is opaque bytes: the
// daemon never parses it, so `KEY=value` is the caller's convention and not a
// grammar anything checks (Q77). Only an empty secret is refused, because it
// reads back exactly like never having set one.
//
// A secret lives on an agent, a service or a group (holdsPrivate). A user,
// queue or pubsub record is only reached by sending to its name, so there is
// nothing for a credential there to unlock.
// See docs/06-services.md#secrets.
func (b *Bus) SetSecret(name, caller, secret string) (protocol.Record, error) {
	n, err := canon(name)
	if err != nil {
		return protocol.Record{}, err
	}
	who, err := canon(caller)
	if err != nil {
		return protocol.Record{}, err
	}
	if secret == "" {
		return protocol.Record{}, fmt.Errorf("%w, and an empty one is the absence of one", ErrNoSecret)
	}
	if err := validEnv(secret); err != nil {
		return protocol.Record{}, err
	}
	b.mu.Lock()
	defer b.unlock()
	if err := b.acting(who); err != nil {
		return protocol.Record{}, err
	}
	r, known := b.entity(n)
	if !known {
		return protocol.Record{}, fmt.Errorf("%w: %s", ErrUnknown, n)
	}
	if !holdsPrivate(r.Kind) {
		return protocol.Record{}, fmt.Errorf("%w: %s is %s", ErrSecret, n, r.Kind)
	}
	// Writing is the owner's and the record's own, exactly as a configuration
	// is: a credential is not something any caller may overwrite.
	if !b.manages(who, r) {
		return protocol.Record{}, fmt.Errorf("%w: %s is %s's", ErrNotOwner, n, r.Owner)
	}
	r.Secret = secret
	r.At = time.Now()
	b.setRecord(n, r)
	// Acknowledged only once it is durable: a credential the caller was told
	// was stored, and which a restart then loses, is worse than a refusal.
	if err := b.commit(); err != nil {
		return protocol.Record{}, err
	}
	return r.Public(), nil
}

// Secret reads one back. Unlike a configuration, a secret exists to be read:
// it is returned to whoever the record's own [ACL](docs/02-access.md#acl)
// already admits, with no second list to keep in step with the first.
// See docs/06-services.md#secrets.
func (b *Bus) Secret(name, caller string) (string, error) {
	n, err := canon(name)
	if err != nil {
		return "", err
	}
	who, err := canon(caller)
	if err != nil {
		return "", err
	}
	b.mu.Lock()
	defer b.unlock()
	if err := b.acting(who); err != nil {
		return "", err
	}
	r, known := b.entity(n)
	// Asked before the kind and before the secret exists, so a caller who may
	// not see the name learns only that — the same order Send uses. A record
	// with a principal of its own reads its secret itself; any other is read
	// by whoever its ACL admits (docs/constitution.md#-private-values).
	if !known || !b.mayReadPrivate(who, r) {
		return "", fmt.Errorf("%w: %s", ErrUnknown, n)
	}
	if !holdsPrivate(r.Kind) {
		return "", fmt.Errorf("%w: %s is %s", ErrSecret, n, r.Kind)
	}
	if r.Secret == "" {
		return "", fmt.Errorf("%w: %s", ErrNoSecret, n)
	}
	return r.Secret, nil
}

// Lookup answers what a name is, so a face can tell a channel from a filter
// without guessing. See docs/03-records.md.
func (b *Bus) Lookup(caller, name string) (protocol.Record, bool) {
	n, err := canon(name)
	if err != nil {
		return protocol.Record{}, false
	}
	b.mu.Lock()
	defer b.unlock()
	r, ok := b.entity(n)
	// A name you may not see does not exist as far as you are concerned:
	// hiding it and refusing it are different answers, and discovery is the
	// half that hides. See docs/02-access.md#acl.
	if !ok || !b.canSee(caller, r) {
		return protocol.Record{}, false
	}
	return b.visible(caller, r), true
}

// OwnerOf reports who owns a name, and nothing more. It is a query, not the
// first half of a decision: the answer is stale the moment the lock is
// released, so anything that acts on ownership asks under the hold it acts in
// (IssueFor is what that looks like). Deciding out here is what let a transfer
// land between the answer and the act.
func (b *Bus) OwnerOf(name string) (string, bool) {
	n, err := canon(name)
	if err != nil {
		return "", false
	}
	b.mu.Lock()
	defer b.unlock()
	r, ok := b.record(n)
	return r.Owner, ok
}

// withLiveness answers the question a listing is really asked: not "does
// this name exist" but "is anything serving it". The daemon knows exactly —
// an inbox with a read outstanding has something on the other end.
//
// Held state, not stored state: it is attached on the way out and never
// written back, so nothing in the registry depends on who happened to be
// connected. Caller holds the lock.
func (b *Bus) withLiveness(name string, r protocol.Record) protocol.Record {
	clearLiveRecord(&r)
	// A service has no queue, so it has no reader count either: a measured
	// zero here would be an observation of something that does not exist.
	if !onBus(r) {
		return r
	}
	in, ok := b.inboxes[name]
	readers := 0
	r.Readers = &readers
	if !ok {
		return r
	}
	readers = len(in.waiters)
	r.Queued, r.In, r.Out = len(in.queue), in.in, in.out
	// Answered here rather than left to be worked out from `queued`, because
	// a record that declares no bound takes the daemon's and a reader cannot
	// know what that is (docs/05-discovery.md#what-a-listing-answers).
	r.AtBound = len(in.queue) >= boundOf(r)
	r.Dropped, r.Expired = in.dropped, in.expired
	if len(in.queue) > 0 {
		// The head is the oldest: a queue is appended to and read from the
		// front (docs/04-messaging.md#inbox-queues).
		r.Oldest = time.Since(in.queue[0].At).Round(time.Second).String()
	}
	for _, w := range in.waiters {
		if !w.filtered {
			r.Reading = true
			break
		}
	}
	return r
}

// clearLiveRecord keeps caller-specific and process-local observations out of
// registration and persistence. They are reconstructed from the caller and
// inbox only when an answer leaves the core.
func clearLiveRecord(r *protocol.Record) {
	r.CanManage, r.CanTransfer, r.RouteAllowed = false, false, nil
	r.Reading, r.Readers, r.Queued, r.In, r.Out = false, nil, 0, 0, 0
	r.Dropped, r.Expired, r.Oldest, r.AtBound = 0, 0, "", false
	r.LastUsed = nil
}

// boundOf is how many messages a record's inbox may hold: what it declared,
// or the daemon's if it declared nothing.
func boundOf(r protocol.Record) int {
	if r.Bound == 0 {
		return maxQueue
	}
	return r.Bound
}

func (b *Bus) List(caller, kind string) []protocol.Record {
	b.mu.Lock()
	defer b.unlock()
	out := make([]protocol.Record, 0, len(b.records))
	for _, r := range b.records {
		if kind != "" && r.Kind != kind {
			continue
		}
		// Two things are held back: what the caller may not see at all,
		// and the configuration, which is nobody's but the service's — a
		// digest of it is what a query gets instead.
		// An inactive record is in no listing; the web face's read-only
		// view below is the one place it shows.
		if !b.live(r) || !b.canSee(caller, r) {
			continue
		}
		out = append(out, b.visible(caller, r))
	}
	return out
}

// Inactive is the one read-only view of inactive records, for the web face:
// each shown to the actors its ACL admits, and always to the daemon Owner. It
// reads; nothing else names an inactive record except its reactivation
// (docs/constitution.md#common-record-fields).
func (b *Bus) Inactive(caller string) []protocol.Record {
	b.mu.Lock()
	defer b.unlock()
	if b.acting(caller) != nil {
		return []protocol.Record{}
	}
	out := []protocol.Record{}
	for _, r := range b.records {
		if b.live(r) || caller != b.admin && !b.may(caller, r) {
			continue
		}
		out = append(out, b.visible(caller, r))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// take removes one message and releases it. Shortening a slice leaves the
// vacated slot pointing at the envelope, so a drained inbox goes on holding
// every body it ever handed out; clearing the tail is what actually frees
// them. One place to get this right, because it is one line to forget.
//
// The backing array is kept — reusing a zeroed allocation is the point — but
// nothing dead stays reachable through it.
//
// Taking from the head by stepping past it instead of shifting was measured
// and rejected: it wins past a backlog of ~300 and on a full ring (2443 ns
// to 733), loses below that, and allocates 330-520 B on every message where
// this allocates none. New garbage per message is the worse trade inside a
// component that is 2.6% of the daemon's CPU.
func take(q []protocol.Envelope, i int) []protocol.Envelope {
	copy(q[i:], q[i+1:])
	q[len(q)-1] = protocol.Envelope{}
	return q[:len(q)-1]
}

// drop removes one waiter, and releases it for the same reason take does: a
// dead waiter holds a channel, and a slot that still points at it keeps that
// channel alive for as long as the inbox exists.
func drop(ws []*waiter, i int) []*waiter {
	copy(ws[i:], ws[i+1:])
	ws[len(ws)-1] = nil
	return ws[:len(ws)-1]
}

// ensure creates the inbox for a name. Every registered principal has one;
// the address outlives the process, so a message can arrive while it is down.
func (b *Bus) ensure(name string) *inbox {
	in, ok := b.inboxes[name]
	if !ok {
		in = &inbox{act: activity.Start(b.clock(), activity.Counts{})}
		b.inboxes[name] = in
	}
	return in
}

// Send delivers one message to one inbox. A waiting consume takes it
// directly; otherwise it queues.
func (b *Bus) Send(e protocol.Envelope) (protocol.Envelope, error) {
	to, err := canon(e.To)
	if err != nil {
		return protocol.Envelope{}, err
	}
	from, err := canon(e.From)
	if err != nil {
		return protocol.Envelope{}, err
	}
	b.mu.Lock()
	defer b.unlock()
	// Before the receiver is looked up, so that a caller who is nobody is not
	// told which names exist by which refusal it gets.
	if err := b.acting(from); err != nil {
		return protocol.Envelope{}, err
	}
	// An inactive receiver is no such receiver, whoever asks: the daemon
	// owner is refused as readily as a stranger (docs/constitution.md#common-record-fields).
	rec, known := b.entity(to)
	if !known {
		return protocol.Envelope{}, fmt.Errorf("no such receiver: %s (%w)", to, ErrUnknown)
	}
	// Writing goes through the same two layers as reading: a principal that
	// may not see a service may not enqueue to it either, and is told so
	// rather than left to wonder. See docs/02-access.md#acl.
	//
	// A User's own inbox has no list of its own: an Agent may send to a User
	// exactly when that Agent's ACL admits the User — whoever may reach an
	// Agent may be answered by it — and no User sends to another
	// (docs/constitution.md#-channels).
	if rec.Kind == protocol.KindUser {
		if err := b.mayDeliverToUser(from, rec); err != nil {
			return protocol.Envelope{}, err
		}
	} else if !b.may(from, rec) {
		return protocol.Envelope{}, fmt.Errorf("%s may not send to %s: %w", from, to, ErrNotAllow)
	}
	// Asked after the ACL, so a caller who may not see the name is told that
	// and not which kind it is. See docs/03-records.md#record-kinds.
	if !onBus(rec) {
		return protocol.Envelope{}, fmt.Errorf("%w: %s is external — call it at %s, it is not sent to over this bus", ErrKind, to, rec.Addr)
	}
	// An answer that cannot be routed is the requester's problem to hear
	// about now. Registered, not live: the name owns a queue whether or not
	// anything is reading it. See docs/04-messaging.md#reply-routing.
	if e.ReplyTo != nil {
		back, err := canon(e.ReplyTo.Name)
		if err != nil {
			return protocol.Envelope{}, fmt.Errorf("reply-to: %w", err)
		}
		if _, known := b.entity(back); !known {
			return protocol.Envelope{}, fmt.Errorf("no such reply address: %s (%w)", back, ErrUnknown)
		}
		e.ReplyTo = &protocol.ReplyTo{Name: back, Topic: e.ReplyTo.Topic, Tag: e.ReplyTo.Tag}
	}
	// Receipts are a closed set: a caller decides whether a message is an
	// answer by looking at this field, so a third value would read as one.
	if e.Receipt != "" && e.Receipt != protocol.ReceiptAck && e.Receipt != protocol.ReceiptDone {
		return protocol.Envelope{}, fmt.Errorf("%w, not %q", ErrReceipt, e.Receipt)
	}
	e.To, e.From = to, from
	// Provenance is the daemon's: a caller stating where a message came
	// through, or how far it travelled, grants it nothing.
	e.OriginalTo, e.Forwards = "", 0
	e.ID = newID()
	e.At = time.Now()
	d, err := life(rec.TTL, e.TTL)
	if err != nil {
		return protocol.Envelope{}, err
	}
	e.Expires = time.Time{}
	if d > 0 {
		e.Expires = e.At.Add(d)
	}
	// The caller's deadline travels with the request. The bus works out the
	// moment, like Expires, so both ends read one clock — and then does
	// nothing with it: giving up early is the service's call, not the
	// transport's. A caller may not state the moment itself, for the same
	// reason it may not state its own name.
	// See docs/04-messaging.md#request-and-reply.
	e.Deadline = time.Time{}
	if e.Wait != "" {
		w, err := time.ParseDuration(e.Wait)
		if err != nil || w <= 0 {
			return protocol.Envelope{}, fmt.Errorf("%w, not %q", ErrWait, e.Wait)
		}
		e.Deadline = e.At.Add(w)
	}
	// A 📮 channel is an inbox with a name, so publishing to one is an
	// ordinary send. A 📣 channel keeps nothing of its own instead, and an
	// agent or queue with a route passes the message on.
	// One send is one entry in the envelope feed, however many steps and
	// copies it becomes.
	if err := b.route(rec, e); err != nil {
		return protocol.Envelope{}, err
	}
	b.note(e)
	return e, nil
}

// route puts e into rec: into its inbox, or — for an 👾 or 📮 with a
// one-slot deliver_to — one forwarding step on, into the destination, which
// the source keeps no copy of and counts nothing for
// (docs/constitution.md#-channels). Every check comes before anything is
// stored, so a refusal answers the original sender with nothing changed.
// Caller holds the lock.
func (b *Bus) route(rec protocol.Record, e protocol.Envelope) error {
	if rec.Kind == protocol.KindPubSub {
		_, err := b.fanout(rec, e)
		return err
	}
	if (rec.Kind == protocol.KindAgent || rec.Kind == protocol.KindQueue) && len(rec.Subs) == 1 {
		next := rec.Subs[0]
		if e.Forwards+1 > maxForwards {
			return fmt.Errorf("%w: at %s", ErrForwards, next)
		}
		// A route that was configured and has stopped working is the
		// source's Owner's to fix, not the sender's mistake: it goes to the
		// error log as well as to the sender (Q108).
		dst, ok := b.entity(next)
		if !ok {
			b.report(ports.Warning, "the route from %s to %s is broken: %s is absent or inactive", rec.Name, next, next)
			return fmt.Errorf("no such route destination: %s, from %s (%w)", next, rec.Name, ErrUnknown)
		}
		// The destination's ACL must list the forwarding record itself:
		// neither the sender's nor the source's Owner's access stands in.
		if !b.admits(dst, rec.Name) {
			b.report(ports.Warning, "the route from %s to %s is broken: %s no longer allows %s", rec.Name, next, next, rec.Name)
			return fmt.Errorf("%s may not forward to %s: %w", rec.Name, next, ErrNotAllow)
		}
		e.Forwards++
		if e.OriginalTo == "" {
			e.OriginalTo = rec.Name
		}
		e.To = dst.Name
		// The destination's own TTL, as for a direct send.
		if d, err := life(dst.TTL, e.TTL); err == nil {
			e.Expires = time.Time{}
			if d > 0 {
				e.Expires = e.At.Add(d)
			}
		}
		return b.route(dst, e)
	}
	return b.deliver(rec, b.ensure(rec.Name), e)
}

// admits says whether a record's allow list itself names name — the one list
// a forwarding hop is judged by (docs/constitution.md#-channels). Caller holds
// b.mu.
func (b *Bus) admits(r protocol.Record, name string) bool {
	for _, s := range r.Allow {
		// "*" is every active User and Agent (docs/02-access.md#acl), so it
		// admits a forwarding agent and never a queue or topic source.
		if s == "*" && b.actor(name) || s == name || b.member(name, s) || b.runtimeTerm(name, s, r) {
			return true
		}
	}
	return false
}

// actor says whether name speaks on the bus: a User or a live Agent.
// Caller holds b.mu.
func (b *Bus) actor(name string) bool {
	if _, user := b.users[name]; user {
		return true
	}
	r, ok := b.entity(name)
	return ok && r.Kind == protocol.KindAgent
}

// deliver puts one envelope into one inbox: into a waiter if one is there,
// into the queue otherwise, and into neither if the queue is full and its
// record said strict. It is the only door into an inbox, so a fan-out copy
// obeys the receiver's bound and overflow exactly as a send does.
// Caller holds the lock.
func (b *Bus) deliver(rec protocol.Record, in *inbox, e protocol.Envelope) error {
	// Asked again here, at the hand-over: a waiter whose standing went while
	// it blocked is released, never handed the message
	// (docs/constitution.md#common-record-fields).
	for i := len(in.waiters) - 1; i >= 0; i-- {
		if w := in.waiters[i]; !b.may(w.caller, rec) {
			err := ErrNotAllow
			if e := b.acting(w.caller); e != nil {
				err = e
			}
			w.stopped <- err
			in.waiters = drop(in.waiters, i)
		}
	}
	// A waiter that asked for this topic and tag is served ahead of the
	// unfiltered reader — otherwise a `call` loses its reply to whichever
	// reader happened to block first.
	// See docs/04-messaging.md#one-reader-per-inbox.
	for _, filteredPass := range []bool{true, false} {
		for i, w := range in.waiters {
			if w.filtered != filteredPass {
				continue
			}
			if w.filtered && !selects(w.topic, w.tag, e) {
				continue
			}
			in.waiters = drop(in.waiters, i)
			in.in, in.out = in.in+1, in.out+1 // straight through: in and out at once
			w.ch <- e
			return nil
		}
	}
	// A full queue either refuses the new message or forgets the oldest, and
	// the receiver's record says which. Refusing is the default because
	// losing a job silently is worse than failing visibly; a ring is for the
	// streams where the newest matters most (docs/04-messaging.md#overflow).
	bound := boundOf(rec)
	if len(in.queue) >= bound {
		// What has already expired is not fullness: count it as expiry and
		// see whether there is room after all.
		b.prune(in, e.At)
	}
	if len(in.queue) >= bound {
		if rec.Full != protocol.OverflowRing {
			return fmt.Errorf("%w: %s holds %d", ErrFull, rec.Name, len(in.queue))
		}
		in.queue = take(in.queue, 0)
		in.dropped++ // a real loss, so `status` and `ls` report it
	}
	in.queue = append(in.queue, e)
	in.in++
	return nil
}

// fanout is what a 📣 channel does instead of holding a queue: one copy
// into each subscriber's own inbox, where that subscriber's bound, overflow,
// TTL and reader apply. The channel itself keeps nothing.
//
// One subscriber cannot stop the channel: a copy that will not fit is counted
// as a drop and the others still go. A publisher a stopped reader can block
// is a queue, and a 📮 channel is what that caller wanted.
// Caller holds the lock.

// maxForwards is how many forwarding steps a message may take: the step that
// would go past it is refused before anything is stored
// (docs/constitution.md#-channels).
const maxForwards = 10

// ErrForwards is a message that would take one forwarding step too many.
var ErrForwards = errors.New("the message would pass through more than ten forwarding steps")

// fanout publishes e to topic. Each recipient is a branch one forwarding step
// on, answered by its own destination's rules — a 📣 recipient a further
// publication. The topic stores nothing; its in counts publications at least
// one recipient took, and its out the copies accepted
// (docs/constitution.md#pubsub-routing). Caller holds the lock.
func (b *Bus) fanout(topic protocol.Record, e protocol.Envelope) (protocol.Envelope, error) {
	// The topic's ACL is not consulted here. It says who may publish, and
	// who receives is this separate list — so a recipient goes on receiving
	// whether or not it could ever publish, and the way to stop the copies
	// is to take the name off the list.
	// See docs/04-messaging.md#subscribers.
	//
	// Whether the caller is refused depends on whether anything gets
	// through (docs/constitution.md#common-record-fields): a copy that one
	// recipient cannot take is that recipient's own drop, written to the
	// error log, while the others still get theirs; a publication that no
	// recipient takes is refused before anything is stored or counted.
	// An inactive Group on the list grants nothing: its members take no copy
	// through it, it is no failed recipient and adds no drop, and the log says
	// so (docs/constitution.md#common-record-fields).
	for _, term := range topic.Subs {
		if r, ok := b.records[term]; ok && r.Kind == protocol.KindGroup && !b.live(r) {
			b.report(ports.Warning, "a publication to %s ignored its deliver-to group %s, which is inactive", topic.Name, term)
		}
	}
	var failed []string
	var why []error
	delivered := 0
	for _, s := range b.deliverTo(topic) {
		sub, known := b.records[s]
		// A kind that cannot receive is the snapshot case, Manage having
		// refused one since 0.6.15; a name with no record was never on the
		// list, removal taking its references with it. Neither is a
		// recipient, so neither is a failed one.
		if !known || !canReceive(sub) && sub.Kind != protocol.KindPubSub {
			continue
		}
		if !b.live(sub) {
			failed, why = append(failed, s), append(why, fmt.Errorf("%w: %s is inactive", ErrUnknown, s))
			continue
		}
		// Each branch is a forwarding step, and is answered by its own
		// destination's rules, its route included.
		if e.Forwards+1 > maxForwards {
			failed, why = append(failed, s), append(why, fmt.Errorf("%w: at %s", ErrForwards, s))
			continue
		}
		c := e
		c.To, c.Forwards = s, e.Forwards+1
		if c.OriginalTo == "" {
			c.OriginalTo = topic.Name
		}
		if err := b.route(sub, c); err != nil {
			failed, why = append(failed, s), append(why, err)
			continue
		}
		delivered++
	}
	if delivered == 0 && len(failed) == 0 {
		// Nothing to take it is the flow broken, not a success nobody hears:
		// an empty list, or one whose every term reached no active recipient.
		if len(topic.Subs) > 0 {
			b.report(ports.Warning, "a publication to %s had no recipient left: no deliver-to term reached an active recipient", topic.Name)
		}
		return protocol.Envelope{}, fmt.Errorf("%s has no recipient to take it (%w)", topic.Name, ErrUnknown)
	}
	if delivered == 0 && len(failed) > 0 {
		b.report(ports.Warning, "a publication to %s reached none of its %d recipients: %s", topic.Name, len(failed), strings.Join(failed, ", "))
		return protocol.Envelope{}, fmt.Errorf("no recipient of %s took it: %w", topic.Name, why[0])
	}
	// Routing counters, neither depth nor consumption: one publication in,
	// one out per accepted copy, never again when a copy is read.
	router := b.ensure(topic.Name)
	router.in++
	router.out += delivered
	for i, s := range failed {
		// The copy that could not land is the RECIPIENT's loss: its page is
		// where that has to show, and the error log says which and why.
		b.ensure(s).dropped++
		b.report(ports.Warning, "a copy of a publication to %s was not delivered to %s: %s", topic.Name, s, why[i])
	}
	return e, nil
}

// Subscribe takes the caller off a 📣 channel's Deliver-To list. Putting a
// name on it is the channel manager's, through Manage: delivery is no longer
// gated by the topic's ACL, so a name adding itself would be answering to
// nobody. Taking yourself off stays yours, because it is your inbox that
// fills.
// See docs/04-messaging.md#subscribers.
func (b *Bus) Subscribe(caller, channel string, on bool) (protocol.Record, error) {
	who, err := canon(caller)
	if err != nil {
		return protocol.Record{}, err
	}
	n, err := canon(channel)
	if err != nil {
		return protocol.Record{}, err
	}
	b.mu.Lock()
	defer b.unlock()
	if err := b.acting(who); err != nil {
		return protocol.Record{}, err
	}
	r, known := b.entity(n)
	// A channel you may not see does not exist as far as you are concerned,
	// exactly as a lookup answers — except to a name it delivers to, which
	// would otherwise have no way to stop copies it never asked for. Being
	// on the list is not being allowed to publish.
	// See docs/02-access.md#acl.
	if !known || !(b.may(who, r) || b.receives(r, who)) {
		return protocol.Record{}, fmt.Errorf("%w: %s", ErrUnknown, n)
	}
	if r.Kind != protocol.KindPubSub {
		return protocol.Record{}, fmt.Errorf("%w: only a pubsub channel has a deliver-to list, and %s is not one", ErrKind, n)
	}
	if on {
		return protocol.Record{}, fmt.Errorf("%w: %s decides who %s delivers to; ask to be put on its list", ErrNotOwner, r.Owner, n)
	}
	// Removing a name that is not there is a no-op, but removing a name that
	// receives through a group is not: drop1 would take nothing out and the
	// copies would keep arriving, so say which group instead of reporting a
	// removal that did not happen.
	if g := b.receivesThrough(r, who); g != "" {
		return protocol.Record{}, fmt.Errorf("%w: %s is on %s through %s, so leaving %s is what stops the copies", ErrNotOwner, who, n, g, g)
	}
	r.Subs = drop1(r.Subs, who)
	b.setRecord(n, r)
	if err := b.commit(); err != nil {
		return protocol.Record{}, err
	}
	return b.withLiveness(n, r.Public()), nil
}

// drop1 returns the list without s, in a slice of its own: the stored record
// shares its backing array with whatever a query already handed out.
func drop1(list []string, s string) []string {
	out := list[:0:0]
	for _, x := range list {
		if x != s {
			out = append(out, x)
		}
	}
	return out
}

// life is how long this message is worth delivering: what the sender asked
// for, never more than what the receiver's queue allows. Either may be
// unset; unset on both means it waits until consumed.
// See docs/04-messaging.md#message-ttl.
func life(queue, msg string) (time.Duration, error) {
	var q, m time.Duration
	var err error
	if queue != "" {
		q, _ = time.ParseDuration(queue) // validated at Register
	}
	if msg != "" {
		if m, err = time.ParseDuration(msg); err != nil || m <= 0 {
			return 0, fmt.Errorf("%w, not %q", ErrTTL, msg)
		}
	}
	switch {
	case m == 0:
		return q, nil
	case q == 0 || m < q:
		return m, nil
	default:
		// Asking for longer than the queue keeps is not an error — the queue
		// simply does not keep it that long. The receiver owns its retention
		// the same way it owns its overflow.
		return q, nil
	}
}

// prune drops what has outlived its moment, counting it apart from overflow.
// It runs where the queue is already being walked, so the hot path pays for
// nothing it was not already doing.
func (b *Bus) prune(in *inbox, now time.Time) {
	if len(in.queue) == 0 {
		return
	}
	kept := in.queue[:0]
	for _, e := range in.queue {
		if !e.Expires.IsZero() && now.After(e.Expires) {
			in.expired++
			continue
		}
		kept = append(kept, e)
	}
	for i := len(kept); i < len(in.queue); i++ {
		in.queue[i] = protocol.Envelope{}
	}
	in.queue = kept
}

// Consume takes one message, at-most-once: it is handed over and gone.
// A filter waits for one topic+tag; an unfiltered read takes the next of
// anything, and there may be only one of those at a time unless the readers
// asked to share the inbox.
// See docs/04-messaging.md#one-reader-per-inbox.
func (b *Bus) Consume(ctx context.Context, name, topic, tag string, filtered, share bool) (protocol.Envelope, error) {
	return b.ConsumeAs(ctx, name, name, topic, tag, filtered, share)
}

func (b *Bus) ConsumeAs(ctx context.Context, caller, name, topic, tag string, filtered, share bool) (protocol.Envelope, error) {
	name, err := canon(name)
	if err != nil {
		return protocol.Envelope{}, err
	}
	b.mu.Lock()
	if err := b.acting(caller); err != nil {
		b.unlock()
		return protocol.Envelope{}, err
	}
	// An inbox belongs to a registered name. Creating one for whoever asks
	// would let any caller name leave a permanent entry behind — and the
	// wait could never end anyway, because Send refuses an unknown
	// receiver, so nothing can ever arrive in it.
	rec, known := b.entity(name)
	if !known {
		b.unlock()
		return protocol.Envelope{}, fmt.Errorf("no inbox for %s: register it first (%w)", name, ErrUnknown)
	}
	if !b.may(caller, rec) {
		b.unlock()
		return protocol.Envelope{}, ErrNotAllow
	}
	// After the ACL, for the same reason Send asks in that order.
	if !onBus(rec) {
		b.unlock()
		return protocol.Envelope{}, fmt.Errorf("%w: %s is external and has no queue here to read", ErrKind, name)
	}
	in := b.ensure(name)
	// Never handed to a consumer: the check is here, where the message would
	// otherwise be handed over. See docs/04-messaging.md#message-ttl.
	b.prune(in, time.Now())

	for i, e := range in.queue {
		if !filtered || selects(topic, tag, e) {
			in.queue = take(in.queue, i)
			in.out++
			b.unlock()
			return e, nil
		}
	}
	// Two unfiltered readers are a pool when they both said so, and a bug
	// otherwise — a session's CLI reading the queue its push adapter is
	// reading. Asking is what tells the daemon which it has, and it costs a
	// worker one word. Either side declining keeps the old refusal: a reader
	// that wants the inbox to itself still gets it.
	// See docs/04-messaging.md#one-reader-per-inbox.
	if !filtered {
		for _, w := range in.waiters {
			if !w.filtered && !(share && w.share) {
				b.unlock()
				return protocol.Envelope{}, ErrTwoReads
			}
		}
	}
	w := &waiter{topic: topic, tag: tag, filtered: filtered, share: share, ch: make(chan protocol.Envelope, 1), caller: caller, stopped: make(chan error, 1)}
	in.waiters = append(in.waiters, w)
	b.unlock()

	select {
	case err := <-w.stopped:
		return protocol.Envelope{}, err
	case e := <-w.ch:
		return e, nil
	case <-ctx.Done():
		// A send may have handed us a message just as the wait expired. Settle
		// that race under the same lock a send holds: either we take what was
		// delivered, or we remove the waiter so nothing can be delivered to it.
		if e, delivered := b.settle(name, w); delivered {
			return e, nil
		}
		return protocol.Envelope{}, ctx.Err()
	}
}

// selects says whether a filtered read takes e. Each filter names one field
// and an absent one matches anything, so `--topic X` alone takes a tagged
// message on X (docs/04-messaging.md#inbox-selection-and-filters).
func selects(topic, tag string, e protocol.Envelope) bool {
	return (topic == "" || e.Topic == topic) && (tag == "" || e.Tag == tag)
}

// settle removes a waiter, unless a message reached it first — in which case
// it returns that message. Nothing is dropped in between.
func (b *Bus) settle(name string, w *waiter) (protocol.Envelope, bool) {
	b.mu.Lock()
	defer b.unlock()
	select {
	case e := <-w.ch:
		return e, true
	default:
	}
	in := b.inboxes[name]
	if in == nil {
		return protocol.Envelope{}, false
	}
	for i, x := range in.waiters {
		if x == w {
			in.waiters = drop(in.waiters, i)
			break
		}
	}
	return protocol.Envelope{}, false
}

type Status struct {
	Up       string `json:"up"`
	Services int    `json:"services"`
	Queued   int    `json:"queued"`
	Waiting  int    `json:"waiting"`
	Dropped  int    `json:"dropped"` // lost to overflow since start
	Expired  int    `json:"expired"` // outlived their TTL since start
	// Absent unless it has something to say: a first start has no previous
	// stop to have been clean or otherwise.
	Unclean bool `json:"unclean,omitempty"` // the last run did not stop cleanly
	// Refusals by kind, and only the kinds that have happened. A reason
	// with a zero beside it is noise on every other node.
	Refused map[string]int `json:"refused,omitempty"`
	// Records by kind, node-wide, and every kind of the closed set whether or
	// not the node holds one. A total alone cannot say which of the four
	// listings grew, and an absent kind here would read as an older daemon
	// rather than as a node with none of them.
	// See docs/03-records.md#record-kinds.
	Kinds map[string]int `json:"kinds"`
	// Records inactive only because their owning User is, and the messages
	// they hold. Node-wide, so it is answered to the daemon Owner and the
	// Administrators — who can reactivate that User — and absent for anybody
	// else, which a face must read as unobserved rather than as none.
	// See docs/05-discovery.md#overview-and-diagnostics.
	OwnerInactive *OwnerInactive `json:"owner_inactive,omitempty"`
}

// OwnerInactive counts what an inactive User's reactivation would bring back:
// records whose own status is active, owned by a User who is not, other than
// that User's own record.
type OwnerInactive struct {
	Records  int `json:"records"`
	Messages int `json:"messages"`
}

// Uptime reads only the process start time, immutable after New. Public node
// identity must not scan private inbox state to obtain this one value.
func (b *Bus) Uptime() string { return time.Since(b.started).Round(time.Second).String() }

func (b *Bus) Status() Status {
	b.mu.Lock()
	defer b.unlock()
	return b.status()
}

// StatusFor is Status as caller may read it: the owner-inactive count only to
// the daemon Owner and an active Administrator.
func (b *Bus) StatusFor(caller string) Status {
	b.mu.Lock()
	defer b.unlock()
	s := b.status()
	if caller == b.admin || b.acting(caller) == nil && b.isAdministrator(caller) {
		s.OwnerInactive = b.ownerInactive()
	}
	return s
}

// ownerInactive is the count behind OwnerInactive. Caller holds b.mu.
func (b *Bus) ownerInactive() *OwnerInactive {
	out := &OwnerInactive{}
	for name, r := range b.records {
		// A record owned by no User is a leftover, not a suspension.
		if _, isUser := b.users[r.Owner]; !isUser || b.userActive(r.Owner) || r.Status == protocol.StatusInactive || name == r.Owner {
			continue
		}
		out.Records++
		if in := b.inboxes[name]; in != nil {
			out.Messages += len(in.queue)
		}
	}
	return out
}

// status is the node-wide answer. Caller holds b.mu.
func (b *Bus) status() Status {
	s := Status{
		Up:    b.Uptime(),
		Kinds: map[string]int{},
	}
	for _, kind := range protocol.Kinds {
		s.Kinds[kind] = 0
	}
	// Inactive records are no entities, so the node counts only the live.
	for _, r := range b.records {
		if b.live(r) {
			s.Kinds[r.Kind]++
			s.Services++
		}
	}
	// Summed, not counted a second time: the node's total has one home, and
	// it is the inboxes that lost the work.
	for _, in := range b.inboxes {
		s.Queued += len(in.queue)
		s.Waiting += len(in.waiters)
		s.Dropped += in.dropped
		s.Expired += in.expired
	}
	s.Unclean = b.unclean
	if len(b.refused) > 0 {
		s.Refused = make(map[string]int, len(b.refused))
		for k, n := range b.refused {
			s.Refused[k] = n
		}
	}
	return s
}

// Owned lists the names this caller is answerable for: the records they own,
// and their own name, which holds a credential whether or not anything is
// registered under it. It is the registry half of the dashboard's my-names
// view — the credential half is the token store's
// (docs/05-discovery.md#what-it-shows).
func (b *Bus) Owned(caller string) []string {
	b.mu.Lock()
	defer b.unlock()
	if b.acting(caller) != nil {
		return []string{}
	}
	out := []string{caller}
	for name, r := range b.records {
		if name != caller && r.Owner == caller && b.live(r) {
			out = append(out, name)
		}
	}
	return out
}

// Refuse records that a call was turned away, and for what reason. The face
// calls it because one of the kinds — a credential that is not one — is
// refused before core is ever reached
// (docs/02-access.md#what-a-call-carries).
func (b *Bus) Refuse(kind string) {
	b.mu.Lock()
	defer b.unlock()
	b.refused[kind]++
}

func newID() string {
	var b [8]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

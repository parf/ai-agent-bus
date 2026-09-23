package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Envelope is what the bus reads, counts and routes. The body is carried but
// not interpreted; from R1 it is ciphertext.
// See docs/02-access.md#trust-boundary.
type Envelope struct {
	ID    string    `json:"message_id"`
	From  string    `json:"from"`
	To    string    `json:"to"`
	Topic string    `json:"topic,omitempty"`
	Tag   string    `json:"tag,omitempty"`
	Body  string    `json:"body"`
	At    time.Time `json:"at"`

	// A receipt is an ordinary message that happens to say something about
	// another one: ack = got it, done = finished. Both come from the
	// receiver, carry the original topic and tag so the sender's wait matches
	// them, and name the message they are about.
	// See docs/04-messaging.md#receipts.
	Receipt string `json:"receipt,omitempty"` // "", "ack" or "done"
	Re      string `json:"re,omitempty"`      // the message_id this is about

	// ReplyTo sends the answer somewhere other than back to the sender. The
	// default route needs no field — the sender's own name with this topic
	// and tag — so this is set only when the answer belongs to a third
	// party. Its service must be REGISTERED when the request is accepted:
	// an answer that cannot be routed is a fact the requester should learn
	// now, not when it bounces.
	// See docs/04-messaging.md#reply-routing.
	ReplyTo *ReplyTo `json:"reply_to,omitempty"`

	// TTL is how long this message is worth delivering, stated by the
	// sender; the receiver's queue bounds it. A question nobody should
	// answer late sets one.
	// See docs/04-messaging.md#message-ttl.
	TTL string `json:"ttl,omitempty"`

	// Expires is when the daemon will stop delivering it, worked out once
	// at accept from the two TTLs. Absolute, so the queue compares rather
	// than parses — the sender states a duration, the bus keeps a moment.
	Expires time.Time `json:"expires,omitempty"`

	// Wait is how long the caller will wait for an answer, and Deadline is
	// the moment that lands on, worked out at accept the way Expires is.
	// It travels so that a service can see the answer is already too late
	// and not do the work at all.
	//
	// It is the CALLER's, which is what makes it a second pair rather than
	// a spelling of the TTL: the receiver's queue bounds a TTL and does not
	// bound this, and the bus never acts on it — a message whose caller has
	// gone is still delivered, because only the service knows whether the
	// work is worth doing for somebody else.
	// See docs/04-messaging.md#request-and-reply.
	Wait     string    `json:"wait,omitempty"`
	Deadline time.Time `json:"deadline,omitempty"`
}

// TooLate says the caller has stopped waiting. No deadline means no answer
// was promised by any moment, which is not the same as a deadline that has
// passed — so a message without one is never too late.
func (e Envelope) TooLate(now time.Time) bool {
	return !e.Deadline.IsZero() && now.After(e.Deadline)
}

// ReplyTo is a route, not a promise: registered says the name owns a queue,
// never that anything is reading it.
// See docs/04-messaging.md#reply-routing.
type ReplyTo struct {
	// Name, not Service: what an answer goes to is a name with a queue
	// here, and a service is the external case that has none.
	// See docs/06-services.md#it-has-no-queue-here.
	Name  string `json:"name"`
	Topic string `json:"topic,omitempty"`
	Tag   string `json:"tag,omitempty"`
}

// MaintainerList is the resource's explicit management grants. New answers use
// an array; accepting the legacy string keeps old snapshots and pre-migration
// clients readable during the one-way move from one group to a list.
type MaintainerList []string

func (m *MaintainerList) UnmarshalJSON(data []byte) error {
	var many []string
	if err := json.Unmarshal(data, &many); err == nil {
		*m = many
		return nil
	}
	var one string
	if err := json.Unmarshal(data, &one); err != nil {
		return fmt.Errorf("maintainers must be an array of names: %w", err)
	}
	if one == "" {
		*m = nil
	} else {
		*m = MaintainerList{one}
	}
	return nil
}

// Record is one registered name, of one of the five kinds below. Registering
// is pushing a description; the thing itself need not know the bus exists.
// See docs/03-records.md#five-record-kinds.
type Record struct {
	// ID is the internal registry_id: stable, persisted, never reused and
	// never the public identity, so it is on no answer
	// (docs/constitution.md#common-record-fields).
	ID   uint32 `json:"-"`
	Name string `json:"name"`
	Kind string `json:"kind"`           // one of Kinds; a closed set
	Addr string `json:"addr,omitempty"` // host:port, a path, a URL

	// Proto says HOW to call this, where Addr says where. Empty is the
	// answer for almost everything: no protocol means one of the four kinds
	// with a queue here — send to its name and the daemon delivers to its
	// inbox. A service names a protocol the *caller* speaks directly: a hint
	// in the registry, not something the daemon implements or checks.
	// /etc/services is the suggested vocabulary and cannot be more than
	// that — it is outdated and incomplete, and much of what gets
	// registered here is not in it.
	// See docs/06-services.md#how-to-call-it.
	Proto string `json:"protocol,omitempty"`

	Descr string `json:"descr,omitempty"`    // what ls and the MCP catalog show
	Full  string `json:"overflow,omitempty"` // ring or strict; strict if unset

	// TTL and Bound are the queue's, declared on the record like overflow:
	// how long anything in it is worth keeping, and how much of it there
	// may be. Both unset take the daemon's defaults. A message may ask for
	// less than TTL and never for more.
	// See docs/07-channels.md#the-two-channel-kinds.
	TTL   string `json:"ttl,omitempty"`
	Bound int    `json:"bound,omitempty"`

	Owner       string         `json:"owner"`
	Maintainers MaintainerList `json:"maintainers,omitempty"`
	// Personal groups a service in the owner's web view. It changes neither
	// delivery nor access; core only enforces which authority assignments may
	// coexist with it. See docs/03-records.md#personal-and-shared.
	Personal bool      `json:"personal,omitempty"`
	Disabled bool      `json:"disabled,omitempty"`
	At       time.Time `json:"at"`
	// Created is when the record was first stored; At is when it last was.
	// Both are the system's, never a caller's.
	Created time.Time `json:"created_at,omitzero"`

	// Allow is the service ACL: who may see and use this. It is a field on
	// the record, not something inside Config, because the daemon enforces
	// it and will not read a private configuration. `*` means anyone who
	// can authenticate; empty means only management authority and the record's
	// own principal. `@owner` means the direct Owner's Service and Agent
	// principals; it is resolved at use time rather than stored as a group.
	// See docs/02-access.md#acl.
	Allow []string `json:"allow,omitempty"`

	// Subs is who receives a copy of what is published to a pub/sub topic.
	// A subscription is a record, so it lives here rather than in a
	// connection: it survives a restart with the rest of the registry, and
	// a subscriber that is down keeps its backlog in its own inbox.
	// Written by Subscribe alone — a registration never carries it.
	// See docs/04-messaging.md#push-and-pull.
	Subs []string `json:"subs,omitempty"`

	// Config is what an agent template was configured with. It is opaque:
	// the bus checks that it is JSON and never reads inside — no server,
	// user, mailbox or credential is looked for. It leaves the daemon for
	// one caller only, the record it belongs to; every other answer
	// carries ConfigSHA in its place.
	// See docs/03-records.md#configuring-a-template.
	Config json.RawMessage `json:"config,omitempty"`

	// ConfigSHA is what a query gets instead: enough to see that a record
	// is configured, that a write landed, and that two are the same, without
	// handing the configuration to anyone.
	ConfigSHA string `json:"config_sha,omitempty"`

	// Secret is the credential for reaching a 📡, and lives on that kind
	// alone. It is opaque bytes: `KEY=value` is what callers agree to write
	// and the daemon never parses it, so blank lines, comments, `export`,
	// duplicate keys and an invalid identifier are all the caller's business
	// (Q77). Unlike Config it exists to be read back — by whoever the
	// record's own ACL admits, with no second list. Every answer that is not
	// that read carries SecretSHA in its place.
	// See docs/06-services.md#secrets.
	Secret string `json:"secret,omitempty"`

	// SecretSHA is what every other answer gets: enough to see that a
	// service has a credential, that a write landed, and that two hosts
	// hold the same one, without handing it out.
	SecretSHA string `json:"secret_sha,omitempty"`

	// Live state, filled in on the way out of a query and never stored:
	// a registry record says a name exists, while these fields report what
	// the daemon observes now. A registration is only a
	// description — "there is a MySQL on host:port" registers fine and
	// nothing on this bus answers for it — so "is it in the registry?" and
	// "can I call it through the daemon?" are different questions.
	// See docs/05-discovery.md#what-a-listing-answers.
	CanManage   bool `json:"can_manage,omitempty"`
	CanTransfer bool `json:"can_transfer,omitempty"`
	Reading     bool `json:"reading,omitempty"` // an unfiltered read is outstanding; retained for compatibility
	Readers     *int `json:"readers,omitempty"` // all outstanding reads, filtered and unfiltered together; nil means unobserved
	Queued      int  `json:"queued,omitempty"`  // messages waiting in it
	In          int  `json:"in,omitempty"`      // accepted for it since the daemon started
	Out         int  `json:"out,omitempty"`     // handed to a reader of it since then
	// Loss, per inbox rather than per daemon: a total tells an operator that
	// something is losing work, and not which name to go and look at.
	Dropped int `json:"dropped,omitempty"` // lost to its overflow, or skipped while off, since then
	Expired int `json:"expired,omitempty"` // outlived their TTL in it since then
	// How long the message at the head of its queue has been waiting. A
	// backlog nobody reads is what an incident looks like, and a count alone
	// cannot say whether that queue is busy or stalled.
	Oldest string `json:"oldest,omitempty"`
	// AtBound says the queue is at the limit it is allowed, so the next
	// message is refused or something is lost. The daemon answers it because
	// a record that declares no bound takes the daemon's, and a reader has
	// no way to know what that is.
	AtBound bool `json:"at_bound,omitempty"`
}

// AccountMapping says which bus principal one local OS account's private
// socket authenticates as. It is configuration, never a credential: the
// socket's ownership is the credential. See docs/09-setup.md#local-users.
type AccountMapping struct {
	Account   string `json:"account"`
	Principal string `json:"principal"`
}

// AccountMappings is the administrative view of the durable map. A change is
// persisted immediately but listeners belong to the supervisor, so the view
// says when a full daemon restart is still needed to apply it.
type AccountMappings struct {
	Mappings        []AccountMapping `json:"mappings"`
	RestartRequired bool             `json:"restart_required"`
}

// Public is what a record looks like to anyone but the service itself: the
// configuration replaced by a digest of it. Every answer that carries a
// record goes through here — a listing, a lookup, a registration and the
// answer to configuring one — because leaving it out has already been the
// same omission twice.
//
// The digest is what makes a query useful without exposing anything
// (ConfigSHA says what it answers).
//
// It is over the bytes AS STORED, which the bus has already compacted, so
// reformatting a configuration file does not look like changing it — and
// `sha256sum cfg.json` will not match one that has whitespace in it. Being a
// digest of the bytes, a SHORT, GUESSABLE configuration is recoverable by
// trying candidates; the answer to that is sealing, not a longer hash.
// See docs/03-records.md#configuring-a-template.
func (r Record) Public() Record {
	if len(r.Config) > 0 {
		sum := sha256.Sum256(r.Config)
		r.ConfigSHA = hex.EncodeToString(sum[:])
		r.Config = nil
	}
	// The secret leaves by its own read and by nothing else. Redacting here
	// is what makes every listing, lookup and page safe by construction
	// rather than by each of them remembering.
	// See docs/06-services.md#secrets.
	if len(r.Secret) > 0 {
		sum := sha256.Sum256([]byte(r.Secret))
		r.SecretSHA = hex.EncodeToString(sum[:])
		r.Secret = ""
	}
	return r
}

// Receipt values. A closed set: anything else is not a receipt.
// See docs/04-messaging.md#receipts.
const (
	ReceiptAck  = "ack"
	ReceiptDone = "done"
)

// What a full queue does, declared per record, inboxes included.
// See docs/04-messaging.md#overflow.
const (
	OverflowStrict = "strict" // refuse the send, and say so
	OverflowRing   = "ring"   // drop the oldest to make room
)

// Record kinds: a closed set, so the daemon answers what a record is rather
// than a reader inferring it. A queue and a pub/sub topic are kinds of their
// own, which is why no separate mode is stored.
// See docs/03-records.md#five-record-kinds.
const (
	KindUser    = "user"    // the queue a person reads
	KindAgent   = "agent"   // the queue an agent reads
	KindQueue   = "queue"   // a queue created to be shared
	KindPubSub  = "pubsub"  // copied to every subscriber, keeps nothing
	KindService = "service" // something external, not on this bus
	// KindGroup is a named list of actors, whose allow list is its
	// membership (docs/constitution.md#-group). It has no queue.
	KindGroup = "group"
)

// Kinds is the whole set, in the order an error message should name them.
var Kinds = []string{KindUser, KindAgent, KindQueue, KindPubSub, KindService}

// ValidKind reports whether the daemon knows this kind. An empty kind is not
// one: a caller that states nothing is given the default before it gets here.
func ValidKind(kind string) bool {
	for _, k := range Kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// KindNames lists the set for a refusal, so a caller is told what it may say.
func KindNames() string { return strings.Join(Kinds, ", ") }

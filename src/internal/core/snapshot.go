package core

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Snapshot is the bus's memory at this moment, for whatever writes it down.
// Records travel with the queues because an inbox without its record is a
// backlog nobody may read. See docs/04-messaging.md#durability.
func (b *Bus) Snapshot() ports.Snapshot {
	b.mu.Lock()
	defer b.unlock()
	return b.snapshot()
}

// snapshot is called with b.mu held.
func (b *Bus) snapshot() ports.Snapshot {
	s := ports.Snapshot{
		OwnerEstablished:    b.admin != "",
		Owner:               b.admin,
		AccountsEstablished: true,
		At:                  time.Now(),
		NextRecordID:        b.nextRecordID,
		NextUserID:          b.nextUserID,
	}
	for account, principal := range b.accounts {
		s.Accounts = append(s.Accounts, protocol.AccountMapping{Account: account, Principal: principal})
	}
	sort.Slice(s.Accounts, func(i, j int) bool { return s.Accounts[i].Account < s.Accounts[j].Account })
	for _, user := range b.users {
		s.Users = append(s.Users, user)
	}
	for _, r := range b.records {
		s.Records = append(s.Records, r)
	}
	for name, in := range b.inboxes {
		// A blocked reader is not state: its connection died with the
		// process, so it is not saved and not counted.
		if in.in == 0 && in.out == 0 && len(in.queue) == 0 {
			continue
		}
		s.Queues = append(s.Queues, ports.Queue{
			Name: name, In: in.in, Out: in.out,
			Dropped: in.dropped, Expired: in.expired,
			Messages: append([]protocol.Envelope(nil), in.queue...),
			Activity: in.act.Save(in.totals()),
		})
	}
	s.Activity = b.node.Save(b.refusedTotal())
	return s
}

// Persistence binds the store before serving requests. Unbound buses are
// intentionally in-memory (unit tests and embedded use).
func (b *Bus) Persistence(st ports.Store) {
	b.mu.Lock()
	defer b.unlock()
	b.store = st
}

// queueMark is what a queue's counters were when it was last saved. Every
// change to a queue's contents moves one of them — a delivery moves in, a
// read out, a loss dropped or expired — and every change to its activity one
// of those or refused, so an unchanged mark is an unchanged queue. A ring
// that only moved on through quiet slots needs no save: a restore reads the
// time since as zero.
type queueMark struct{ in, out, dropped, expired, refused int }

// FlushQueues writes the queues whose state moved since the last flush, as one
// batch: every minute and at a graceful stop, never once per message. clean
// says this is the graceful stop's, which is how the next start tells a clean
// stop from a death (docs/04-messaging.md#durability).
func (b *Bus) FlushQueues(clean bool) error {
	b.mu.Lock()
	defer b.unlock()
	if b.store == nil {
		return nil
	}
	var qs []ports.Queue
	marks := map[string]queueMark{}
	for name, in := range b.inboxes {
		// A queue under a name with no record would be a backlog nobody may
		// read; it is not saved.
		if _, known := b.records[name]; !known {
			continue
		}
		m := queueMark{in.in, in.out, in.dropped, in.expired, in.refused}
		if old, saved := b.flushed[name]; saved && old == m {
			continue
		}
		qs = append(qs, ports.Queue{
			Name: name, In: in.in, Out: in.out,
			Dropped: in.dropped, Expired: in.expired,
			Messages: append([]protocol.Envelope(nil), in.queue...),
			Activity: in.act.Save(in.totals()),
		})
		marks[name] = m
	}
	if err := b.store.SaveQueues(qs, b.node.Save(b.refusedTotal()), clean); err != nil {
		return fmt.Errorf("persist queues: %w", err)
	}
	for name, m := range marks {
		b.flushed[name] = m
	}
	return nil
}

// unlock releases b.mu. A management write that reached here without
// committing failed on the way — validation or otherwise — and is put back,
// so a refused write publishes nothing (docs/constitution.md#persistence-and-loading).
func (b *Bus) unlock() {
	if b.staging != nil {
		b.discard()
	}
	b.mu.Unlock()
}

// Restore puts a snapshot back. Expiry is not re-checked here: a message
// carries the moment it stops being worth delivering, and Consume is the one
// place that decides — a reloaded queue is no different from one that sat
// through a quiet hour. Uptime is not restored: it is this run's.
func (b *Bus) Restore(s ports.Snapshot) {
	b.mu.Lock()
	defer b.unlock()
	b.ownerRestored = s.OwnerEstablished || s.Owner != ""
	b.ownerRestoreErr = nil
	if s.Owner != "" && !s.OwnerEstablished {
		b.admin = ""
		b.ownerRestoreErr = fmt.Errorf("snapshot has daemon owner without establishment marker")
	}
	if s.OwnerEstablished {
		b.admin = ""
		owner, err := canon(s.Owner)
		if err != nil {
			b.ownerRestoreErr = fmt.Errorf("snapshot daemon owner: %w", err)
		} else {
			b.admin = owner
		}
	}
	b.accountsRestored = s.AccountsEstablished || len(s.Accounts) > 0
	b.accountRestoreErr = nil
	b.accounts = map[string]string{}
	if len(s.Accounts) > 0 && !s.AccountsEstablished {
		b.accountRestoreErr = fmt.Errorf("snapshot has local account mappings without establishment marker")
	}
	if s.AccountsEstablished {
		for _, mapping := range s.Accounts {
			principal, err := canon(mapping.Principal)
			if mapping.Account == "" || err != nil {
				b.accountRestoreErr = fmt.Errorf("snapshot local account mapping %q=%q is invalid", mapping.Account, mapping.Principal)
				break
			}
			if _, duplicate := b.accounts[mapping.Account]; duplicate {
				b.accountRestoreErr = fmt.Errorf("snapshot repeats local account %q", mapping.Account)
				break
			}
			b.accounts[mapping.Account] = principal
		}
	}
	// A database that has never been written has had no run to lose.
	b.unclean = !s.Clean && !s.At.IsZero()
	if s.NextRecordID > b.nextRecordID {
		b.nextRecordID = s.NextRecordID
	}
	if s.NextUserID > b.nextUserID {
		b.nextUserID = s.NextUserID
	}
	for _, user := range s.Users {
		// A snapshot may come from another version or an embedding caller.
		// Derived answers belong to this process and its current visitor.
		clearDerivedUser(&user)
		b.users[user.Name] = user
	}
	// The groups that carry authority are checked before anything serves: a
	// stored runtime term, a nested administrators group, or one whose Owner
	// is not the daemon Owner refuses the start (docs/constitution.md#-group).
	for _, r := range s.Records {
		if b.ownerRestoreErr != nil {
			break
		}
		switch {
		case reservedTerm(r.Name):
			b.ownerRestoreErr = fmt.Errorf("snapshot contains runtime ACL term %s as a stored group", r.Name)
		case r.Name == AdministratorsGroup:
			for _, member := range r.Allow {
				if groupName(member) {
					b.ownerRestoreErr = fmt.Errorf("snapshot %s contains nested group %s; administrative membership is direct-only", AdministratorsGroup, member)
					break
				}
			}
			if b.ownerRestoreErr == nil && s.OwnerEstablished && r.Owner != b.admin {
				b.ownerRestoreErr = fmt.Errorf("snapshot %s is owned by %s, not the daemon owner %s", AdministratorsGroup, r.Owner, b.admin)
			}
		}
	}
	if s.OwnerEstablished && b.ownerRestoreErr == nil {
		if _, known := b.users[b.admin]; !known {
			b.ownerRestoreErr = fmt.Errorf("snapshot daemon owner %s is not a registered user", b.admin)
		} else if !b.userActive(b.admin) {
			b.ownerRestoreErr = fmt.Errorf("snapshot daemon owner %s is not active", b.admin)
		}
	}
	// Restore loads; it commits nothing and repairs nothing.
	b.staging = nil
	for _, r := range s.Records {
		// A snapshot may have been written by another version or supplied by
		// an embedding caller. Live fields belong to this process and its
		// current inboxes, never to durable registry state.
		clearLiveRecord(&r)
		b.records[r.Name] = r
	}
	b.indexIDs()
	b.ignoreIncorrect()
	b.ignoreNonUserAdministrators()
	for _, q := range s.Queues {
		// A queue whose record is absent, ignored, or of a kind that holds no
		// queue is incorrect like the record would be: ignored and reported,
		// never reattached (docs/constitution.md#persistence-and-loading).
		r, known := b.records[q.Name]
		if !known || !onBus(r) {
			b.report(ports.Alert, "stored queue %s has no record that can hold it; its %d messages are ignored", q.Name, len(q.Messages))
			continue
		}
		in := b.ensure(q.Name)
		in.in, in.out = q.In, q.Out
		in.dropped, in.expired = q.Dropped, q.Expired
		in.queue = append(in.queue, q.Messages...)
		var readable bool
		in.act, readable = b.restoreActivity(q.Name, q.Activity, in.totals())
		if readable {
			b.flushed[q.Name] = queueMark{in.in, in.out, in.dropped, in.expired, in.refused}
		}
	}
	b.node, _ = b.restoreActivity("the node", s.Activity, b.refusedTotal())
}

// indexIDs rebuilds the ID indexes from the loaded entities and moves the
// high-water marks past every ID in use. A stored database always carries an
// ID; an entity without one comes from a test or an embedding caller and is
// given the next. Two entities claiming one ID are damage: the second is
// ignored and reported (docs/constitution.md#persistence-and-loading).
// Caller holds b.mu.
func (b *Bus) indexIDs() {
	b.recordByID, b.userByID = map[uint32]string{}, map[uint32]string{}
	names := make([]string, 0, len(b.records))
	for name := range b.records {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		r := b.records[name]
		if r.ID >= b.nextRecordID {
			b.nextRecordID = r.ID + 1
		}
	}
	for _, name := range names {
		r := b.records[name]
		if r.ID == 0 {
			r.ID = b.nextRecordID
			b.nextRecordID++
			b.records[name] = r
		}
		if other, taken := b.recordByID[r.ID]; taken {
			b.report(ports.Alert, "stored records %s and %s share internal ID %d; %s is ignored", other, name, r.ID, name)
			delete(b.records, name)
			continue
		}
		b.recordByID[r.ID] = name
	}
	users := make([]string, 0, len(b.users))
	for name := range b.users {
		users = append(users, name)
	}
	sort.Strings(users)
	for _, name := range users {
		if u := b.users[name]; u.ID >= b.nextUserID {
			b.nextUserID = u.ID + 1
		}
	}
	for _, name := range users {
		u := b.users[name]
		if u.ID == 0 {
			u.ID = b.nextUserID
			b.nextUserID++
			b.users[name] = u
		}
		if other, taken := b.userByID[u.ID]; taken {
			b.report(ports.Alert, "stored users %s and %s share internal ID %d; %s is ignored", other, name, u.ID, name)
			delete(b.users, name)
			continue
		}
		b.userByID[u.ID] = name
	}
}

// RecordName and UserName answer an internal ID with the name it belongs to,
// from the index rebuilt at load.
func (b *Bus) RecordName(id uint32) (string, bool) {
	b.mu.Lock()
	defer b.unlock()
	name, ok := b.recordByID[id]
	return name, ok
}

func (b *Bus) UserName(id uint32) (string, bool) {
	b.mu.Lock()
	defer b.unlock()
	name, ok := b.userByID[id]
	return name, ok
}

// ignoreIncorrect takes out of the loaded view every entity this version could
// not have written, and reports each: an incorrect record is always ignored —
// not loaded, not repaired — while the rest of the node starts
// (docs/constitution.md#persistence-and-loading). It runs to a fixed point,
// because ignoring a User makes the records that User owned incorrect too.
// The database is not touched; what is ignored stays there for an operator.
// Caller holds b.mu.
func (b *Bus) ignoreIncorrect() {
	b.ignoreSharedIdentities()
	for {
		var gone []string
		names := make([]string, 0, len(b.records))
		for name := range b.records {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if why := b.incorrect(b.records[name]); why != "" {
				b.report(ports.Alert, "stored record %s is ignored: %s", name, why)
				gone = append(gone, name)
				b.ignored[name] = true
			}
		}
		users := make([]string, 0, len(b.users))
		for name := range b.users {
			users = append(users, name)
		}
		sort.Strings(users)
		for _, name := range users {
			if r, has := b.records[name]; !has || r.Kind != protocol.KindUser {
				b.report(ports.Alert, "stored user %s has no user record of its own and is ignored", name)
				delete(b.userByID, b.users[name].ID)
				delete(b.users, name)
				gone = append(gone, "user "+name)
			}
		}
		for _, name := range gone {
			if r, has := b.records[name]; has {
				delete(b.recordByID, r.ID)
				delete(b.records, name)
			}
		}
		if len(gone) == 0 {
			return
		}
	}
}

// ignoreNonUserAdministrators ignores every @administrators line that names
// no User once the load has ignored what it could not have written: only a
// User administers, and a missing one is not manufactured. A daemon Owner so
// ignored refuses the start. Caller holds b.mu.
func (b *Bus) ignoreNonUserAdministrators() {
	admins, ok := b.records[AdministratorsGroup]
	if ok {
		kept := admins.Allow[:0:0]
		for _, m := range admins.Allow {
			if _, user := b.users[m]; !user {
				b.report(ports.Alert, "stored %s member %s is ignored: it is no user", AdministratorsGroup, m)
				continue
			}
			kept = append(kept, m)
		}
		if len(kept) != len(admins.Allow) {
			admins.Allow = kept
			b.records[AdministratorsGroup] = admins
		}
	}
	if b.ownerRestoreErr == nil && b.admin != "" && b.ownerRestored {
		if _, user := b.users[b.admin]; !user {
			b.ownerRestoreErr = fmt.Errorf("snapshot daemon owner %s is not a registered user", b.admin)
		}
	}
}

// ignoreSharedIdentities ignores every stored User that holds an identifying
// field an earlier User (by ID) already holds: no write of this version lets
// two Users share an email, a GitHub login or a Twitter/X name. The first
// keeps it; the later one is reported and ignored, and the records it owned
// with it. Caller holds b.mu.
func (b *Bus) ignoreSharedIdentities() {
	names := make([]string, 0, len(b.users))
	for name := range b.users {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return b.users[names[i]].ID < b.users[names[j]].ID })
	held := map[string]string{}
	for _, name := range names {
		u := b.users[name]
		keys := map[string]string{}
		if u.Email != "" {
			keys["email "+u.Email] = "email " + u.Email
		}
		if u.GithubUser != "" {
			keys["github "+u.GithubUser] = "GitHub login " + u.GithubUser
		}
		if u.GithubTwitterUsername != "" {
			keys["twitter "+strings.ToLower(u.GithubTwitterUsername)] = "Twitter/X name " + u.GithubTwitterUsername
		}
		clash := ""
		for key, field := range keys {
			if first, taken := held[key]; taken {
				clash = fmt.Sprintf("its %s is %s's", field, first)
				break
			}
		}
		if clash != "" {
			b.report(ports.Alert, "stored user %s is ignored: %s", name, clash)
			delete(b.userByID, u.ID)
			delete(b.users, name)
			continue
		}
		for key := range keys {
			held[key] = name
		}
	}
}

// incorrect says why a stored record could not have been written by this
// version, or "" when it could. Caller holds b.mu.
func (b *Bus) incorrect(r protocol.Record) string {
	if err := validateKind(r); err != nil {
		return err.Error()
	}
	if _, user := b.users[r.Owner]; !user {
		return "its owner " + r.Owner + " is not a user"
	}
	if r.Kind == protocol.KindUser && r.Owner != r.Name {
		return "a user record belongs to its own user"
	}
	if err := b.validatePersonal(r); err != nil {
		return err.Error()
	}
	return ""
}

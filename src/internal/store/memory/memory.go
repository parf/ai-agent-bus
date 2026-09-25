// Package memory is the store adapter that keeps nothing: credentials live
// for as long as the process does. It is what a test uses instead of a
// temporary directory, and the second implementation that makes the port
// more than a name — swapping one is meant to reach nothing inward
// (docs/10-modules.md#the-rule).
package memory

import (
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Tokens holds credentials by name, seeded with whatever the caller wants
// already there. Err, when set, refuses every write.
type Tokens struct {
	mu    sync.Mutex
	Err   error
	creds map[string]ports.Credential
}

func NewTokens(seed ...ports.Credential) *Tokens {
	t := &Tokens{creds: map[string]ports.Credential{}}
	for _, c := range seed {
		t.creds[c.Name] = c
	}
	return t
}

func (t *Tokens) Load() ([]ports.Credential, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]ports.Credential, 0, len(t.creds))
	for _, c := range t.creds {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (t *Tokens) Put(c ports.Credential) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Err != nil {
		return t.Err
	}
	t.creds[c.Name] = c
	return nil
}

func (t *Tokens) Drop(name string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Err != nil {
		return t.Err
	}
	delete(t.creds, name)
	return nil
}

func (t *Tokens) Touch(used map[string]time.Time) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Err != nil {
		return t.Err
	}
	for name, at := range used {
		if c, has := t.creds[name]; has {
			c.Used = at
			t.creds[name] = c
		}
	}
	return nil
}

// apply is a committed change's credential half, as the database applies it
// in the same transaction.
func (t *Tokens) apply(pairs map[string]*ports.CredentialPair) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for name, p := range pairs {
		if p == nil {
			delete(t.creds, name)
		} else if c, has := t.creds[name]; has {
			c.CredentialPair = *p
			t.creds[name] = c
		}
	}
}

// State is a durable-state store that keeps it in memory: a Change applies to
// maps exactly as a database transaction would to tables, so a test can read
// back what the daemon committed and nothing else. Err, when set, refuses every
// write; Commits counts the ones that landed.
type State struct {
	mu       sync.Mutex
	Err      error
	Commits  int
	owner    string
	hasOwner bool
	accounts map[string]string
	hasAccts bool
	users    map[string]protocol.User
	records  map[string]protocol.Record
	queues   map[string]ports.Queue
	tokens   *Tokens
	clean    bool
	nextRec  uint32
	nextUser uint32
	activity []byte
	days     map[[2]string]ports.ActivityDay
	// Enter, when set, is closed as the first commit starts, which then waits
	// for Release: a test's way to hold a write open.
	Enter, Release chan struct{}
	entered        bool
}

func NewState() *State {
	return &State{
		accounts: map[string]string{},
		users:    map[string]protocol.User{},
		records:  map[string]protocol.Record{},
		queues:   map[string]ports.Queue{},
		days:     map[[2]string]ports.ActivityDay{},
		tokens:   NewTokens(),
	}
}

// Tokens is the credential half of the same state, as a database's
// credentials are the same file: a committed change's credentials land here.
func (s *State) Tokens() *Tokens { return s.tokens }

func (s *State) Load() (ports.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := ports.Snapshot{
		OwnerEstablished: s.hasOwner, Owner: s.owner,
		AccountsEstablished: s.hasAccts, Clean: s.clean,
		NextRecordID: s.nextRec, NextUserID: s.nextUser,
		Activity: append([]byte(nil), s.activity...),
	}
	for account, principal := range s.accounts {
		snap.Accounts = append(snap.Accounts, protocol.AccountMapping{Account: account, Principal: principal})
	}
	sort.Slice(snap.Accounts, func(i, j int) bool { return snap.Accounts[i].Account < snap.Accounts[j].Account })
	for _, u := range s.users {
		snap.Users = append(snap.Users, u)
	}
	for _, r := range s.records {
		snap.Records = append(snap.Records, r)
	}
	for _, q := range s.queues {
		q.Messages = append([]protocol.Envelope(nil), q.Messages...)
		q.Activity = append([]byte(nil), q.Activity...)
		snap.Queues = append(snap.Queues, q)
	}
	return snap, nil
}

func (s *State) Commit(c ports.Change) error {
	s.mu.Lock()
	if s.Enter != nil && !s.entered {
		s.entered = true
		close(s.Enter)
		s.mu.Unlock()
		<-s.Release
		s.mu.Lock()
	}
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	if c.Owner != nil {
		s.owner, s.hasOwner = *c.Owner, true
	}
	if c.Accounts != nil {
		s.accounts, s.hasAccts = map[string]string{}, true
		for account, principal := range c.Accounts {
			s.accounts[account] = principal
		}
	}
	for name, u := range c.Users {
		if u == nil {
			delete(s.users, name)
		} else {
			s.users[name] = *u
		}
	}
	for name, r := range c.Records {
		if r == nil {
			delete(s.records, name)
		} else {
			s.records[name] = *r
		}
	}
	for _, name := range c.DropQueues {
		delete(s.queues, name)
		for k := range s.days {
			if k[1] == name {
				delete(s.days, k)
			}
		}
	}
	s.tokens.apply(c.Credentials)
	if c.NextRecordID != nil {
		s.nextRec = *c.NextRecordID
	}
	if c.NextUserID != nil {
		s.nextUser = *c.NextUserID
	}
	s.Commits++
	return nil
}

func (s *State) SaveQueues(qs []ports.Queue, activity []byte, clean bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	for _, q := range qs {
		q.Messages = append([]protocol.Envelope(nil), q.Messages...)
		q.Activity = append([]byte(nil), q.Activity...)
		s.queues[q.Name] = q
	}
	if activity != nil {
		s.activity = append([]byte(nil), activity...)
	}
	s.clean = clean
	return nil
}

func (s *State) Close() error { return nil }

func dayKey(date int, name string) [2]string { return [2]string{strconv.Itoa(date), name} }

func (s *State) SaveActivityDays(days []ports.ActivityDay) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	for _, d := range days {
		if len(d.Slots) == 0 {
			delete(s.days, dayKey(d.Date, d.Name))
			continue
		}
		d.Slots = append([]byte(nil), d.Slots...)
		s.days[dayKey(d.Date, d.Name)] = d
	}
	return nil
}

func (s *State) ActivityDays(from, to int, names []string) ([]ports.ActivityDay, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	var out []ports.ActivityDay
	for _, d := range s.days {
		if d.Date >= from && d.Date <= to && (names == nil || want[d.Name]) {
			out = append(out, ports.ActivityDay{Date: d.Date, Name: d.Name, Slots: append([]byte(nil), d.Slots...)})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (s *State) PruneActivity(before int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, d := range s.days {
		if d.Date < before {
			delete(s.days, k)
		}
	}
	return nil
}

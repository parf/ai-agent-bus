// Package memory is the store adapter that keeps nothing: credentials live
// for as long as the process does. It is what a test uses instead of a
// temporary directory, and the second implementation that makes the port
// more than a name — swapping one is meant to reach nothing inward
// (docs/10-modules.md#the-rule).
package memory

import (
	"sort"
	"sync"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Tokens holds the set, seeded with whatever the caller wants already there.
type Tokens struct {
	mu    sync.Mutex
	creds []ports.Credential
}

func NewTokens(seed ...ports.Credential) *Tokens { return &Tokens{creds: seed} }

func (t *Tokens) Load() ([]ports.Credential, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]ports.Credential(nil), t.creds...), nil
}

func (t *Tokens) Save(creds []ports.Credential) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.creds = append([]ports.Credential(nil), creds...)
	return nil
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
	groups   map[string][]string
	queues   map[string]ports.Queue
	clean    bool
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
		groups:   map[string][]string{},
		queues:   map[string]ports.Queue{},
	}
}

func (s *State) Load() (ports.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := ports.Snapshot{
		OwnerEstablished: s.hasOwner, Owner: s.owner,
		AccountsEstablished: s.hasAccts, Clean: s.clean,
		Groups: map[string][]string{},
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
	for name, members := range s.groups {
		snap.Groups[name] = append([]string{}, members...)
	}
	for _, q := range s.queues {
		q.Messages = append([]protocol.Envelope(nil), q.Messages...)
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
	for name, members := range c.Groups {
		if members == nil {
			delete(s.groups, name)
		} else {
			s.groups[name] = append([]string{}, (*members)...)
		}
	}
	for _, name := range c.DropQueues {
		delete(s.queues, name)
	}
	s.Commits++
	return nil
}

func (s *State) SaveQueues(qs []ports.Queue, clean bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	for _, q := range qs {
		q.Messages = append([]protocol.Envelope(nil), q.Messages...)
		s.queues[q.Name] = q
	}
	s.clean = clean
	return nil
}

func (s *State) Close() error { return nil }

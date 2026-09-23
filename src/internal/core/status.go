package core

import (
	"fmt"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Every User and record is active or inactive, and an inactive one is no such
// entity: it is in no listing, every operation naming it answers unknown, it
// grants nothing and it keeps its name reserved. An inactive User makes every
// record it owns inactive too. The one exception is the web face's read-only
// view of inactive records (Inactive), and the way back is a status edit by
// the record's Owner or a Maintainer, never re-creation.
// See docs/constitution.md#common-record-fields.

// userActive is a User's own status. A name that is no User is not active.
// Caller holds b.mu.
func (b *Bus) userActive(name string) bool {
	u, ok := b.users[name]
	return ok && u.Status != protocol.StatusInactive
}

// live says whether a record is an entity: its own status active and its
// Owner an active User. A User's own record lives exactly as its User does.
// Caller holds b.mu.
func (b *Bus) live(r protocol.Record) bool {
	return r.Status != protocol.StatusInactive && b.userActive(r.Owner)
}

// entity is the record under name when it is live. Every path that serves a
// name asks this rather than the map, so an inactive record and an absent one
// are the same answer. Caller holds b.mu.
func (b *Bus) entity(name string) (protocol.Record, bool) {
	r, ok := b.records[name]
	if !ok || !b.live(r) {
		return protocol.Record{}, false
	}
	return r, true
}

// callerActive says why a known principal may not act, or nil: a User must be
// active, and an Agent live — its own status and its Owner's. The answer is
// `403 suspended`, never unknown: the caller is somebody, and is told it is
// the one refused. Caller holds b.mu.
func (b *Bus) callerActive(name string) error {
	if _, user := b.users[name]; user {
		if !b.userActive(name) {
			return fmt.Errorf("%w: %s is inactive", ErrInactive, name)
		}
		return nil
	}
	r, ok := b.records[name]
	if !ok {
		return nil
	}
	if r.Status == protocol.StatusInactive {
		return fmt.Errorf("%w: %s is inactive", ErrInactive, name)
	}
	if !b.userActive(r.Owner) {
		return fmt.Errorf("%w: %s is owned by %s, who is inactive", ErrInactive, name, r.Owner)
	}
	return nil
}

// validStatus normalizes a status write: empty keeps what there is.
func validStatus(s string) error {
	if s != "" && s != protocol.StatusActive && s != protocol.StatusInactive {
		return fmt.Errorf("%w: status is active or inactive, not %q", ErrBadName, s)
	}
	return nil
}

// groupMembers is a live Group's membership, its allow list; an absent or
// inactive Group has none and grants nothing (docs/constitution.md#-group).
// Caller holds b.mu.
func (b *Bus) groupMembers(name string) ([]string, bool) {
	r, ok := b.records[name]
	if !ok || r.Kind != protocol.KindGroup || !b.live(r) {
		return nil, false
	}
	return r.Allow, true
}

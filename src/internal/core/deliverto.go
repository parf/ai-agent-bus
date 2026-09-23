package core

import (
	"fmt"
	"strings"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Deliver-To is who receives a copy of a 📣 publication, and is the second of
// a topic's two lists: the ACL says who may publish, this says who gets the
// traffic, and neither stands in for the other. It is stored in Record.Subs.
// See docs/04-messaging.md#subscribers.

// canReceive is the kinds a copy can be put into. A User receives only
// replies to what it sent, never a published copy
// (docs/constitution.md#-channels), and a 📡 has no queue at all. **Pending
// for 0.7:** a 📮 or 📣 recipient, which is forwarding (K.15).
func canReceive(r protocol.Record) bool {
	return r.Kind == protocol.KindAgent
}

// normalizeDeliverTo checks a whole replacement before Manage or a
// registration stores any of it, the way a Maintainer list is checked. A
// group is kept as the group rather than flattened into its members here, so
// that adding somebody to it adds them to the delivery; expansion is
// deliverTo's job, at the publish.
// Caller holds b.mu.
func (b *Bus) normalizeDeliverTo(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, raw := range in {
		var term string
		if strings.HasPrefix(strings.TrimSpace(raw), "@") {
			term = strings.TrimSpace(raw)
			if reservedTerm(term) {
				return nil, fmt.Errorf("%w: %s is a runtime ACL term about who may publish, not a set of inboxes", ErrBadName, term)
			}
			if !groupName(term) {
				return nil, fmt.Errorf("%w: invalid deliver-to group %q", ErrBadName, raw)
			}
			if _, ok := b.groups[term]; !ok {
				return nil, fmt.Errorf("%w: deliver-to %s", ErrUnknown, term)
			}
		} else {
			var err error
			term, err = canon(raw)
			if err != nil {
				return nil, err
			}
			r, registered := b.records[term]
			if !registered {
				return nil, fmt.Errorf("%w: deliver-to %s, which has to be registered first so its copies have somewhere to land", ErrUnknown, term)
			}
			if !canReceive(r) {
				return nil, fmt.Errorf("%w: deliver-to %s must be an agent or a group, and a %s takes no published copy", ErrBadName, term, r.Kind)
			}
		}
		if seen[term] {
			return nil, fmt.Errorf("%w: duplicate deliver-to %s", ErrBadName, term)
		}
		seen[term] = true
		out = append(out, term)
	}
	return out, nil
}

// deliverTo is the list with its groups expanded, each name once, in the order
// the list states them. It is resolved at the publish rather than at the write
// because membership then is what the owner means: putting somebody in the
// group is how they start receiving.
//
// Nesting inside @administrators is not followed, matching memberThrough: that
// group answers one question for the ACL and cannot answer a different one
// here, or the same entry would mean two sets of people.
// Caller holds b.mu.
func (b *Bus) deliverTo(topic protocol.Record) []string {
	out := make([]string, 0, len(topic.Subs))
	seen := map[string]bool{}
	walked := map[string]bool{}
	var add func(string)
	add = func(term string) {
		if groupName(term) {
			if walked[term] {
				return // already expanded, or nested in itself
			}
			walked[term] = true
			for _, m := range b.groups[term] {
				if groupName(m) && term == AdministratorsGroup {
					continue
				}
				add(m)
			}
			return
		}
		if seen[term] {
			return
		}
		seen[term] = true
		out = append(out, term)
	}
	for _, s := range topic.Subs {
		add(s)
	}
	return out
}

// receives says whether name gets copies of what is published to topic,
// through the list itself or through a group on it. Caller holds b.mu.
func (b *Bus) receives(topic protocol.Record, name string) bool {
	for _, s := range b.deliverTo(topic) {
		if s == name {
			return true
		}
	}
	return false
}

// receivesThrough names the group a name receives through, or "" when it is on
// the list under its own name or not at all. It is the difference between a
// removal that works and one that silently takes nothing out.
// Caller holds b.mu.
func (b *Bus) receivesThrough(topic protocol.Record, name string) string {
	for _, s := range topic.Subs {
		if s == name {
			return ""
		}
	}
	for _, s := range topic.Subs {
		if groupName(s) && b.receives(protocol.Record{Subs: []string{s}}, name) {
			return s
		}
	}
	return ""
}

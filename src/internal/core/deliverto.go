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

// canReceive is the kinds a published copy can be put into: an Agent's or a
// Queue's inbox. A 📣 recipient is a further publication, and a User receives
// only replies to what it sent, never a published copy
// (docs/constitution.md#-channels); a 📡 has no queue at all.
func canReceive(r protocol.Record) bool {
	return r.Kind == protocol.KindAgent || r.Kind == protocol.KindQueue
}

// deliverToKinds is where each kind's deliver_to may point: a 📣 list takes
// Agents, Groups, Queues and PubSubs; the one slot of an 👾 or 📮 takes an
// Agent, a Queue or a PubSub (docs/constitution.md#common-record-fields).
func deliverToAccepts(owner, target string) bool {
	switch target {
	case protocol.KindAgent, protocol.KindQueue, protocol.KindPubSub:
		return owner == protocol.KindPubSub || owner == protocol.KindAgent || owner == protocol.KindQueue
	case protocol.KindGroup:
		return owner == protocol.KindPubSub
	}
	return false
}

// normalizeDeliverTo checks a whole replacement for r's deliver_to before any
// of it is stored, the way a Maintainer list is checked. Each term is resolved
// against the registry and refused for its kind, never for permission; a
// group is kept as the group rather than flattened, so that adding somebody to
// it adds them to the delivery — expansion is deliverTo's job, at the publish.
// An 👾 or 📮 holds at most one destination. Caller holds b.mu.
func (b *Bus) normalizeDeliverTo(in []string, r protocol.Record) ([]string, error) {
	oneSlot := r.Kind == protocol.KindAgent || r.Kind == protocol.KindQueue
	if len(in) > 0 && r.Kind != protocol.KindPubSub && !oneSlot {
		return nil, fmt.Errorf("%w: a %s has no deliver_to", ErrKind, r.Kind)
	}
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, raw := range in {
		var term string
		if strings.HasPrefix(strings.TrimSpace(raw), "@") {
			term = strings.ToLower(strings.TrimSpace(raw))
			if reservedTerm(term) {
				return nil, fmt.Errorf("%w: %s is a runtime ACL term about who may publish, not a set of inboxes", ErrBadName, term)
			}
			if !groupName(term) {
				return nil, fmt.Errorf("%w: invalid deliver-to group %q", ErrBadName, raw)
			}
			if _, ok := b.groupMembers(term); !ok {
				return nil, fmt.Errorf("%w: deliver-to %s", ErrUnknown, term)
			}
			if !deliverToAccepts(r.Kind, protocol.KindGroup) {
				return nil, fmt.Errorf("%w: a %s delivers to one destination, and group %s is many", ErrBadName, r.Kind, term)
			}
		} else {
			var err error
			term, err = canon(raw)
			if err != nil {
				return nil, err
			}
			target, registered := b.entity(term)
			if !registered {
				return nil, fmt.Errorf("%w: deliver-to %s, which has to be registered first so its copies have somewhere to land", ErrUnknown, term)
			}
			if !deliverToAccepts(r.Kind, target.Kind) {
				return nil, fmt.Errorf("%w: deliver-to %s must be an agent, a queue or a pubsub, and a %s takes no delivered copy", ErrBadName, term, target.Kind)
			}
			// A route is stored only where its destination lists the
			// forwarding record itself, and is checked again at delivery
			// (docs/constitution.md#-channels).
			if oneSlot && !b.admits(target, r.Name) {
				return nil, fmt.Errorf("%w: %s does not allow %s, so it cannot be %s's route", ErrNotAllow, term, r.Name, r.Name)
			}
		}
		if seen[term] {
			return nil, fmt.Errorf("%w: duplicate deliver-to %s", ErrBadName, term)
		}
		seen[term] = true
		out = append(out, term)
	}
	if oneSlot && len(out) > 1 {
		return nil, fmt.Errorf("%w: a %s's deliver_to holds one destination, not %d", ErrBadName, r.Kind, len(out))
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
			members, _ := b.groupMembers(term)
			for _, m := range members {
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

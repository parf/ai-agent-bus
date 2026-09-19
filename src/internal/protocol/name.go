// Package protocol is pure: names, the envelope, and nothing that touches the
// outside world. See docs/10-modules.md.
package protocol

import (
	"fmt"
	"strings"
)

// A principal is user@realm; a service is [template/]name@host. The name is
// the identity — never a provider's numeric id. See docs/01-identity-and-roles.md#names.
//
// Every name is canonicalised as lower-case and ASCII only, trimmed as a whole
// and again per component, so "mail-sender / parf@comfi.com @ host" is the one
// name mail-sender/parf@comfi.com@host. Two spellings of one name are one
// name, so a registration and a send cannot land in different inboxes, and
// nothing depends on how a shell or a config file happened to capitalise it.
//
// The template and the realm are [a-z0-9._-], starting alphanumeric — a realm
// can be a host, parf@om.parf.dev. The instance name is wider; see isLocal.
// The whole name is at most MaxName.
//
// The optional template part says which service template this service was
// configured from — code-review/claude-2@rdvp. It is part of the identity and
// the inbox, not a lookup: two services from one template are two services
// with two inboxes. A service that is its own template omits it.

// part reports whether s is one component. A hand-written loop rather than a
// regexp: ParseName runs five times on an ordinary send-and-read pair — the
// auth header, the two envelope ends, the inbox — and the two regexps were
// 84% of it. This is the same grammar, 5x faster and allocation-free.
func part(s string, plus bool) bool {
	if s == "" || !alnum(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if c := s[i]; !alnum(c) && c != '.' && c != '_' && c != '-' && !(plus && c == '+') {
			return false
		}
	}
	return true
}

func alnum(c byte) bool { return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' }

// An instance name may itself contain "@" — an address is a perfectly good
// name for the thing that reads it: mail-sender/parf@comfi.com@host. So the
// realm is split off at the LAST "@", and only the instance name carries the
// wider charset. The bus never reads meaning out of it: it is a name, not a
// mailbox it parses. See docs/03-records.md#agent-templates.
//
// It is ordinary components joined by single "@", not a free-for-all: empty
// @-separated components are not allowed, so "parf@" and "parf@@x" are
// refused. That is a deliberately narrow contract, not a canonicalisation
// necessity — nothing can infer from an opaque name that "parf@@host" was
// meant to be "parf@host", so this refuses the typo rather than guessing.
//
// The components also take "+", so plus-addressing works:
// mail-sender/parf+alerts@comfi.com@host. That is one deliberate character,
// added on the owner's word — not email syntax. Anything a mailbox grammar
// allows beyond this charset is still refused, and widening it again is a
// decision to take on purpose rather than by adopting RFC 5322.
func isLocal(s string) bool {
	for {
		before, after, cut := strings.Cut(s, "@")
		if !part(before, true) {
			return false
		}
		if !cut {
			return true
		}
		s = after
	}
}

// MaxName bounds a canonical name, `user@realm` and the @ included. A name is
// an identifier, not a payload: it is logged, indexed and shown in a list.
const MaxName = 64

// Name is how every principal is written.
type Name struct {
	Template string // service template this service was configured from, or ""
	Local    string // user, or service name
	Realm    string // the name a daemon answers for: a host by default, a pool, a provider
}

func (n Name) String() string {
	if n.Template == "" {
		return n.Local + "@" + n.Realm
	}
	return n.Template + "/" + n.Local + "@" + n.Realm
}

// ParseName accepts "user@realm" or "template/name@realm" in any
// capitalisation, with surrounding space, and returns the one canonical form.
// A bare "user" is not a name: the realm is what makes it addressable from
// anywhere. The realm is whatever follows the LAST "@", and never holds a "/"
// — a realm is a name, not a path.
func ParseName(s string) (Name, error) {
	s = strings.TrimSpace(s)
	if !isASCII(s) {
		return Name{}, fmt.Errorf("name %q is not ASCII: names are ASCII only", s)
	}
	lower := strings.ToLower(s)
	at := strings.LastIndex(lower, "@")
	if at < 0 {
		return Name{}, fmt.Errorf("name %q has no realm: want user@realm", s)
	}
	local, realm := lower[:at], lower[at+1:]
	local, realm = strings.TrimSpace(local), strings.TrimSpace(realm)
	var template string
	if before, after, cut := strings.Cut(local, "/"); cut {
		template, local = strings.TrimSpace(before), strings.TrimSpace(after)
		if !part(template, false) {
			return Name{}, fmt.Errorf("bad template in %q: a-z 0-9 . _ - only, starting alphanumeric", s)
		}
	}
	if !isLocal(local) {
		return Name{}, fmt.Errorf("bad name %q: a-z 0-9 . _ - + @ only, starting alphanumeric", s)
	}
	if !part(realm, false) {
		return Name{}, fmt.Errorf("bad realm in %q: a-z 0-9 . _ - only, starting alphanumeric", s)
	}
	// Counted rather than rendered: String allocates, and this runs on every
	// call the daemon serves.
	size := len(local) + 1 + len(realm)
	if template != "" {
		size += len(template) + 1
	}
	if size > MaxName {
		return Name{}, fmt.Errorf("name %q is %d characters: at most %d", s, size, MaxName)
	}
	return Name{Template: template, Local: local, Realm: realm}, nil
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 0x7f {
			return false
		}
	}
	return true
}

// SigNamespace is what an enrolment signature is made for. It keeps a
// signature made for this bus from being usable anywhere else that verifies
// sshsig, and the other way round — it is on the wire, so it belongs here
// rather than in whatever tool checks it.
// See docs/01-identity-and-roles.md#registration.
const SigNamespace = "agent-bus"

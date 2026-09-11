// Package protocol is pure: names, the envelope, and nothing that touches the
// outside world. See docs/10-modules.md.
package protocol

import (
	"fmt"
	"regexp"
	"strings"
)

// A principal is user@realm; a service is [template/]name@host. All parts are
// the same shape, and the name is the identity — never a provider's numeric
// id. See docs/01-identity.md#names.
//
// Every name is canonicalised as trim, lower-case, ASCII only. Two spellings
// of one name are one name, so a registration and a send cannot land in
// different inboxes, and nothing depends on how a shell or a config file
// happened to capitalise it.
//
// Every part is [a-z0-9._-], starting alphanumeric: one charset, and a realm
// can be a host — parf@om.parf.dev. The whole name is at most MaxName.
//
// The optional template part says which service template this service was
// configured from — code-review/claude-2@rdvp. It is part of the identity and
// the inbox, not a lookup: two services from one template are two services
// with two inboxes. A service that is its own template omits it.
var partRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// An instance name may itself contain "@" — an address is a perfectly good
// name for the thing that reads it: mail-sender/parf@comfi.com@host. So the
// realm is split off at the LAST "@", and only the instance name carries the
// wider charset. The bus never reads meaning out of it: it is a name, not a
// mailbox it parses. See docs/03-services-and-topics.md#service-and-template.
//
// It is ordinary parts joined by single "@", not a free-for-all: "parf@" and
// "parf@@x" are typos, and a name with a dangling separator is one intent with
// two spellings.
var localRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*(@[a-z0-9][a-z0-9._-]*)*$`)

// MaxName bounds a canonical name, `user@realm` and the @ included. A name is
// an identifier, not a payload: it is logged, indexed and shown in a list.
const MaxName = 64

// Name is one half of the two parameters every call carries.
type Name struct {
	Template string // service template this service was configured from, or ""
	Local    string // user, or service name
	Realm    string // host, identity provider, or team
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
// — a realm is a host, not a path.
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
		if !partRe.MatchString(template) {
			return Name{}, fmt.Errorf("bad template in %q: a-z 0-9 . _ - only, starting alphanumeric", s)
		}
	}
	if !localRe.MatchString(local) {
		return Name{}, fmt.Errorf("bad name %q: a-z 0-9 . _ - @ only, starting alphanumeric", s)
	}
	if !partRe.MatchString(realm) {
		return Name{}, fmt.Errorf("bad realm in %q: a-z 0-9 . _ - only, starting alphanumeric", s)
	}
	n := Name{Template: template, Local: local, Realm: realm}
	if len(n.String()) > MaxName {
		return Name{}, fmt.Errorf("name %q is %d characters: at most %d", s, len(n.String()), MaxName)
	}
	return n, nil
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 0x7f {
			return false
		}
	}
	return true
}

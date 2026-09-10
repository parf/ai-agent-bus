// Package protocol is pure: names, the envelope, and nothing that touches the
// outside world. See docs/10-modules.md.
package protocol

import (
	"fmt"
	"regexp"
	"strings"
)

// A principal is user@realm; a service instance is name@host. Both are the
// same shape, and the name is the identity — never a provider's numeric id.
// See docs/01-identity.md#names.
//
// Every name is canonicalised as trim, lower-case, ASCII only. Two spellings
// of one name are one name, so a registration and a send cannot land in
// different inboxes, and nothing depends on how a shell or a config file
// happened to capitalise it.
//
// Both halves are [a-z0-9._-], starting alphanumeric: one charset, and a realm
// can be a host — parf@om.parf.dev.
var partRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// Name is one half of the two parameters every call carries.
type Name struct {
	Local string // user, or service name
	Realm string // host, identity provider, or team
}

func (n Name) String() string { return n.Local + "@" + n.Realm }

// ParseName accepts "user@realm" in any capitalisation, with surrounding
// space, and returns the one canonical form. A bare "user" is not a name: the
// realm is what makes it addressable from anywhere.
func ParseName(s string) (Name, error) {
	s = strings.TrimSpace(s)
	if !isASCII(s) {
		return Name{}, fmt.Errorf("name %q is not ASCII: names are ASCII only", s)
	}
	local, realm, found := strings.Cut(strings.ToLower(s), "@")
	if !found {
		return Name{}, fmt.Errorf("name %q has no realm: want user@realm", s)
	}
	local, realm = strings.TrimSpace(local), strings.TrimSpace(realm)
	if !partRe.MatchString(local) {
		return Name{}, fmt.Errorf("bad name %q: a-z 0-9 . _ - only, starting alphanumeric", s)
	}
	if !partRe.MatchString(realm) {
		return Name{}, fmt.Errorf("bad realm in %q: a-z 0-9 . _ - only, starting alphanumeric", s)
	}
	return Name{Local: local, Realm: realm}, nil
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 0x7f {
			return false
		}
	}
	return true
}

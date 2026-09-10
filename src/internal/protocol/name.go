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
// See docs/01-identity.md.
var partRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

// Name is one half of the two parameters every call carries.
type Name struct {
	Local string // user, or service name
	Realm string // host, identity provider, or team
}

func (n Name) String() string { return n.Local + "@" + n.Realm }

// ParseName accepts "user@realm". A bare "user" is not a name: the realm is
// what makes it addressable from anywhere.
func ParseName(s string) (Name, error) {
	local, realm, found := strings.Cut(strings.TrimSpace(s), "@")
	if !found {
		return Name{}, fmt.Errorf("name %q has no realm: want user@realm", s)
	}
	if !partRe.MatchString(local) {
		return Name{}, fmt.Errorf("bad name %q", s)
	}
	if !partRe.MatchString(realm) {
		return Name{}, fmt.Errorf("bad realm in %q", s)
	}
	return Name{Local: local, Realm: realm}, nil
}

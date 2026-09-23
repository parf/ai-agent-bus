package protocol

import "testing"

// The sigil rule leans on what a name may start with, so the two must agree:
// a name that begins alphanumeric is a User's or a record's, a "#" makes it an
// Agent's, and "@" is left to Groups and the runtime terms, which ParseName
// never accepts. See docs/constitution.md#actors-and-ascii-textarea-syntax.
func TestSigilsAreFreeOfNames(t *testing.T) {
	if n, err := ParseName("0xdead@github"); err != nil || n.Agent {
		t.Fatalf("a digit-initial handle is a real, unprefixed name: %+v, %v", n, err)
	}
	if n, err := ParseName("#batcher@srv1"); err != nil || !n.Agent {
		t.Fatalf("an agent's name is its own sigil: %+v, %v", n, err)
	}
	for _, s := range []string{"@dev@company", "@owner", "@agent"} {
		if _, err := ParseName(s); err == nil {
			t.Fatalf("%q parsed as a name; the group sigil would be ambiguous", s)
		}
	}
}

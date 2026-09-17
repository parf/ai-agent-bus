package protocol

import "testing"

// The sigil rule leans on what a name may start with, so the two must agree:
// anything a name can begin with is a user, and the sigils are free.
// See Plans/R1/identity.md#sigils.
func TestSigilsAreFreeOfNames(t *testing.T) {
	if _, err := ParseName("0xdead@github"); err != nil {
		t.Fatalf("a digit-initial handle is a real name: %v", err)
	}
	for _, s := range []string{"@dev@company", "#batcher@srv1"} {
		if _, err := ParseName(s); err == nil {
			t.Fatalf("%q parsed as a name; the sigil would be ambiguous", s)
		}
	}
}

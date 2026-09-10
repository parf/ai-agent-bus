package protocol

import (
	"strings"
	"testing"
)

func TestParseNameCanonicalises(t *testing.T) {
	// trim, lower-case, ASCII only — one name has one spelling.
	for _, in := range []string{"parf@localhost", "  parf@localhost  ", "PARF@LOCALHOST", "Parf@LocalHost", "parf @ localhost"} {
		n, err := ParseName(in)
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if n.String() != "parf@localhost" {
			t.Fatalf("%q canonicalised to %q", in, n)
		}
	}
}

// Dots are allowed on both sides: a realm is often a host, and a local part
// may be dotted too.
func TestDotsAreAllowedOnBothSides(t *testing.T) {
	for in, want := range map[string]string{
		"Parf@OM.Parf.Dev":  "parf@om.parf.dev",
		"first.last@srv1":   "first.last@srv1",
		"slack.reader@host": "slack.reader@host",
	} {
		n, err := ParseName(in)
		if err != nil || n.String() != want {
			t.Fatalf("%q: got %q, %v", in, n, err)
		}
	}
}

// The whole name is bounded, not each half: user@realm and the @ included.
func TestNameLengthIsBounded(t *testing.T) {
	fits := strings.Repeat("a", MaxName-len("@srv1")) + "@srv1"
	if n, err := ParseName(fits); err != nil || len(n.String()) != MaxName {
		t.Fatalf("a %d-character name should fit: %v", MaxName, err)
	}
	if _, err := ParseName(strings.Repeat("a", MaxName) + "@srv1"); err == nil {
		t.Fatal("a name longer than the bound was accepted")
	}
	// Halves that each fit but together do not.
	long := strings.Repeat("a", 40) + "@" + strings.Repeat("b", 40)
	if _, err := ParseName(long); err == nil {
		t.Fatal("two 40-character halves make an 81-character name; it was accepted")
	}
}

func TestParseNameRejects(t *testing.T) {
	for _, in := range []string{
		"parf",           // no realm
		"@localhost",     // no local part
		"parf@",          // no realm part
		"parf@локалхост", // not ASCII
		"pärf@host",      // not ASCII
		"-parf@host",     // must start alphanumeric
		"parf@host/x",    // no slashes: a name is not a path
		"",
	} {
		if n, err := ParseName(in); err == nil {
			t.Fatalf("%q was accepted as %q", in, n)
		}
	}
}

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
		"parf@host/x",    // the realm is a name, never a path
		"",
	} {
		if n, err := ParseName(in); err == nil {
			t.Fatalf("%q was accepted as %q", in, n)
		}
	}
}

// A service is service@host, or template/instance-name@host when it was
// configured from a service template. The template part is part of the
// identity: it is carried, canonicalised and printed back.
func TestTemplateNames(t *testing.T) {
	for in, want := range map[string]string{
		"code-review/claude-2@rdvp":       "code-review/claude-2@rdvp",
		"  IMAP-Mail-Reader/Billing@RDVP": "imap-mail-reader/billing@rdvp",
		"imap-mail-reader/parf@rdvp":      "imap-mail-reader/parf@rdvp",
	} {
		n, err := ParseName(in)
		if err != nil || n.String() != want {
			t.Fatalf("%q: got %q, %v", in, n, err)
		}
		if n.Template == "" {
			t.Fatalf("%q parsed with no template part", in)
		}
	}
}

// The template part is optional: a service that is its own template is
// service@host, and it must not acquire an empty template part.
func TestTemplateIsOptional(t *testing.T) {
	n, err := ParseName("claude@rdvp")
	if err != nil {
		t.Fatal(err)
	}
	if n.Template != "" || n.String() != "claude@rdvp" {
		t.Fatalf("a two-part name grew a template: %+v", n)
	}
}

// Two services from one template are two services: same template, different
// instance, different name — so different inboxes.
func TestOneTemplateManyServices(t *testing.T) {
	a, _ := ParseName("code-review/claude@rdvp")
	b, _ := ParseName("code-review/claude-2@rdvp")
	if a == b || a.String() == b.String() {
		t.Fatal("two instances of one template collapsed into one name")
	}
	if a.Template != b.Template {
		t.Fatal("the shared template part did not survive parsing")
	}
}

func TestTemplateNamesRejected(t *testing.T) {
	for _, in := range []string{
		"/claude@rdvp",             // no template part
		"code-review/@rdvp",        // no instance name
		"code-review//claude@rdvp", // the second slash is not a name character
		"-code-review/claude@rdvp", // must start alphanumeric
		"code-review/claude@rd/vp", // a realm is a name, not a path
		"a/b/c@rdvp",               // one slash, not two
	} {
		if n, err := ParseName(in); err == nil {
			t.Fatalf("%q was accepted as %q", in, n)
		}
	}
}

// The bound is the whole name, template and both separators included.
func TestTemplateCountsTowardTheBound(t *testing.T) {
	long := strings.Repeat("a", 30) + "/" + strings.Repeat("b", 30) + "@srv1"
	if len(long) <= MaxName {
		t.Fatalf("test is wrong: %d characters does not exceed the bound", len(long))
	}
	if _, err := ParseName(long); err == nil {
		t.Fatal("a name over the bound was accepted because the template was not counted")
	}
}

// The realm is whatever follows the LAST "@". An instance name may be an
// address — the thing that reads a mailbox is reasonably named after it — and
// the bus reads no meaning out of it.
func TestRealmIsSplitOnTheLastAt(t *testing.T) {
	n, err := ParseName("mail-sender/parf@comfi.com@host")
	if err != nil {
		t.Fatal(err)
	}
	if n.Template != "mail-sender" {
		t.Fatalf("template: %q", n.Template)
	}
	if n.Local != "parf@comfi.com" {
		t.Fatalf("instance name: %q", n.Local)
	}
	if n.Realm != "host" {
		t.Fatalf("host: %q", n.Realm)
	}
	if n.String() != "mail-sender/parf@comfi.com@host" {
		t.Fatalf("round trip: %q", n.String())
	}
}

// The same, with no template part, and with capitals and space to prove the
// one-spelling rule still holds over the wider charset.
func TestAddressShapedNameWithoutTemplate(t *testing.T) {
	n, err := ParseName("  Parf@Comfi.COM@RDVP ")
	if err != nil {
		t.Fatal(err)
	}
	if n.Template != "" || n.Local != "parf@comfi.com" || n.Realm != "rdvp" {
		t.Fatalf("%+v", n)
	}
	if n.String() != "parf@comfi.com@rdvp" {
		t.Fatalf("round trip: %q", n.String())
	}
}

// Splitting on the last "@" must not make an empty instance name or an empty
// host legal, and must not let the wider charset leak into the host.
func TestLastAtDoesNotLegaliseEmptyParts(t *testing.T) {
	for _, in := range []string{
		"parf@comfi.com@", // no host
		"@comfi.com@host", // no instance name
		"@@host",          // neither
		"parf@@host",      // an empty label inside the instance name
		"mail-sender/@host",
		"parf@comfi.com@ho@st/x", // a host is still not a path
	} {
		if n, err := ParseName(in); err == nil {
			t.Fatalf("%q was accepted as %q", in, n)
		}
	}
}

// The host keeps the narrow charset: "@" belongs to the instance name only.
func TestHostKeepsTheNarrowCharset(t *testing.T) {
	n, err := ParseName("a@b@c@d")
	if err != nil {
		t.Fatal(err)
	}
	if n.Local != "a@b@c" || n.Realm != "d" {
		t.Fatalf("%+v", n)
	}
}

// The forbidden punctuation, component by component: a name is an identifier,
// and "? = %" in one would have to survive a URL, a log line and a filename.
func TestForbiddenPunctuationInEveryComponent(t *testing.T) {
	for _, in := range []string{
		"pa?rf@host", "pa=rf@host", "pa%rf@host", // instance name
		"parf@ho?st", "parf@ho=st", "parf@ho%st", // host
		"cr?/claude@host", "cr=/claude@host", "cr%/claude@host", // template
		"parf@com?fi.com@host", // inside an address-shaped instance name
	} {
		if n, err := ParseName(in); err == nil {
			t.Fatalf("%q was accepted as %q", in, n)
		}
	}
}

// The charset is a contract, so it is checked as one: every ASCII byte, in
// every component, accepted exactly when the rule says so. Sampling three
// punctuation marks is not the same thing — a parser that let "~" through
// passed the sample above and was caught only by this
// (docs/01-identity.md#names).
func TestTheCharsetIsExhaustive(t *testing.T) {
	alnum := func(c byte) bool { return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' }
	// What each component takes after its first character.
	tail := func(c byte, plus bool) bool {
		return alnum(c) || c == '.' || c == '_' || c == '-' || (plus && c == '+')
	}
	for i := 0; i < 128; i++ {
		c := byte(i)
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f' {
			continue // trimmed, not part of the charset question
		}
		lower := c
		if c >= 'A' && c <= 'Z' {
			lower = c + 32 // canonicalised before the charset is applied
		}
		for _, tc := range []struct {
			what string
			name string
			want bool
		}{
			// "@" and "/" are separators, so they are not charset questions
			// in the positions where they would split the name instead.
			{"instance name", "aa" + string(c) + "aa@host", tail(lower, true) || c == '@'},
			// "@" inside the host is not a charset question: it just moves
			// the last-"@" split, and the name is read differently.
			{"host", "parf@aa" + string(c) + "aa", tail(lower, false) || c == '@'},
			{"template", "aa" + string(c) + "aa/claude@host", tail(lower, false)},
			{"first character of the instance name", string(c) + "aa@host", alnum(lower)},
			{"first character of the host", "parf@" + string(c) + "aa", alnum(lower)},
		} {
			if c == '/' && tc.what != "host" {
				continue // a second "/" is its own rule, tested elsewhere
			}
			_, err := ParseName(tc.name)
			if got := err == nil; got != tc.want {
				t.Fatalf("%s: %q accepted=%v, want %v", tc.what, tc.name, got, tc.want)
			}
		}
	}
}

// Exactly at the bound and exactly over it, on the longest shape we have: a
// template, an embedded "@" and both separators all counted.
func TestBoundIsExactWithTemplateAndEmbeddedAt(t *testing.T) {
	// "cr/" + local + "@" + "h" — local carries the embedded "@".
	build := func(total int) string {
		fixed := len("cr/") + len("@h")
		local := strings.Repeat("a", total-fixed-len("@b")) + "@b"
		return "cr/" + local + "@h"
	}
	at := build(MaxName)
	if len(at) != MaxName {
		t.Fatalf("test is wrong: built %d characters", len(at))
	}
	n, err := ParseName(at)
	if err != nil {
		t.Fatalf("a name of exactly %d characters was refused: %v", MaxName, err)
	}
	if n.String() != at {
		t.Fatalf("round trip at the bound: %q", n.String())
	}
	over := build(MaxName + 1)
	if len(over) != MaxName+1 {
		t.Fatalf("test is wrong: built %d characters", len(over))
	}
	if _, err := ParseName(over); err == nil {
		t.Fatalf("a name of %d characters was accepted", MaxName+1)
	}
}

// Plus-addressing is supported in the instance name, on the owner's word.
// "+" is one character added on purpose; it is not email syntax, and it stays
// out of the template and the host.
func TestPlusAddressingInTheInstanceName(t *testing.T) {
	n, err := ParseName("mail-sender/parf+alerts@comfi.com@host")
	if err != nil {
		t.Fatal(err)
	}
	if n.Template != "mail-sender" || n.Local != "parf+alerts@comfi.com" || n.Realm != "host" {
		t.Fatalf("%+v", n)
	}
	if n.String() != "mail-sender/parf+alerts@comfi.com@host" {
		t.Fatalf("round trip: %q", n.String())
	}
	for _, in := range []string{
		"mail+er/claude@host",  // not in the template
		"parf@comfi.com@ho+st", // not in the host
		"+parf@host",           // still must start alphanumeric
	} {
		if got, err := ParseName(in); err == nil {
			t.Fatalf("%q was accepted as %q", in, got)
		}
	}
}

// Canonicalisation trims each component, not just the whole string.
func TestEachComponentIsTrimmed(t *testing.T) {
	n, err := ParseName("  mail-sender / parf@comfi.com @ host ")
	if err != nil {
		t.Fatal(err)
	}
	if n.String() != "mail-sender/parf@comfi.com@host" {
		t.Fatalf("got %q", n.String())
	}
}

func BenchmarkParseName(b *testing.B) {
	for _, in := range []string{"svc@h", "mail-sender/parf+alerts@comfi.com@srv1"} {
		b.Run(in, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := ParseName(in); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

package protocol

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestRoleSyntaxAndSplitPreserveEachAssignment(t *testing.T) {
	input := "bob(deploy, read_only), @dev(test)\n*(guest)"
	want := []string{"bob(deploy, read_only)", "@dev(test)", "*(guest)"}
	if got := SplitACLText(input); !slices.Equal(got, want) {
		t.Fatalf("split: %q", got)
	}
	a, r, e := ParseACLTerm(want[0])
	if e != nil || a != "bob" || !slices.Equal(r, []string{"deploy", "read_only"}) {
		t.Fatalf("parse: %q %v %v", a, r, e)
	}
	for _, bad := range []string{"bob()", "bob(deploy,)", "bob(Deploy)", "bob(read-only)", "bob(owner)", "*(maintainer)", "bob(deploy))", "bob((deploy))", "bob(deploy)x", "bob)", "bob($(touch x))"} {
		if _, _, e := ParseACLTerm(bad); e == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	long := strings.Repeat("r", 100)
	if !ValidRole(long) || ValidRole("") || ValidRole("R") || ValidRole("a-b") {
		t.Fatal("wrong shared naming rule")
	}
	if _, _, e := ParseACLTerm("bob(" + long + ")"); e != nil {
		t.Fatal(e)
	}
}

func TestPlainACLHasNoDerivedStateOrAllocations(t *testing.T) {
	plain := []string{"bob", "@dev", "*"}
	n := testing.AllocsPerRun(100, func() {
		p, e := CompileACL(plain)
		if e != nil || p != nil {
			panic("plain ACL has derived state")
		}
	})
	if n != 0 {
		t.Fatalf("plain compile allocated %v", n)
	}
	p, e := CompileACL([]string{"bob(deploy)", "*"})
	if e != nil {
		t.Fatal(e)
	}
	r := Record{Allow: []string{"bob(deploy)", "*"}, ParsedAllow: p}
	if !slices.Equal(r.ACLActors(), []string{"bob", "*"}) {
		t.Fatal(r.ACLActors())
	}
	raw, _ := json.Marshal(r)
	clean := r
	clean.ParsedAllow = nil
	wire, _ := json.Marshal(clean)
	if !bytes.Equal(raw, wire) || r.Public().ParsedAllow != nil {
		t.Fatalf("derived state public: %s", raw)
	}
}

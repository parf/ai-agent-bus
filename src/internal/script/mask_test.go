package script

import (
	"reflect"
	"testing"
)

// What a log must hide: each value and each line of a value, from four
// bytes, once, the longest first. The four-byte floor is stated, not an
// accident: "abc" is not hidden, and a short common line of a value is hidden
// along with it (over-masking is the safe side), so the floor is not raised.
func TestMaskValuesAreValuesAndTheirLinesLongestFirst(t *testing.T) {
	got := MaskValues([]string{"A=abcd", "B=abcdEFGHsecret", "KEY=-----BEGIN-----\nMIIEbody\r\nend", "SHORT=abc", "NOEQUALS", "DUP=abcd"})
	want := []string{"-----BEGIN-----\nMIIEbody\r\nend", "-----BEGIN-----", "abcdEFGHsecret", "MIIEbody", "abcd"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

package sandbox

import "testing"

// The filter compiles for this machine, every name in it a real system call,
// into whole sock_filter entries.
func TestTheFilterCompiles(t *testing.T) {
	f, err := Filter()
	if err != nil {
		t.Fatal(err)
	}
	if len(f) == 0 || len(f)%8 != 0 {
		t.Fatalf("the filter is %d bytes, not whole 8-byte entries", len(f))
	}
}

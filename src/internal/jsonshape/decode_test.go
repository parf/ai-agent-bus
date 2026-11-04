package jsonshape

import (
	"strings"
	"testing"
)

func TestTypedMapKeysAreUnique(t *testing.T) {
	type entry struct {
		Enabled bool `json:"enabled"`
	}
	var out map[string]entry
	if err := Decode(strings.NewReader(`{"hello":{"enabled":true},"hello":{"enabled":false}}`), &out); err == nil {
		t.Fatal("duplicate host entry accepted")
	}
	if err := Decode(strings.NewReader(`{"hello":{"enabled":true},"other":{"enabled":false}}`), &out); err != nil {
		t.Fatalf("distinct entries refused: %v", err)
	}
}

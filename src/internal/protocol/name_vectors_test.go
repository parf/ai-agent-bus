package protocol

import (
	"encoding/json"
	"os"
	"testing"
)

func TestSharedAgentNameVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/names.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct {
		Input     string
		Canonical *string
	}
	if err = json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		n, err := ParseName(v.Input)
		if v.Canonical == nil {
			if err == nil {
				t.Errorf("%q accepted as %s", v.Input, n.String())
			}
		} else if err != nil || n.String() != *v.Canonical {
			t.Errorf("%q: %s (%v), want %s", v.Input, n.String(), err, *v.Canonical)
		}
	}
}

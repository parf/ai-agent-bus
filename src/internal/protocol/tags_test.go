package protocol

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

func TestTagsHaveOneCommaGrammar(t *testing.T) {
	data, err := os.ReadFile("testdata/tags.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Input, Expected []string }
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		if got := NormalizeTags(c.Input); !slices.Equal(got, c.Expected) {
			t.Fatalf("%q: got %#v want %#v", c.Input, got, c.Expected)
		}
	}
}

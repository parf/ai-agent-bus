package envfile

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

// The shared data is also consumed by core, runner and other-language readers.
func TestENVConformanceCases(t *testing.T) {
	b, err := os.ReadFile("testdata/common.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name, Source string
		Want         []Var
		Line         int
	}
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			got, err := Parse([]byte(c.Source))
			if c.Line == 0 {
				if err != nil || !reflect.DeepEqual(got, c.Want) {
					t.Fatalf("got %#v, %v; want %#v", got, err, c.Want)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("line %d", c.Line)) {
				t.Fatalf("wanted safe refusal on line %d, got %v", c.Line, err)
			}
			if strings.Contains(err.Error(), "PRIVATE_SENTINEL") {
				t.Fatal("a refused input was echoed")
			}
			if got != nil {
				t.Fatal("a refused file returned partial assignments")
			}
		})
	}
}

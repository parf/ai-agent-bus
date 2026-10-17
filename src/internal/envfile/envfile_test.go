package envfile

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
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

// A secret is parsed under the bus lock on every write and at every start, so
// a large one must cost time in proportion to its size: line-start quotes no
// pair closes once made this quadratic, about a minute for a megabyte. Eight
// times the input may cost about eight times the time, never sixty-four; a
// ratio holds under -race and a loaded machine where a fixed bound would not.
func TestParsingALargeFileStaysLinear(t *testing.T) {
	for name, line := range map[string]string{
		"unclosed line-start quotes": "'x\u2028",
		"many assignments":           "A=b\n",
		"unclosed opening quotes":    "A='x\n",
	} {
		took := func(n int) time.Duration {
			src := []byte(strings.Repeat(line, n))
			if name == "unclosed line-start quotes" {
				src = append(append([]byte("A="), src...), "y\n"...)
			}
			best := time.Duration(1<<63 - 1)
			for range 3 {
				start := time.Now()
				if _, err := Parse(src); err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				best = min(best, time.Since(start))
			}
			return best
		}
		small, large := took(2<<10), took(16<<10)
		if large > 16*small {
			t.Errorf("%s: 8x the input took %.0fx the time (%v, %v)", name, float64(large)/float64(small), small, large)
		}
	}
}

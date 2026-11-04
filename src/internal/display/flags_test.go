package display

import (
	"flag"
	"io"
	"testing"
)

func TestParseFlagsRequiresCanonicalOptionSpelling(t *testing.T) {
	for _, tc := range []struct {
		args  []string
		valid bool
	}{
		{[]string{"--name", "-opaque", "--count=-2", "--ready"}, true},
		{[]string{"--name=value", "--", "-name"}, true},
		{[]string{"--name", "value", "positional", "-name"}, true},
		{[]string{"-name", "value"}, false},
		{[]string{"-ready"}, false},
		{[]string{"-h"}, false},
	} {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		fs.String("name", "", "")
		fs.Int("count", 0, "")
		fs.Bool("ready", false, "")
		if err := ParseFlags(fs, tc.args); (err == nil) != tc.valid {
			t.Errorf("%q: %v", tc.args, err)
		}
	}
}

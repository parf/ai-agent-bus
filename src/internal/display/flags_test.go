package display

import (
	"errors"
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

func TestCanonicalHelpHasNoBuiltinShortAliasOrTrailingArguments(t *testing.T) {
	for _, args := range [][]string{{"--h"}, {"--help", "junk"}, {"--ready", "--help"}} {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		fs.Bool("ready", false, "")
		if err := ParseFlags(fs, args); err == nil || errors.Is(err, flag.ErrHelp) {
			t.Errorf("noncanonical help %q accepted as help: %v", args, err)
		}
	}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := ParseFlags(fs, []string{"--help"}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("canonical help did not reach flag help: %v", err)
	}
}

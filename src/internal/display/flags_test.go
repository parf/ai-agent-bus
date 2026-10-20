package display

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func TestLongOptionHelpKeepsShortOptionsAndLegacyParsing(t *testing.T) {
	for _, spelling := range []string{"-owner=alice", "--owner=alice"} {
		fs := flag.NewFlagSet("fixture", flag.ContinueOnError)
		owner := fs.String("owner", "", "initial owner")
		fs.Bool("h", false, "help")
		var out bytes.Buffer
		fs.SetOutput(&out)
		PrintFlagDefaults(fs)
		if !strings.Contains(out.String(), "  --owner string") || !strings.Contains(out.String(), "  -h\t") || strings.Contains(out.String(), "  -owner ") {
			t.Fatalf("help: %s", &out)
		}
		if err := fs.Parse([]string{spelling}); err != nil || *owner != "alice" {
			t.Fatalf("legacy parsing: %q %v", *owner, err)
		}
	}
}

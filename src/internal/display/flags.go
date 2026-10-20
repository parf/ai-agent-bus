package display

import (
	"bytes"
	"flag"
	"fmt"
	"strings"
)

// PrintFlagDefaults keeps flag's descriptions and defaults, spelling long
// option headings with two dashes. Parsing remains flag's, including legacy input.
func PrintFlagDefaults(fs *flag.FlagSet) {
	out := fs.Output()
	var b bytes.Buffer
	fs.SetOutput(&b)
	fs.PrintDefaults()
	fs.SetOutput(out)
	for _, line := range strings.SplitAfter(b.String(), "\n") {
		if strings.HasPrefix(line, "  -") {
			name := strings.Fields(strings.TrimPrefix(line, "  -"))
			if len(name) > 0 && len(name[0]) > 1 {
				line = "  -" + line[2:]
			}
		}
		fmt.Fprint(out, line)
	}
}

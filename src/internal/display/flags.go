package display

import (
	"bytes"
	"flag"
	"fmt"
	"strings"
)

// PrintFlagDefaults keeps flag's descriptions and defaults, spelling long
// option headings with two dashes.
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

// ParseFlags accepts one spelling for each program option: two leading
// dashes. Values remain opaque, including values that begin with a dash.
func ParseFlags(fs *flag.FlagSet, args []string) error {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" || arg == "-" || !strings.HasPrefix(arg, "-") {
			break
		}
		if !strings.HasPrefix(arg, "--") {
			return fmt.Errorf("invalid option %q: use --name", arg)
		}
		name, _, hasValue := strings.Cut(arg[2:], "=")
		f := fs.Lookup(name)
		if f == nil || hasValue {
			continue
		}
		if boolean, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && boolean.IsBoolFlag() {
			continue
		}
		i++
	}
	return fs.Parse(args)
}

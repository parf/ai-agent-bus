package main

import (
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// call <to> --help and --mcp ask through the envelope's field, with no text;
// text beside either, or both at once, is refused before anything is sent
// (docs/constitution.md#agent-self-description).
func TestHelpAndMCPAreAsksNotText(t *testing.T) {
	for args, want := range map[string]string{
		"#a@h --help": protocol.AskHelp, "--help #a@h": protocol.AskHelp,
		"#a@h --mcp": protocol.AskMCP, "#a@h --wait 5s --mcp": protocol.AskMCP,
		"#a@h hello": "",
	} {
		pos, flags := split(strings.Fields(args))
		got, err := askOf(pos, flags)
		if err != nil || got != want {
			t.Errorf("%q asks %q (%v), want %q", args, got, err, want)
		}
	}
	for _, args := range []string{"#a@h --help hello", "#a@h hello --help", "#a@h --help --mcp", "--help"} {
		pos, flags := split(strings.Fields(args))
		if got, err := askOf(pos, flags); err == nil {
			t.Errorf("%q was taken as ask %q", args, got)
		}
	}
}

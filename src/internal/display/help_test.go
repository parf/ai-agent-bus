package display

import (
	"strings"
	"testing"
)

func TestManRowsHaveAlignedDescriptionsAndPlaceholders(t *testing.T) {
	in := "OPTIONS\n  --topic=topic  Message topic\n  --reply-to=name  Reply inbox\n\nMAIN COMMANDS\n  send <to> <text> — send once\n  ls [<name>] — list records"
	out := ManText(in)
	if !strings.Contains(out, "--topic=<topic>") || !strings.Contains(out, "--reply-to=<name>") {
		t.Fatal(out)
	}
	lines := strings.Split(out, "\n")
	if strings.Index(lines[1], "Message") != strings.Index(lines[2], "Reply") || strings.Index(lines[5], "send once") != strings.Index(lines[6], "list records") {
		t.Fatal(out)
	}
	if strings.Contains(out, " — ") {
		t.Fatal(out)
	}
}

func TestManDescriptionColumnIsPageWideAndBounded(t *testing.T) {
	out := ManText("MAIN\n  short — first\n\nMESSAGES\n  wider <argument> — second\n  enormous <argument-with-a-really-long-name> — third")
	lines := strings.Split(out, "\n")
	if strings.Index(lines[1], "first") != 34 || strings.Index(lines[4], "second") != 34 || lines[6] != strings.Repeat(" ", 34)+"third" {
		t.Fatal(out)
	}
}

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
	if strings.Index(lines[1], "first") != 34 || strings.Index(lines[4], "second") != 34 || lines[5] != "  enormous <argument-with-a-really-long-name> third" {
		t.Fatal(out)
	}
}

func TestOnlyDescriptionParagraphsWrapAtEightyColumns(t *testing.T) {
	sentence := strings.Repeat("readable description words ", 12)
	row := "  publish --channel=<name> <very-long-argument> Publish once without waiting"
	out := ManText("USAGE\n" + row + "\n\nDESCRIPTION\n  " + sentence + "\n\nOPTIONS\n  --long-option=<very-long-value>  " + strings.Repeat("option detail ", 8))
	description := strings.Split(strings.Split(out, "DESCRIPTION\n")[1], "\n\nOPTIONS")[0]
	for _, line := range strings.Split(description, "\n") {
		if len(line) > 80 || !strings.HasPrefix(line, "  ") {
			t.Fatal(out)
		}
	}
	if len(strings.Split(description, "\n")) < 2 || strings.Join(strings.Fields(description), " ") != strings.Join(strings.Fields(sentence), " ") {
		t.Fatal(out)
	}
	if !strings.Contains(out, row) || !strings.Contains(out, strings.TrimSpace(strings.Repeat("option detail ", 8))) {
		t.Fatal("table rows wrapped: " + out)
	}
}

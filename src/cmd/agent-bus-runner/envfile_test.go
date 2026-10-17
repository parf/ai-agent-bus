package main

import (
	"strings"
	"testing"
)

// The env files read as dotenv does, with no $ expanded: comments, export,
// both quotes, and every malformed line refused with its file and line.
func TestEnvFilesReadAsDotenvWithNothingExpanded(t *testing.T) {
	src := "# a comment\n" +
		"\n" +
		"HOST=imap.example.com   # after the value\n" +
		"export PORT=993\n" +
		"PASS=pa$word\n" +
		"QUOTED='p@ss#word $HOME'\n" +
		`ESC="a\"b\\c\nd $X"` + "\n" +
		"EMPTY=\n" +
		"HASH=a#b\r\n"
	got, err := parseEnv("env", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	want := []envVar{
		{"HOST", "imap.example.com"},
		{"PORT", "993"},
		{"PASS", "pa$word"},
		{"QUOTED", "p@ss#word $HOME"},
		{"ESC", "a\"b\\c\nd $X"},
		{"EMPTY", ""},
		{"HASH", "a#b"},
	}
	if len(got) != len(want) {
		t.Fatalf("read %d variables, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("variable %d is %+v, want %+v", i, got[i], want[i])
		}
	}
	for _, bad := range []struct{ line, says string }{
		{"NAME", "env:1: \"NAME\" has no ="},
		{"1BAD=x", "is not a variable name"},
		{"Q='open", "never closed"},
		{`Q="open`, "never closed"},
		{"Q='a' tail", `"tail" after the closing quote`},
	} {
		if _, err := parseEnv("env", []byte(bad.line)); err == nil || !strings.Contains(err.Error(), bad.says) {
			t.Errorf("%q: %v, want it to say %q", bad.line, err, bad.says)
		}
	}
}

// Layers apply lowest first, the later winning; an env.dist variable with no
// value is required, and still missing after every layer is named.
func TestEnvLayersWinInOrderAndNameWhatIsStillRequired(t *testing.T) {
	dist := []envVar{{"HOST", "default"}, {"USER", ""}, {"PASS", ""}}
	secret := []envVar{{"PASS", "from-bus"}}
	agent := []envVar{{"HOST", "agent-wide"}, {"EXTRA", "1"}}
	instance := []envVar{{"HOST", "this-one"}}
	env, missing := layered(dist, secret, agent, instance)
	if strings.Join(env, " ") != "HOST=this-one USER= PASS=from-bus EXTRA=1" {
		t.Fatalf("env is %q", env)
	}
	if strings.Join(missing, ",") != "USER" {
		t.Fatalf("missing is %v, want USER", missing)
	}
	if _, missing := layered(dist, []envVar{{"USER", "u"}, {"PASS", "p"}}); len(missing) != 0 {
		t.Fatalf("a filled-in instance is still missing %v", missing)
	}
}

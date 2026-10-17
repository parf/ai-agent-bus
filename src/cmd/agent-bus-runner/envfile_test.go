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
		{Key: "HOST", Value: "imap.example.com"},
		{Key: "PORT", Value: "993"},
		{Key: "PASS", Value: "pa$word"},
		{Key: "QUOTED", Value: "p@ss#word $HOME"},
		{Key: "ESC", Value: `a\"b\\c` + "\nd $X"},
		{Key: "EMPTY", Value: ""},
		{Key: "HASH", Value: "a"},
	}
	if len(got) != len(want) {
		t.Fatalf("read %d variables, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("variable %d is %+v, want %+v", i, got[i], want[i])
		}
	}
	for _, bad := range []string{"NAME", "!BAD=x", "=secret"} {
		if _, err := parseEnv("env", []byte(bad)); err == nil || !strings.Contains(err.Error(), "env: line 1: expected a dotenv assignment") {
			t.Errorf("missing safe refusal: %v", err)
		}
	}

}

// Layers apply lowest first, the later winning; an env.dist variable with no
// value is required, and still missing after every layer is named.
func TestEnvLayersWinInOrderAndNameWhatIsStillRequired(t *testing.T) {
	dist := []envVar{{Key: "HOST", Value: "default"}, {Key: "USER", Value: ""}, {Key: "PASS", Value: ""}}
	secret := []envVar{{Key: "PASS", Value: "from-bus"}}
	agent := []envVar{{Key: "HOST", Value: "agent-wide"}, {Key: "EXTRA", Value: "1"}}
	instance := []envVar{{Key: "HOST", Value: "this-one"}}
	env, missing := layered(dist, secret, agent, instance)
	if strings.Join(env, " ") != "HOST=this-one USER= PASS=from-bus EXTRA=1" {
		t.Fatalf("env is %q", env)
	}
	if strings.Join(missing, ",") != "USER" {
		t.Fatalf("missing is %v, want USER", missing)
	}
	if _, missing := layered(dist, []envVar{{Key: "USER", Value: "u"}, {Key: "PASS", Value: "p"}}); len(missing) != 0 {
		t.Fatalf("a filled-in instance is still missing %v", missing)
	}
}

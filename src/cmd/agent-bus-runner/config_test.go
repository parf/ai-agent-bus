package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// What is installed loads as the approved formats say: a good agent with its
// instances, and every bad file refused by name rather than skipped.
func TestTheRunnerLoadsWhatIsInstalledAndRefusesTheRestByName(t *testing.T) {
	home := t.TempDir()
	agents, host := filepath.Join(home, "agent.d"), filepath.Join(home, "runner")
	writeFile(t, filepath.Join(agents, "hello", "agent.json"), `{"command":"./hello.sh","form":"args","processes":"0..3","idle":"5m"}`)
	writeFile(t, filepath.Join(agents, "mail", "agent.json"), `{"command":"./fetch.sh","processes":2}`)
	writeFile(t, filepath.Join(agents, "typo", "agent.json"), `{"comand":"./x.sh"}`)
	writeFile(t, filepath.Join(agents, "nocmd", "agent.json"), `{"form":"json"}`)
	writeFile(t, filepath.Join(agents, "badspan", "agent.json"), `{"command":"x","processes":"5..2"}`)
	writeFile(t, filepath.Join(agents, "bare", "README"), "no agent.json here")
	writeFile(t, filepath.Join(host, "agents.json"), `{
		"mail/work": {"enabled": true},
		"mail/home": {"enabled": false},
		"ghost": {"enabled": true}
	}`)

	list, err := load(agents, host)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]installed{}
	for _, in := range list {
		got[in.Key] = in
	}
	want := map[string]string{ // key: the refusal it must carry, "" for none
		"hello":     "",
		"mail/work": "",
		"mail/home": "",
		"typo":      `unknown field "comand"`,
		"nocmd":     "command is required",
		"badspan":   `processes "5..2"`,
		"bare":      "agent.json: missing",
		"ghost":     "no agent ghost is installed",
	}
	if len(got) != len(want) {
		t.Fatalf("loaded %d, want %d: %+v", len(got), len(want), list)
	}
	for key, refusal := range want {
		in, ok := got[key]
		switch {
		case !ok:
			t.Errorf("%s was not loaded", key)
		case refusal == "" && in.Refused != "":
			t.Errorf("%s was refused: %s", key, in.Refused)
		case refusal != "" && !strings.Contains(in.Refused, refusal):
			t.Errorf("%s: refused %q, want it to say %q", key, in.Refused, refusal)
		}
	}
	if s := got["hello"].Span; s.Min != 0 || s.Max != 3 {
		t.Errorf("hello's processes are %+v, want 0..3", s)
	}
	if s := got["mail/work"].Span; s.Min != 2 || s.Max != 2 {
		t.Errorf("mail's processes are %+v, want 2", s)
	}
	if got["mail/home"].enabled() || !got["mail/work"].enabled() || !got["hello"].enabled() {
		t.Error("enabled is not what agents.json says, with on as the default")
	}
}

// A broken agents.json refuses the whole load: every host decision is in it.
func TestABrokenHostFileRefusesTheLoad(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, "agent.d", "hello", "agent.json"), `{"command":"x"}`)
	writeFile(t, filepath.Join(home, "runner", "agents.json"), `{"hello": {"enabld": true}}`)
	if _, err := load(filepath.Join(home, "agent.d"), filepath.Join(home, "runner")); err == nil || !strings.Contains(err.Error(), `unknown field "enabld"`) {
		t.Fatalf("a typo in agents.json was not refused by name: %v", err)
	}
}

// The agents this repository ships load cleanly: a typo in one is caught here,
// not when somebody first deploys it through the runner.
func TestTheShippedAgentsLoad(t *testing.T) {
	source, err := filepath.Abs(filepath.Join("..", "..", "agents"))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	packages := t.TempDir()
	for _, entry := range entries {
		// Standalone agents have their own units; every runner package still
		// goes through load, including packages with a missing agent.json.
		if !entry.IsDir() || entry.Name() == "standalone" {
			continue
		}
		dir := filepath.Join(packages, entry.Name())
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(source, entry.Name(), "agent.json"), filepath.Join(dir, "agent.json")); err != nil {
			t.Fatal(err)
		}
	}
	list, err := load(packages, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		t.Fatal("no shipped agent was found")
	}
	for _, in := range list {
		if in.Refused != "" {
			t.Errorf("shipped agent %s is refused: %s", in.Key, in.Refused)
		}
		// The shipped agents carry their own labels, read from their agent.json.
		if in.Key == "slack" && strings.Join(in.Agent.Tags, ",") != "slack" {
			t.Errorf("the shipped slack agent's tags are %q, not [slack]", in.Agent.Tags)
		}
		if in.Key == "telegram" && strings.Join(in.Agent.Tags, ",") != "telegram" {
			t.Errorf("the shipped telegram agent's tags are %q, not [telegram]", in.Agent.Tags)
		}
	}
}

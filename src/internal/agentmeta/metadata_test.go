package agentmeta

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const fixture = `{"description":"Fixture agent","license":"MIT","methods":{"read":{"help":"Read facts","args":[{"name":"format","help":"Output form","type":["text","json"],"optional":true}],"returns":"facts"},"destroy":{"help":"Remove it","role":"admin","destructive":true}},"config":{"format":{"help":"Output format","type":["text","json"],"example":"other"}}}`

func TestStrictMetadataAndGeneratedHelp(t *testing.T) {
	m, err := Decode([]byte(fixture))
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		strings.Replace(fixture, `"license"`, `"licence"`, 1),
		strings.Replace(fixture, `"optional"`, `"optionl"`, 1),
		strings.Replace(fixture, `["text","json"]`, `"float"`, 1),
		strings.Replace(fixture, `["text","json"]`, `[]`, 1),
		fixture + ` {}`,
	} {
		if _, err := Decode([]byte(bad)); err == nil {
			t.Fatalf("invalid metadata loaded: %s", bad)
		}
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "env.dist"), []byte("# Token description.\n# Second comment.\nTOKEN=\n# Optional endpoint.\nENDPOINT=never-print-this-sentinel\n"), 0600); err != nil {
		t.Fatal(err)
	}
	help, err := m.Answer(dir, "help", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, word := range []string{"Fixture agent", " · MIT", "More: agent-bus help <name> config | secret | json", "read [format]", "text|json"} {
		if !strings.Contains(help, word) {
			t.Fatalf("help missing %q: %s", word, help)
		}
	}
	if strings.Contains(help, "destroy") {
		t.Fatal("unheld method role shown")
	}
	owner, _ := m.Answer(dir, "help", []string{"owner"}, true)
	if !strings.Contains(owner, `"destroy"`) {
		t.Fatal(owner)
	}
	cfg, _ := m.Answer(dir, "help-config", nil, false)
	if !strings.Contains(cfg, `example: "other"`) {
		t.Fatalf("stated enum example hidden: %s", cfg)
	}
	for _, jsonForm := range []bool{false, true} {
		secrets, err := m.Answer(dir, "help-secret", nil, jsonForm)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(secrets, "never-print-this-sentinel") {
			t.Fatalf("help-secret exposed value: %s", secrets)
		}
		for _, word := range []string{"TOKEN", "Token description.", "Second comment.", "ENDPOINT", "required"} {
			if !strings.Contains(secrets, word) {
				t.Fatalf("secret metadata missing %q: %s", word, secrets)
			}
		}
	}
}

func TestConfigWarningsNameUnknownAndEnumKeysWithoutValues(t *testing.T) {
	m, err := Decode([]byte(fixture))
	if err != nil {
		t.Fatal(err)
	}
	warnings := m.ConfigWarnings([]string{`AGENT_BUS_CONFIG={"unknown":"sentinel","format":"outside"}`})
	if len(warnings) != 2 || !strings.Contains(strings.Join(warnings, " "), "enum") || strings.Contains(strings.Join(warnings, " "), "sentinel") {
		t.Fatal(warnings)
	}
	if warnings := m.ConfigWarnings([]string{`AGENT_BUS_CONFIG={"format":"text"}`}); len(warnings) != 0 {
		t.Fatal(warnings)
	}
}

func TestMethodRoleAnyOfAndAutomaticRoles(t *testing.T) {
	m, err := Decode([]byte(`{"description":"Roles","methods":{"restart":{"help":"Restart","role":["operate","admin"]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"operate", "admin", "owner", "maintainer"} {
		body, err := m.Answer(t.TempDir(), "help", []string{role}, false)
		if err != nil || !strings.Contains(body, "restart:") {
			t.Fatalf("role %s missing method: %s %v", role, body, err)
		}
	}
	body, _ := m.Answer(t.TempDir(), "help", []string{"reader"}, false)
	if strings.Contains(body, "restart:") {
		t.Fatal("unheld method shown")
	}
	for _, raw := range []string{`[]`, `null`, `["admin","admin"]`, `["bad-role"]`, `7`} {
		if _, err := Decode([]byte(`{"description":"x","methods":{"m":{"help":"h","role":` + raw + `}}}`)); err == nil {
			t.Fatalf("bad role loaded: %s", raw)
		}
	}
}

func TestMachineHelpIncludesWholeDescriptionWithoutValues(t *testing.T) {
	m, err := Decode([]byte(fixture))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "env.dist"), []byte("# Token purpose\nTOKEN=secret-sentinel-value\n"), 0600)
	for _, format := range []bool{false, true} {
		body, err := m.Answer(dir, "help-json", nil, format)
		if err != nil {
			t.Fatal(err)
		}
		var data map[string]json.RawMessage
		if err := json.Unmarshal([]byte(body), &data); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"description", "license", "version", "methods", "config", "secrets", "asks"} {
			if _, ok := data[key]; !ok {
				t.Fatalf("missing %s: %s", key, body)
			}
		}
		if strings.Contains(body, "secret-sentinel-value") || strings.Contains(body, "destroy") {
			t.Fatal(body)
		}
		if !strings.Contains(body, `"required":false`) || !strings.Contains(body, "Token purpose") || !strings.Contains(body, "help-json") {
			t.Fatal(body)
		}
	}
}

func TestReadableHelpPreservesOrderAndSeparatesTextFromJSON(t *testing.T) {
	raw := `{"description":"Route conversations","license":"PolyForm-Noncommercial-1.0.0","author":"Full author","homepage":"https://example.test/repo","arguments":{"channel":{"help":"Channel name","type":"string"}},"methods":{"post":{"help":"Post a message","args":[{"name":"channel"},{"name":"thread","help":"Thread ID","json_only":true},{"name":"text","help":"Message","rest":true}],"example":"agent-bus call '#chat@t' 'post #general Hello'"},"add":{"text":"sub add <channel> <target>","help":"Forward messages","args":[{"name":"channel"},{"name":"target","help":"Destination"}],"destructive":true},"help":{"help":"Help"}}}`
	m, err := Decode([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	body, err := m.Answer(t.TempDir(), "help", nil, false, "#chat/work@t")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"post <channel> <text...>:", "[destructive] sub add <channel> <target>:", "Arguments\nchannel (string): Channel name", "Examples\nagent-bus call", "PolyForm-NC · example.test/repo", "More: agent-bus help '#chat/work@t' config | secret | json"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q: %s", want, body)
		}
	}
	for _, absent := range []string{"<thread>", "help:", "Author:", "License:", "Asks:", "sub.add"} {
		if strings.Contains(body, absent) {
			t.Fatalf("text contains %q: %s", absent, body)
		}
	}
	if strings.Count(body, "Channel name") != 1 || strings.Index(body, "post <") > strings.Index(body, "sub add <") {
		t.Fatal("shared argument duplicated or author order lost: " + body)
	}

	// The same fixture must render identically in the native Python fallback.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "agent.json"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	helper, err := filepath.Abs("../../agents/shared")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", "-c", "import sys; sys.path.insert(0,sys.argv[1]); from metadata import answer; print(answer(sys.argv[2], roles=[]))", helper, dir)
	cmd.Env = append(os.Environ(), "AGENT_BUS_NAME=#chat/work@t", "PYTHONDONTWRITEBYTECODE=1")
	native, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Python help: %v: %s", err, native)
	}
	if strings.TrimSpace(string(native)) != body {
		t.Fatalf("Python/Go help differs:\nPython:\n%s\nGo:\n%s", native, body)
	}
	machine, _ := m.Answer(t.TempDir(), "help-json", nil, false)
	for _, want := range []string{`"thread"`, `"json_only":true`, `"author":"Full author"`, `"license":"PolyForm-Noncommercial-1.0.0"`, `"help":"Channel name"`} {
		if !strings.Contains(machine, want) {
			t.Fatalf("machine help lost %q: %s", want, machine)
		}
	}
	for _, bad := range []string{strings.Replace(raw, `"name":"channel"`, `"name":"missing"`, 1), strings.Replace(raw, `"json_only":true`, `"json_only":"yes"`, 1), strings.Replace(raw, `"text":"sub add <channel> <target>"`, `"text":3`, 1)} {
		if _, err := Decode([]byte(bad)); err == nil {
			t.Fatal("invalid metadata loaded: " + bad)
		}
	}
}

func TestNativeHelpDeclaration(t *testing.T) {
	for _, value := range []string{`"native"`, `""`} {
		m, err := Decode([]byte(`{"description":"fixture","help":` + value + `}`))
		if err != nil {
			t.Fatal(err)
		}
		if value == `"native"` && m.Help != "native" {
			t.Fatal("native help switch lost")
		}
	}
	for _, value := range []string{`"generic"`, `true`, `[]`} {
		if _, err := Decode([]byte(`{"description":"fixture","help":` + value + `}`)); err == nil {
			t.Fatalf("invalid help accepted: %s", value)
		}
	}
}

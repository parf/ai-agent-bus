package agentmeta

import (
	"encoding/json"
	"os"
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
	for _, word := range []string{"Fixture agent", "License: MIT", "--help-config", "--help-secret", "read [format]", "text|json"} {
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

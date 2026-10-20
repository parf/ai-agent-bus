package main

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/agentmeta"
)

// Compare actual parser/handler checks with both package-help forms. This pins
// metadata to executable grammar, rather than keeping two expected method lists.
func TestBundledMetadataHelpMatchesExecutableMethods(t *testing.T) {
	for _, name := range []string{"slack", "telegram", "systemd", "git", "server-info", "logwatch", "debug"} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join("..", "..", "agents", name)
			if name != "slack" && name != "telegram" && name != "debug" {
				dir = filepath.Join("..", "..", "agents", "standalone", name)
			}
			var command *exec.Cmd
			if name == "slack" || name == "telegram" {
				script := "import {KEYS} from " + "'" + filepath.ToSlash(filepath.Join(dir, "parse.ts")) + "'; console.log(JSON.stringify(Object.keys(KEYS).sort()))"
				command = exec.Command("bun", "-e", script)
			} else {
				command = exec.Command("python3", "../../agents/shared/metadata_contract.py", dir, name)
			}
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("%s grammar check: %v\n%s", name, err, output)
			}
			var expected []string
			if err := json.Unmarshal(output, &expected); err != nil {
				t.Fatalf("grammar output: %v\n%s", err, output)
			}
			sort.Strings(expected)
			m, err := agentmeta.Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			data, err := m.Answer(dir, "help", []string{"owner"}, true)
			if err != nil {
				t.Fatal(err)
			}
			var help struct {
				Methods map[string]json.RawMessage `json:"methods"`
			}
			if err := json.Unmarshal([]byte(data), &help); err != nil {
				t.Fatal(err)
			}
			actual := make([]string, 0, len(help.Methods))
			for method := range help.Methods {
				actual = append(actual, method)
			}
			sort.Strings(actual)
			if !reflect.DeepEqual(actual, expected) {
				t.Fatalf("help methods %v; executable methods %v", actual, expected)
			}
			text, err := m.Answer(dir, "help", []string{"owner"}, false)
			if err != nil {
				t.Fatal(err)
			}
			for _, method := range expected {
				found := false
				for _, line := range strings.Split(text, "\n") {
					if strings.HasPrefix(line, method+":") || strings.HasPrefix(line, method+" ") {
						found = true
					}
				}
				if !found {
					t.Errorf("text help missing executable method %q", method)
				}
			}
		})
	}
}

// Doctor behavior is part of the normal Go shard, including its negative probes.
func TestDebugDoctor(t *testing.T) {
	command := exec.Command("python3", "-m", "unittest", "discover", "-s", "../../agents/debug", "-p", "test_doctor.py")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("debug doctor: %v\n%s", err, output)
	}
}

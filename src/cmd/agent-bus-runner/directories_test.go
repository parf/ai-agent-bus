package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func directoryAgent(t *testing.T, root, name, label string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	writeFile(t, filepath.Join(dir, "agent.json"), `{"description":"Directory fixture","command":"./answer.sh","form":"args","deadline":"2s"}`)
	writeFile(t, filepath.Join(dir, "env.dist"), "FLAVOUR="+label+"\n")
	writeFile(t, filepath.Join(dir, "answer.sh"), "#!/bin/sh\nprintf '%s' \"$FLAVOUR\"\n")
	if err := os.Chmod(filepath.Join(dir, "answer.sh"), 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func directoryRow(t *testing.T, r *runner, key string) agentStatus {
	t.Helper()
	for _, row := range r.status().Agents {
		if row.Key == key {
			return row
		}
	}
	t.Fatalf("no %s in status: %+v", key, r.status())
	return agentStatus{}
}

func TestBundledAgentIsAvailableUntilExplicitlyEnabled(t *testing.T) {
	r, b, script := configurationFixture(t, "", "printf old")
	if err := os.RemoveAll(filepath.Dir(script)); err != nil {
		t.Fatal(err)
	}
	r.bundled = t.TempDir()
	dir := directoryAgent(t, r.bundled, "fixture", "bundled")
	r.reload()
	for _, inventory := range []string{`{"fixture":{"sandbox":"off"}}`, `{"fixture":{"sandbox":"off","name":"#fixture@h"}}`} {
		writeFile(t, filepath.Join(r.host, "agents.json"), inventory)
		r.reload()
		row := directoryRow(t, r, "fixture")
		if row.Enabled || row.State != "available" || row.AgentDir != dir {
			t.Fatalf("bundled opt-in/source: %+v", row)
		}
		r.mu.Lock()
		served := len(r.served)
		r.mu.Unlock()
		if served != 0 {
			t.Fatalf("unenabled bundled agent served: %d", served)
		}
	}
	if err := r.control("enable", "fixture"); err != nil {
		t.Fatal(err)
	}
	row := directoryRow(t, r, "fixture")
	if !row.Enabled || row.State != "idle" {
		t.Fatalf("enable: %+v", row)
	}
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: "hello"}); err != nil {
		t.Fatal(err)
	}
	if got := configurationReply(t, b); got.Body != "bundled" {
		t.Fatalf("bundled command/defaults answered %q", got.Body)
	}
	if err := r.control("disable", "fixture"); err != nil {
		t.Fatal(err)
	}
	if row := directoryRow(t, r, "fixture"); row.Enabled || row.State != "disabled" {
		t.Fatalf("explicit disable: %+v", row)
	}
}

func TestSharedAgentWinsAndRemovalFallsBackToBundle(t *testing.T) {
	r, b, _ := configurationFixture(t, "", "printf old")
	r.bundled = t.TempDir()
	shared := directoryAgent(t, r.agents, "fixture", "shared")
	bundled := directoryAgent(t, r.bundled, "fixture", "bundled")
	writeFile(t, filepath.Join(r.host, "agents.json"), `{"fixture":{"sandbox":"off","enabled":true}}`)
	check := func(want, dir string) {
		t.Helper()
		r.reload()
		if row := directoryRow(t, r, "fixture"); row.AgentDir != dir || !row.Enabled {
			t.Fatalf("selected source: %+v, want %s", row, dir)
		}
		r.mu.Lock()
		selected := r.served["#fixture@h"].dir
		r.mu.Unlock()
		if selected != dir {
			t.Fatalf("serving snapshot still binds %s, want %s", selected, dir)
		}
		if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: "hello"}); err != nil {
			t.Fatal(err)
		}
		if got := configurationReply(t, b); got.Body != want {
			t.Fatalf("selected command/defaults: %q, want %q", got.Body, want)
		}
		file, err := r.readFile("agent.d/fixture/env.dist")
		if err != nil || file.Content == nil || *file.Content != "FLAVOUR="+want+"\n" {
			t.Fatalf("read selected author: %+v %v", file, err)
		}
	}
	check("shared", shared)
	if err := os.RemoveAll(shared); err != nil {
		t.Fatal(err)
	}
	check("bundled", bundled)
	// A newly installed override is enabled by default even without an inventory row.
	directoryAgent(t, r.agents, "fixture", "replacement")
	writeFile(t, filepath.Join(r.host, "agents.json"), `{"fixture":{"sandbox":"off"}}`)
	check("replacement", shared)
}

func TestReloadFindsNewBundledAndStandaloneAgents(t *testing.T) {
	r, _, _ := configurationFixture(t, "", "printf old")
	r.bundled = t.TempDir()
	directoryAgent(t, r.bundled, "first", "first")
	writeFile(t, filepath.Join(r.bundled, "shared", "support.py"), "# support\n")
	r.reload()
	if row := directoryRow(t, r, "first"); row.State != "available" {
		t.Fatalf("first bundle: %+v", row)
	}
	directoryAgent(t, r.bundled, "upgrade", "upgrade")
	standalone := directoryAgent(t, filepath.Join(r.bundled, "standalone"), "git", "standalone")
	r.reload()
	if row := directoryRow(t, r, "upgrade"); row.State != "available" {
		t.Fatalf("upgrade bundle: %+v", row)
	}
	if row := directoryRow(t, r, "git"); row.State != "available" || row.AgentDir != standalone {
		t.Fatalf("standalone bundle: %+v", row)
	}
	if len(r.status().Agents) != 4 {
		t.Fatalf("support directories appeared as agents: %+v", r.status().Agents)
	}
}

func TestBundledInstancesAndOverridesKeepTheirHostPaths(t *testing.T) {
	root, host := t.TempDir(), t.TempDir()
	directoryAgent(t, root, "fixture", "bundled")
	writeFile(t, filepath.Join(root, "fixture", "settings.json"), `{"a":1}`)
	writeFile(t, filepath.Join(host, "agents.json"), `{"fixture/work":{"enabled":true},"fixture/off":{}}`)
	writeFile(t, filepath.Join(host, "fixture", "agent.json"), `{"deadline":"3s"}`)
	writeFile(t, filepath.Join(host, "fixture", "work", "settings.json"), `{"a":2}`)
	list, err := loadSources([]agentSource{{path: root, bundled: true}}, host)
	if err != nil || len(list) != 2 {
		t.Fatalf("instances: %+v %v", list, err)
	}
	for _, in := range list {
		if in.Refused != "" || in.Agent.Deadline != "3s" || in.Source != root {
			t.Fatalf("instance source/override: %+v", in)
		}
		if in.enabled() != (in.Key == "fixture/work") {
			t.Fatalf("instance opt-in: %+v", in)
		}
		if in.Key == "fixture/work" {
			if len(in.Over) != 1 {
				t.Fatalf("missing shipped override: %+v", in)
			}
			data, err := os.ReadFile(in.Over[0].Src)
			var got map[string]int
			if err != nil || json.Unmarshal(data, &got) != nil || got["a"] != 2 {
				t.Fatalf("merged override: %s %v", data, err)
			}
		}
	}
	// Exercise the same host-file validation used by the bus write API.
	r := &runner{agents: filepath.Join(root, "absent"), bundled: root, host: host}
	if err := r.checkSettingsOverride("runner/fixture/settings.json", []byte(`{"a":3}`)); err != nil {
		t.Fatal(err)
	}
}

func TestBundledDirectoryDefaultsDistinguishGitFromReleaseLinks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "checkout", "src", "go.mod"), "module fixture\n")
	writeFile(t, filepath.Join(root, "checkout", ".git"), "gitdir: fixture\n")
	git := filepath.Join(root, "checkout", "src", "agent-bus-runner")
	release := filepath.Join(root, "release", "agent-bus-runner")
	writeFile(t, git, "")
	writeFile(t, release, "")
	for _, tc := range []struct{ name, target, want string }{{"git", git, filepath.Join(root, "checkout", "src", "agents")}, {"release", release, releaseBundledDirectory}} {
		link := filepath.Join(root, tc.name+"-link")
		if err := os.Symlink(tc.target, link); err != nil {
			t.Fatal(err)
		}
		if got := bundledDirectory(link); got != tc.want {
			t.Fatalf("%s default: %s, want %s", tc.name, got, tc.want)
		}
	}
}

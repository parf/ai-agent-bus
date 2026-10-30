package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func installDepends(t *testing.T, agents, name, depends string) {
	t.Helper()
	d := ""
	if depends != "" {
		d = `,"depends":` + depends
	}
	writeFile(t, filepath.Join(agents, name, "agent.json"), `{"description":"Test agent","command":"./x.sh"`+d+`}`)
}

func keysOf(list []installed) string {
	var keys []string
	for _, in := range list {
		keys = append(keys, in.Key)
	}
	return strings.Join(keys, " ")
}

func refusals(list []installed) map[string]string {
	out := map[string]string{}
	for _, in := range list {
		out[in.Key] = in.Refused
	}
	return out
}

// An agent comes after the ones it names; the rest keep their order, and a
// name with a realm or an agent with instances is understood.
func TestAnAgentComesAfterTheOnesItDependsOn(t *testing.T) {
	home := t.TempDir()
	agents, host := filepath.Join(home, "agent.d"), filepath.Join(home, "runner")
	installDepends(t, agents, "a", `["#z"]`)
	installDepends(t, agents, "b", "")
	installDepends(t, agents, "m", `["#a"]`)
	installDepends(t, agents, "z", "")
	installDepends(t, agents, "mail", "")
	installDepends(t, agents, "w", `["#mail","#z@somehost"]`)
	writeFile(t, filepath.Join(host, "agents.json"), `{"mail/work":{},"mail/home":{}}`)
	list, err := load(agents, host)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := keysOf(list), "z a b m mail/home mail/work w"; got != want {
		t.Fatalf("start order %q, want %q", got, want)
	}
	for key, refusal := range refusals(list) {
		if refusal != "" {
			t.Errorf("%s was refused: %s", key, refusal)
		}
	}
}

// A name that is nothing installed here refuses its agent, naming it, and a
// host that runs the dependency elsewhere removes the entry in its override.
func TestAnUnknownDependencyIsRefusedByNameUnlessTheHostRemovesIt(t *testing.T) {
	home := t.TempDir()
	agents, host := filepath.Join(home, "agent.d"), filepath.Join(home, "runner")
	installDepends(t, agents, "needy", `["#ghost"]`)
	installDepends(t, agents, "remote", `["#elsewhere"]`)
	installDepends(t, agents, "fine", "")
	writeFile(t, filepath.Join(host, "remote", "agent.json"), `{"depends":[]}`)
	list, err := load(agents, host)
	if err != nil {
		t.Fatal(err)
	}
	r := refusals(list)
	if !strings.Contains(r["needy"], "#ghost") || !strings.Contains(r["needy"], "not installed here") {
		t.Errorf("needy: %q, want a refusal naming #ghost", r["needy"])
	}
	if r["remote"] != "" || r["fine"] != "" {
		t.Errorf("remote %q and fine %q must load", r["remote"], r["fine"])
	}
}

// A cycle refuses every agent on it with the cycle named, an agent that only
// leans on it is not part of it, and an agent depending on itself is a cycle.
func TestACycleIsRefusedAndNamed(t *testing.T) {
	home := t.TempDir()
	agents, host := filepath.Join(home, "agent.d"), filepath.Join(home, "runner")
	installDepends(t, agents, "a", `["#b"]`)
	installDepends(t, agents, "b", `["#c"]`)
	installDepends(t, agents, "c", `["#a"]`)
	installDepends(t, agents, "lean", `["#a"]`)
	installDepends(t, agents, "self", `["#self"]`)
	installDepends(t, agents, "ok", "")
	list, err := load(agents, host)
	if err != nil {
		t.Fatal(err)
	}
	r := refusals(list)
	for _, key := range []string{"a", "b", "c"} {
		if !strings.Contains(r[key], "cycle: #a -> #b -> #c -> #a") {
			t.Errorf("%s: %q, want the cycle named", key, r[key])
		}
	}
	if !strings.Contains(r["self"], "cycle: #self -> #self") {
		t.Errorf("self: %q", r["self"])
	}
	if r["lean"] != "" || r["ok"] != "" {
		t.Errorf("lean %q and ok %q are not on a cycle", r["lean"], r["ok"])
	}
	if len(list) != 6 {
		t.Errorf("a cycle must not drop agents: %s", keysOf(list))
	}
}

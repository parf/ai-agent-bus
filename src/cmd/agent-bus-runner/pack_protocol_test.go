package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// The pack declares the daemon protocols it speaks in protocol.json and the runner refuses, by name, a pack agent the daemon
// cannot serve. A user's own agent is never checked.

func identityIs(protocol, min string) daemonProtocol {
	return func() ([]byte, error) {
		return []byte(`{"protocol":` + protocol + `,"protocol_min":` + min + `,"version":"x"}`), nil
	}
}

func packFixture(t *testing.T, declaration string) *runner {
	t.Helper()
	r, _, _ := configurationFixture(t, "", "printf old")
	r.bundled = t.TempDir()
	directoryAgent(t, r.bundled, "packed", "bundled")
	if declaration != "" {
		writeFile(t, filepath.Join(r.bundled, packProtocolName), declaration)
	}
	return r
}

func refusal(t *testing.T, r *runner, key string) string {
	t.Helper()
	r.reload()
	row := directoryRow(t, r, key)
	if row.State == "refused" && strings.HasPrefix(row.Reason, "pack protocol unsupported") {
		return row.Reason
	}
	return ""
}

// protocolRefused says whether a row is refused for the pack's protocol, not for any other reason the host may have
// (a fixture host that cannot give a cgroup refuses an agent whose sandbox is on).
func protocolRefused(row agentStatus) bool {
	return row.State == "refused" && strings.HasPrefix(row.Reason, "pack protocol unsupported")
}

func TestAPackAgentThatTheDaemonCanServeLoads(t *testing.T) {
	r := packFixture(t, `{"protocol":1,"protocol_min":1}`)
	if why := refusal(t, r, "packed"); why != "" {
		t.Fatalf("the real daemon says 1..1, the pack declares 1..1: refused %q", why)
	}
	r.identity = identityIs("1", "1")
	if why := refusal(t, r, "packed"); why != "" {
		t.Fatalf("refused %q", why)
	}
}

func TestAPackAgentThatTheDaemonCannotServeIsRefusedByName(t *testing.T) {
	for _, c := range []struct {
		name, pack, protocol, min string
		want                      string // "" loads
	}{
		{"daemon newer", `{"protocol":1,"protocol_min":1}`, "2", "2", "pack 1..1, daemon 2..2"},
		{"daemon far newer", `{"protocol":1,"protocol_min":1}`, "3", "3", "pack 1..1, daemon 3..3"},
		{"daemon older", `{"protocol":2,"protocol_min":2}`, "1", "1", "pack 2..2, daemon 1..1"},
		{"daemon zero", `{"protocol":1,"protocol_min":1}`, "0", "0", "pack 1..1, daemon 0..0"},
		// Two protocols: the daemon speaks 1 and 2, the pack only 1. The clients take no daemon that may speak a protocol they lack.
		{"daemon speaks two, pack one", `{"protocol":1,"protocol_min":1}`, "2", "1", "pack 1..1, daemon 1..2"},
		{"pack speaks two, daemon one", `{"protocol":2,"protocol_min":1}`, "1", "1", ""},
		{"pack speaks two, daemon the other", `{"protocol":2,"protocol_min":1}`, "2", "2", ""},
		{"pack speaks two, daemon both", `{"protocol":2,"protocol_min":1}`, "2", "1", ""},
		{"pack speaks two, daemon beyond", `{"protocol":2,"protocol_min":1}`, "3", "3", "pack 1..2, daemon 3..3"},
		{"pack speaks two, daemon reaches beyond", `{"protocol":2,"protocol_min":1}`, "3", "2", "pack 1..2, daemon 2..3"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := packFixture(t, c.pack)
			r.identity = identityIs(c.protocol, c.min)
			why := refusal(t, r, "packed")
			if c.want == "" {
				if why != "" {
					t.Fatalf("refused %q", why)
				}
				return
			}
			if !strings.HasPrefix(why, "pack protocol unsupported: ") || !strings.Contains(why, c.want) {
				t.Fatalf("reason %q, want it to name %q", why, c.want)
			}
		})
	}
}

func TestAMissingOrMalformedDeclarationRefusesThePackAgent(t *testing.T) {
	for _, c := range []struct{ name, body, want string }{
		{"missing", "", "protocol.json: missing"},
		{"not json", `1..1`, "protocol.json"},
		{"empty object", `{}`, "both required"},
		{"one field", `{"protocol":1}`, "both required"},
		{"extra field", `{"protocol":1,"protocol_min":1,"version":"1"}`, "protocol.json"},
		{"string", `{"protocol":"1","protocol_min":1}`, "protocol.json"},
		{"negative", `{"protocol":1,"protocol_min":-1}`, "nonnegative"},
		{"null", `{"protocol":null,"protocol_min":1}`, "both required"},
		{"min over protocol", `{"protocol":1,"protocol_min":2}`, "protocol_min exceeds protocol"},
		{"duplicate key", `{"protocol":1,"protocol":1,"protocol_min":1}`, "protocol.json"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := packFixture(t, c.body)
			r.identity = identityIs("1", "1")
			why := refusal(t, r, "packed")
			if !strings.HasPrefix(why, "pack protocol unsupported: ") || !strings.Contains(why, c.want) {
				t.Fatalf("reason %q, want it to name %q", why, c.want)
			}
		})
	}
}

func TestAnUnusableIdentityRefusesAndAnUnreachableOneChecksNothing(t *testing.T) {
	r := packFixture(t, `{"protocol":1,"protocol_min":1}`)
	for _, body := range []string{`"nope"`, `{"protocol":1}`, `{"protocol":1,"protocol_min":2}`, `{"protocol":-1,"protocol_min":-1}`, `{"protocol":null,"protocol_min":1}`} {
		r.identity = func() ([]byte, error) { return []byte(body), nil }
		if why := refusal(t, r, "packed"); !strings.HasPrefix(why, "pack protocol unsupported: ") {
			t.Fatalf("identity %s: reason %q", body, why)
		}
	}
	r.identity = func() ([]byte, error) { return nil, errors.New("daemon unreachable") }
	if why := refusal(t, r, "packed"); why != "" {
		t.Fatalf("an unreachable daemon checks nothing here (the agent's own client still refuses at connect), got %q", why)
	}
}

func TestOnlyPackAgentsAreCheckedAUsersOwnAgentIsUntouched(t *testing.T) {
	r := packFixture(t, `{"protocol":1,"protocol_min":1}`)
	r.identity = identityIs("9", "9")
	// The user's own agent in agent.d: no declaration, and one beside it would still not be read.
	mine := directoryAgent(t, r.agents, "mine", "own")
	writeFile(t, filepath.Join(mine, "protocol.json"), `not even json`)
	// A package deployed from the pack carries support/agents/VERSION and its own declaration, and is checked.
	deployed := directoryAgent(t, r.agents, "deployed", "pack")
	writeFile(t, filepath.Join(deployed, "support", "agents", "VERSION"), "1.0.0\n")
	writeFile(t, filepath.Join(deployed, "support", "agents", packProtocolName), `{"protocol":1,"protocol_min":1}`)
	r.reload()
	if row := directoryRow(t, r, "mine"); strings.Contains(row.Reason, "protocol") {
		t.Fatalf("a user's own agent was checked: %+v", row)
	}
	for _, key := range []string{"packed", "deployed"} {
		if row := directoryRow(t, r, key); !protocolRefused(row) || !strings.Contains(row.Reason, "daemon 9..9") {
			t.Fatalf("%s: %+v", key, row)
		}
	}
	// A pack package without its declaration is refused named; the same package marked nothing is the user's own.
	writeFile(t, filepath.Join(deployed, "support", "agents", packProtocolName), `{`)
	r.reload()
	if row := directoryRow(t, r, "deployed"); !protocolRefused(row) || !strings.Contains(row.Reason, "protocol.json") {
		t.Fatalf("a malformed package declaration: %+v", row)
	}
}

func TestADaemonProtocolChangeBetweenTwoReloadsIsSeenByTheSecond(t *testing.T) {
	r := packFixture(t, `{"protocol":1,"protocol_min":1}`)
	current := identityIs("1", "1")
	r.identity = func() ([]byte, error) { return current() }
	if why := refusal(t, r, "packed"); why != "" {
		t.Fatalf("first load refused %q", why)
	}
	current = identityIs("2", "2")
	if why := refusal(t, r, "packed"); !strings.Contains(why, "daemon 2..2") {
		t.Fatalf("after the daemon moved to 2, reload must refuse; got %q", why)
	}
	current = identityIs("1", "1")
	if why := refusal(t, r, "packed"); why != "" {
		t.Fatalf("after it came back, reload must load again; got %q", why)
	}
}

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// The agent pack declares the daemon protocols its agents speak in protocol.json,
// the two fields GET /identity answers with (docs/09-daemon-api.md § Protocol
// version). The runner checks it when it loads, so an agent of the pack the daemon
// cannot serve is refused by name before a child is started; the agent's own client
// still checks at connect. Only pack agents are checked: a user's own agent has no
// declaration and the runner leaves it alone.

// packProtocolName is the pack's declaration, beside VERSION in the pack and in each package's support/agents.
const packProtocolName = "protocol.json"

type packProtocol struct {
	Protocol    *int `json:"protocol"`
	ProtocolMin *int `json:"protocol_min"`
}

// readPackProtocol reads one declaration, strictly: both fields, nonnegative whole numbers, no others.
func readPackProtocol(path string) (protocol, protocolMin int, err error) {
	var d packProtocol
	if err := strict(path, &d); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, 0, errors.New("protocol.json: missing")
		}
		return 0, 0, err
	}
	return checkDeclared(d)
}

func checkDeclared(d packProtocol) (int, int, error) {
	if d.Protocol == nil || d.ProtocolMin == nil {
		return 0, 0, errors.New("protocol.json: protocol and protocol_min are both required")
	}
	if *d.Protocol < 0 || *d.ProtocolMin < 0 {
		return 0, 0, errors.New("protocol.json: protocol and protocol_min are nonnegative integers")
	}
	if *d.ProtocolMin > *d.Protocol {
		return 0, 0, errors.New("protocol.json: protocol_min exceeds protocol")
	}
	return *d.Protocol, *d.ProtocolMin, nil
}

// packProtocolFile is where an installed agent's declaration is, and whether it belongs to the pack at all: a bundled agent
// reads the pack's own file, one deployed from a pack package carries support/agents/protocol.json beside the
// support/agents/VERSION that marks the package as the pack's. Anything else is the user's own agent.
func packProtocolFile(in installed, bundledRoot string) (string, bool) {
	if in.Bundled {
		return filepath.Join(bundledRoot, packProtocolName), true
	}
	if in.Path == "" {
		return "", false
	}
	support := filepath.Join(in.Path, "support", "agents")
	if _, err := os.Stat(filepath.Join(support, "VERSION")); err == nil {
		return filepath.Join(support, packProtocolName), true
	}
	return "", false
}

// daemonProtocol is the daemon's identity as the runner reads it; swapped in tests.
type daemonProtocol func() ([]byte, error)

func (r *runner) fetchIdentity() ([]byte, error) {
	if r.bus == nil {
		return nil, errors.New("no bus")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, code, err := r.bus.do(ctx, "GET", "/identity", nil, nil)
	if err != nil {
		return nil, err
	}
	if code >= 400 && code < 500 {
		return nil, &identityRefused{code}
	}
	if code >= 400 {
		return nil, fmt.Errorf("identity answered %d", code)
	}
	return out, nil
}

// identityRefused is the daemon answering the identity request with a refusal (a 4xx): it is up and said no, so an agent of the pack
// cannot be shown to work with it. A 5xx or a transport error is the daemon not answering, which checks nothing.
type identityRefused struct{ code int }

func (e *identityRefused) Error() string { return fmt.Sprintf("the daemon refused its identity request (%d)", e.code) }

// daemonRange reads protocol and protocol_min from an identity answer.
func daemonRange(body []byte) (protocol, protocolMin int, err error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil || fields == nil {
		return 0, 0, errors.New("the daemon's identity is not an object")
	}
	values := [2]int{}
	for i, name := range []string{"protocol", "protocol_min"} {
		raw, found := fields[name]
		if !found || json.Unmarshal(raw, &values[i]) != nil || string(raw) == "null" || values[i] < 0 {
			return 0, 0, fmt.Errorf("the daemon's identity has no usable %s", name)
		}
	}
	if values[1] > values[0] {
		return 0, 0, errors.New("the daemon's protocol_min exceeds its protocol")
	}
	return values[0], values[1], nil
}

// packProtocolRefusal is "" when the agent may load. Every protocol the daemon may speak (its protocol_min to its protocol)
// must be one the pack declares, the same strictness as a client library, which takes nothing else. An identity that
// could not be fetched checks nothing: the agent's own client still refuses at connect.
func packProtocolRefusal(in installed, bundledRoot string, identity []byte, identityErr error) string {
	file, pack := packProtocolFile(in, bundledRoot)
	if !pack {
		return ""
	}
	protocol, protocolMin, err := readPackProtocol(file)
	if err != nil {
		return "pack protocol unsupported: " + err.Error()
	}
	var refused *identityRefused
	if errors.As(identityErr, &refused) {
		return "pack protocol unsupported: " + refused.Error()
	}
	if identityErr != nil {
		return ""
	}
	daemon, daemonMin, err := daemonRange(identity)
	if err != nil {
		return "pack protocol unsupported: " + err.Error()
	}
	if daemonMin < protocolMin || daemon > protocol {
		return fmt.Sprintf("pack protocol unsupported: pack %d..%d, daemon %d..%d", protocolMin, protocol, daemonMin, daemon)
	}
	return ""
}

// checkPackProtocols applies the refusal to a fresh load. The daemon is asked once per load, so a protocol change between two
// reloads is seen by the second.
func (r *runner) checkPackProtocols(list []installed) {
	fetch := r.identity
	if fetch == nil {
		fetch = r.fetchIdentity
	}
	var identity []byte
	var identityErr error
	asked := false
	for i := range list {
		if list[i].Refused != "" {
			continue
		}
		if _, pack := packProtocolFile(list[i], r.bundled); !pack {
			continue
		}
		if !asked {
			identity, identityErr = fetch()
			asked = true
		}
		if why := packProtocolRefusal(list[i], r.bundled, identity, identityErr); why != "" {
			list[i].Refused = why
		}
	}
}

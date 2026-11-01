// `agent-bus enrol` is how a newcomer becomes a name in a realm somebody
// vouches for: the bus says what to sign, the key on this machine signs it,
// and the bus checks the answer against what the directory publishes.
// Fetching a public key proves nothing on its own — this is the step that
// does. See docs/01-identity-and-authority.md#registration.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/parf/ai-agent-bus/internal/keyproof"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func enrol(args []string) error {
	// Like token's argv form, a file after a bare --key is an ambiguous
	// positional argument. Refuse it before asking the node for a proof.
	for i, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--key" && i+1 < len(args) {
			if _, err := os.Stat(args[i+1]); err == nil {
				return fmt.Errorf("--key takes its file as --key=<file>")
			}
		}
	}
	pos, flags, err := split(args)
	if err := only(flags, err, "key"); err != nil {
		return err
	}
	if len(pos) != 1 {
		return fmt.Errorf("enrol wants one name: agent-bus enrol <user[@realm]> [--key=<file>]")
	}
	name, err := protocol.ParseName(pos[0])
	if err != nil {
		return err
	}
	key := flags["key"]
	if key == "" {
		key = keyproof.Default()
	}
	out, code, err := call("POST", "/enrol", nil, map[string]string{"name": name.String()})
	if err != nil {
		return err
	}
	if code >= 400 {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	var ask keyproof.Challenge
	if err := json.Unmarshal(out, &ask); err != nil {
		return err
	}
	if err := ask.Check(name.String()); err != nil {
		return err
	}
	sig, err := keyproof.Sign(key, ask.Nonce)
	if err != nil {
		return err
	}
	return post("/enrol", map[string]string{"nonce": ask.Nonce, "signature": sig})
}

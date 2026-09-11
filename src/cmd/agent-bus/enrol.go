// `agent-bus enrol` is how a newcomer becomes a name in a realm somebody
// vouches for: the bus says what to sign, the key on this machine signs it,
// and the bus checks the answer against what the directory publishes.
// Fetching a public key proves nothing on its own — this is the step that
// does. See docs/01-identity.md#registration.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/parf/ai-agent-bus/internal/keyproof"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func enrol(args []string) error {
	pos, flags := split(args)
	if len(pos) != 1 {
		return fmt.Errorf("enrol wants one name: agent-bus enrol <user@realm> [--key ~/.ssh/id_ed25519]")
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
	var ask struct{ Name, Nonce, Namespace string }
	if err := json.Unmarshal(out, &ask); err != nil {
		return err
	}
	sig, err := keyproof.Sign(key, ask.Namespace, ask.Nonce)
	if err != nil {
		return err
	}
	return post("/enrol", map[string]string{"nonce": ask.Nonce, "signature": sig})
}

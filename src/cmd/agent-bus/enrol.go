// `agent-bus enrol` is how a newcomer becomes a name in a realm somebody
// vouches for: the bus says what to sign, the key on this machine signs it,
// and the bus checks the answer against what the directory publishes.
// Fetching a public key proves nothing on its own — this is the step that
// does. See docs/01-identity.md#registration.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
		home, _ := os.UserHomeDir()
		key = filepath.Join(home, ".ssh", "id_ed25519")
	}
	if _, err := os.Stat(key); err != nil {
		return fmt.Errorf("no key to prove it with: %w", err)
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
	sig, err := sign(key, ask.Namespace, ask.Nonce)
	if err != nil {
		return err
	}
	return post("/enrol", map[string]string{"nonce": ask.Nonce, "signature": sig})
}

// sign asks the system's own tool, which is the only thing here that touches
// a private key — it never leaves the machine and this process never reads it
// (docs/10-modules.md#the-rule).
func sign(key, namespace, message string) (string, error) {
	cmd := exec.Command("ssh-keygen", "-Y", "sign", "-f", key, "-n", namespace, "-q", "-")
	cmd.Stdin = strings.NewReader(message)
	cmd.Stderr = os.Stderr
	sig, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ssh-keygen sign: %w", err)
	}
	return string(sig), nil
}

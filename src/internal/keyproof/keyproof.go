// The client half of proving possession: the bus says what to sign, and this
// signs it with a key that never leaves the machine. It runs on the holder's
// side, not the daemon's — the daemon's half is internal/signature, behind
// the signatures port. See docs/02-access.md#proving-possession.
package keyproof

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Default is the key a person almost always means.
func Default() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "id_ed25519")
}

// Challenge is the answer to the first POST /enrol: the bus names what to sign.
type Challenge struct{ Name, Nonce, Namespace string }

// Check refuses an answer that asks for anything but the bus's own proof. A
// node that is not the one meant (a wrong address, a plain-http one) answers
// first, and what it asks to have signed must not be signed: a signature made
// for another namespace is valid for that namespace, git commits among them.
// The errors name the member and never echo what the node sent.
// asked is the name this side sent.
func (c Challenge) Check(asked string) error {
	if c.Name != asked {
		return fmt.Errorf("the node answered for another name")
	}
	if len(c.Nonce) != 32 || strings.Trim(c.Nonce, "0123456789abcdef") != "" {
		return fmt.Errorf("the node sent a nonce that is not 32 hex digits")
	}
	if c.Namespace != "" && c.Namespace != protocol.SigNamespace {
		return fmt.Errorf("the node asked for a signature in another namespace")
	}
	return nil
}

// Sign asks the system's own tool, which is the only thing here that touches
// a private key — this process never reads it (src/MODULES.md#the-rule). The
// namespace is not a parameter: the bus's proof is made for protocol.SigNamespace
// and for nothing else, whatever the node asked.
func Sign(key, message string) (string, error) {
	if _, err := os.Stat(key); err != nil {
		return "", fmt.Errorf("no key to prove it with: %w", err)
	}
	cmd := exec.Command("ssh-keygen", "-Y", "sign", "-f", key, "-n", protocol.SigNamespace, "-q", "-")
	cmd.Stdin = strings.NewReader(message)
	cmd.Stderr = os.Stderr
	sig, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ssh-keygen sign: %w", err)
	}
	return string(sig), nil
}

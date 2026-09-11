// The client half of proving possession: the bus says what to sign, and this
// signs it with a key that never leaves the machine. It runs on the holder's
// side, not the daemon's — the daemon's half is internal/signature, behind
// the signatures port. See docs/01-identity.md#proving-possession.
package keyproof

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Default is the key a person almost always means.
func Default() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "id_ed25519")
}

// Sign asks the system's own tool, which is the only thing here that touches
// a private key — this process never reads it (docs/10-modules.md#the-rule).
func Sign(key, namespace, message string) (string, error) {
	if _, err := os.Stat(key); err != nil {
		return "", fmt.Errorf("no key to prove it with: %w", err)
	}
	cmd := exec.Command("ssh-keygen", "-Y", "sign", "-f", key, "-n", namespace, "-q", "-")
	cmd.Stdin = strings.NewReader(message)
	cmd.Stderr = os.Stderr
	sig, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ssh-keygen sign: %w", err)
	}
	return string(sig), nil
}

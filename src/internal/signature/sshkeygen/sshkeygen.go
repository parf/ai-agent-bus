// Package sshkeygen verifies a signature with the system's own tool. There is
// no crypto here and there is not meant to be: `ssh-keygen -Y` already
// implements the sshsig scheme, every host with ssh has it, and the keys a
// directory hands back are in exactly the format it reads
// (docs/10-modules.md#the-rule).
package sshkeygen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

type Verifier struct{}

func New() *Verifier { return &Verifier{} }

// Verify says whether message was signed by the holder of one of keys. id is
// the principal the signature is claimed for; it appears in the allowed
// signers line and in the tool's own answer.
func (v *Verifier) Verify(id, message, signature string, keys []string) error {
	dir, err := os.MkdirTemp("", "agent-bus-verify-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	var allowed strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&allowed, "%s %s\n", id, strings.TrimSpace(k))
	}
	af := filepath.Join(dir, "allowed_signers")
	if err := os.WriteFile(af, []byte(allowed.String()), 0o600); err != nil {
		return err
	}
	sf := filepath.Join(dir, "sig")
	if err := os.WriteFile(sf, []byte(signature), 0o600); err != nil {
		return err
	}
	cmd := exec.Command("ssh-keygen", "-Y", "verify", "-f", af, "-I", id, "-n", protocol.SigNamespace, "-s", sf)
	cmd.Stdin = strings.NewReader(message)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// The tool's own words: "Could not verify signature" is more use to
		// whoever is enrolling than anything this package could invent.
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

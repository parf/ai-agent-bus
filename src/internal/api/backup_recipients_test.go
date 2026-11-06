package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/parf/ai-agent-bus/internal/authkeys"
)

// Only a line that is ours, forced to the credential wrapper for the caller, makes a backup recipient:
// a foreign command that happens to end in the caller's name does not.
func TestBackupRecipientsAreOnlyOurOwnLinesForTheCaller(t *testing.T) {
	ours, foreign1, foreign2 := edKey(t), edKey(t), edKey(t)
	line, err := authkeys.Line("/usr/local/bin/agent-bus-token", "alice@h", ours)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "authorized_keys")
	body := line + "\n" +
		`restrict,command="/usr/bin/python3 /opt/hook.py alice@h" ` + foreign1 + "\n" +
		`restrict,command="/usr/bin/foo alice@h" ` + foreign2 + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	s := &Server{}
	s.SSHKeys(path, "/usr/local/bin/agent-bus-token")
	recipients, _, err := s.backupRecipients("alice@h")
	if err != nil {
		t.Fatal(err)
	}
	if len(recipients) != 1 || recipients[0] != ours {
		t.Fatalf("recipients %v, want only the line agent-bus-token wrote", recipients)
	}
}

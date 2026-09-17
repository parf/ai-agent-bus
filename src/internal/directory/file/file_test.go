package file

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLookupPublishesKeysWithoutInventingAProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys")
	if err := os.WriteFile(path, []byte("alice ssh-ed25519 AAAAone\nbob ssh-rsa AAAAtwo\nalice ssh-rsa AAAAthree comment\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	entry, err := New(path).Lookup("alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(entry.Keys) != 2 || entry.PersonName != "" {
		t.Fatalf("manual directory entry: %+v", entry)
	}
}

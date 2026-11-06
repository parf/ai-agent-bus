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
	if len(entry.Keys) != 2 || entry.Profile.PersonName != "" {
		t.Fatalf("manual directory entry: %+v", entry)
	}
}

// The login Lookup is asked for is a canonical (lower-case) name; a line written with capitals names the
// same person. Only ASCII folds: the Kelvin sign is not a k.
func TestLookupMatchesTheLoginWithoutRegardToASCIICase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys")
	lines := "Alice ssh-ed25519 AAAAone\nALICE ssh-rsa AAAAtwo comment\nalice ssh-ed25519 AAAAthree\nBob ssh-ed25519 AAAAbob\n\u212Aeith ssh-ed25519 AAAAkelvin\nkeith ssh-ed25519 AAAAkeith\n"
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	d := New(path)
	entry, err := d.Lookup("alice")
	if err != nil || len(entry.Keys) != 3 {
		t.Fatalf("alice: %+v %v", entry, err)
	}
	if entry, _ := d.Lookup("bob"); len(entry.Keys) != 1 || entry.Keys[0] != "ssh-ed25519 AAAAbob" {
		t.Fatalf("a capitalised line is not found: %+v", entry)
	}
	if entry, _ := d.Lookup("keith"); len(entry.Keys) != 1 || entry.Keys[0] != "ssh-ed25519 AAAAkeith" {
		t.Fatalf("the Kelvin-sign line stood for keith: %+v", entry)
	}
	if entry, _ := d.Lookup("carol"); len(entry.Keys) != 0 {
		t.Fatalf("an unknown login published keys: %+v", entry)
	}
}

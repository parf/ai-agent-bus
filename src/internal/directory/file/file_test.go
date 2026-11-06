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

// The login ends at the first space or tab, and any run of them may follow it, as in authorized_keys;
// what follows is kept as written.
func TestLookupSplitsTheLineOnSpacesOrTabs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys")
	lines := "alice\tssh-ed25519 AAAAtab\n" +
		"alice \t ssh-ed25519\tAAAAmixed\tcomment with  spaces\n" +
		"  alice\t\tssh-rsa AAAArun\r\n" +
		"alicia\tssh-ed25519 AAAAother\n" +
		"alice\n" +
		"alice\t\n" +
		"alice ssh-ed25519 AAAAspace\n"
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	entry, err := New(path).Lookup("alice")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ssh-ed25519 AAAAtab", "ssh-ed25519\tAAAAmixed\tcomment with  spaces", "ssh-rsa AAAArun", "ssh-ed25519 AAAAspace"}
	if len(entry.Keys) != len(want) {
		t.Fatalf("keys %q, want %q", entry.Keys, want)
	}
	for i := range want {
		if entry.Keys[i] != want[i] {
			t.Errorf("key %d is %q, want %q", i, entry.Keys[i], want[i])
		}
	}
}

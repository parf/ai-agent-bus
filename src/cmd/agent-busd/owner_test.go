package main

import "testing"

func TestDaemonOwnerMustBeExplicit(t *testing.T) {
	for _, value := range []string{"", "-local-account", "owner@"} {
		if _, err := requiredOwner(value); err == nil {
			t.Fatalf("accepted missing or invalid owner %q", value)
		}
	}
	if got, err := requiredOwner(" Owner@Host "); err != nil || got.String() != "owner@host" {
		t.Fatalf("valid explicit owner: %s, %v", got, err)
	}
	// A bare account name is a whole name: the 0.7 default Owner.
	if got, err := requiredOwner("parf"); err != nil || got.String() != "parf" {
		t.Fatalf("bare owner: %s, %v", got, err)
	}
}

package main

import (
	"errors"
	"os/user"
	"strings"
	"testing"
)

func TestDarwinExistingRoleRequiresConsistentGroupWithoutMutation(t *testing.T) {
	for _, name := range []string{"matching", "missing", "different-id", "group-unavailable", "user-unavailable", "invalid-id"} {
		t.Run(name, func(t *testing.T) {
			l, ran := fakeDarwin(t)
			oldUser, oldGroup := darwinLookupUser, darwinLookupGroup
			t.Cleanup(func() { darwinLookupUser, darwinLookupGroup = oldUser, oldGroup })
			darwinLookupUser = func(account string) (*user.User, error) {
				if name == "user-unavailable" {
					return nil, errors.New("directory unavailable")
				}
				gid := "201"
				if name == "invalid-id" {
					gid = "bad"
				}
				return &user.User{Username: account, Uid: "201", Gid: gid}, nil
			}
			darwinLookupGroup = func(account string) (*user.Group, error) {
				if name == "missing" {
					return nil, user.UnknownGroupError(account)
				}
				if name == "group-unavailable" {
					return nil, errors.New("directory unavailable")
				}
				gid := "201"
				if name == "different-id" {
					gid = "202"
				}
				return &user.Group{Name: account, Gid: gid}, nil
			}
			err := l.ensureAccount()
			if (err == nil) != (name == "matching") {
				t.Fatalf("%s: %v", name, err)
			}
			if name == "missing" && !strings.Contains(err.Error(), "existing group") {
				t.Fatalf("unclear missing group refusal: %v", err)
			}
			if len(*ran) != 0 {
				t.Fatalf("existing role account was mutated: %v", *ran)
			}
		})
	}
}

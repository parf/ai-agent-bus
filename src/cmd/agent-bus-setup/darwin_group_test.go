package main

import (
	"errors"
	"os/user"
	"strings"
	"testing"
)

type roleGroups struct {
	groups    map[string]*user.Group
	create    bool
	lookupErr error
}

func fakeExistingDarwinRole(t *testing.T) (darwinLayout, *[]string, *roleGroups) {
	t.Helper()
	l, ran := fakeDarwin(t)
	state := &roleGroups{groups: map[string]*user.Group{}, create: true}
	oldUser, oldGroup, oldID, oldCommand := darwinLookupUser, darwinLookupGroup, darwinLookupGroupID, darwinCommand
	t.Cleanup(func() {
		darwinLookupUser, darwinLookupGroup, darwinLookupGroupID, darwinCommand = oldUser, oldGroup, oldID, oldCommand
	})
	darwinLookupUser = func(name string) (*user.User, error) { return &user.User{Username: name, Uid: "201", Gid: "201"}, nil }
	darwinLookupGroup = func(name string) (*user.Group, error) {
		if state.lookupErr != nil {
			return nil, state.lookupErr
		}
		if group := state.groups[name]; group != nil {
			return group, nil
		}
		return nil, user.UnknownGroupError(name)
	}
	darwinLookupGroupID = func(gid string) (*user.Group, error) {
		for _, group := range state.groups {
			if group.Gid == gid {
				return group, nil
			}
		}
		return nil, user.UnknownGroupIdError(gid)
	}
	darwinCommand = func(stdin []byte, name string, args ...string) error {
		if err := oldCommand(stdin, name, args...); err != nil {
			return err
		}
		if state.create && name == "dseditgroup" && len(args) == 5 && args[0] == "-o" && args[1] == "create" && args[2] == "-i" {
			state.groups[args[4]] = &user.Group{Name: args[4], Gid: args[3]}
		}
		return nil
	}
	return l, ran, state
}

func TestDarwinExistingRoleRestoresMissingGroup(t *testing.T) {
	l, ran, state := fakeExistingDarwinRole(t)
	if err := l.ensureAccount(); err != nil {
		t.Fatal(err)
	}
	want := "dseditgroup -o create -i 201 " + l.account
	if len(*ran) != 1 || (*ran)[0] != want {
		t.Fatalf("commands=%v, want only %q; user must remain untouched", *ran, want)
	}
	if group := state.groups[l.account]; group == nil || group.Gid != "201" {
		t.Fatalf("role group not restored at existing PrimaryGroupID: %+v", group)
	}
}
func TestDarwinExistingRoleRefusesGroupIDClash(t *testing.T) {
	l, ran, state := fakeExistingDarwinRole(t)
	state.groups["other-group"] = &user.Group{Name: "other-group", Gid: "201"}
	err := l.ensureAccount()
	if err == nil || !strings.Contains(err.Error(), l.account) || !strings.Contains(err.Error(), "other-group") || !strings.Contains(err.Error(), "201") {
		t.Fatalf("gid clash not refused by both names: %v", err)
	}
	if len(*ran) != 0 {
		t.Fatalf("clashing gid changed directory: %v", *ran)
	}
}
func TestDarwinExistingRoleRefusesDifferentNamedGroupID(t *testing.T) {
	l, ran, state := fakeExistingDarwinRole(t)
	state.groups[l.account] = &user.Group{Name: l.account, Gid: "202"}
	err := l.ensureAccount()
	if err == nil || !strings.Contains(err.Error(), "201") || !strings.Contains(err.Error(), "202") {
		t.Fatalf("wrong named group not refused with both ids: %v", err)
	}
	if len(*ran) != 0 {
		t.Fatalf("wrong named group changed: %v", *ran)
	}
}
func TestDarwinExistingRoleReusesMatchingGroup(t *testing.T) {
	l, ran, state := fakeExistingDarwinRole(t)
	state.groups[l.account] = &user.Group{Name: l.account, Gid: "201"}
	if err := l.ensureAccount(); err != nil {
		t.Fatal(err)
	}
	if len(*ran) != 0 {
		t.Fatalf("matching role pair changed: %v", *ran)
	}
}
func TestDarwinExistingRoleVerifiesGroupCreation(t *testing.T) {
	l, _, state := fakeExistingDarwinRole(t)
	state.create = false
	if err := l.ensureAccount(); err == nil || !strings.Contains(err.Error(), "not created") {
		t.Fatalf("unverified creation accepted: %v", err)
	}
}
func TestDarwinExistingRoleLookupFailureDoesNotCreateGroup(t *testing.T) {
	l, ran, state := fakeExistingDarwinRole(t)
	state.lookupErr = errors.New("directory offline")
	if err := l.ensureAccount(); err == nil {
		t.Fatal("directory failure was accepted")
	}
	if len(*ran) != 0 {
		t.Fatalf("lookup failure changed group: %v", *ran)
	}
}

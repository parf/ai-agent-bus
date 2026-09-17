package main

import (
	"reflect"
	"testing"
)

func TestSSHRequestKeepsTheKeyEntitlementSeparate(t *testing.T) {
	for _, tc := range []struct {
		name     string
		argv     []string
		request  string
		want     []string
		entitled string
		invalid  bool
	}{
		{"own token", []string{"owner@h"}, "token", []string{"token"}, "owner@h", false},
		{"rotation", []string{"owner@h"}, "token --rotate", []string{"token", "--rotate"}, "owner@h", false},
		{"other name remains untrusted", []string{"owner@h"}, "token alice@h", []string{"token", "alice@h"}, "owner@h", false},
		{"empty command is not a console alias", []string{"owner@h"}, "", nil, "", true},
		{"operator verb", []string{"owner@h"}, "user list", []string{"user", "list"}, "owner@h", false},
		{"account administration", []string{"owner@h"}, "account set local alice@h", []string{"account", "set", "local", "alice@h"}, "owner@h", false},
		{"console", []string{"token", "alice@h"}, "", []string{"token", "alice@h"}, "", false},
		{"missing entitlement", nil, "user list", nil, "", true},
		{"extra forced args", []string{"owner@h", "alice@h"}, "token", nil, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args, entitled, err := adminRequest(tc.argv, tc.request)
			if (err != nil) != tc.invalid {
				t.Fatalf("error=%v, want invalid=%v", err, tc.invalid)
			}
			if !tc.invalid && (!reflect.DeepEqual(args, tc.want) || entitled != tc.entitled) {
				t.Fatalf("args=%v entitlement=%q, want %v %q", args, entitled, tc.want, tc.entitled)
			}
		})
	}
}

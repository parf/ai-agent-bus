package main

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestUserUninstallRefusesARegistryListingFailureBeforePlanningRemoval(t *testing.T) {
	for _, failure := range []string{"unavailable", "malformed", "transport"} {
		t.Run(failure, func(t *testing.T) {
			u := userRunner{person: "alice", uid: 1001, gid: 1001, name: "#runner/alice-user@fixture"}
			c := &ownerClient{http: http.Client{Transport: setupLookupTransport(func(r *http.Request) (*http.Response, error) {
				code, body := 200, `{"kind":"agent","owner":"alice"}`
				if r.URL.Path == "/lookup" && r.URL.Query().Get("name") == "alice" {
					body = `{"kind":"user"}`
				}
				if r.URL.Path == "/ls" {
					switch failure {
					case "unavailable":
						code, body = 503, `{"error":"offline"}`
					case "malformed":
						body = `not json`
					case "transport":
						return nil, errors.New("registry unavailable")
					}
				}
				return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}}
			st, err := observeUser(u, hostUserOps{bus: c}, true)
			ops := &fakeUserOps{}
			if err == nil {
				plan, planErr := userPlan("uninstall", u, st, "", false, ops)
				if planErr != nil {
					t.Fatal(planErr)
				}
				for _, a := range plan {
					if err := a.do(); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err == nil || !strings.Contains(err.Error(), "list agents") {
				t.Fatalf("failed listing allowed uninstall planning: error=%v actions=%v", err, ops.did)
			}
			if len(ops.did) != 0 {
				t.Fatalf("failed listing performed destructive actions: %v", ops.did)
			}
		})
	}
}

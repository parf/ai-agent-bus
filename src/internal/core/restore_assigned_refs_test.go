package core

import (
	"slices"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestRestoreKeepsAssignedACLsAfterAnotherPrincipalsLegalEdit(t *testing.T) {
	for _, edit := range []string{"inactive-group", "later-agent"} {
		t.Run(edit, func(t *testing.T) {
			b := rolesFixture(t)
			term, caller := "@crew(deploy)", "carol@h"
			if edit == "later-agent" {
				term, caller = "future@h(deploy)", "#future@h"
			}
			grant(t, b, "#svc@h", term, "*(guest)")
			queued, err := b.Send(protocol.Envelope{From: "alice@h", To: "#svc@h", Body: "queued before the edit"})
			if err != nil {
				t.Fatal(err)
			}
			if edit == "inactive-group" {
				if _, err := b.Manage("admin@h", Management{Name: "@crew", Status: ptr("inactive")}); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := b.Register(protocol.Record{Name: "#future@h", Kind: protocol.KindAgent, Owner: "bob@h"}); err != nil {
					t.Fatal(err)
				}
			}
			snapshot := b.Snapshot()
			restored := New()
			if err := restored.Restore(snapshot); err != nil {
				t.Fatalf("legal edit made the snapshot unrestorable: %v", err)
			}
			record, ok := restored.Lookup("alice@h", "#svc@h")
			if !ok || !slices.Equal(record.Allow, []string{term, "*(guest)"}) {
				t.Fatalf("restore changed the record or its grants: %+v", record)
			}
			if got := next(t, restored, "#svc@h"); got.ID != queued.ID || got.Body != queued.Body {
				t.Fatalf("restore lost the queued message: %+v", got)
			}
			if roles := sendRoles(t, restored, caller, "#svc@h", "#svc@h"); !slices.Equal(roles, []string{"guest"}) {
				t.Fatalf("unresolved role term granted roles: %v", roles)
			}
			if _, err := restored.Manage("alice@h", Management{Name: "#svc@h", Allow: ptr([]string{term, "*(guest)"})}); err == nil {
				t.Fatal("fresh write accepted a currently invalid reference")
			}
			if edit == "later-agent" {
				// Unknown User terms are accepted on fresh writes too. Once
				// that User exists, the restored term must behave exactly
				// like a newly written term, never like the #Agent alias.
				if _, err := restored.SetUser("admin@h", protocol.User{Name: "future@h"}, true); err != nil {
					t.Fatal(err)
				}
				if _, err := restored.Register(protocol.Record{Name: "#fresh@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{term, "*(guest)"}}); err != nil {
					t.Fatalf("fresh write refused the same User term: %v", err)
				}
				storedRoles := sendRoles(t, restored, "future@h", "#svc@h", "#svc@h")
				freshRoles := sendRoles(t, restored, "future@h", "#fresh@h", "#fresh@h")
				if !slices.Equal(storedRoles, []string{"deploy", "guest"}) || !slices.Equal(storedRoles, freshRoles) {
					t.Fatalf("restored term differs from fresh-write authority: stored=%v fresh=%v", storedRoles, freshRoles)
				}
			}

		})
	}
}

func TestRestoreStillRefusesMalformedAssignedACLTerms(t *testing.T) {
	for _, tc := range []struct{ name, target, term string }{
		{"bad-user", "#svc@h", "bad user(deploy)"},
		{"bad-group", "#svc@h", "@bad group(deploy)"},
		{"wrong-kind", "jobs@h", "@agent(deploy)"},
		{"reserved-role", "#svc@h", "bob@h(owner)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := rolesFixture(t).Snapshot()
			for i := range snapshot.Records {
				if snapshot.Records[i].Name == tc.target {
					snapshot.Records[i].Allow = []string{tc.term}
				}
			}
			if err := ValidateSnapshot(snapshot); err == nil {
				t.Fatalf("malformed stored ACL term accepted: %q", tc.term)
			}
		})
	}
}

package core

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func grant(t *testing.T, b *Bus, name string, allow ...string) {
	t.Helper()
	if _, e := b.Manage("alice@h", Management{Name: name, Allow: &allow}); e != nil {
		t.Fatal(e)
	}
}
func sendRoles(t *testing.T, b *Bus, from, to, inbox string) []string {
	t.Helper()
	if _, e := b.Send(protocol.Envelope{From: from, To: to, Body: "work"}); e != nil {
		t.Fatal(e)
	}
	return next(t, b, inbox).Roles
}

func TestAssignedRolesUnionIncludesManagementAndAllMatchingTerms(t *testing.T) {
	b := rolesFixture(t)
	grant(t, b, "#svc@h", "bob@h(deploy,read_only)", "*(guest)", "bob@h(deploy)", "@crew(review)", "alice@h(admin)", "carol@h(audit)")
	for _, c := range []struct {
		from string
		want []string
	}{
		{"bob@h", []string{"deploy", "read_only", "guest"}},
		{"alice@h", []string{"owner", "guest", "admin"}},
		{"carol@h", []string{"maintainer", "guest", "review", "audit"}},
	} {
		if got := sendRoles(t, b, c.from, "#svc@h", "#svc@h"); !slices.Equal(got, c.want) {
			t.Fatalf("%s roles %v want %v", c.from, got, c.want)
		}
	}
	r, _ := b.Lookup("alice@h", "#svc@h")
	if len(r.Roles) != 0 {
		t.Fatal("fixture supported list unexpectedly populated")
	}
	if _, e := b.Manage("alice@h", Management{Name: "#svc@h", Roles: ptr([]string{"irrelevant"})}); e != nil {
		t.Fatal(e)
	}
	grant(t, b, "#svc@h", "bob@h")
	if got := sendRoles(t, b, "bob@h", "#svc@h", "#svc@h"); len(got) != 0 {
		t.Fatalf("supported roles created assignments: %v", got)
	}
}

func TestReservedRoleAssignmentsRejectWholeSave(t *testing.T) {
	b := rolesFixture(t)
	for _, bad := range []string{"bob@h(owner)", "@crew(deploy,maintainer)", "*(owner)", "bob@h(Deploy)", "bob@h(read-only)", "bob@h()"} {
		before, _ := b.Lookup("alice@h", "#svc@h")
		desc := "must not change"
		if _, e := b.Manage("alice@h", Management{Name: "#svc@h", Allow: ptr([]string{bad}), Descr: &desc}); e == nil {
			t.Fatalf("saved %s", bad)
		}
		after, _ := b.Lookup("alice@h", "#svc@h")
		if after.Descr != before.Descr || !slices.Equal(after.Allow, before.Allow) {
			t.Fatal("refused save partly applied")
		}
		if _, e := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#bad@h", Owner: "alice@h", Allow: []string{bad}}); e == nil {
			t.Fatalf("registered %s", bad)
		}
	}
	if _, e := b.Manage("alice@h", Management{Name: "#svc@h", Maintainers: ptr(protocol.MaintainerList{"bob@h(admin)"})}); e == nil {
		t.Fatal("assigned a role in Maintainers")
	}
	if _, e := b.Register(protocol.Record{Kind: protocol.KindGroup, Name: "@role-members", Owner: "alice@h", Allow: []string{"bob@h(admin)"}}); e == nil {
		t.Fatal("group accepted a role assignment")
	}
}

func TestAssignedRolesNestedGroupsStayLiveAndCycleSafe(t *testing.T) {
	b := rolesFixture(t)
	for _, n := range []string{"@a", "@b", "@c", "@top"} {
		if e := b.SetGroup("admin@h", n, nil); e != nil {
			t.Fatal(e)
		}
	}
	for n, m := range map[string][]string{"@a": {"@b"}, "@b": {"@a", "@c"}, "@c": {"bob@h"}, "@top": {"@a", "@b"}} {
		if e := b.SetGroup("admin@h", n, m); e != nil {
			t.Fatal(e)
		}
	}
	grant(t, b, "#svc@h", "@a(one)", "@b(two)", "@top(three)", "*(guest)")
	if got := sendRoles(t, b, "bob@h", "#svc@h", "#svc@h"); !slices.Equal(got, []string{"one", "two", "three", "guest"}) {
		t.Fatal(got)
	}
	if e := b.SetGroup("admin@h", "@c", nil); e != nil {
		t.Fatal(e)
	}
	if got := sendRoles(t, b, "bob@h", "#svc@h", "#svc@h"); !slices.Equal(got, []string{"guest"}) {
		t.Fatalf("stale group roles: %v", got)
	}
	// Differentially compare the multi-root algorithm with existing membership.
	b.mu.Lock()
	defer b.mu.Unlock()
	gs := []protocol.RoleGrant{{Actor: "@a"}, {Actor: "@b"}, {Actor: "@c"}, {Actor: "@top"}, {Actor: AdministratorsGroup}}
	for _, who := range []string{"bob@h", "admin@h", "nobody@h"} {
		for _, g := range gs {
			if b.roleGroups(who, gs)[g.Actor] != b.member(who, g.Actor) {
				t.Fatalf("membership differs %s %s", who, g.Actor)
			}
		}
	}
}

func TestAssignedRolesUseRuntimeTermsAndPersonalActorConstraints(t *testing.T) {
	b := rolesFixture(t)
	if _, e := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#own@h", Owner: "alice@h"}); e != nil {
		t.Fatal(e)
	}
	grant(t, b, "#svc@h", "@owner(cohort)", "@agent(self)")
	if got := sendRoles(t, b, "#own@h", "#svc@h", "#svc@h"); !slices.Equal(got, []string{"cohort"}) {
		t.Fatal(got)
	}
	if got := sendRoles(t, b, "#svc@h", "#svc@h", "#svc@h"); !slices.Equal(got, []string{"cohort", "self"}) {
		t.Fatal(got)
	}
	if _, e := b.Manage("alice@h", Management{Name: "#svc@h", Maintainers: ptr(protocol.MaintainerList{}), Personal: ptr(true)}); e != nil {
		t.Fatal(e)
	}
	if _, e := b.Manage("alice@h", Management{Name: "#svc@h", Allow: ptr([]string{"bob@h(deploy)"})}); !errors.Is(e, ErrPersonal) {
		t.Fatalf("Personal widened: %v", e)
	}
}

func TestAssignedRolesRollbackRestoreAndNoReadWrites(t *testing.T) {
	d := &countingState{State: memory.NewState()}
	b := durabilityFixture(t, d)
	grant(t, b, "#svc@h", "bob@h(original)")
	d.last = ports.Change{}
	if got := sendRoles(t, b, "bob@h", "#svc@h", "#svc@h"); !slices.Equal(got, []string{"original"}) {
		t.Fatal(got)
	}
	if len(d.last.Records) > 0 {
		t.Fatal("send wrote role state")
	}
	d.State.Err = errors.New("disk error")
	if _, e := b.Manage("alice@h", Management{Name: "#svc@h", Allow: ptr([]string{"bob@h(stale)"})}); e == nil {
		t.Fatal("failed commit succeeded")
	}
	if got := sendRoles(t, b, "bob@h", "#svc@h", "#svc@h"); !slices.Equal(got, []string{"original"}) {
		t.Fatalf("failed commit poisoned cache %v", got)
	}
	d.State.Err = nil
	saved, _ := d.Load()
	restored := New()
	restored.Restore(saved)
	if got := sendRoles(t, restored, "bob@h", "#svc@h", "#svc@h"); !slices.Equal(got, []string{"original"}) {
		t.Fatalf("restore lost roles %v", got)
	}
	for _, r := range saved.Records {
		if r.ParsedAllow != nil {
			t.Fatal("derived ACL persisted")
		}
	}
	bad := protocol.Record{Kind: protocol.KindAgent, Name: "#bad@h", Owner: "alice@h", Allow: []string{"bob@h(owner)"}}
	saved.Records = append(saved.Records, bad)
	restored = New()
	restored.Restore(saved)
	if _, ok := restored.Lookup("alice@h", "#bad@h"); ok {
		t.Fatal("malformed stored role record survived")
	}
}

func TestAssignedRolesFollowForwardsWithoutRecomputation(t *testing.T) {
	b := rolesFixture(t)
	grant(t, b, "jobs@h", "bob@h(source)")
	grant(t, b, "#worker@h", "jobs@h(destination)", "news@h(copy_target)")
	if got := sendRoles(t, b, "bob@h", "jobs@h", "#worker@h"); !slices.Equal(got, []string{"source"}) {
		t.Fatal(got)
	}
	grant(t, b, "news@h", "bob@h(published)")
	if got := sendRoles(t, b, "bob@h", "news@h", "#worker@h"); !slices.Equal(got, []string{"published"}) {
		t.Fatal(got)
	}
	grant(t, b, "relay@h", "bob@h(blocked)")
	if _, e := b.Send(protocol.Envelope{From: "bob@h", To: "relay@h", Body: "work"}); !errors.Is(e, ErrNotAllow) {
		t.Fatalf("cross-owner roles passed %v", e)
	}
}

func TestRoleFreeCollectionAllocatesNothing(t *testing.T) {
	b := rolesFixture(t)
	r := b.records["#svc@h"]
	if r.ParsedAllow != nil {
		t.Fatal("plain ACL compiled assignments")
	}
	if n := testing.AllocsPerRun(100, func() {
		if len(b.assignedRoles("bob@h", r, nil)) != 0 {
			panic("unexpected roles")
		}
	}); n != 0 {
		t.Fatalf("plain collection allocates %v", n)
	}
	long := strings.Repeat("x", 100)
	grant(t, b, "#svc@h", "bob@h("+long+")")
	if got := sendRoles(t, b, "bob@h", "#svc@h", "#svc@h"); !slices.Equal(got, []string{long}) {
		t.Fatal(got)
	}
}

func TestRoleAssignmentDeltasAndNameCleanup(t *testing.T) {
	b := rolesFixture(t)
	if _, e := b.Manage("alice@h", Management{Name: "#svc@h", Add: &ListDelta{Allow: []string{"bob@h(deploy, read_only)"}}}); e != nil {
		t.Fatal(e)
	}
	if got := sendRoles(t, b, "bob@h", "#svc@h", "#svc@h"); !slices.Equal(got, []string{"deploy", "read_only"}) {
		t.Fatal(got)
	}
	if _, e := b.Manage("alice@h", Management{Name: "#svc@h", Remove: &ListDelta{Allow: []string{"bob@h(deploy,read_only)"}}}); e != nil {
		t.Fatal(e)
	}
	if got := sendRoles(t, b, "bob@h", "#svc@h", "#svc@h"); len(got) != 0 {
		t.Fatal(got)
	}
	if _, e := b.Register(protocol.Record{Name: "#reused@h", Owner: "alice@h", Kind: protocol.KindAgent}); e != nil {
		t.Fatal(e)
	}
	grant(t, b, "#svc@h", "#reused@h(deploy)", "*")
	if e := b.Unregister("#reused@h", "alice@h"); e != nil {
		t.Fatal(e)
	}
	if _, e := b.Register(protocol.Record{Name: "#reused@h", Owner: "eve@h", Kind: protocol.KindAgent}); e != nil {
		t.Fatal(e)
	}
	if got := sendRoles(t, b, "#reused@h", "#svc@h", "#svc@h"); len(got) != 0 {
		t.Fatalf("new name inherited roles %v", got)
	}
	grant(t, b, "#svc@h", "@owner(cohort)")
	if _, e := b.Register(protocol.Record{Name: "#personal@h", Owner: "alice@h", Kind: protocol.KindAgent, Personal: true, Allow: []string{"#svc@h(read)"}}); e != nil {
		t.Fatal(e)
	}
	if _, e := b.Manage("alice@h", Management{Name: "#svc@h", Owner: ptr("eve@h")}); e != nil {
		t.Fatal(e)
	}
	r, _ := b.Lookup("alice@h", "#personal@h")
	if len(r.Allow) != 0 {
		t.Fatalf("transfer left Personal grant %v", r.Allow)
	}
}

func TestAssignedRoleUnionCrossesSmallSetBoundary(t *testing.T) {
	b := rolesFixture(t)
	for _, n := range []int{8, 9, 30} {
		var names []string
		for i := 0; i < n; i++ {
			names = append(names, "r"+strconv.Itoa(i))
		}
		grant(t, b, "#svc@h", "bob@h("+strings.Join(names, ",")+")", "*("+strings.Join(names, ",")+")")
		if got := sendRoles(t, b, "bob@h", "#svc@h", "#svc@h"); !slices.Equal(got, names) {
			t.Fatalf("%d union %v", n, got)
		}
	}
}

func TestInactiveGroupContributesNoAssignedRoles(t *testing.T) {
	b := rolesFixture(t)
	grant(t, b, "#svc@h", "@crew(review)", "*(guest)")
	if got := sendRoles(t, b, "carol@h", "#svc@h", "#svc@h"); !slices.Equal(got, []string{"maintainer", "review", "guest"}) {
		t.Fatal(got)
	}
	if _, e := b.Manage("admin@h", Management{Name: "@crew", Status: ptr("inactive")}); e != nil {
		t.Fatal(e)
	}
	if got := sendRoles(t, b, "carol@h", "#svc@h", "#svc@h"); !slices.Equal(got, []string{"guest"}) {
		t.Fatalf("inactive group leaked roles %v", got)
	}
}

func TestAssignedRolesKeepAdministratorMembershipDirectOnly(t *testing.T) {
	b := rolesFixture(t)
	if e := b.SetGroup("admin@h", "@admin-wrapper", []string{AdministratorsGroup}); e != nil {
		t.Fatal(e)
	}
	grant(t, b, "#svc@h", "@admin-wrapper(via_admin)")
	if got := sendRoles(t, b, "admin@h", "#svc@h", "#svc@h"); !slices.Equal(got, []string{"via_admin"}) {
		t.Fatal(got)
	}
	// A nested group is refused at the authority write boundary as before.
	if e := b.SetGroup("admin@h", AdministratorsGroup, []string{"@crew"}); e == nil {
		t.Fatal("nested administrative membership accepted")
	}
	// Also pin the traversal boundary defensively against an in-memory bad edge.
	r := b.records[AdministratorsGroup]
	r.Allow = append(append([]string{}, r.Allow...), "@crew")
	b.records[r.Name] = r
	if got := sendRoles(t, b, "carol@h", "#svc@h", "#svc@h"); !slices.Equal(got, []string{"maintainer"}) {
		t.Fatalf("traversed below administrators: %v", got)
	}
}

func TestACLRemovalRevokesDecoratedActorsAndIgnoresRoleOrder(t *testing.T) {
	b := rolesFixture(t)
	grant(t, b, "#svc@h", "bob@h(deploy,read_only)", "bob@h(audit)", "carol@h")
	if _, e := b.Manage("alice@h", Management{Name: "#svc@h", Remove: &ListDelta{Allow: []string{"bob@h(read_only,deploy)"}}}); e != nil {
		t.Fatal(e)
	}
	if got := sendRoles(t, b, "bob@h", "#svc@h", "#svc@h"); !slices.Equal(got, []string{"audit"}) {
		t.Fatalf("role-order removal did not revoke grant: %v", got)
	}
	if _, e := b.Manage("alice@h", Management{Name: "#svc@h", Remove: &ListDelta{Allow: []string{"bob@h"}}}); e != nil {
		t.Fatal(e)
	}
	if _, e := b.Send(protocol.Envelope{From: "bob@h", To: "#svc@h", Body: "must be revoked"}); !errors.Is(e, ErrNotAllow) {
		t.Fatalf("bare-name revocation left access: %v", e)
	}
	r, _ := b.Lookup("alice@h", "#svc@h")
	if !slices.Equal(r.Allow, []string{"carol@h"}) {
		t.Fatalf("unrelated grants changed: %v", r.Allow)
	}
}

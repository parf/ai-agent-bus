package core

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// rolesFixture: alice owns #svc@h and jobs@h; @crew (carol) maintains both;
// bob is only on their allow lists. jobs@h forwards to eve's #worker@h, which
// admits it; news@h, alice's, copies to #svc@h.
func rolesFixture(t *testing.T) *Bus {
	t.Helper()
	b := New()
	b.SetDaemonOwner("admin@h")
	for _, who := range []string{"alice@h", "bob@h", "carol@h", "eve@h"} {
		if _, err := b.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.SetGroup("admin@h", "@crew", []string{"carol@h"}); err != nil {
		t.Fatal(err)
	}
	for _, r := range []protocol.Record{
		{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "alice@h", Allow: []string{"bob@h", "carol@h", "news@h"}, Maintainers: protocol.MaintainerList{"@crew"}},
		{Kind: protocol.KindAgent, Name: "#worker@h", Owner: "eve@h", Allow: []string{"jobs@h"}},
		{Kind: protocol.KindQueue, Name: "jobs@h", Owner: "alice@h", Allow: []string{"bob@h", "carol@h"}, Maintainers: protocol.MaintainerList{"@crew"}, Subs: []string{"#worker@h"}},
		{Kind: protocol.KindPubSub, Name: "news@h", Owner: "alice@h", Allow: []string{"*"}, Subs: []string{"#svc@h"}},
	} {
		if _, err := b.Register(r); err != nil {
			t.Fatalf("%s: %v", r.Name, err)
		}
	}
	return b
}

func next(t *testing.T, b *Bus, inbox string) protocol.Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	e, err := b.Consume(ctx, inbox, "", "", false, false)
	if err != nil {
		t.Fatalf("%s: %v", inbox, err)
	}
	return e
}

// A message carries the roles its sender holds toward the record it
// addressed: owner, maintainer (through a Group too), or none.
func TestAMessageCarriesItsSendersRoles(t *testing.T) {
	b := rolesFixture(t)
	for _, c := range []struct {
		from string
		want []string
	}{
		{"alice@h", []string{RoleOwner}},
		{"carol@h", []string{RoleMaintainer}},
		{"bob@h", nil},
	} {
		if _, err := b.Send(protocol.Envelope{From: c.from, To: "#svc@h", Body: "hi"}); err != nil {
			t.Fatalf("%s: %v", c.from, err)
		}
		if got := next(t, b, "#svc@h").Roles; !slices.Equal(got, c.want) {
			t.Errorf("%s arrived with roles %v, want %v", c.from, got, c.want)
		}
	}
	// A sender states none: the daemon works them out.
	if _, err := b.Send(protocol.Envelope{From: "bob@h", To: "#svc@h", Body: "hi", Roles: []string{RoleOwner}}); !errors.Is(err, ErrKind) {
		t.Fatalf("a sender claimed owner: %v", err)
	}
	if _, err := b.Send(protocol.Envelope{From: "bob@h", To: "#svc@h", Body: "hi", Roles: []string{}}); !errors.Is(err, ErrKind) {
		t.Fatalf("a sender stated an empty list: %v", err)
	}
}

// Roles are worked out at the record the sender addressed and pass through a
// forward and a copy unchanged: the worker behind jobs@h sees carol as the
// queue's Maintainer, which she is not of the worker.
func TestRolesPassThroughForwardsAndCopies(t *testing.T) {
	b := rolesFixture(t)
	if _, err := b.Send(protocol.Envelope{From: "carol@h", To: "jobs@h", Body: "work"}); err != nil {
		t.Fatal(err)
	}
	if e := next(t, b, "#worker@h"); !slices.Equal(e.Roles, []string{RoleMaintainer}) || e.OriginalTo != "jobs@h" {
		t.Fatalf("the forwarded message arrived with roles %v through %q", e.Roles, e.OriginalTo)
	}
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "news@h", Body: "news"}); err != nil {
		t.Fatal(err)
	}
	if e := next(t, b, "#svc@h"); !slices.Equal(e.Roles, []string{RoleOwner}) {
		t.Fatalf("the published copy arrived with roles %v", e.Roles)
	}
}

// An Agent lists the roles it understands: valid names, each once, on an
// Agent alone; a refresh that states none keeps them; a settings edit
// replaces them.
func TestAnAgentListsTheRolesItUnderstands(t *testing.T) {
	b := rolesFixture(t)
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "alice@h", Roles: []string{"admin", "read_only"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "#svc@h", Descr: "refreshed"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := b.Lookup("alice@h", "#svc@h"); !slices.Equal(got.Roles, []string{"admin", "read_only"}) {
		t.Fatalf("a refresh dropped the roles: %v", got.Roles)
	}
	for _, bad := range [][]string{{"Read-Only"}, {"read-only"}, {"admin", "admin"}, {""}} {
		if _, err := b.Manage("alice@h", Management{Name: "#svc@h", Roles: &bad}); !errors.Is(err, ErrKind) {
			t.Errorf("roles %q were taken: %v", bad, err)
		}
	}
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", Roles: &[]string{"admin"}}); !errors.Is(err, ErrKind) {
		t.Errorf("a queue took roles: %v", err)
	}
	if _, err := b.Manage("carol@h", Management{Name: "#svc@h", Roles: &[]string{"deploy"}}); err != nil {
		t.Fatalf("a Maintainer could not set them: %v", err)
	}
	if got, _ := b.Lookup("alice@h", "#svc@h"); !slices.Equal(got.Roles, []string{"deploy"}) {
		t.Fatalf("the edit left %v", got.Roles)
	}
}

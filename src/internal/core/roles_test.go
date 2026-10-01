package core

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// rolesFixture: alice owns #svc@h and jobs@h; @crew (carol) maintains both;
// bob is only on their allow lists. jobs@h forwards to eve's #worker@h, and
// news@h, alice's, copies to it; alice maintains the worker, so roles may pass
// there. relay@h, alice's, forwards to eve's #stranger@h, which she does not.
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
		{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "alice@h", Allow: []string{"bob@h", "carol@h"}, Maintainers: protocol.MaintainerList{"@crew"}},
		{Kind: protocol.KindAgent, Name: "#worker@h", Owner: "eve@h", Allow: []string{"jobs@h", "news@h"}, Maintainers: protocol.MaintainerList{"alice@h"}},
		{Kind: protocol.KindAgent, Name: "#stranger@h", Owner: "eve@h", Allow: []string{"relay@h"}},
		{Kind: protocol.KindQueue, Name: "relay@h", Owner: "alice@h", Allow: []string{"bob@h"}, Maintainers: protocol.MaintainerList{"@crew"}, Subs: []string{"#stranger@h"}},
		{Kind: protocol.KindQueue, Name: "jobs@h", Owner: "alice@h", Allow: []string{"bob@h", "carol@h"}, Maintainers: protocol.MaintainerList{"@crew"}, Subs: []string{"#worker@h"}},
		{Kind: protocol.KindPubSub, Name: "news@h", Owner: "alice@h", Allow: []string{"*"}, Subs: []string{"#worker@h"}},
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
	if e := next(t, b, "#worker@h"); !slices.Equal(e.Roles, []string{RoleOwner}) || e.OriginalTo != "news@h" {
		t.Fatalf("the published copy arrived with roles %v through %q", e.Roles, e.OriginalTo)
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

// Queued messages restored at start keep the roles they were sent with and
// gain none: one stored before roles existed arrives with none.
func TestRestoredMessagesKeepTheirRoles(t *testing.T) {
	b := New()
	b.Restore(ports.Snapshot{
		Users:   []protocol.User{{Name: "alice@h", Status: "active"}},
		Records: []protocol.Record{userRecord("alice@h"), {Name: "#svc@h", Owner: "alice@h", Kind: protocol.KindAgent, Full: protocol.OverflowStrict}},
		Queues: []ports.Queue{{Name: "#svc@h", Messages: []protocol.Envelope{
			{ID: "new", From: "alice@h", To: "#svc@h", Body: "a", Roles: []string{RoleOwner}, At: time.Now()},
			{ID: "old", From: "alice@h", To: "#svc@h", Body: "b", At: time.Now()},
		}}},
	})
	if err := b.EstablishDaemonOwner("alice@h"); err != nil {
		t.Fatal(err)
	}
	if e := next(t, b, "#svc@h"); e.ID != "new" || !slices.Equal(e.Roles, []string{RoleOwner}) {
		t.Fatalf("the restored message arrived as %s with roles %v", e.ID, e.Roles)
	}
	if e := next(t, b, "#svc@h"); e.ID != "old" || e.Roles != nil {
		t.Fatalf("the message from before roles arrived as %s with roles %v", e.ID, e.Roles)
	}
}

// Roles pass from A to B only when A's Owner owns or maintains B; otherwise
// that forward fails loudly — refused, and in the error log. A message
// holding no roles passes as before.
func TestRolesDoNotCrossToARecordItsOwnerDoesNotAnswerFor(t *testing.T) {
	b := rolesFixture(t)
	rep := &reports{}
	b.Journal(rep)
	if _, err := b.Send(protocol.Envelope{From: "carol@h", To: "relay@h", Body: "x"}); !errors.Is(err, ErrNotAllow) {
		t.Fatalf("a maintainer's message crossed to a stranger's agent: %v", err)
	}
	if !rep.has("a message holding roles maintainer was not passed from relay@h to #stranger@h") {
		t.Fatalf("the refusal was not logged: %v", rep.lines)
	}
	if _, err := b.Send(protocol.Envelope{From: "bob@h", To: "relay@h", Body: "y"}); err != nil {
		t.Fatalf("a message holding no roles was stopped: %v", err)
	}
	if e := next(t, b, "#stranger@h"); e.Body != "y" || e.Roles != nil {
		t.Fatalf("the stranger got %q with roles %v", e.Body, e.Roles)
	}
}

// A publication checks each recipient: a copy holding roles goes only where
// the topic's Owner answers, the refused one is that recipient's logged drop,
// and a publication no recipient may take is refused.
func TestPublishedRolesDoNotCrossToARecipientItsOwnerDoesNotAnswerFor(t *testing.T) {
	b := rolesFixture(t)
	for _, r := range []protocol.Record{
		{Kind: protocol.KindPubSub, Name: "mixed@h", Owner: "alice@h", Allow: []string{"*"}, Subs: []string{"#worker@h", "#stranger@h"}},
		{Kind: protocol.KindPubSub, Name: "alone@h", Owner: "alice@h", Allow: []string{"*"}, Subs: []string{"#stranger@h"}},
	} {
		if _, err := b.Register(r); err != nil {
			t.Fatalf("%s: %v", r.Name, err)
		}
	}
	rep := &reports{}
	b.Journal(rep)
	empty := func(inbox string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		if e, err := b.Consume(ctx, inbox, "", "", false, false); err == nil {
			t.Fatalf("%s got %q with roles %v", inbox, e.Body, e.Roles)
		}
	}
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "alone@h", Body: "x"}); !errors.Is(err, ErrNotAllow) {
		t.Fatalf("an owner's publication crossed to a stranger's agent: %v", err)
	}
	if !rep.has("a message holding roles owner was not passed from alone@h to #stranger@h") {
		t.Fatalf("the refusal was not logged: %v", rep.lines)
	}
	empty("#stranger@h")
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "mixed@h", Body: "y"}); err != nil {
		t.Fatalf("a publication one recipient may take was refused: %v", err)
	}
	if e := next(t, b, "#worker@h"); e.Body != "y" || !slices.Equal(e.Roles, []string{RoleOwner}) {
		t.Fatalf("the maintained worker got %q with roles %v", e.Body, e.Roles)
	}
	empty("#stranger@h")
	if !rep.has("a copy of a publication to mixed@h was not delivered to #stranger@h") {
		t.Fatalf("the stranger's drop was not logged: %v", rep.lines)
	}
	if _, err := b.Send(protocol.Envelope{From: "bob@h", To: "alone@h", Body: "z"}); err != nil {
		t.Fatalf("a publication holding no roles was stopped: %v", err)
	}
	if e := next(t, b, "#stranger@h"); e.Body != "z" || e.Roles != nil {
		t.Fatalf("the stranger got %q with roles %v", e.Body, e.Roles)
	}
}

package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// The name says the kind: an agent's begins with "#" and nothing else's does,
// in either direction. See docs/01-identity-and-roles.md#names.
func TestOnlyAnAgentsNameBeginsWithHash(t *testing.T) {
	b := New()
	known(t, b, "alice@h")
	for _, kind := range []string{protocol.KindQueue, protocol.KindPubSub} {
		if _, err := b.Register(protocol.Record{Name: "#jobs@h", Kind: kind, Owner: "alice@h"}); !errors.Is(err, ErrKind) {
			t.Errorf("a %s registered under an agent's name: %v", kind, err)
		}
	}
	if _, err := b.Register(protocol.Record{Name: "worker@h", Kind: protocol.KindAgent, Owner: "alice@h"}); !errors.Is(err, ErrKind) {
		t.Errorf("an agent registered without its #: %v", err)
	}
}

// A User's inbox takes a direct send from an Agent exactly when that Agent's
// ACL admits the User, and never from another User.
// See docs/04-messaging.md#request-and-reply.
func TestAUserHearsOnlyFromAgentsThatAdmitIt(t *testing.T) {
	b := New()
	// Both user records admit everyone, so it is the user delivery rule and
	// not an ACL that refuses bob.
	known(t, b, "alice@h", "bob@h")
	provision(t, b, nil,
		protocol.Record{Name: "#open@h", Kind: protocol.KindAgent, Owner: "bob@h", Allow: []string{"alice@h"}},
		protocol.Record{Name: "#closed@h", Kind: protocol.KindAgent, Owner: "bob@h", Allow: []string{"bob@h"}},
	)
	if _, err := b.Send(protocol.Envelope{From: "#open@h", To: "alice@h", Body: "answer"}); err != nil {
		t.Fatalf("an agent whose ACL admits alice could not answer her: %v", err)
	}
	if _, err := b.Send(protocol.Envelope{From: "#closed@h", To: "alice@h", Body: "spam"}); !errors.Is(err, ErrNotAllow) {
		t.Errorf("an agent whose ACL does not admit alice reached her: %v", err)
	}
	if _, err := b.Send(protocol.Envelope{From: "bob@h", To: "alice@h", Body: "hi"}); !errors.Is(err, ErrNotAllow) {
		t.Errorf("one user sent to another: %v", err)
	}
	if got, _ := b.Consume(context.Background(), "alice@h", "", "", false, false); got.Body != "answer" {
		t.Fatalf("alice's inbox holds %+v, want only the admitted agent's answer", got)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if got, _ := b.Consume(ctx, "alice@h", "", "", false, false); got.Body != "" {
		t.Fatalf("a refused send was queued anyway: %+v", got)
	}
}

// A User takes no published copy, so a Deliver-To list naming one is refused
// for its kind. See docs/04-messaging.md#subscribers.
func TestADeliverToListCannotNameAUser(t *testing.T) {
	b := New()
	known(t, b, "alice@h", "bob@h")
	if _, err := b.Register(protocol.Record{Name: "news@h", Kind: protocol.KindPubSub, Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "news@h", Subs: ptr([]string{"bob@h"})}); !errors.Is(err, ErrBadName) {
		t.Fatalf("a user was put on a Deliver-To list: %v", err)
	}
	if r, _ := b.Lookup("alice@h", "news@h"); len(r.Subs) != 0 {
		t.Fatalf("the refused list stored %v", r.Subs)
	}
}

// @agent is the record's own agent principal, resolved at each check. That
// principal already manages its own record (docs/constitution.md#authority-rules),
// so the term is an alias and never widens who manages it.
func TestAgentTermResolvesToTheRecordsOwnAgent(t *testing.T) {
	b := New()
	provision(t, b, []string{"alice@h"},
		protocol.Record{Name: "#self@h", Kind: protocol.KindAgent, Owner: "alice@h", Maintainers: protocol.MaintainerList{"@agent"}},
		protocol.Record{Name: "#other@h", Kind: protocol.KindAgent, Owner: "alice@h"},
	)
	descr := "mine"
	if _, err := b.Manage("#self@h", Management{Name: "#self@h", Descr: &descr}); err != nil {
		t.Fatalf("@agent in maintainers did not admit the record's own agent: %v", err)
	}
	if _, err := b.Manage("#other@h", Management{Name: "#self@h", Descr: &descr}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("@agent admitted another agent: %v", err)
	}
}

// A group's name after its @ is a name like any other, realm optional: the one
// parser decides, and a term it refuses is refused as a group too.
// See docs/01-identity-and-roles.md#names.
func TestAGroupNameFollowsTheNameGrammar(t *testing.T) {
	for _, ok := range []string{"@ops", "@ops@h", "@1crew", "@team.a_b-c@srv1"} {
		if !groupName(ok) {
			t.Errorf("%s was refused as a group name", ok)
		}
	}
	for _, bad := range []string{"@", "@#ops", "@Ops@h ", "@-ops", "@ops@", "@a b", "@pärf"} {
		if groupName(bad) {
			t.Errorf("%s was accepted as a group name", bad)
		}
	}
}

// An unprefixed term names a User. One that names no User while an agent
// holds the name with its "#" is refused with the agent's spelling, never
// retyped: in an ACL, a Group, the Maintainers and a deliver_to list.
func TestAnUnmarkedAgentTermIsRefusedWithItsSpelling(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h")
	provision(t, b, nil,
		protocol.Record{Name: "#worker@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict},
		protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "alice@h", Full: protocol.OverflowStrict},
		protocol.Record{Name: "news@h", Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"*"}},
	)
	for what, err := range map[string]error{
		"allow": func() error {
			_, e := b.Manage("alice@h", Management{Name: "jobs@h", Allow: &[]string{"worker@h"}})
			return e
		}(),
		"maintainers": func() error {
			_, e := b.Manage("alice@h", Management{Name: "jobs@h", Maintainers: ptr(protocol.MaintainerList{"worker@h"})})
			return e
		}(),
		"deliver_to": func() error {
			_, e := b.Manage("alice@h", Management{Name: "news@h", Subs: &[]string{"worker@h"}})
			return e
		}(),
		"members": b.SetGroup("alice@h", "@crew", []string{"worker@h"}),
	} {
		if !errors.Is(err, ErrBadName) || !strings.Contains(fmt.Sprint(err), "#worker@h") {
			t.Errorf("%s took an unmarked agent term: %v", what, err)
		}
	}
	// The marked spelling is accepted.
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", Allow: &[]string{"#worker@h"}}); err != nil {
		t.Fatalf("the agent's own spelling: %v", err)
	}
}

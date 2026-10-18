package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A refused /manage tells the caller nothing about the record's lists: it is
// authorized before any delta is resolved, and a record the caller may not
// see is no such record (issues/done/2026-10/2026-10-17.md#A5).
func TestARefusedManageReadsNothingBack(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "bob@h", "carol@h", "#secret@h")
	provision(t, b, nil, protocol.Record{Name: "q@h", Kind: protocol.KindQueue, Owner: "alice@h",
		Allow: []string{"carol@h"}, Full: protocol.OverflowStrict})
	for label, change := range map[string]Management{
		"add allow":    {Name: "q@h", Add: &ListDelta{Allow: []string{"carol@h"}}},
		"remove allow": {Name: "q@h", Remove: &ListDelta{Allow: []string{"dave@h"}}},
		"add subs":     {Name: "q@h", Add: &ListDelta{Subs: []string{"#secret@h"}}},
	} {
		_, err := b.Manage("bob@h", change)
		if !errors.Is(err, ErrUnknown) {
			t.Errorf("%s by a caller who cannot see the record: %v, want no such name", label, err)
		}
		if err != nil && strings.Contains(err.Error(), "carol") {
			t.Errorf("%s: the refusal read back the allow list: %v", label, err)
		}
	}
	// A caller the record admits sees it, and is told it may not manage it.
	if _, err := b.Manage("carol@h", Management{Name: "q@h", Add: &ListDelta{Allow: []string{"carol@h"}}}); !errors.Is(err, ErrNotOwner) {
		t.Errorf("an admitted non-manager: %v, want not owner", err)
	}
}

// An Administrator edits another User's group's membership and nothing else
// of it (issues/done/2026-10/2026-10-17.md#A6).
func TestAnAdministratorEditsOnlyAGroupsMembership(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "ops@h", "bob@h")
	if err := b.SetGroup("admin@h", AdministratorsGroup, []string{"admin@h", "ops@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("alice@h", "@team", []string{"alice@h"}); err != nil {
		t.Fatal(err)
	}
	inactive, descr := "inactive", "rewritten"
	if _, err := b.Manage("ops@h", Management{Name: "@team", Status: &inactive}); err == nil {
		t.Error("an Administrator deactivated another user's group")
	}
	if _, err := b.Manage("ops@h", Management{Name: "@team", Descr: &descr}); err == nil {
		t.Error("an Administrator rewrote another user's group's description")
	}
	if _, err := b.Manage("ops@h", Management{Name: "@team", Add: &ListDelta{Allow: []string{"bob@h"}}, Descr: &descr}); err == nil {
		t.Error("an Administrator's membership edit carried a description")
	}
	if _, err := b.Manage("ops@h", Management{Name: "@team", Add: &ListDelta{Allow: []string{"bob@h"}}}); err != nil {
		t.Fatalf("an Administrator's membership edit: %v", err)
	}
	r, ok := b.Lookup("alice@h", "@team")
	if !ok || !b.live(r) || r.Descr == descr || !strings.Contains(strings.Join(r.Allow, ","), "bob@h") {
		t.Fatalf("the group after the Administrator's edits: %+v, %v", r, ok)
	}
}

// The digests of a record's private values go only to who may read those
// values: to anyone else the record admits, a digest of a short secret is the
// secret (issues/done/2026-10/2026-10-17.md#A16).
func TestPrivateDigestsGoOnlyToTheirReaders(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "bob@h", "maint@h")
	provision(t, b, nil, protocol.Record{Name: "#svc@h", Kind: protocol.KindAgent, Owner: "alice@h",
		Allow: []string{"*"}, Maintainers: []string{"maint@h"}, Full: protocol.OverflowStrict})
	if _, err := b.SetSecret("#svc@h", "alice@h", "PASSWORD=hunter2\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Configure("#svc@h", "alice@h", []byte(`{"k":1}`)); err != nil {
		t.Fatal(err)
	}
	listed := func(caller string) protocol.Record {
		for _, r := range b.List(caller, "") {
			if r.Name == "#svc@h" {
				return r
			}
		}
		t.Fatalf("%s does not see #svc@h listed", caller)
		return protocol.Record{}
	}
	for _, who := range []string{"bob@h", "admin@h"} {
		r, ok := b.Lookup(who, "#svc@h")
		if !ok {
			t.Fatalf("%s cannot see the record", who)
		}
		if r.SecretSHA != "" || r.ConfigSHA != "" {
			t.Errorf("%s, who may not read them, was given the digests on a lookup: %q %q", who, r.SecretSHA, r.ConfigSHA)
		}
		if r := listed(who); r.SecretSHA != "" || r.ConfigSHA != "" {
			t.Errorf("%s, who may not read them, was given the digests in a listing: %q %q", who, r.SecretSHA, r.ConfigSHA)
		}
	}
	descr := "managed"
	if r, err := b.Manage("admin@h", Management{Name: "#svc@h", Descr: &descr}); err != nil || r.SecretSHA != "" || r.ConfigSHA != "" {
		t.Errorf("the daemon owner's manage answer carried the digests: %q %q, %v", r.SecretSHA, r.ConfigSHA, err)
	}
	// The controls: whoever reads the values is given the digests.
	for _, who := range []string{"alice@h", "maint@h", "#svc@h"} {
		if r, ok := b.Lookup(who, "#svc@h"); !ok || r.SecretSHA == "" || r.ConfigSHA == "" {
			t.Errorf("%s, who reads the values, lost the digests: %+v, %v", who, r, ok)
		}
	}
}

// A name on a 📣 both under its own name and through a group is not told it
// left while the group still delivers to it: nothing changes, and the answer
// names the group (issues/done/2026-10/2026-10-17.md#A19).
func TestLeavingAChannelAGroupStillDeliversIsRefused(t *testing.T) {
	b := New()
	known(t, b, "alice@h")
	provision(t, b, nil,
		protocol.Record{Name: "#w@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict},
		protocol.Record{Name: "#solo@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict})
	if err := b.SetGroup("alice@h", "@crew", []string{"#w@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "news@h", Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"*"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "news@h", Subs: &[]string{"#w@h", "@crew", "#solo@h"}}); err != nil {
		t.Fatal(err)
	}
	_, err := b.Subscribe("#w@h", "news@h", false)
	if !errors.Is(err, ErrNotOwner) || !strings.Contains(err.Error(), "through @crew") {
		t.Fatalf("leaving a channel a group still delivers: %v, want refused naming @crew", err)
	}
	if r, _ := b.Lookup("alice@h", "news@h"); !strings.Contains(strings.Join(r.Subs, ","), "#w@h") {
		t.Fatalf("the refused leave took the direct entry: %v", r.Subs)
	}
	// The control: a name on it only under its own name leaves.
	if _, err := b.Subscribe("#solo@h", "news@h", false); err != nil {
		t.Fatalf("a direct recipient leaving: %v", err)
	}
}

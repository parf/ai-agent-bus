package core

import (
	"errors"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func ptr[T any](v T) *T { return &v }

func TestManagementRejectsPartialInvalidChanges(t *testing.T) {
	b := New()
	known(t, b, "owner@h")
	b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "owner@h", Descr: "original"})
	if _, err := b.Manage("owner@h", Management{Name: "#svc@h", Descr: ptr("lost"), Bound: ptr(-1)}); !errors.Is(err, ErrBound) {
		t.Fatal(err)
	}
	got, _ := b.Lookup("owner@h", "#svc@h")
	if got.Descr != "original" {
		t.Fatal("invalid change partly applied")
	}
}

func TestChannelManagersCanRemoveButStrangersCannot(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "owner@h")
	b.Register(protocol.Record{Name: "news@h", Owner: "owner@h", Allow: []string{"*"}, Kind: protocol.KindPubSub})
	known(t, b, "#subscriber@h", "stranger@h", "maint@h")
	maintainedBy(t, b, "owner@h", "#subscriber@h")
	if _, err := b.Manage("owner@h", Management{Name: "news@h", Subs: ptr([]string{"#subscriber@h"})}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.RemoveRecipient("stranger@h", "news@h", "#subscriber@h"); !errors.Is(err, ErrNotOwner) {
		t.Fatal("stranger removed subscription", err)
	}
	if err := b.SetGroup("admin@h", "@ops", []string{"maint@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("owner@h", Management{Name: "news@h", Maintainers: ptr(protocol.MaintainerList{"@ops"})}); err != nil {
		t.Fatal(err)
	}
	r, err := b.RemoveRecipient("maint@h", "news@h", "#subscriber@h")
	if err != nil || len(r.Subs) != 0 {
		t.Fatal("channel maintainer could not remove subscription", err)
	}
}

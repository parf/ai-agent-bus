package core

import (
	"encoding/json"
	"errors"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"slices"
	"strings"
	"testing"
)

func TestRecordTagsPersistWithoutStoringSyntheticLabels(t *testing.T) {
	b := personalFixture(t)
	tags := []string{"  BUILD Notes,claude,CLAUDE,no-tags "}
	got, err := b.Manage("alice@h", Management{Name: "jobs@h", Tags: &tags})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Tags, []string{"build notes", "claude"}) {
		t.Fatalf("tags=%v", got.Tags)
	}
	restored := New()
	restored.Restore(b.Snapshot())
	var ok bool
	got, ok = restored.Lookup("alice@h", "jobs@h")
	if !ok || !slices.Contains(got.Tags, "build notes") {
		t.Fatalf("restore %v %v", got.Tags, err)
	}
	empty := []string{}
	got, err = b.Manage("alice@h", Management{Name: "jobs@h", Tags: &empty})
	if err != nil || !slices.Equal(got.Tags, []string{"no-tags"}) {
		t.Fatalf("empty %v %v", got.Tags, err)
	}
	data, _ := json.Marshal(b.Snapshot())
	if strings.Contains(string(data), "no-tags") {
		t.Fatal("synthetic tag persisted")
	}
}
func TestPersonalTagCannotBypassAuthorityOrValidation(t *testing.T) {
	b := personalFixture(t)
	m := protocol.MaintainerList{"maint@h"}
	_, err := b.Manage("alice@h", Management{Name: "jobs@h", Maintainers: &m})
	if err != nil {
		t.Fatal(err)
	}
	tags := []string{"personal"}
	if _, err = b.Manage("maint@h", Management{Name: "jobs@h", Tags: &tags}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("maintainer changed classification: %v", err)
	}
	if _, err = b.Manage("alice@h", Management{Name: "jobs@h", Tags: &tags}); !errors.Is(err, ErrPersonal) {
		t.Fatalf("personal with outside maintainer accepted: %v", err)
	}
	if _, err = b.Register(protocol.Record{Name: "#tagged@h", Kind: protocol.KindAgent, Owner: "alice@h", Tags: tags, Allow: []string{"*"}}); !errors.Is(err, ErrPersonal) {
		t.Fatalf("register bypass: %v", err)
	}
	_, err = b.Register(protocol.Record{Name: "#tagged@h", Kind: protocol.KindAgent, Owner: "alice@h", Tags: tags})
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.Register(protocol.Record{Name: "#tagged@h", Kind: protocol.KindAgent, Owner: "alice@h", Tags: []string{"runner"}})
	if err != nil || !got.Personal || !slices.Equal(got.Tags, []string{"personal", "runner"}) {
		t.Fatalf("refresh lost labels %v %v", got, err)
	}
	clear := []string{}
	if _, err = b.Manage("#tagged@h", Management{Name: "#tagged@h", Tags: &clear}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("agent cleared owner personal classification: %v", err)
	}
	if _, err = b.Manage("admin@h", Management{Name: AdministratorsGroup, Tags: &tags}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("protected group made personal: %v", err)
	}
	empty := []string{}
	if _, err = b.Manage("alice@h", Management{Name: "alice@h", Tags: &empty}); !errors.Is(err, ErrPersonal) {
		t.Fatalf("user tag removed: %v", err)
	}
}
func TestTagDeltasAreAdditiveAndUseTheSameNormalization(t *testing.T) {
	b := personalFixture(t)
	_, err := b.Manage("alice@h", Management{Name: "jobs@h", AddToSet: &ListDelta{Tags: []string{" Build notes,claude "}}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.Manage("alice@h", Management{Name: "jobs@h", AddToSet: &ListDelta{Tags: []string{"runner"}}, Remove: &ListDelta{Tags: []string{"CLAUDE"}}})
	if err != nil || !slices.Equal(got.Tags, []string{"build notes", "runner"}) {
		t.Fatalf("deltas: %v %v", got.Tags, err)
	}
}

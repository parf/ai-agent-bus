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
	tags := []string{"BUILD-Notes,claude,CLAUDE,no-tags"}
	got, err := b.Manage("alice@h", Management{Name: "jobs@h", Tags: &tags})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Tags, []string{"build-notes", "claude"}) {
		t.Fatalf("tags=%v", got.Tags)
	}
	restored := New()
	restored.Restore(b.Snapshot())
	var ok bool
	got, ok = restored.Lookup("alice@h", "jobs@h")
	if !ok || !slices.Contains(got.Tags, "build-notes") {
		t.Fatalf("restore %v %v", got.Tags, err)
	}
	empty := []string{}
	got, err = b.Manage("alice@h", Management{Name: "jobs@h", Tags: &empty})
	if err != nil || len(got.Tags) != 0 {
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
	// An Owner's label other than personal, which SyncTags would not put back:
	// a refresh replacing the labels instead of adding to them loses it.
	if _, err = b.Manage("alice@h", Management{Name: "#tagged@h", AddToSet: &ListDelta{Tags: []string{"build"}}}); err != nil {
		t.Fatal(err)
	}
	got, err := b.Register(protocol.Record{Name: "#tagged@h", Kind: protocol.KindAgent, Owner: "alice@h", Tags: []string{"runner"}})
	if err != nil || !got.Personal || !slices.Equal(got.Tags, []string{"build", "personal", "runner"}) {
		t.Fatalf("refresh lost labels %v %v", got, err)
	}
	// The personal tag and the Personal field stated together must agree.
	yes := true
	ops := []string{"ops"}
	if _, err = b.Manage("alice@h", Management{Name: "#tagged@h", Tags: &ops, Personal: &yes}); !errors.Is(err, ErrPersonal) {
		t.Fatalf("tags without personal and Personal true were accepted together: %v", err)
	}
	if r, _ := b.Lookup("alice@h", "#tagged@h"); !slices.Equal(r.Tags, []string{"build", "personal", "runner"}) {
		t.Fatalf("the refused edit changed the labels to %v", r.Tags)
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
	_, err := b.Manage("alice@h", Management{Name: "jobs@h", AddToSet: &ListDelta{Tags: []string{"Build-notes,claude"}}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.Manage("alice@h", Management{Name: "jobs@h", AddToSet: &ListDelta{Tags: []string{"runner"}}, Remove: &ListDelta{Tags: []string{"CLAUDE"}}})
	if err != nil || !slices.Equal(got.Tags, []string{"build-notes", "runner"}) {
		t.Fatalf("deltas: %v %v", got.Tags, err)
	}
}

func TestBadTagsAreRefusedAndStoredTagsAreDroppedIndividually(t *testing.T) {
	b := personalFixture(t)
	bad := []string{"prod web"}
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", Tags: &bad}); err == nil || !strings.Contains(err.Error(), "prod web") {
		t.Fatalf("invalid tag accepted or unnamed: %v", err)
	}
	if _, err := b.Register(protocol.Record{Name: "#bad@h", Kind: protocol.KindAgent, Owner: "alice@h", Tags: bad}); err == nil {
		t.Fatal("invalid registration tag accepted")
	}
	for _, delta := range []*ListDelta{{Tags: bad}} {
		if _, err := b.Manage("alice@h", Management{Name: "jobs@h", AddToSet: delta}); err == nil {
			t.Fatal("invalid delta tag accepted")
		}
	}
	snapshot := b.Snapshot()
	for i := range snapshot.Records {
		if snapshot.Records[i].Name == "jobs@h" {
			snapshot.Records[i].Tags = []string{"prod-web", "bad tag", "v2"}
		}
	}
	restored := New()
	j := &reports{}
	restored.Journal(j)
	restored.Restore(snapshot)
	got, ok := restored.Lookup("alice@h", "jobs@h")
	if !ok || !slices.Equal(got.Tags, []string{"prod-web", "v2"}) {
		t.Fatalf("record or good tags lost: %v %v", ok, got.Tags)
	}
	if !j.has("bad tag") || !j.has("tag dropped") {
		t.Fatalf("bad stored tag not reported: %v", j.lines)
	}
}

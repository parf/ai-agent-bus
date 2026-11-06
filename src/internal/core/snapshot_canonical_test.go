package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func TestRestoreCanonicalStateOrRefuseBeforeMutation(t *testing.T) {
	for name, change := range map[string]func(*ports.Snapshot){
		"owner marker":      func(s *ports.Snapshot) { s.OwnerEstablished = false },
		"accounts marker":   func(s *ports.Snapshot) { s.AccountsEstablished = false },
		"record ID":         func(s *ports.Snapshot) { s.Records[0].ID = 0 },
		"owner ID":          func(s *ports.Snapshot) { s.Records[0].OwnerID = 0 },
		"user ID":           func(s *ports.Snapshot) { s.Users[0].ID = 0 },
		"stray address":     func(s *ports.Snapshot) { s.Records[0].Addr = "old-runner" },
		"empty sentinel":    func(s *ports.Snapshot) { s.Records[0].Tags = []string{"no-tags"} },
		"tag normalization": func(s *ports.Snapshot) { s.Records[0].Tags = []string{"Build,ops"} },
		"bad tag":           func(s *ports.Snapshot) { s.Records[0].Tags = []string{"bad tag"} },
		"unknown kind":      func(s *ports.Snapshot) { s.Records[0].Kind = "retired" },
	} {
		t.Run(name, func(t *testing.T) {
			source := ownerFixture(t)
			saved := source.Snapshot()
			target := New()
			target.SetDaemonOwner("existing@h")
			if err := ValidateSnapshot(saved); err != nil {
				t.Fatalf("control snapshot: %v", err)
			}
			change(&saved)
			assertRestoreRefused(t, target, saved)
		})
	}
}

func TestFreshDaemonInitializationIsAtomic(t *testing.T) {
	st := memory.NewState()
	b := New()
	if err := b.Persistence(st); err != nil {
		t.Fatal(err)
	}
	st.Err = errors.New("write refused")
	if err := b.EstablishDaemon("owner@h", map[string]string{"local": "owner@h"}); err == nil {
		t.Fatal("failed commit accepted")
	}
	if b.DaemonOwner() != "" || len(b.Snapshot().Records) != 0 {
		t.Fatal("failed initialization changed state")
	}
	st.Err = nil
	if err := b.EstablishDaemon("owner@h", map[string]string{"local": "missing@h"}); err == nil {
		t.Fatal("unknown initial mapping accepted")
	}
	if b.DaemonOwner() != "" {
		t.Fatal("invalid mapping partially established owner")
	}
	if err := b.EstablishDaemon("owner@h", map[string]string{"local": "owner@h"}); err != nil {
		t.Fatal(err)
	}
	saved, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSnapshot(saved); err != nil {
		t.Fatalf("initial commit is not restartable: %v", err)
	}
	restored := New()
	if err := restored.Restore(saved); err != nil {
		t.Fatal(err)
	}
	if err := restored.EstablishDaemon("contrary@h", map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if restored.DaemonOwner() != "owner@h" {
		t.Fatal("restart replaced owner")
	}
}

func TestCreatedUserRecordCarriesItsOwnerID(t *testing.T) {
	b := New()
	b.SetDaemonOwner("owner@h")
	if _, err := b.SetUser("owner@h", protocol.User{Name: "created@h"}, true); err != nil {
		t.Fatal(err)
	}
	s := b.Snapshot()
	if err := ValidateSnapshot(s); err != nil {
		t.Fatal(err)
	}
	for _, r := range s.Records {
		if r.Name == "created@h" && r.OwnerID == 0 {
			t.Fatal("new user record omitted owner ID")
		}
	}
}

func TestStoredRecordErrorNamesTheRefusedField(t *testing.T) {
	b := ownerFixture(t)
	s := b.Snapshot()
	for i := range s.Records {
		if s.Records[i].Name == "#svc@h" {
			s.Records[i].Addr = "old-runner"
		}
	}
	if err := New().Restore(s); err == nil || !strings.Contains(err.Error(), "#svc@h") || !strings.Contains(err.Error(), "address") {
		t.Fatalf("unclear refusal: %v", err)
	}
}

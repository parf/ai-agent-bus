package core

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

type credentialsOnlyPreflightStore struct {
	*memory.State
	calls      int
	scanned    []uint32
	principals []string
}

func (s *credentialsOnlyPreflightStore) ExportState(ids []uint32, principals []string) (ports.TransferState, error) {
	s.calls++
	s.principals = append([]string(nil), principals...)
	if len(ids) != 0 {
		s.scanned = append(s.scanned, ids...)
		return ports.TransferState{}, fmt.Errorf("preflight attempted a KV scan")
	}
	return s.State.ExportState(ids, principals)
}

func TestImportPreflightReadsCredentialsWithoutScanningKV(t *testing.T) {
	for _, dry := range []bool{false, true} {
		t.Run(fmt.Sprintf("dry-run=%v", dry), func(t *testing.T) {
			_, _, backup, _, current := transferSource(t)
			store := &credentialsOnlyPreflightStore{State: memory.NewState()}
			b := New()
			b.Persistence(store)
			if err := b.EstablishDaemonOwner("owner@source"); err != nil {
				t.Fatal(err)
			}
			tokens, err := auth.Load(store.Tokens())
			if err != nil {
				t.Fatal(err)
			}
			b.BindCredentials(tokens)
			const existing = "#existing@source"
			if _, err = b.Register(protocol.Record{Name: existing, Kind: protocol.KindAgent, Owner: "owner@source"}); err != nil {
				t.Fatal(err)
			}
			oldToken, err := tokens.Issue(existing)
			if err != nil {
				t.Fatal(err)
			}
			if err = b.KVSet("owner@source", existing, ports.KVString, "private", ports.KVValue{Bytes: []byte("untouched")}, KVSet); err != nil {
				t.Fatal(err)
			}
			report, err := b.ImportRecords("owner@source", backup, ImportChoices{KeepExisting: []string{"all"}, DryRun: dry})
			if len(store.scanned) != 0 {
				t.Errorf("preflight requested KV record IDs: %v", store.scanned)
			}
			if err != nil {
				t.Fatalf("credential-only preflight refused import: %v", err)
			}
			if store.calls != 1 {
				t.Errorf("preflight reads = %d, want 1", store.calls)
			}
			sort.Strings(store.principals)
			if !reflect.DeepEqual(store.principals, []string{existing, "owner@source"}) {
				t.Errorf("credential principals = %v", store.principals)
			}
			if report.DryRun != dry {
				t.Errorf("dry run = %v, want %v", report.DryRun, dry)
			}
			if got, err := b.KVGet("owner@source", existing, ports.KVString, "private"); err != nil || string(got.Bytes) != "untouched" {
				t.Fatalf("existing KV changed: %+v, %v", got, err)
			}
			if who, _, ok := tokens.Credential(oldToken); !ok || who != existing {
				t.Fatal("existing credential changed")
			}
			_, _, ok := tokens.Credential(current)
			if ok == dry {
				t.Errorf("imported credential present = %v for dry run %v", ok, dry)
			}
		})
	}
}

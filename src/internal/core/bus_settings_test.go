package core

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// audits is a journal that keeps audit entries as well as reports.
type audits struct {
	reports
	amu     sync.Mutex
	entries []ports.AuditEntry
}

func (a *audits) Audit(e ports.AuditEntry) {
	a.amu.Lock()
	a.entries = append(a.entries, e)
	a.amu.Unlock()
}

func busSetupFixture(t *testing.T) (*Bus, *audits) {
	t.Helper()
	b := ownerFixture(t)
	j := &audits{}
	b.Journal(j)
	if err := b.EstablishDaemonAgent("Node.Example."); err != nil {
		t.Fatal(err)
	}
	if len(j.entries) != 1 || j.entries[0].Actor != "(daemon start)" || j.entries[0].Result != "ok: (none) -> node.example" {
		t.Fatalf("first-start realm not audited: %+v", j.entries)
	}
	j.entries = nil
	return b, j
}

func TestBusSetupDefaultsToTheHostAndOnlyTheDaemonOwnerWritesIt(t *testing.T) {
	b, j := busSetupFixture(t)
	if got := b.Realm(); got != "node.example" {
		t.Fatalf("first-start realm: %q", got)
	}
	if got := b.Status().Realm; got != "node.example" {
		t.Fatalf("status realm: %q", got)
	}
	// An Administrator, an Agent the daemon Owner owns, and an ordinary User
	// all fail; none of them is the daemon Owner.
	if err := b.SetGroup("owner@h", AdministratorsGroup, []string{"owner@h", "alice@h"}); err != nil {
		t.Fatal(err)
	}
	if !b.IsAdministrator("alice@h") {
		t.Fatal("fixture: alice is no Administrator")
	}
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#owned@h", Owner: "owner@h"}); err != nil {
		t.Fatal(err)
	}
	for _, who := range []string{"alice@h", "#owned@h", "next@h"} {
		if _, err := b.Configure(DaemonAgent, who, []byte(`{"realm":"stolen"}`)); !errors.Is(err, ErrNotOwner) {
			t.Fatalf("%s wrote the bus setup: %v", who, err)
		}
	}
	if b.Realm() != "node.example" || len(j.entries) != 0 {
		t.Fatalf("a refused write changed or audited the realm: %q %+v", b.Realm(), j.entries)
	}
	if _, err := b.Configure(DaemonAgent, "owner@h", []byte(`{"realm":"short","alert_entries":75}`)); err != nil {
		t.Fatal(err)
	}
	if b.Realm() != "short" || b.Status().Realm != "short" {
		t.Fatalf("save did not apply at once: %q %q", b.Realm(), b.Status().Realm)
	}
	if len(j.entries) != 1 || j.entries[0].Actor != "owner@h" || j.entries[0].Target != DaemonAgent || j.entries[0].Result != "ok: node.example -> short" {
		t.Fatalf("audit: %+v", j.entries)
	}
	// The same realm again is no realm change to audit.
	if _, err := b.Configure(DaemonAgent, "owner@h", []byte(`{"realm":"short"}`)); err != nil || len(j.entries) != 1 {
		t.Fatalf("unchanged realm audited: %v %+v", err, j.entries)
	}
	// Existing names keep their old spelling and meaning.
	if r, ok := b.Lookup("alice@h", "#svc@h"); !ok || r.Name != "#svc@h" {
		t.Fatalf("existing name changed: %+v", r)
	}
	if cfg, err := b.Config(DaemonAgent, "owner@h"); err != nil || string(cfg) != `{"realm":"short"}` {
		t.Fatalf("owner read: %s %v", cfg, err)
	}
	if r, ok := b.Lookup("owner@h", DaemonAgent); !ok || r.ConfigSHA == "" {
		t.Fatalf("owner sees no digest of the bus setup: %+v", r)
	}
	for _, who := range []string{"alice@h", "#owned@h"} {
		if r, ok := b.Lookup(who, DaemonAgent); !ok || r.ConfigSHA != "" {
			t.Fatalf("%s sees the bus setup digest: %+v", who, r)
		}
		if _, err := b.Config(DaemonAgent, who); !errors.Is(err, ErrPrivate) {
			t.Fatalf("%s read the bus setup: %v", who, err)
		}
	}
}

func TestBusSetupValidationNeverTrimsOrTruncates(t *testing.T) {
	b, j := busSetupFixture(t)
	for _, cfg := range []string{
		`[]`, `"short"`, `{}`, `{"realm":""}`, `{"realm":"Short"}`, `{"realm":" short"}`, `{"realm":"short "}`,
		`{"realm":"-short"}`, `{"realm":"a/b"}`, `{"realm":"a@b"}`, `{"realm":"` + strings.Repeat("a", 63) + `"}`,
		`{"realm":1}`, `{"realm":"short","colour":"red"}`, `{"realm":"short","notice_entries":0}`,
		`{"realm":"short","notice_entries":10001}`, `{"realm":"short","notice_days":1.5}`,
		`{"realm":"short","alert_days":"30"}`, `{"realm":"short","warning_days":3651}`,
		`{"realm":"short","realm":"other"}`, `{"realm":"short","notice_days":1,"notice_days":2}`, `{"realm":"short"} {}`,
	} {
		if _, err := b.Configure(DaemonAgent, "owner@h", []byte(cfg)); !errors.Is(err, ErrConfig) {
			t.Fatalf("accepted %s: %v", cfg, err)
		}
	}
	if b.Realm() != "node.example" || len(j.entries) != 0 {
		t.Fatalf("a refused save changed the realm: %q", b.Realm())
	}
	longest := strings.Repeat("a", 62)
	if _, err := b.Configure(DaemonAgent, "owner@h", []byte(`{"realm":"`+longest+`","notice_entries":10000,"warning_days":3650}`)); err != nil || b.Realm() != longest {
		t.Fatalf("bounds refused: %v %q", err, b.Realm())
	}
}

func TestBusSetupSurvivesRestoreAndTheHostNeverOverwritesIt(t *testing.T) {
	b, _ := busSetupFixture(t)
	if _, err := b.Configure(DaemonAgent, "owner@h", []byte(`{"realm":"short"}`)); err != nil {
		t.Fatal(err)
	}
	restored := New()
	restored.Restore(b.Snapshot())
	if err := restored.EstablishDaemonOwner("owner@h"); err != nil {
		t.Fatal(err)
	}
	j := &audits{}
	restored.Journal(j)
	if err := restored.EstablishDaemonAgent("other.host"); err != nil {
		t.Fatal(err)
	}
	if restored.Realm() != "short" || len(j.entries) != 0 {
		t.Fatalf("restart lost or overwrote the realm: %q", restored.Realm())
	}
	// A daemon Owner transfer keeps the setup and moves the right to write it.
	if _, err := restored.TransferDaemonOwner("owner@h", "next@h"); err != nil {
		t.Fatal(err)
	}
	if restored.Realm() != "short" {
		t.Fatalf("owner transfer lost the realm: %q", restored.Realm())
	}
	if _, err := restored.Configure(DaemonAgent, "owner@h", []byte(`{"realm":"old-owner"}`)); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("former owner wrote: %v", err)
	}
	if _, err := restored.Configure(DaemonAgent, "next@h", []byte(`{"realm":"next"}`)); err != nil || restored.Realm() != "next" {
		t.Fatalf("new owner refused: %v", err)
	}
}

func TestBusSetupStoredInvalidIsIgnoredAtRestore(t *testing.T) {
	b, _ := busSetupFixture(t)
	snap := b.Snapshot()
	for i, r := range snap.Records {
		if r.Name == DaemonAgent {
			r.Config = []byte(`{"realm":"Bad Realm"}`)
			snap.Records[i] = r
		}
	}
	journal := &reports{}
	restored := New()
	restored.Journal(journal)
	restored.Restore(snap)
	if !journal.has("stored bus setup is invalid") {
		t.Fatal("invalid stored setup was not reported")
	}
	if err := restored.EstablishDaemonOwner("owner@h"); err != nil {
		t.Fatal(err)
	}
	if err := restored.EstablishDaemonAgent("fallback.host"); err != nil {
		t.Fatal(err)
	}
	if restored.Realm() != "fallback.host" {
		t.Fatalf("invalid stored realm survived: %q", restored.Realm())
	}
}

func TestBusSetupHostThatIsNoRealmSavesNone(t *testing.T) {
	b := ownerFixture(t)
	j := &audits{}
	b.Journal(j)
	if err := b.EstablishDaemonAgent("bad host!"); err != nil {
		t.Fatal(err)
	}
	if b.Realm() != "" || b.Status().Realm != "" || len(j.entries) != 0 {
		t.Fatalf("invented a realm: %q", b.Realm())
	}
}

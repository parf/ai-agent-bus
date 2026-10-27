package core

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func transferBus(t *testing.T, owner string) (*Bus, *auth.Tokens, *memory.State) {
	t.Helper()
	s := memory.NewState()
	b := New()
	b.Persistence(s)
	if e := b.EstablishDaemonOwner(owner); e != nil {
		t.Fatal(e)
	}
	tokens, e := auth.Load(s.Tokens(), owner)
	if e != nil {
		t.Fatal(e)
	}
	b.BindCredentials(tokens)
	return b, tokens, s
}
func transferSource(t *testing.T) (*Bus, *auth.Tokens, RecordBackup, string, string) {
	t.Helper()
	b, tokens, _ := transferBus(t, "owner@source")
	if _, e := b.SetUser("owner@source", protocol.User{Name: "peer@source", Status: protocol.StatusActive}, true); e != nil {
		t.Fatal(e)
	}
	if e := b.SetGroup("owner@source", "@team@source", []string{"peer@source"}); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"#runner@source", "#standalone@source"} {
		if _, e := b.Register(protocol.Record{Name: name, Kind: protocol.KindAgent, Owner: "owner@source", Allow: []string{"peer@source(operate)"}, Maintainers: []string{"owner@source"}, Roles: []string{"operate"}, Tags: []string{"ops"}}); e != nil {
			t.Fatal(e)
		}
		if _, e := b.Configure(name, "owner@source", json.RawMessage(`{"mode":"fixture"}`)); e != nil {
			t.Fatal(e)
		}
		if _, e := b.SetSecret(name, "owner@source", "TOKEN=fixture-private-value"); e != nil {
			t.Fatal(e)
		}
		if e := b.KVSet("owner@source", name, ports.KVString, "bytes", ports.KVValue{Bytes: []byte{0, 1, 255}}, KVSet); e != nil {
			t.Fatal(e)
		}
		if e := b.KVSet("owner@source", name, ports.KVInt, "count", ports.KVValue{Int: 9007199254740993}, KVSet); e != nil {
			t.Fatal(e)
		}
		if e := b.KVSet("owner@source", name, ports.KVJSON, "json", ports.KVValue{Bytes: []byte(`{"n":9007199254740993}`)}, KVSet); e != nil {
			t.Fatal(e)
		}
	}
	first, e := tokens.Issue("#runner@source")
	if e != nil {
		t.Fatal(e)
	}
	second, e := tokens.Rotate("#runner@source")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = tokens.Issue("#standalone@source"); e != nil {
		t.Fatal(e)
	}
	if _, e = b.Send(protocol.Envelope{From: "owner@source", To: "#runner@source", Body: "queued-private-not-in-backup"}); e != nil {
		t.Fatal(e)
	}
	data, _, e := b.ExportRecords("owner@source", RecordSelection{Include: []string{"tag:ops"}})
	if e != nil {
		t.Fatal(e)
	}
	return b, tokens, data, first, second
}
func TestRecordImportRestoresPrivateKVAndOldCredentialPairs(t *testing.T) {
	_, source, data, previous, current := transferSource(t)
	target, tokens, _ := transferBus(t, "owner@source")
	report, e := target.ImportRecords("owner@source", data, ImportChoices{KeepExisting: []string{"all"}})
	if e != nil {
		t.Fatal(e)
	}
	if len(report.Imported) < 2 {
		t.Fatal("fresh agents were incorrectly kept instead of imported")
	}
	for _, value := range []string{previous, current} {
		who, pair, ok := tokens.Credential(value)
		if !ok || who != "#runner@source" || target.CheckCredential(who, pair) != nil {
			t.Fatal("old credential did not survive import with destination IDs")
		}
	}
	own, e := source.Issue("#standalone@source")
	if e != nil {
		t.Fatal(e)
	}
	who, pair, ok := tokens.Credential(own)
	if !ok || target.CheckCredential(who, pair) != nil {
		t.Fatal("standalone token not retained")
	}
	for _, r := range data.Records {
		if !strings.HasPrefix(r.Name, "#") {
			continue
		}
		cfg, e := target.Config(r.Name, "owner@source")
		if e != nil || string(cfg) != string(r.Config) {
			t.Fatal("configuration changed")
		}
		secret, e := target.Secret(r.Name, "owner@source")
		if e != nil || secret != r.Secret {
			t.Fatal("secret changed")
		}
		got, _ := target.Lookup("owner@source", r.Name)
		if !reflect.DeepEqual(got.Allow, r.Allow) || !reflect.DeepEqual([]string(got.Maintainers), []string(r.Maintainers)) {
			t.Fatal("authority changed")
		}
	}
	for _, v := range data.KV {
		got, e := target.KVGet("owner@source", v.Record, v.Kind, v.Name)
		if e != nil || !reflect.DeepEqual(got, v.Value) {
			t.Fatal("typed KV changed")
		}
	}
	raw, _ := json.Marshal(data)
	if strings.Contains(string(raw), "queued-private-not-in-backup") || strings.Contains(string(raw), `"queues"`) || strings.Contains(string(raw), `"locks"`) {
		t.Fatal("ephemeral state exported")
	}
	if got := target.StatusFor("owner@source").Queued; got != 0 {
		t.Fatal("queued messages were imported")
	}
}
func TestRecordImportPreflightDryRunRollbackAndConflicts(t *testing.T) {
	_, _, data, _, _ := transferSource(t)
	target, tokens, store := transferBus(t, "owner@source")
	snapshot := func() ports.Snapshot {
		s := target.Snapshot()
		s.At = time.Time{}
		sort.Slice(s.Records, func(i, j int) bool { return s.Records[i].Name < s.Records[j].Name })
		sort.Slice(s.Users, func(i, j int) bool { return s.Users[i].Name < s.Users[j].Name })
		return s
	}
	before := snapshot()
	pairs := tokens.Pairs()
	commits := store.Commits
	report, e := target.ImportRecords("owner@source", data, ImportChoices{})
	if e == nil || len(report.Conflicts) < 2 {
		t.Fatal("not every existing user/group conflict was reported")
	}
	if !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("conflict mutated registry")
	}
	report, e = target.ImportRecords("owner@source", data, ImportChoices{KeepExisting: []string{"all"}, DryRun: true})
	if e != nil || !report.DryRun {
		t.Fatal(e)
	}
	if _, ok := target.Lookup("owner@source", "#runner@source"); ok {
		t.Fatal("dry run changed records")
	}
	store.Err = errors.New("injected private-store-sentinel")
	_, e = target.ImportRecords("owner@source", data, ImportChoices{KeepExisting: []string{"all"}})
	if e == nil || strings.Contains(e.Error(), "private-store-sentinel") {
		t.Fatal("failure did not safely report rollback")
	}
	if _, ok := target.Lookup("owner@source", "#runner@source"); ok || store.Commits != commits || !reflect.DeepEqual(pairs, tokens.Pairs()) {
		t.Fatal("failed commit partially changed state")
	}
	store.Err = nil
	if _, e = target.ImportRecords("owner@source", data, ImportChoices{KeepExisting: []string{"all"}}); e != nil {
		t.Fatal(e)
	}
	old, e := tokens.Issue("#runner@source")
	if e != nil {
		t.Fatal(e)
	}
	changed := data
	changed.Records = append([]protocol.Record(nil), data.Records...)
	for i := range changed.Records {
		if changed.Records[i].Name == "#runner@source" {
			changed.Records[i].Descr = "replacement"
		}
	}
	report, e = target.ImportRecords("owner@source", changed, ImportChoices{Replace: []string{"tag:ops"}, KeepExisting: []string{"name:owner@source", "name:peer@source", "name:@administrators", "name:@team@source"}})
	if e != nil {
		t.Fatal(e)
	}
	now, _ := tokens.Issue("#runner@source")
	if now != old || len(report.TokensKept) < 2 {
		t.Fatal("existing tokens silently changed")
	}
}
func TestRecordSelectorsAuthorityAndTypedRealmRewrite(t *testing.T) {
	b, _, data, _, token := transferSource(t)
	if _, _, e := b.ExportRecords("peer@source", RecordSelection{}); e == nil {
		t.Fatal("non-Owner exported private records")
	}
	if _, e := b.ImportRecords("peer@source", data, ImportChoices{KeepExisting: []string{"all"}}); e == nil {
		t.Fatal("non-Owner imported records")
	}
	if _, _, e := b.ExportRecords("owner@source", RecordSelection{Include: []string{"missing"}}); e == nil {
		t.Fatal("unmatched selector accepted")
	}
	if _, e := b.Register(protocol.Record{Name: "ops", Kind: protocol.KindAgent, Owner: "owner@source"}); e == nil {
		t.Fatal("fixture must use canonical agent name")
	}
	if _, e := b.Register(protocol.Record{Name: "ops", Kind: protocol.KindQueue, Owner: "owner@source"}); e != nil {
		t.Fatal(e)
	}
	if _, _, e := b.ExportRecords("owner@source", RecordSelection{Include: []string{"ops"}}); e == nil {
		t.Fatal("ambiguous selector guessed")
	}
	selected, names, e := b.ExportRecords("owner@source", RecordSelection{Include: []string{"tag:ops"}, Exclude: []string{"name:#standalone@source"}})
	if e != nil {
		t.Fatal(e)
	}
	for _, n := range names {
		if n == "#standalone@source" {
			t.Fatal("exclude did not win")
		}
	}
	_ = selected
	target, tokens, _ := transferBus(t, "owner@destination")
	report, e := target.ImportRecords("owner@destination", data, ImportChoices{KeepExisting: []string{"all"}, RenameRealm: map[string]string{"source": "destination"}})
	if e != nil {
		t.Fatal(e)
	}
	who, pair, ok := tokens.Credential(token)
	if !ok || who != "#runner@destination" || target.CheckCredential(who, pair) != nil {
		t.Fatal("realm rewrite did not bind old token to destination identity")
	}
	got, _ := target.Lookup("owner@destination", "#runner@destination")
	if got.Owner != "owner@destination" || got.Allow[0] != "peer@destination(operate)" || report.Renamed["#runner@source"] != "#runner@destination" {
		t.Fatal("typed references not rewritten")
	}
	config, _ := target.Config("#runner@destination", "owner@destination")
	if string(config) != `{"mode":"fixture"}` {
		t.Fatal("opaque config was rewritten")
	}
}

func TestRecordSelectorUnionExplicitPrefixesAndAllConflictChoices(t *testing.T) {
	index := map[string]protocol.Record{
		"ops":  {Name: "ops", Kind: protocol.KindQueue},
		"#a@h": {Name: "#a@h", Tags: []string{"ops"}},
		"#b@h": {Name: "#b@h", Tags: []string{"messaging"}},
	}
	if _, e := resolveSelectors([]string{"ops"}, index, false); e == nil {
		t.Fatal("ambiguous bare selector accepted")
	}
	if _, e := resolveSelectors([]string{"nothing"}, index, false); e == nil {
		t.Fatal("empty match accepted")
	}
	selected, e := resolveSelectors([]string{"name:ops,tag:ops", "tag:messaging", "name:#a@h"}, index, false)
	if e != nil || len(selected) != 3 {
		t.Fatal("commas/repeats were not unioned")
	}
	all, e := resolveSelectors([]string{"all"}, index, true)
	if e != nil || len(all) != 3 {
		t.Fatal("all conflict choice")
	}
	if _, e = resolveSelectors([]string{"all"}, index, false); e == nil {
		t.Fatal("all accepted for export")
	}
}

func TestRecordImportNeverAcceptsDerivedAuthorityClaims(t *testing.T) {
	_, _, data, _, _ := transferSource(t)
	for i := range data.Users {
		if data.Users[i].Name == "peer@source" {
			data.Users[i].DaemonOwner = true
			data.Users[i].Administrator = true
			data.Users[i].CanEdit = true
		}
	}
	target, _, _ := transferBus(t, "owner@source")
	if _, e := target.ImportRecords("owner@source", data, ImportChoices{KeepExisting: []string{"all"}}); e != nil {
		t.Fatal(e)
	}
	for _, u := range target.Snapshot().Users {
		if u.Name == "peer@source" && (u.DaemonOwner || u.Administrator || u.CanEdit) {
			t.Fatal("import retained derived authority claim")
		}
	}
	if target.DaemonOwner() != "owner@source" || target.IsAdministrator("peer@source") {
		t.Fatal("archive changed daemon authority")
	}
}

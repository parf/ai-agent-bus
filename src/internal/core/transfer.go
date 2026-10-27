package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

type RecordKV struct {
	Record string        `json:"record"`
	Kind   ports.KVKind  `json:"kind"`
	Name   string        `json:"name"`
	Value  ports.KVValue `json:"value"`
}
type RecordBackup struct {
	Format      int                `json:"format"`
	Records     []protocol.Record  `json:"records"`
	Users       []protocol.User    `json:"users"`
	KV          []RecordKV         `json:"kv"`
	Credentials []ports.Credential `json:"credentials"`
}
type ImportReport struct {
	Imported   []string          `json:"imported"`
	Kept       []string          `json:"kept"`
	TokensKept []string          `json:"archived_tokens_not_retained"`
	Renamed    map[string]string `json:"renamed"`
	Conflicts  []string          `json:"conflicts,omitempty"`
	DryRun     bool              `json:"dry_run"`
}

func cleanBackupUser(u protocol.User) protocol.User {
	u.Kind = ""
	u.CanRemove = false
	u.Administrator = false
	u.DaemonOwner = false
	u.CanEdit = false
	u.CanSetEmail = false
	u.CanActivate = false
	u.Groups = nil
	u.Records = nil
	u.Services = nil
	return u
}

// ExportRecords reads private values only after the node-wide Owner check.
func (b *Bus) ExportRecords(caller string, selection RecordSelection) (RecordBackup, []string, error) {
	b.mu.Lock()
	defer b.unlock()
	var out RecordBackup
	if caller != b.admin {
		return out, nil, ErrNotOwner
	}
	store, ok := b.store.(ports.TransferStore)
	if !ok {
		return out, nil, fmt.Errorf("record backups require a transactional store")
	}
	index := map[string]protocol.Record{}
	for n, r := range b.records {
		if n != DaemonAgent {
			index[n] = r
		}
	}
	for n := range b.users {
		if _, ok := index[n]; !ok {
			index[n] = protocol.Record{Name: n, Kind: protocol.KindUser}
		}
	}
	chosen, e := resolveSelectors(selection.Include, index, false)
	if e != nil {
		return out, nil, e
	}
	excluded, e := resolveSelectors(selection.Exclude, index, false)
	if e != nil {
		return out, nil, e
	}
	if len(selection.Include) == 0 {
		for n := range index {
			chosen[n] = true
		}
	}
	// People and groups carry authority references and are included by default.
	for n, r := range index {
		if r.Kind == protocol.KindUser || r.Kind == protocol.KindGroup {
			chosen[n] = true
		}
	}
	for n := range excluded {
		delete(chosen, n)
	}
	out.Format = 1
	var ids []uint32
	var principals []string
	byID := map[uint32]string{}
	names := sortedNames(chosen)
	for _, n := range names {
		if u, ok := b.users[n]; ok {
			out.Users = append(out.Users, cleanBackupUser(u))
			principals = append(principals, n)
		}
		if r, ok := b.records[n]; ok {
			clearLiveRecord(&r)
			r.ConfigSHA = ""
			r.SecretSHA = ""
			out.Records = append(out.Records, r)
			ids = append(ids, r.ID)
			byID[r.ID] = n
			if r.Kind == protocol.KindAgent {
				principals = append(principals, n)
			}
		}
	}
	state, e := store.ExportState(ids, principals)
	if e != nil {
		return out, nil, fmt.Errorf("cannot read private record state")
	}
	out.Credentials = state.Credentials
	for _, v := range state.KV {
		out.KV = append(out.KV, RecordKV{Record: byID[v.RecordID], Kind: v.Kind, Name: v.Name, Value: v.Value})
	}
	sort.Slice(out.KV, func(i, j int) bool {
		a, c := out.KV[i], out.KV[j]
		if a.Record != c.Record {
			return a.Record < c.Record
		}
		if a.Kind != c.Kind {
			return a.Kind < c.Kind
		}
		return a.Name < c.Name
	})
	return out, names, nil
}

// Realm changes apply only to typed identity references. Private content is
// opaque; a realm-looking string in Configuration or KV is never rewritten.
func rewriteBackup(data RecordBackup, realms map[string]string) (RecordBackup, map[string]string, error) {
	raw, e := json.Marshal(data)
	if e != nil {
		return data, nil, fmt.Errorf("invalid record backup")
	}
	var out RecordBackup
	if json.Unmarshal(raw, &out) != nil {
		return data, nil, fmt.Errorf("invalid record backup")
	}
	for from, to := range realms {
		for _, r := range []string{from, to} {
			if r != "" {
				n, e := protocol.ParseName("fixture@" + r)
				if e != nil || n.Realm != r {
					return data, nil, fmt.Errorf("invalid realm mapping")
				}
			}
		}
	}
	renamed := map[string]string{}
	rewrite := func(value string) (string, error) {
		next, e := rewriteName(value, realms)
		if e == nil && next != value {
			renamed[value] = next
		}
		return next, e
	}
	for i := range out.Users {
		out.Users[i].Name, e = rewrite(out.Users[i].Name)
		if e != nil {
			return data, nil, e
		}
	}
	for i := range out.Records {
		r := &out.Records[i]
		r.Name, e = rewrite(r.Name)
		if e != nil {
			return data, nil, e
		}
		r.Owner, e = rewrite(r.Owner)
		if e != nil {
			return data, nil, e
		}
		for j, v := range r.Allow {
			r.Allow[j], e = rewriteTerm(v, realms)
			if e != nil {
				return data, nil, e
			}
		}
		for j, v := range r.Maintainers {
			r.Maintainers[j], e = rewrite(v)
			if e != nil {
				return data, nil, e
			}
		}
		for j, v := range r.Subs {
			r.Subs[j], e = rewrite(v)
			if e != nil {
				return data, nil, e
			}
		}
		if r.Resource != nil {
			r.Resource.Source, e = rewrite(r.Resource.Source)
			if e != nil {
				return data, nil, e
			}
		}
	}
	for i := range out.KV {
		out.KV[i].Record, e = rewrite(out.KV[i].Record)
		if e != nil {
			return data, nil, e
		}
	}
	for i := range out.Credentials {
		out.Credentials[i].Name, e = rewrite(out.Credentials[i].Name)
		if e != nil {
			return data, nil, e
		}
	}
	return out, renamed, nil
}

func (b *Bus) ImportRecords(caller string, data RecordBackup, choices ImportChoices) (ImportReport, error) {
	b.mu.Lock()
	defer b.unlock()
	report := ImportReport{DryRun: choices.DryRun}
	if caller != b.admin {
		return report, ErrNotOwner
	}
	if _, ok := b.store.(ports.TransferStore); !ok {
		return report, fmt.Errorf("record import requires a transactional store")
	}
	if data.Format != 1 {
		return report, fmt.Errorf("unsupported record backup version")
	}
	var e error
	data, report.Renamed, e = rewriteBackup(data, choices.RenameRealm)
	if e != nil {
		return report, e
	}
	records := map[string]protocol.Record{}
	users := map[string]protocol.User{}
	for _, u := range data.Users {
		n, e := canon(u.Name)
		if e != nil || n != u.Name || !plainName(n) || u.Status != protocol.StatusActive && u.Status != protocol.StatusInactive {
			return report, fmt.Errorf("invalid user in backup")
		}
		if _, ok := users[n]; ok {
			return report, fmt.Errorf("duplicate user in backup")
		}
		users[n] = cleanBackupUser(u)
	}
	for _, r := range data.Records {
		n, e := canon(r.Name)
		if e != nil || n != r.Name || n == DaemonAgent || reservedTerm(n) {
			return report, fmt.Errorf("invalid or reserved record in backup")
		}
		if _, ok := records[n]; ok {
			return report, fmt.Errorf("duplicate record in backup")
		}
		if r.Status != "" && r.Status != protocol.StatusActive && r.Status != protocol.StatusInactive {
			return report, fmt.Errorf("invalid record status: %s", n)
		}
		if _, err := protocol.CompileACL(r.Allow); err != nil {
			return report, fmt.Errorf("invalid ACL: %s", n)
		}
		clearLiveRecord(&r)
		r.ConfigSHA = ""
		r.SecretSHA = ""
		r.ParsedAllow = nil
		if validateKind(r) != nil || validRoles(r) != nil || r.Bound < 0 {
			return report, fmt.Errorf("invalid record shape: %s", n)
		}
		if r.Full != "" && r.Full != protocol.OverflowStrict && r.Full != protocol.OverflowRing {
			return report, fmt.Errorf("invalid overflow: %s", n)
		}
		if r.TTL != "" {
			d, e := time.ParseDuration(r.TTL)
			if e != nil || d <= 0 {
				return report, fmt.Errorf("invalid retention: %s", n)
			}
		}
		if r.Secret != "" && validEnv(r.Secret) != nil {
			return report, fmt.Errorf("invalid Secret: %s", n)
		}
		tags, e := protocol.NormalizeTags(r.Tags)
		if e != nil {
			return report, fmt.Errorf("invalid tags: %s", n)
		}
		r.Tags = tags
		r.SyncTags()
		records[n] = r
	}
	choiceIndex := map[string]protocol.Record{}
	for n, r := range records {
		choiceIndex[n] = r
	}
	for n := range users {
		if _, ok := choiceIndex[n]; !ok {
			choiceIndex[n] = protocol.Record{Name: n, Kind: protocol.KindUser}
		}
	}
	keep, e := resolveSelectors(choices.KeepExisting, choiceIndex, true)
	if e != nil {
		return report, e
	}
	replace, e := resolveSelectors(choices.Replace, choiceIndex, true)
	if e != nil {
		return report, e
	}
	replaceUsers, e := resolveSelectors(choices.ReplaceUsers, choiceIndex, true)
	if e != nil {
		return report, e
	}
	replaceTokens, e := resolveSelectors(choices.ReplaceTokens, choiceIndex, true)
	if e != nil {
		return report, e
	}
	for n := range keep {
		_, u := b.users[n]
		_, r := b.records[n]
		if !u && !r {
			delete(keep, n)
		}
	}
	incoming := map[string]bool{}
	for n := range records {
		incoming[n] = true
	}
	for n := range users {
		incoming[n] = true
	}
	for _, n := range sortedNames(incoming) {
		if keep[n] && (replace[n] || replaceUsers[n] || replaceTokens[n]) {
			report.Conflicts = append(report.Conflicts, n+": contradicting keep/replace choices")
			continue
		}
		_, profile := users[n]
		r, hasRecord := records[n]
		userKind := profile || hasRecord && r.Kind == protocol.KindUser
		_, oldUser := b.users[n]
		_, oldRecord := b.records[n]
		if oldUser || oldRecord {
			resolved := keep[n] || userKind && replaceUsers[n] || !userKind && replace[n]
			if !resolved {
				report.Conflicts = append(report.Conflicts, n+": existing name requires an explicit keep or replace choice")
			}
		}
	}

	var ids []uint32
	var principalNames []string
	for n, r := range b.records {
		ids = append(ids, r.ID)
		if r.Kind == protocol.KindAgent {
			principalNames = append(principalNames, n)
		}
	}
	for n := range b.users {
		principalNames = append(principalNames, n)
	}
	state, e := b.store.(ports.TransferStore).ExportState(ids, principalNames)
	if e != nil {
		return report, fmt.Errorf("import preflight read failed")
	}
	held := map[string]ports.Credential{}
	tokenOwners := map[string]string{}
	for _, c := range state.Credentials {
		held[c.Name] = c
		tokenOwners[c.Current] = c.Name
		if c.Previous != "" {
			tokenOwners[c.Previous] = c.Name
		}
	}
	creds := map[string]ports.Credential{}
	seenCredentials := map[string]bool{}
	for _, c := range data.Credentials {
		if !incoming[c.Name] || c.Current == "" || len(c.Current) > 4096 || len(c.Previous) > 4096 {
			return report, fmt.Errorf("invalid credential in backup")
		}
		if seenCredentials[c.Name] {
			return report, fmt.Errorf("duplicate credential in backup")
		}
		seenCredentials[c.Name] = true
		if keep[c.Name] {
			report.TokensKept = append(report.TokensKept, c.Name)
			continue
		}
		if _, exists := held[c.Name]; exists && !replaceTokens[c.Name] {
			report.TokensKept = append(report.TokensKept, c.Name)
			continue
		}
		for _, v := range []string{c.Current, c.Previous} {
			if v == "" {
				continue
			}
			if other, ok := tokenOwners[v]; ok && other != c.Name {
				report.Conflicts = append(report.Conflicts, c.Name+": credential conflicts with "+other)
			}
			tokenOwners[v] = c.Name
		}
		creds[c.Name] = c
	}
	if len(report.Conflicts) > 0 {
		return report, fmt.Errorf("import conflicts: %s", strings.Join(report.Conflicts, "; "))
	}
	// No fallible decoding remains. Staged map writes are invisible under b.mu;
	// unlock discards all staging on any validation failure or dry run.
	for _, n := range sortedNames(incoming) {
		if keep[n] {
			report.Kept = append(report.Kept, n)
			continue
		}
		if u, ok := users[n]; ok {
			b.setUser(n, u)
			stored := b.users[n]
			stored.Created = u.Created
			stored.Updated = u.Updated
			b.users[n] = stored
			report.Imported = append(report.Imported, n)
		}
	}
	for _, n := range sortedNames(incoming) {
		if keep[n] {
			continue
		}
		r, ok := records[n]
		if !ok {
			continue
		}
		if _, ok := b.users[r.Owner]; !ok {
			return report, fmt.Errorf("missing owning user: %s", r.Owner)
		}
		r.OwnerID = b.users[r.Owner].ID
		b.setRecord(n, r)
		stored := b.records[n]
		stored.Created = r.Created
		b.records[n] = stored
		report.Imported = append(report.Imported, n)
	}
	for _, n := range sortedNames(incoming) {
		if keep[n] {
			continue
		}
		r, ok := b.records[n]
		if !ok {
			continue
		}
		allow, e := b.normalizeAllow(r.Allow, r)
		if e != nil {
			return report, fmt.Errorf("invalid ACL dependencies: %s", n)
		}
		maintainers, e := b.normalizeMaintainers(r.Maintainers, r)
		if e != nil {
			return report, fmt.Errorf("invalid maintainer dependencies: %s", n)
		}
		r.Allow = allow
		r.Maintainers = maintainers
		if e = b.validatePersonal(r); e != nil {
			return report, fmt.Errorf("invalid personal audience: %s", n)
		}
		b.setRecord(n, r)
	}
	kv := map[uint32][]ports.StoredKV{}
	for n := range records {
		if !keep[n] {
			kv[b.records[n].ID] = nil
		}
	}
	seenKV := map[string]bool{}
	counts := map[string]int{}
	for _, v := range data.KV {
		if _, ok := records[v.Record]; !ok {
			return report, fmt.Errorf("KV record is absent from backup")
		}
		if len(v.Name) == 0 || len(v.Name) > KVMaxName || !utf8.ValidString(v.Name) {
			return report, fmt.Errorf("invalid KV name")
		}
		key := v.Record + "\x00" + string(v.Kind) + "\x00" + v.Name
		if seenKV[key] {
			return report, fmt.Errorf("duplicate KV value")
		}
		seenKV[key] = true
		switch v.Kind {
		case ports.KVString:
			if len(v.Value.Bytes) > KVMaxValue {
				return report, ErrKVLimit
			}
		case ports.KVJSON:
			if len(v.Value.Bytes) > KVMaxValue || !json.Valid(v.Value.Bytes) {
				return report, fmt.Errorf("invalid JSON store value")
			}
		case ports.KVInt:
		default:
			return report, fmt.Errorf("invalid KV kind")
		}
		countKey := v.Record + "\x00" + string(v.Kind)
		counts[countKey]++
		if counts[countKey] > KVMaxNames {
			return report, ErrKVLimit
		}
		if keep[v.Record] {
			continue
		}
		id := b.records[v.Record].ID
		kv[id] = append(kv[id], ports.StoredKV{RecordID: id, Kind: v.Kind, Name: v.Name, Value: v.Value})
	}
	for n, c := range creds {
		pair, e := b.pairFor(n)
		if e != nil {
			return report, fmt.Errorf("credential cannot be retained for %s", n)
		}
		c.CredentialPair = pair
		creds[n] = c
	}
	for n := range incoming {
		if keep[n] {
			continue
		}
		if _, ok := held[n]; ok {
			pair, e := b.pairFor(n)
			if e == nil {
				b.stageCredential(n, &pair)
			}
		}
	}
	sort.Strings(report.Imported)
	report.Imported = compactStrings(report.Imported)
	sort.Strings(report.TokensKept)
	if choices.DryRun {
		b.discard()
		return report, nil
	}
	if b.staging == nil {
		return report, nil
	}
	if e = b.commitTransfer(creds, kv); e != nil {
		return report, e
	}
	return report, nil
}

func compactStrings(in []string) []string {
	out := in[:0]
	for _, s := range in {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	return out
}

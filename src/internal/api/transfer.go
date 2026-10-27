package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/authkeys"
	"github.com/parf/ai-agent-bus/internal/backup"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/version"
)

type localSocketKey struct{}

func (s *Server) RecordBackups(directory string) { s.backupDirectory = directory }

type ExportRequest struct {
	core.RecordSelection
	Download bool `json:"download"`
}
type ExportReply struct {
	Ciphertext  []byte         `json:"ciphertext,omitempty"`
	Sidecar     backup.Sidecar `json:"sidecar"`
	ArchivePath string         `json:"archive_path,omitempty"`
	InfoPath    string         `json:"info_path,omitempty"`
}
type ImportRequest struct {
	Data    core.RecordBackup  `json:"data"`
	Choices core.ImportChoices `json:"choices"`
}

// Only keys enrolled for the authenticated caller are recipients. Request data
// cannot name a recipient or impersonate the administrator's original caller.
func (s *Server) backupRecipients(caller string) ([]string, []string, error) {
	if s.sshKeys == "" {
		return nil, nil, errors.New("export requires an enrolled Ed25519 key")
	}
	lines, e := authkeys.Read(s.sshKeys)
	if e != nil {
		return nil, nil, errors.New("cannot read enrolled backup keys")
	}
	keys := map[string]string{}
	for _, line := range lines {
		who, _ := authkeys.Whose(line)
		if who != caller {
			continue
		}
		// The key follows the closing quote of the forced command, never a comment.
		pos := strings.Index(line, `command="`)
		if pos < 0 {
			continue
		}
		rest := line[pos+len(`command="`):]
		_, rest, ok := strings.Cut(rest, `"`)
		if !ok {
			continue
		}
		key, e := authkeys.Ed25519(strings.TrimSpace(rest))
		if e != nil {
			continue
		}
		fp, e := backup.Fingerprint(key)
		if e == nil {
			keys[fp] = key
		}
	}
	var fingerprints, recipients []string
	for fp := range keys {
		fingerprints = append(fingerprints, fp)
	}
	sort.Strings(fingerprints)
	for _, fp := range fingerprints {
		recipients = append(recipients, keys[fp])
	}
	if len(recipients) == 0 {
		return nil, nil, errors.New("export requires an enrolled Ed25519 key")
	}
	return recipients, fingerprints, nil
}

func (s *Server) exportRecords(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	if caller.String() != s.bus.DaemonOwner() {
		s.reply(w, nil, core.ErrNotOwner)
		return
	}
	var in ExportRequest
	if !s.readStrict(w, r, &in) {
		return
	}
	recipients, fingerprints, e := s.backupRecipients(caller.String())
	if e != nil {
		s.refuse(w, http.StatusBadRequest, "malformed", e.Error())
		return
	}
	data, names, e := s.bus.ExportRecords(caller.String(), in.RecordSelection)
	if e != nil {
		s.reply(w, nil, e)
		return
	}
	kind := "full"
	if len(in.Include) > 0 || len(in.Exclude) > 0 {
		kind = "partial"
	}
	host, e := os.Hostname()
	if e != nil {
		s.reply(w, nil, errors.New("cannot read source host"))
		return
	}
	info := backup.Info{Kind: kind, Host: host, Version: version.String, When: time.Now().UTC(), Who: caller.String(), Include: in.Include, Exclude: in.Exclude, Names: names, Recipients: fingerprints}
	body, e := json.Marshal(data)
	if e != nil {
		s.reply(w, nil, errors.New("cannot encode record backup"))
		return
	}
	packed, e := backup.Pack(info, map[string][]byte{"records.json": body})
	if e != nil {
		s.reply(w, nil, e)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	encrypted, e := backup.Encrypt(ctx, packed, recipients)
	if e != nil {
		s.reply(w, nil, e)
		return
	}
	out := ExportReply{Sidecar: backup.Sidecar{Info: info, SHA256: backup.Hash(encrypted)}}
	if in.Download {
		out.Ciphertext = encrypted
	} else {
		if s.backupDirectory == "" {
			s.reply(w, nil, errors.New("record backup directory is not configured"))
			return
		}
		path := backup.Path(s.backupDirectory, info)
		sidecar, e := backup.SidecarBytes(info, encrypted)
		if e == nil {
			e = backup.WriteFiles(path, encrypted, sidecar)
		}
		if e != nil {
			s.reply(w, nil, errors.New("cannot write record backup"))
			return
		}
		out.ArchivePath, out.InfoPath = path, path+".info"
	}
	s.logs().Audit(ports.AuditEntry{At: time.Now(), Actor: caller.String(), Operation: "export", Result: "ok", ClientIP: clientIP(r)})
	ok(w, out)
}

func (s *Server) importRecords(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	if caller.String() != s.bus.DaemonOwner() {
		s.reply(w, nil, core.ErrNotOwner)
		return
	}
	if local, _ := r.Context().Value(localSocketKey{}).(bool); !local {
		s.refuse(w, http.StatusForbidden, "authority", "record import requires the caller's local login socket")
		return
	}
	var in ImportRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, backup.MaxBytes*2))
	dec.DisallowUnknownFields()
	if e := dec.Decode(&in); e != nil {
		s.badBody(w, e)
		return
	}
	if dec.Decode(new(any)) != io.EOF {
		s.badBody(w, errors.New("expected one import body"))
		return
	}
	report, e := s.bus.ImportRecords(caller.String(), in.Data, in.Choices)
	if e != nil {
		// All preflight conflicts are public names, never payloads or credentials.
		if len(report.Conflicts) > 0 {
			w.WriteHeader(http.StatusConflict)
			ok(w, struct {
				Error  string            `json:"error"`
				Report core.ImportReport `json:"report"`
			}{e.Error(), report})
			return
		}
		s.reply(w, nil, e)
		return
	}
	s.logs().Audit(ports.AuditEntry{At: time.Now(), Actor: caller.String(), Operation: "import", Result: "ok", ClientIP: clientIP(r)})
	ok(w, report)
}

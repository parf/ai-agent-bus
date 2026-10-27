package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/authkeys"
	"github.com/parf/ai-agent-bus/internal/backup"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func transferServer(t *testing.T) (*Server, *auth.Tokens) {
	t.Helper()
	b := core.New()
	store := memory.NewState()
	b.Persistence(store)
	if e := b.EstablishDaemonOwner("owner@h"); e != nil {
		t.Fatal(e)
	}
	tokens, e := auth.Load(store.Tokens(), "owner@h")
	if e != nil {
		t.Fatal(e)
	}
	return New(b, tokens, "owner@h"), tokens
}
func localTransfer(t *testing.T, s *Server, who, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, e := json.Marshal(body)
	if e != nil {
		t.Fatal(e)
	}
	n, e := protocol.ParseName(who)
	if e != nil {
		t.Fatal(e)
	}
	w := httptest.NewRecorder()
	s.HandlerFor(n).ServeHTTP(w, httptest.NewRequest("POST", path, bytes.NewReader(raw)))
	return w
}
func TestRecordTransferAPIEnrolledRecipientAndOriginalAuthority(t *testing.T) {
	s, tokens := transferServer(t)
	for _, name := range []string{"administrator@h", "ordinary@h"} {
		if _, e := s.bus.SetUser("owner@h", protocol.User{Name: name}, true); e != nil {
			t.Fatal(e)
		}
	}
	if e := s.bus.SetGroup("owner@h", core.AdministratorsGroup, []string{"owner@h", "administrator@h"}); e != nil {
		t.Fatal(e)
	}
	key := filepath.Join(t.TempDir(), "id_ed25519")
	if out, e := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key).CombinedOutput(); e != nil {
		t.Fatalf("ssh-keygen: %s", out)
	}
	pub, e := os.ReadFile(key + ".pub")
	if e != nil {
		t.Fatal(e)
	}
	enrolled := filepath.Join(t.TempDir(), ".ssh", "authorized_keys")
	line, e := authkeys.Line("/usr/local/bin/agent-bus-token", "owner@h", string(pub))
	if e != nil {
		t.Fatal(e)
	}
	if e = authkeys.Write(enrolled, []string{line}); e != nil {
		t.Fatal(e)
	}
	s.SSHKeys(enrolled, "/usr/local/bin/agent-bus-token")
	dir := t.TempDir()
	s.RecordBackups(dir)
	if _, e = s.bus.Register(protocol.Record{Name: "#fixture@h", Kind: protocol.KindAgent, Owner: "owner@h"}); e != nil {
		t.Fatal(e)
	}
	if _, e = s.bus.SetSecret("#fixture@h", "owner@h", "KEY=private-api-fixture"); e != nil {
		t.Fatal(e)
	}
	secretToken, e := tokens.Issue("#fixture@h")
	if e != nil {
		t.Fatal(e)
	}
	for _, who := range []string{"administrator@h", "ordinary@h"} {
		for _, path := range []string{"/records/export", "/records/import"} {
			w := localTransfer(t, s, who, path, map[string]any{})
			if w.Code != http.StatusForbidden {
				t.Fatalf("%s %s: %d", who, path, w.Code)
			}
		}
	}
	w := localTransfer(t, s, "owner@h", "/records/export", ExportRequest{Download: true})
	if w.Code != http.StatusOK {
		t.Fatalf("export: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "private-api-fixture") || strings.Contains(w.Body.String(), secretToken) {
		t.Fatal("export API leaked plaintext")
	}
	var reply ExportReply
	if json.Unmarshal(w.Body.Bytes(), &reply) != nil {
		t.Fatal("export response")
	}
	plain, e := backup.Decrypt(context.Background(), reply.Ciphertext, key)
	if e != nil {
		t.Fatal(e)
	}
	manifest, files, e := backup.Unpack(plain)
	if e != nil {
		t.Fatal(e)
	}
	if manifest.Info.Who != "owner@h" || len(manifest.Info.Recipients) != 1 {
		t.Fatal("recipient/caller info")
	}
	var data core.RecordBackup
	if json.Unmarshal(files["records.json"], &data) != nil {
		t.Fatal("record payload")
	}
	dest, _ := transferServer(t)
	w = localTransfer(t, dest, "owner@h", "/records/import", ImportRequest{Data: data, Choices: core.ImportChoices{KeepExisting: []string{"all"}}})
	if w.Code != http.StatusOK {
		t.Fatalf("import: %d %s", w.Code, w.Body.String())
	}
	if _, pair, ok := dest.tokens.Credential(secretToken); !ok || dest.bus.CheckCredential("#fixture@h", pair) != nil {
		t.Fatal("API import lost credential")
	}
	// Default mode stores only ciphertext plus an adjacent public sidecar.
	w = localTransfer(t, s, "owner@h", "/records/export", ExportRequest{})
	if w.Code != http.StatusOK {
		t.Fatalf("default export: %d", w.Code)
	}
	if json.Unmarshal(w.Body.Bytes(), &reply) != nil {
		t.Fatal("default response")
	}
	cipher, e := os.ReadFile(reply.ArchivePath)
	if e != nil {
		t.Fatal(e)
	}
	sidecar, e := os.ReadFile(reply.InfoPath)
	if e != nil {
		t.Fatal(e)
	}
	p, e := backup.Decrypt(context.Background(), cipher, key)
	if e != nil {
		t.Fatal(e)
	}
	m, _, e := backup.Unpack(p)
	if e != nil {
		t.Fatal(e)
	}
	if e = backup.CheckSidecar(cipher, m, sidecar); e != nil {
		t.Fatal(e)
	}
	// Missing enrolled key must not produce an unencrypted export.
	s.SSHKeys("", "")
	w = localTransfer(t, s, "owner@h", "/records/export", ExportRequest{Download: true})
	if w.Code == http.StatusOK {
		t.Fatal("unenrolled recipient exported")
	}
}

func TestRecordImportAPIRequiresLocalSocketEvenWithOwnerToken(t *testing.T) {
	s, tokens := transferServer(t)
	token, e := tokens.Issue("owner@h")
	if e != nil {
		t.Fatal(e)
	}
	req := httptest.NewRequest("POST", "/records/import", strings.NewReader(`{"data":{"format":1}}`))
	req.Header.Set(HeaderToken, token)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatal("remote Owner submitted plaintext import")
	}
}

func TestRecordTransferOwnerGateBeforeReadingPrivateBody(t *testing.T) {
	s, _ := transferServer(t)
	if _, e := s.bus.SetUser("owner@h", protocol.User{Name: "administrator@h"}, true); e != nil {
		t.Fatal(e)
	}
	if e := s.bus.SetGroup("owner@h", core.AdministratorsGroup, []string{"owner@h", "administrator@h"}); e != nil {
		t.Fatal(e)
	}
	who, _ := protocol.ParseName("administrator@h")
	for _, path := range []string{"/records/export", "/records/import"} {
		body := &countedBody{Reader: strings.NewReader(`{"data":{"format":1}}`)}
		req := httptest.NewRequest("POST", path, nil)
		req.Body = body
		w := httptest.NewRecorder()
		s.HandlerFor(who).ServeHTTP(w, req)
		if w.Code != http.StatusForbidden || body.read != 0 {
			t.Fatalf("%s: Owner gate read %d bytes or answered %d", path, body.read, w.Code)
		}
	}
}

package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/authkeys"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/sqlite"
)

func transferNode(t *testing.T, path string, create bool) (*core.Bus, *auth.Tokens, *sqlite.Store) {
	t.Helper()
	store, e := sqlite.Open(path, create)
	if e != nil {
		t.Fatal(e)
	}
	snapshot, e := store.Load()
	if e != nil {
		t.Fatal(e)
	}
	b := core.New()
	b.Restore(snapshot)
	b.Persistence(store)
	if e = b.EstablishDaemonOwner("owner@fixture"); e != nil {
		t.Fatal(e)
	}
	tokens, e := auth.Load(store.Tokens(), "owner@fixture")
	if e != nil {
		t.Fatal(e)
	}
	b.BindCredentials(tokens)
	return b, tokens, store
}
func serveTransfer(t *testing.T, s *api.Server) string {
	t.Helper()
	socket := filepath.Join(t.TempDir(), "user-fixture.sock")
	l, e := net.Listen("unix", socket)
	if e != nil {
		t.Fatal(e)
	}
	who, e := protocol.ParseName("owner@fixture")
	if e != nil {
		t.Fatal(e)
	}
	server := &http.Server{Handler: s.HandlerFor(who)}
	go server.Serve(l)
	t.Cleanup(func() { server.Close() })
	return socket
}
func TestAdminEncryptedExportImportRebuildAndOldAgentClients(t *testing.T) {
	key := filepath.Join(t.TempDir(), "id_ed25519")
	if e := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key).Run(); e != nil {
		t.Fatal(e)
	}
	pub, e := os.ReadFile(key + ".pub")
	if e != nil {
		t.Fatal(e)
	}
	enrolled := filepath.Join(t.TempDir(), ".ssh/authorized_keys")
	line, e := authkeys.Line("/usr/bin/agent-bus-token", "owner@fixture", string(pub))
	if e != nil {
		t.Fatal(e)
	}
	if e = authkeys.Write(enrolled, []string{line}); e != nil {
		t.Fatal(e)
	}
	source, tokens, sourceStore := transferNode(t, filepath.Join(t.TempDir(), "source.db"), true)
	defer sourceStore.Close()
	held := map[string]string{}
	for _, name := range []string{"#runner@fixture", "#standalone@fixture"} {
		if _, e = source.Register(protocol.Record{Name: name, Kind: protocol.KindAgent, Owner: "owner@fixture", Tags: []string{"ops"}, Allow: []string{"@owner"}}); e != nil {
			t.Fatal(e)
		}
		token, e := tokens.Issue(name)
		if e != nil {
			t.Fatal(e)
		}
		held[name] = token
		if _, e = source.Configure(name, "owner@fixture", json.RawMessage(`{"value":"fixture"}`)); e != nil {
			t.Fatal(e)
		}
	}
	srcAPI := api.New(source, tokens, "owner@fixture")
	srcAPI.SSHKeys(enrolled, "/usr/bin/agent-bus-token")
	t.Setenv("AGENT_BUS_ADDR", serveTransfer(t, srcAPI))
	archive := filepath.Join(t.TempDir(), "partial-fixture.tar.gz.age")
	if e = recordTransfer("export", []string{"--include=tag:ops", "--out=" + archive}); e != nil {
		t.Fatal(e)
	}
	targetPath := filepath.Join(t.TempDir(), "fresh.db")
	target, targetTokens, targetStore := transferNode(t, targetPath, true)
	targetAPI := api.New(target, targetTokens, "owner@fixture")
	t.Setenv("AGENT_BUS_ADDR", serveTransfer(t, targetAPI))
	if e = recordTransfer("import", []string{archive, "--key=" + key, "--keep-existing=all"}); e != nil {
		t.Fatal(e)
	}
	if e = targetStore.Close(); e != nil {
		t.Fatal(e)
	}
	target, targetTokens, targetStore = transferNode(t, targetPath, false)
	defer targetStore.Close()
	// Actual old-token API requests after a database rebuild and restart, from
	// independent runner/standalone clients: no new token is requested.
	targetAPI = api.New(target, targetTokens, "owner@fixture")
	for name, token := range held {
		req, _ := http.NewRequest("GET", "http://fixture/status", nil)
		req.Header.Set(api.HeaderToken, token)
		response := httptest.NewRecorder()
		targetAPI.Handler().ServeHTTP(response, req)
		if response.Code != http.StatusOK {
			t.Fatal("old agent client refused after rebuild")
		}
		var status struct {
			You string `json:"you"`
		}
		if json.Unmarshal(response.Body.Bytes(), &status) != nil || status.You != name {
			t.Fatal("old token authenticated as another principal")
		}
	}
}

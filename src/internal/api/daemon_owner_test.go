package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestDaemonOwnerTransferChangesEveryPublicOwnerAnswer(t *testing.T) {
	b := core.New()
	s, token := serverFor(t, b, "owner@h")
	// Built before the transfer, as the bus child builds it at start.
	ownerSocket := s.HandlerForOwner()
	for _, name := range []string{"next@h", "alice@h"} {
		if _, err := b.SetUser("owner@h", protocol.User{Name: name}, true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	call := func(who, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if who != "" {
			r.Header.Set(HeaderToken, token(who))
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	if w := call("next@h", http.MethodPost, "/owner", `{"kind":"agent","name":"next@h"}`); w.Code != http.StatusForbidden {
		t.Fatalf("non-owner transferred daemon: %d %s", w.Code, w.Body.String())
	}
	w := call("owner@h", http.MethodPost, "/owner", `{"kind":"agent","name":"next@h"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("transfer: %d %s", w.Code, w.Body.String())
	}
	var moved protocol.User
	if err := json.Unmarshal(w.Body.Bytes(), &moved); err != nil || !moved.DaemonOwner || !moved.Administrator {
		t.Fatalf("transfer answer: %+v, %v", moved, err)
	}

	w = call("", http.MethodGet, "/identity", "")
	var node protocol.NodeIdentity
	if err := json.Unmarshal(w.Body.Bytes(), &node); err != nil || node.Owner != "next@h" {
		t.Fatalf("identity retained startup owner: %+v, %v", node, err)
	}
	for who, want := range map[string]bool{"owner@h": false, "next@h": true} {
		w = call(who, http.MethodGet, "/status", "")
		var status struct {
			DaemonOwner bool `json:"daemon_owner"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil || status.DaemonOwner != want {
			t.Fatalf("%s status owner=%v want=%v: %s (%v)", who, status.DaemonOwner, want, w.Body.String(), err)
		}
	}

	// The daemon account's socket speaks for the daemon Owner of each request,
	// so the transfer moves it at once, without a restart: it now answers as
	// the new Owner, who holds the root resource override.
	onOwnerSocket := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		w := httptest.NewRecorder()
		ownerSocket.ServeHTTP(w, r)
		return w
	}
	w = onOwnerSocket(http.MethodGet, "/status", "")
	var onSocket struct {
		You         string `json:"you"`
		DaemonOwner bool   `json:"daemon_owner"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &onSocket); err != nil || onSocket.You != "next@h" || !onSocket.DaemonOwner {
		t.Fatalf("the daemon account's socket did not follow the transfer: %s (%v)", w.Body.String(), err)
	}
	if w := onOwnerSocket(http.MethodPost, "/manage", `{"kind":"agent","name":"#svc@h","descr":"socket root"}`); w.Code != http.StatusOK {
		t.Fatalf("the daemon account's socket lacks the new Owner's root management: %d %s", w.Code, w.Body.String())
	}
	// The former Owner's own socket, where one is mapped, keeps its retained
	// Administrator work and loses the root override.
	seed, err := protocol.ParseName("owner@h")
	if err != nil {
		t.Fatal(err)
	}
	if w := serveOn(s.HandlerFor(seed), http.MethodPost, "/manage", `{"kind":"agent","name":"#svc@h","descr":"former root"}`); w.Code != http.StatusForbidden {
		t.Fatalf("former owner's socket retained root management: %d %s", w.Code, w.Body.String())
	}
	if w := call("next@h", http.MethodPost, "/manage", `{"kind":"agent","name":"#svc@h","descr":"new root"}`); w.Code != http.StatusOK {
		t.Fatalf("new owner lacks root management: %d %s", w.Code, w.Body.String())
	}
}

func serveOn(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

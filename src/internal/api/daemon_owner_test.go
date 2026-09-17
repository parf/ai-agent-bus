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
	for _, name := range []string{"next@h", "alice@h"} {
		if _, err := b.SetUser("owner@h", protocol.User{Name: name}, true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.Register(protocol.Record{Name: "svc@h", Owner: "alice@h"}); err != nil {
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
	if w := call("next@h", http.MethodPost, "/owner", `{"name":"next@h"}`); w.Code != http.StatusForbidden {
		t.Fatalf("non-owner transferred daemon: %d %s", w.Code, w.Body.String())
	}
	w := call("owner@h", http.MethodPost, "/owner", `{"name":"next@h"}`)
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

	// The supervisor keeps the daemon account's socket mapped to the setup
	// seed. After transfer it still authenticates the former Owner, whose
	// retained Administrator position may manage groups but has no root resource
	// override. The new Owner reaches that override through their own credential.
	seed, err := protocol.ParseName("owner@h")
	if err != nil {
		t.Fatal(err)
	}
	onSeedSocket := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		w := httptest.NewRecorder()
		s.HandlerFor(seed).ServeHTTP(w, r)
		return w
	}
	if w := onSeedSocket(http.MethodGet, "/groups", ""); w.Code != http.StatusOK {
		t.Fatalf("former owner's socket lost Administrator work: %d %s", w.Code, w.Body.String())
	}
	if w := onSeedSocket(http.MethodPost, "/manage", `{"name":"svc@h","descr":"former root"}`); w.Code != http.StatusForbidden {
		t.Fatalf("former owner's socket retained root management: %d %s", w.Code, w.Body.String())
	}
	if w := call("next@h", http.MethodPost, "/manage", `{"name":"svc@h","descr":"new root"}`); w.Code != http.StatusOK {
		t.Fatalf("new owner lacks root management: %d %s", w.Code, w.Body.String())
	}
}

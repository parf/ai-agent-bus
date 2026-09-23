package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func userRecord(name string) protocol.Record {
	return protocol.Record{Name: name, Owner: name, Kind: protocol.KindUser, Personal: true, Full: protocol.OverflowStrict}
}

// The bus child serves each inherited socket: the daemon account's as the
// daemon Owner of each request, so a transfer moves it without a restart, and
// none for a mapping the load ignored.
func TestInheritedSocketsFollowTheOwnerAndSkipIgnoredMappings(t *testing.T) {
	bus := core.New()
	bus.Restore(ports.Snapshot{
		Clean: true, OwnerEstablished: true, Owner: "owner@h", AccountsEstablished: true,
		Accounts: []protocol.AccountMapping{{Account: "ghost", Principal: "#gone@h"}, {Account: "ops", Principal: "heir@h"}},
		Users:    []protocol.User{{Name: "owner@h", Status: "active"}, {Name: "heir@h", Status: "active"}},
		Records: []protocol.Record{userRecord("owner@h"), userRecord("heir@h"),
			{Name: core.AdministratorsGroup, Kind: protocol.KindGroup, Owner: "owner@h", Allow: []string{"owner@h"}}},
	})
	tokens, err := auth.Load(memory.NewTokens(), "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	face := api.New(bus, tokens, "owner@h")
	handler := func(in inlet) http.Handler {
		t.Helper()
		h, err := socketHandler(bus, face, in)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	if h := handler(inlet{who: "#gone@h"}); h != nil {
		t.Fatal("the socket of an ignored mapping is served")
	}
	if h := handler(inlet{who: "heir@h"}); h == nil {
		t.Fatal("a loaded mapping's socket is not served")
	}
	own := handler(inlet{who: "owner@h", owner: true})
	you := func() string {
		w := httptest.NewRecorder()
		own.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/status", nil))
		var st struct {
			You string `json:"you"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
			t.Fatalf("%d %s: %v", w.Code, w.Body.String(), err)
		}
		return st.You
	}
	if got := you(); got != "owner@h" {
		t.Fatalf("the daemon account's socket is %q before the transfer", got)
	}
	if _, err := bus.TransferDaemonOwner("owner@h", "heir@h"); err != nil {
		t.Fatal(err)
	}
	if got := you(); got != "heir@h" {
		t.Fatalf("the daemon account's socket is %q after the transfer, not the new Owner", got)
	}
}

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func userRecord(name string) protocol.Record {
	return protocol.Record{Name: name, Owner: name, Kind: protocol.KindUser, Tags: []string{"personal"}, Full: protocol.OverflowStrict}
}

// The bus child serves each inherited socket: the daemon account's as the
// daemon Owner of each request, so a transfer moves it without a restart.
func TestInheritedSocketsFollowTheOwner(t *testing.T) {
	bus := core.New()
	bus.SetDaemonOwner("owner@h")
	if _, err := bus.SetUser("owner@h", protocol.User{Name: "heir@h"}, true); err != nil {
		t.Fatal(err)
	}
	if err := bus.EstablishAccounts(map[string]string{"ops": "heir@h"}); err != nil {
		t.Fatal(err)
	}

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

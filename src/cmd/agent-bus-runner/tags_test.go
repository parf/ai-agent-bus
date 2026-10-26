package main

import (
	"encoding/json"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestManagedTagsCoverCreationAndKeptTokenRefresh(t *testing.T) {
	var created protocol.Record
	var added []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/register":
			json.NewDecoder(r.Body).Decode(&created)
			io.WriteString(w, `{"token":"kept"}`)
		case "/lookup":
			io.WriteString(w, `{"owner":"launcher"}`)
		case "/status":
			io.WriteString(w, `{"you":"#tagged@h"}`)
		case "/manage":
			if r.Header.Get("X-Agent-Bus-Token") != "kept" {
				t.Error("tag refresh did not use kept agent credential")
			}
			var body struct {
				Add struct {
					Tags []string `json:"tags"`
				} `json:"add_to_set"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			added = body.Add.Tags
			io.WriteString(w, `{}`)
		default:
			http.Error(w, "unexpected", 404)
		}
	}))
	defer srv.Close()
	r := &runner{host: t.TempDir(), say: io.Discard, person: "launcher", bus: &client{http: srv.Client(), base: srv.URL, token: "launcher"}}
	in := installed{Agent: agentFile{Tags: []string{"Author-Notes"}}, Host: hostEntry{Tags: []string{"Host-Notes"}}}
	if _, err := r.claim(in, "#tagged@h"); err != nil {
		t.Fatal(err)
	}
	want := []string{"author-notes", "host-notes", "runner"}
	if !slices.Equal(created.Tags, want) {
		t.Fatalf("creation %v", created.Tags)
	}
	in.Host.Tags = []string{"New-Host-Notes"}
	if _, err := r.claim(in, "#tagged@h"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(added, []string{"author-notes", "new-host-notes", "runner"}) {
		t.Fatalf("refresh %v", added)
	}
	if strings.Contains(strings.Join(added, ","), "no-tags") {
		t.Fatal("synthetic persisted")
	}
}

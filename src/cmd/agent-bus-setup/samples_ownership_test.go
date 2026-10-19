package main

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
)

// Exercise the commands over their real owner-client transport: the names
// exist, so a refusal caused by an absent subject cannot protect real data.
func TestSamplesPreserveRealGroupsAndUsers(t *testing.T) {
	oldUsers, oldGroups, oldRecords, oldSends := sampleUsers, sampleGroups, sampleRecords, sampleSends
	t.Cleanup(func() {
		sampleUsers, sampleGroups, sampleRecords, sampleSends = oldUsers, oldGroups, oldRecords, oldSends
	})
	sampleUsers = []sampleUser{{"real@h", "The sample person"}, {"sample@h", "The sample person"}}
	sampleGroups = []sampleGroup{
		{name: "@undescribed", owner: "sample@h", descr: "Sample group"},
		{name: "@described", owner: "sample@h", descr: "Sample group"},
		{name: "@sample", owner: "sample@h", descr: "Sample group"},
	}
	sampleRecords, sampleSends = nil, nil

	// The owner client reaches only this temporary recording socket.
	dir := shortSocketDir(t)
	socket := filepath.Join(dir, "bus.sock")
	listener, err := listenTestUnix(t, socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_BUS_ADDR", socket)
	type write struct {
		path string
		body map[string]any
	}
	var mu sync.Mutex
	var writes []write
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "bad fixture request", 400)
				return
			}
			mu.Lock()
			writes = append(writes, write{r.URL.Path, body})
			mu.Unlock()
			_, _ = w.Write([]byte(`{}`))
			return
		}
		switch r.URL.Path {
		case "/users":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"name": "real@h", "kind": "user", "person_name": "A real person"},
				{"name": "sample@h", "kind": "user", "person_name": "The sample person"},
			})
		case "/lookup":
			record := map[string]any{"name": r.URL.Query().Get("name"), "kind": "group", "owner": "real@h"}
			switch record["name"] {
			case "@described":
				record["descr"] = "A real group"
			case "@sample":
				record["descr"] = "Sample group"
			}
			_ = json.NewEncoder(w).Encode(record)
		default:
			http.Error(w, "unexpected fixture request", 500)
		}
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	if err := addSamples(); err != nil {
		t.Fatal(err)
	}
	if err := removeSamples(); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	groupCreated, groupEmptied, userReactivated, userDeactivated := false, false, false, false
	for _, got := range writes {
		name, _ := got.body["name"].(string)
		if name == "" {
			name, _ = got.body["Name"].(string)
		}
		switch name {
		case "real@h", "@undescribed", "@described":
			t.Errorf("changed real data: %s %s", got.path, name)
		case "@sample":
			groupCreated = groupCreated || got.path == "/group"
			_, hasAllow := got.body["allow"]
			groupEmptied = groupEmptied || got.path == "/manage" && hasAllow
		case "sample@h":
			userReactivated = userReactivated || got.body["status"] == "active"
			userDeactivated = userDeactivated || got.body["status"] == "inactive"
		default:
			t.Errorf("unexpected sample mutation: %s %s", got.path, name)
		}
	}
	if !groupCreated || !groupEmptied || !userReactivated || !userDeactivated {
		t.Fatalf("sample controls: group add=%v remove=%v, user active=%v inactive=%v", groupCreated, groupEmptied, userReactivated, userDeactivated)
	}
}

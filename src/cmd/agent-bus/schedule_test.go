package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// Each time form becomes one add with the message joined from its words; an
// edit keeps what its options do not change.
func TestScheduleVerbsSendTheirRequests(t *testing.T) {
	var got []map[string]any
	var paths []string
	bigDaemon(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		if r.Method == "GET" {
			io.WriteString(w, `{"id":"abc","kind":"recurring","expr":"0 9 * * *","target":"out@h","body":"standup","max_runs":3,"enabled":false}`)
			return
		}
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		json.Unmarshal(raw, &body)
		got = append(got, body)
		io.WriteString(w, `{}`)
	})
	for _, args := range [][]string{
		{"add", "jobs@h", "0 9 * * 1-5", "out@h", "stand", "up", "--max-late=5m"},
		{"in", "jobs@h", "90s", "out@h", "check"},
		{"at", "jobs@h", "09:30", "out@h", "go"},
		{"edit", "abc", "--target=other@h"},
		{"run", "abc"},
	} {
		if _, err := captureStdout(t, func() error { return scheduleVerb(args) }); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	want := []string{"POST /schedule/add", "POST /schedule/add", "POST /schedule/add", "GET /schedule", "POST /schedule/edit", "POST /schedule/run"}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Fatalf("requests %v, want %v", paths, want)
	}
	if b := got[0]; b["record"] != "jobs@h" || b["expr"] != "0 9 * * 1-5" || b["target"] != "out@h" || b["body"] != "stand up" || b["max_late"] != "5m" {
		t.Errorf("add: %v", b)
	}
	if b := got[1]; b["in"] != "90s" || b["body"] != "check" {
		t.Errorf("in: %v", b)
	}
	if b := got[2]; b["at"] != "09:30" {
		t.Errorf("at: %v", b)
	}
	if b := got[3]; b["id"] != "abc" || b["target"] != "other@h" || b["expr"] != "0 9 * * *" || b["body"] != "standup" || b["max_runs"] != float64(3) || b["disabled"] != true {
		t.Errorf("edit lost what it did not change: %v", b)
	}
	for _, bad := range [][]string{{"add", "jobs@h", "0 9 * * *", "out@h"}, {"edit", "abc", "--in=1m", "--at=09:00"}, {"frob"}, {"run"}} {
		if err := scheduleVerb(bad); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
}

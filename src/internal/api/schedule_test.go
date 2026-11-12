package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// The schedule calls cross the API under KV's authority: the holder's Owner
// adds, lists, reads, runs and deletes; a stranger who may call the daemon
// gets, for an existing schedule, the same answer as for a missing one.
func TestScheduleCallsCrossTheAPI(t *testing.T) {
	b := core.New()
	if err := b.Persistence(memory.NewState()); err != nil {
		t.Fatal(err)
	}
	s, tok := serverFor(t, b, "admin@h")
	if _, err := b.SetUser("admin@h", protocol.User{Name: "bob@h"}, true); err != nil {
		t.Fatal(err)
	}
	if err := b.EstablishDaemonAgent("node.example"); err != nil {
		t.Fatal(err)
	}
	for name, allow := range map[string][]string{"jobs@h": {"admin@h"}, "out@h": {"admin@h"}} {
		if _, err := b.Register(protocol.Record{Name: name, Kind: protocol.KindQueue, Owner: "admin@h", Allow: allow}); err != nil {
			t.Fatal(err)
		}
	}
	code, out := send(s, tok("admin@h"), "POST", "/schedule/add", `{"record":"jobs@h","expr":"0 9 * * 1-5","target":"out@h","body":"standup"}`, "")
	if code != http.StatusOK {
		t.Fatalf("add: %d %s", code, out)
	}
	var added struct {
		Schedule struct {
			ID, Holder, Kind, Next, Creator string
			HolderKind                      string `json:"holder_kind"`
		}
	}
	if err := json.Unmarshal([]byte(out), &added); err != nil || added.Schedule.ID == "" || added.Schedule.Holder != "jobs@h" || added.Schedule.Kind != "recurring" || !strings.HasSuffix(added.Schedule.Next, "T09:00:00Z") || added.Schedule.Creator != "admin@h" || added.Schedule.HolderKind != "queue" {
		t.Fatalf("add answered %s", out)
	}
	id := added.Schedule.ID
	if code, out := send(s, tok("admin@h"), "GET", "/schedule/list?record=jobs@h", "", ""); code != http.StatusOK || !strings.Contains(out, id) {
		t.Fatalf("list: %d %s", code, out)
	}
	if code, out := send(s, tok("admin@h"), "POST", "/schedule/run", `{"id":"`+id+`"}`, ""); code != http.StatusOK || !strings.Contains(out, `"outcome":"sent"`) {
		t.Fatalf("run: %d %s", code, out)
	}
	code, out = send(s, tok("admin@h"), "GET", "/schedule?id="+id, "", "")
	var one struct {
		Upcoming    []string
		LastOutcome string `json:"last_outcome"`
		HolderKind  string `json:"holder_kind"`
		Outcomes    []any
	}
	if code != http.StatusOK || json.Unmarshal([]byte(out), &one) != nil || len(one.Upcoming) != 5 || one.LastOutcome != "sent" || one.HolderKind != "queue" || len(one.Outcomes) != 1 {
		t.Fatalf("get: %d %s", code, out)
	}
	if code, out := send(s, tok("admin@h"), "GET", "/schedule/list", "", ""); code != http.StatusOK || !strings.Contains(out, `"last_outcome":"sent"`) {
		t.Fatalf("a listing without the last outcome: %d %s", code, out)
	}
	mcode, mout := send(s, tok("bob@h"), "GET", "/schedule?id=000000000000", "", "")
	if code, out := send(s, tok("bob@h"), "GET", "/schedule?id="+id, "", ""); code != mcode || out != mout || code != http.StatusNotFound {
		t.Fatalf("a stranger reading an existing schedule: %d %s, the missing answer: %d %s", code, out, mcode, mout)
	}
	if code, out := send(s, tok("bob@h"), "GET", "/schedule/list", "", ""); code != http.StatusOK || strings.Contains(out, id) {
		t.Fatalf("a stranger's listing: %d %s", code, out)
	}
	if code, _ := send(s, tok("bob@h"), "POST", "/schedule/delete", `{"id":"`+id+`"}`, ""); code != http.StatusNotFound {
		t.Fatalf("a stranger's delete: %d", code)
	}
	if code, out := send(s, tok("admin@h"), "POST", "/schedule/add", `{"record":"jobs@h","in":"1m","target":"out@h","body":""}`, ""); code != http.StatusBadRequest || !strings.Contains(out, "empty message") {
		t.Fatalf("an empty body: %d %s", code, out)
	}
	if code, out := send(s, tok("admin@h"), "POST", "/schedule/delete", `{"id":"`+id+`"}`, ""); code != http.StatusOK || !strings.Contains(out, `"deleted":true`) {
		t.Fatalf("delete: %d %s", code, out)
	}
}

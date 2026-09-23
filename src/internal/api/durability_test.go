package api

import (
	"errors"
	"testing"

	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func TestPersistenceFailureIsAnHTTPFailureAndCanBeRetried(t *testing.T) {
	b, s, token := groupFixture(t)
	d := memory.NewState()
	d.Err = errors.New("fixture database failed")
	b.Persistence(d)
	if code, body := post(t, s, token, "admin@h", "/user/state", `{"kind":"agent","name":"plain@h","state":"banned"}`); code != 500 {
		t.Fatalf("failed persistence answered %d, want 500: %s", code, body)
	}
	// A write whose commit failed published nothing.
	if err := b.Authenticate("plain@h"); err != nil {
		t.Fatalf("a ban whose commit failed took effect: %v", err)
	}
	d.Err = nil
	if code, body := post(t, s, token, "admin@h", "/user/state", `{"kind":"agent","name":"plain@h","state":"banned"}`); code != 200 {
		t.Fatalf("retry after disk recovery answered %d: %s", code, body)
	}
	if d.Commits != 1 {
		t.Fatalf("retry did not persist before success: %d commits", d.Commits)
	}
	if b.Authenticate("plain@h") == nil {
		t.Fatal("the retried ban did not take effect")
	}
}

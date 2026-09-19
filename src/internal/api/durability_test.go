package api

import (
	"errors"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
)

type failingAdministrativeStore struct {
	fail   bool
	writes int
}

func (d *failingAdministrativeStore) Save(ports.Snapshot) error {
	if d.fail {
		return errors.New("fixture snapshot disk failed")
	}
	d.writes++
	return nil
}
func (*failingAdministrativeStore) Load() (ports.Snapshot, bool, error) {
	return ports.Snapshot{}, false, nil
}

func TestPersistenceFailureIsAnHTTPFailureAndCanBeRetried(t *testing.T) {
	b, s, token := groupFixture(t)
	d := &failingAdministrativeStore{fail: true}
	b.Persistence(d)
	if code, body := post(t, s, token, "admin@h", "/user/state", `{"kind":"agent","name":"plain@h","state":"banned"}`); code != 500 {
		t.Fatalf("failed persistence answered %d, want 500: %s", code, body)
	}
	// No rollback is promised: the applied restriction stays in memory while
	// the caller learns that persistence was not acknowledged.
	if b.Authenticate("plain@h") == nil {
		t.Fatal("failed write silently lifted the in-memory ban")
	}
	d.fail = false
	if code, body := post(t, s, token, "admin@h", "/user/state", `{"kind":"agent","name":"plain@h","state":"banned"}`); code != 200 {
		t.Fatalf("retry after disk recovery answered %d: %s", code, body)
	}
	if d.writes != 1 {
		t.Fatalf("retry did not persist before success: %d writes", d.writes)
	}
}

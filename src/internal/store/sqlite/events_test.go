package sqlite

import (
	"path/filepath"
	"testing"
)

func TestEventReservationsSurviveReopenAndNeverDecrease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.db")
	s, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ReserveEventIDs(7, 64); err != nil {
		t.Fatal(err)
	}
	if err = s.ReserveEventIDs(7, 63); err == nil {
		t.Fatal("reservation decreased")
	}
	s.Close()
	s, err = Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	n, err := s.EventHighWater(7)
	if err != nil || n != 64 {
		t.Fatalf("lost durable reservation: %d %v", n, err)
	}
	n, err = s.EventHighWater(8)
	if err != nil || n != 0 {
		t.Fatalf("new record inherited ID %d %v", n, err)
	}
	for _, bad := range []string{"-1", "064", "9007199254740992", "garbage"} {
		if _, err = s.db.Exec(`UPDATE meta SET value=? WHERE key='event-high:7'`, bad); err != nil {
			t.Fatal(err)
		}
		if _, err = s.EventHighWater(7); err == nil {
			t.Fatalf("damaged high-water %q accepted", bad)
		}
		if _, err = s.Load(); err == nil {
			t.Fatalf("Load accepted damaged high-water %q", bad)
		}
	}
}

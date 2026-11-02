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
		snap, er := s.Load()
		if er != nil || len(snap.EventIDIssues) != 1 {
			t.Fatalf("registry not isolated: %+v %v", snap.EventIDIssues, er)
		}
	}
}

func TestLoadIsolatesDamagedEventReservationKeys(t *testing.T) {
	for _, key := range []string{"event-high:0", "event-high:01", "event-high:x"} {
		t.Run(key, func(t *testing.T) {
			s, _ := open(t)
			if _, err := s.db.Exec(`INSERT INTO meta(key,value) VALUES(?, '64')`, key); err != nil {
				t.Fatal(err)
			}
			snap, err := s.Load()
			if err != nil || len(snap.EventIDIssues) != 1 {
				t.Fatalf("missing damaged-key diagnostic: %v %v", snap.EventIDIssues, err)
			}
			if key == "event-high:01" {
				if _, err = s.EventHighWater(1); err == nil {
					t.Fatal("noncanonical counter reset to zero")
				}
			}
			if err = s.ReserveEventIDs(2, 64); err != nil {
				t.Fatal(err)
			}
			if n, err := s.EventHighWater(2); err != nil || n != 64 {
				t.Fatalf("other log blocked %d %v", n, err)
			}
		})
	}
}
func TestAFirstEventReservationAloneSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.db")
	s, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ReserveEventIDs(7, 64); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if n, err := s.EventHighWater(7); err != nil || n != 64 {
		t.Fatalf("lost first reservation %d %v", n, err)
	}
}

package memory

import (
	"errors"
	"testing"
)

func TestEventReservationsAreMonotonicAndSeparate(t *testing.T) {
	s := NewState()
	if err := s.ReserveEventIDs(1, 64); err != nil {
		t.Fatal(err)
	}
	if err := s.ReserveEventIDs(1, 63); err == nil {
		t.Fatal("reservation decreased")
	}
	n, err := s.EventHighWater(1)
	if err != nil || n != 64 {
		t.Fatalf("reservation %d %v", n, err)
	}
	n, err = s.EventHighWater(2)
	if err != nil || n != 0 {
		t.Fatalf("record ID not isolated %d %v", n, err)
	}
	s.Err = errors.New("failed store")
	if err = s.ReserveEventIDs(1, 128); err == nil {
		t.Fatal("accepted failed reservation")
	}
	s.Err = nil
	n, _ = s.EventHighWater(1)
	if n != 64 {
		t.Fatalf("failed write published %d", n)
	}
}

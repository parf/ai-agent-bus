package bigmem

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
)

var lim = ports.BigLimits{Value: 10, Record: 15, Total: 25, Names: 3}

func put(s *Store, record uint32, name, data string, now time.Time) error {
	_, _, err := s.BigPut(record, name, strings.NewReader(data), "", false, now.Add(time.Hour), now, lim)
	return err
}

func TestTotalsAndCountsAreEnforcedAndReplaceDoesNotCountTwice(t *testing.T) {
	s, now := New(), time.Now()
	if err := put(s, 1, "a", "0123456789", now); err != nil {
		t.Fatal(err)
	}
	if err := put(s, 1, "b", "012345", now); !errors.Is(err, ports.ErrBigFull) {
		t.Fatalf("past the record total = %v", err)
	}
	if err := put(s, 1, "a", "01234567", now); err != nil {
		t.Fatalf("a replace within the record total = %v", err)
	}
	if err := put(s, 2, "a", "0123456789", now); err != nil {
		t.Fatal(err)
	}
	if err := put(s, 3, "a", "0123456789", now); !errors.Is(err, ports.ErrBigFull) {
		t.Fatalf("past the daemon total = %v", err)
	}
	for _, n := range []string{"x", "y"} {
		if err := put(s, 4, n, "1", now); err != nil {
			t.Fatal(err)
		}
	}
	if err := put(s, 4, "z", "1", now); err != nil {
		t.Fatal(err)
	}
	if err := put(s, 4, "w", "1", now); !errors.Is(err, ports.ErrBigFull) {
		t.Fatalf("a fourth key = %v", err)
	}
}

func TestReadsHonourExpiryAndDropRemovesARecord(t *testing.T) {
	s, now := New(), time.Now()
	_, _, err := s.BigPut(1, "k", bytes.NewReader([]byte("x")), "", false, now.Add(time.Second), now, lim)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, found, _ := s.BigGet(1, "k", now); !found {
		t.Fatal("a live key was not found")
	}
	if _, _, found, _ := s.BigGet(1, "k", now.Add(time.Second)); found {
		t.Fatal("a key at its expiry was found")
	}
	if err := put(s, 1, "k", "x", now); err != nil {
		t.Fatal(err)
	}
	s.BigDrop(1)
	if l, _ := s.BigList(1, now); len(l) != 0 {
		t.Fatalf("a dropped record kept %+v", l)
	}
}

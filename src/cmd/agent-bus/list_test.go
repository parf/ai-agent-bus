package main

import "testing"

func TestReaderCountDistinguishesUnavailableFromMeasuredZero(t *testing.T) {
	zero := 0
	two := 2
	if got := readerCount(nil); got != "unavailable" {
		t.Fatalf("absent reader count = %q", got)
	}
	if got := readerCount(&zero); got != "0" {
		t.Fatalf("measured zero reader count = %q", got)
	}
	if got := readerCount(&two); got != "2" {
		t.Fatalf("measured nonzero reader count = %q", got)
	}
}

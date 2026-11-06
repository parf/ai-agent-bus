package main

import (
	"errors"
	"fmt"
	"testing"
)

// C2: a failing sweep is said once per fault, and once when sweeps work again.
func TestASweepFaultIsLoggedOncePerStateChange(t *testing.T) {
	var lines []string
	logf := func(f string, a ...any) { lines = append(lines, fmt.Sprintf(f, a...)) }
	fault, other := errors.New("index locked"), errors.New("disk I/O error")
	failing := ""
	for _, err := range []error{nil, fault, fault, fault, other, nil, nil} {
		failing = sweepReport(failing, err, logf)
	}
	want := []string{
		"bigfile sweep: expired bigfiles were not removed: index locked; the next sweep tries again",
		"bigfile sweep: expired bigfiles were not removed: disk I/O error; the next sweep tries again",
		"bigfile sweep: expired bigfiles are removed again",
	}
	if fmt.Sprint(lines) != fmt.Sprint(want) {
		t.Fatalf("lines = %q\nwant  %q", lines, want)
	}
}

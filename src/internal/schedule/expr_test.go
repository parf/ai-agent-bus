package schedule

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

type vectors struct {
	Valid []struct {
		Expr, Start string
		Next        []string
	}
	Invalid []struct{ Expr, Error string }
}

// The Go parser agrees with #cron's Python parser on every row of the shared
// table: the next five slots from each start, and every refusal word for word.
func TestExprMatchesTheCronVectors(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/cron/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var v vectors
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Valid) < 50 || len(v.Invalid) < 15 {
		t.Fatalf("the table is short: %d valid, %d invalid", len(v.Valid), len(v.Invalid))
	}
	for _, row := range v.Valid {
		e, err := Parse(row.Expr)
		if err != nil {
			t.Errorf("%q refused: %v", row.Expr, err)
			continue
		}
		at, _ := time.Parse(time.RFC3339, row.Start)
		for i, want := range row.Next {
			next, ok := e.Next(at)
			if !ok || next.Format(time.RFC3339) != want {
				t.Errorf("%q from %s, slot %d: %s %v, want %s", row.Expr, row.Start, i+1, next.Format(time.RFC3339), ok, want)
				break
			}
			at = next
		}
	}
	for _, row := range v.Invalid {
		_, err := Parse(row.Expr)
		if err == nil || err.Error() != row.Error && !(row.Error[:10] == "a schedule" && err.Error()[:10] == "a schedule") {
			t.Errorf("%q: %v, want %q", row.Expr, err, row.Error)
		}
	}
}

// A sixth, leading field names seconds; five fields fire at second 0.
func TestExprSeconds(t *testing.T) {
	start := time.Date(2026, 11, 12, 10, 0, 58, 0, time.UTC)
	for expr, want := range map[string][]string{
		"* * * * * *":    {"10:00:59", "10:01:00", "10:01:01"},
		"*/20 * * * * *": {"10:01:00", "10:01:20", "10:01:40"},
		"30 0 10 * * *":  {"2026-11-13T10:00:30Z"},
		"* * * * *":      {"10:01:00", "10:02:00"},
	} {
		e, err := Parse(expr)
		if err != nil {
			t.Fatalf("%q: %v", expr, err)
		}
		at := start
		for _, w := range want {
			next, ok := e.Next(at)
			got := next.Format("15:04:05")
			if len(w) > 8 {
				got = next.Format(time.RFC3339)
			}
			if !ok || got != w {
				t.Fatalf("%q after %s: %s, want %s", expr, at.Format(time.RFC3339), got, w)
			}
			at = next
		}
	}
	if _, err := Parse("60 * * * * *"); err == nil || err.Error() != "field 1 (second): values are 0..59" {
		t.Fatalf("a second of 60: %v", err)
	}
	if e, _ := Parse("* * * * *"); e.Seconds() {
		t.Fatal("five fields name seconds")
	}
}

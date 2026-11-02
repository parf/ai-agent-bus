package subfilter

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

var loneSurrogate = regexp.MustCompile(`(?i)\\ud[89ab][0-9a-f]{2}(\\ud[c-f][0-9a-f]{2})?|\\ud[c-f][0-9a-f]{2}`)

// decode is the string in a JSON value, and whether it holds a lone surrogate
// escape: a Go string cannot, so such a value becomes invalid UTF-8, which is
// what the engine is to refuse.
func decode(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	for _, m := range loneSurrogate.FindAllString(string(raw), -1) {
		if len(m) == 6 { // one escape alone: a lone surrogate
			return "\xed\xa0\x80"
		}
	}
	return s
}

// The shared vectors run against the Go engine: the same 111 filter cases the
// Python and TypeScript engines pass.
func TestSharedFilterVectors(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/subscriptions.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors struct {
		Filters []struct {
			Filter json.RawMessage
			Text   json.RawMessage
			Match  *bool
			Error  string
		}
	}
	if err = json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}
	if len(vectors.Filters) != 111 {
		t.Fatalf("%d filter vectors, want 111: the file changed", len(vectors.Filters))
	}
	for i, v := range vectors.Filters {
		filter := decode(t, v.Filter)
		var text string
		if v.Text != nil {
			text = decode(t, v.Text)
		}
		name := strings.NewReplacer("\n", "\\n", "\r", "\\r").Replace(filter)
		if len(name) > 40 {
			name = name[:40]
		}
		t.Run(name, func(t *testing.T) {
			f, err := Compile(filter)
			if v.Error == "invalid_filter" {
				if !errors.Is(err, ErrInvalidFilter) {
					t.Fatalf("vector %d: filter %q compiled (%v), want invalid_filter", i, filter, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("vector %d: filter %q refused: %v", i, filter, err)
			}
			got, err := f.Match(text, time.Second)
			if v.Error == "invalid_text" {
				if !errors.Is(err, ErrInvalidText) {
					t.Fatalf("vector %d: text %q gave %v, %v; want invalid_text", i, text, got, err)
				}
				return
			}
			if err != nil || got != *v.Match {
				t.Fatalf("vector %d: %q on %q = %v, %v; want %v", i, filter, text, got, err, *v.Match)
			}
		})
	}
}

// A pathological pattern runs out of its budget and says so; it does not hang.
func TestAPathologicalPatternTimesOutAndOthersGoOn(t *testing.T) {
	f, err := Compile(`/(a|a)*$/`)
	if err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		ok  bool
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		ok, err := f.Match(strings.Repeat("a", 4000)+"b", 50*time.Millisecond)
		done <- outcome{ok, err}
	}()
	select {
	case got := <-done:
		if !errors.Is(got.err, ErrTimeout) {
			t.Fatalf("a pathological search gave %v, %v; want filter_timeout", got.ok, got.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a 50 ms budget did not end the search within 5 s")
	}
	g, _ := Compile(`/b/`)
	if ok, err := g.Match("abc", time.Second); err != nil || !ok {
		t.Fatalf("a fresh filter after a timeout: %v %v", ok, err)
	}
}

// Legacy ignore-case leaves a character with a multi-character uppercase
// alone, as ECMAScript's Canonicalize does; with u it folds by Unicode simple
// folding. The shared vectors do not cover it; Python agrees on both.
func TestLegacyIgnoreCaseLeavesMultiCharacterUppercaseAlone(t *testing.T) {
	for _, c := range []struct {
		filter, text string
		want         bool
	}{
		{"/^\u1f80$/i", "\u1f88", false},
		{"/^\u1f80$/iu", "\u1f88", true},
		{"/^\u00df$/i", "\u1e9e", false},
		{"/^.$/s", "\\n", true},
		{"/^.$/s", "\\u2028", true},
		{"/^.$/", "\\n", false},
		{"/^a{2,}$/", "aaa", true},
		{"/^a{2,3}$/", "aaaa", false},
		{"/(ab){2}/", "abab", true},
	} {
		filter, text := unescape(c.filter), unescape(c.text)
		f, err := Compile(filter)
		if err != nil {
			t.Fatalf("%q: %v", c.filter, err)
		}
		if got, err := f.Match(text, 0); err != nil || got != c.want {
			t.Errorf("%q on %q = %v, %v; want %v", c.filter, c.text, got, err, c.want)
		}
	}
}

// unescape reads the \uXXXX escapes of a case written without Go's own.
func unescape(s string) string {
	var out string
	if err := json.Unmarshal([]byte(`"`+s+`"`), &out); err != nil {
		return s
	}
	return out
}

// Text is refused for every filter, the empty one included, before any match.
func TestInvalidTextIsRefusedEvenByTheEmptyFilter(t *testing.T) {
	f, _ := Compile("")
	for name, text := range map[string]string{"too long": strings.Repeat("x", MaxText+1), "not utf-8": "\xff"} {
		if _, err := f.Match(text, 0); !errors.Is(err, ErrInvalidText) {
			t.Errorf("%s: %v, want invalid_text", name, err)
		}
	}
	if ok, err := f.Match(strings.Repeat("x", MaxText), 0); err != nil || !ok {
		t.Errorf("text at the limit: %v %v", ok, err)
	}
}

// matchBounded runs a match and fails on an assertion, not on the go test
// timeout, if it does not end.
func matchBounded(t *testing.T, filter, text string, budget time.Duration) (bool, error, time.Duration) {
	t.Helper()
	f, err := Compile(filter)
	if err != nil {
		t.Fatalf("%.60q: %v", filter, err)
	}
	type outcome struct {
		ok   bool
		err  error
		took time.Duration
	}
	done := make(chan outcome, 1)
	go func() {
		began := time.Now()
		ok, err := f.Match(text, budget)
		done <- outcome{ok, err, time.Since(began)}
	}()
	select {
	case got := <-done:
		return got.ok, got.err, got.took
	case <-time.After(5 * time.Second):
		t.Fatalf("%.60q: a %s budget did not end the search within 5 s", filter, budget)
		return false, nil, 0
	}
}

// Zero-width work is charged to the budget like any other: 2^60 empty
// alternation branches, nested empty groups and empty alternation under bounds
// end at the budget with filter_timeout, each one required to, not allowed to;
// an empty quantified group is linear and finishes with its exact answer.
func TestZeroWidthShapesRespectTheBudget(t *testing.T) {
	for name, c := range map[string]struct {
		filter  string
		timeout bool
	}{
		"60 empty alternations then anchors": {"/" + strings.Repeat("(|)", 60) + "^$/", true},
		"empty alternations with letters":    {"/" + strings.Repeat("(|a|)", 39) + "$^/", true},
		"nested empty groups":                {"/" + strings.Repeat("((|)(|))", 24) + "^$/", true},
		"empty alternation under bounds":     {"/(|){100}(|){100}(|){100}(|){100}y/", true},
		"an empty quantified group":          {"/(?:)*(?:)*(?:)*(?:)*(?:)*(?:)*(?:)*(?:)*(?:)*(?:)*y/", false},
	} {
		t.Run(name, func(t *testing.T) {
			ok, err, took := matchBounded(t, c.filter, "x", time.Millisecond)
			switch {
			case c.timeout && !errors.Is(err, ErrTimeout):
				t.Fatalf("got %v, %v; want filter_timeout", ok, err)
			case !c.timeout && (err != nil || ok):
				t.Fatalf("got %v, %v; want a finished no-match", ok, err)
			}
			if took > 2*time.Second {
				t.Fatalf("ran %s past a 1 ms budget", took)
			}
		})
	}
	// The step count alone bounds it too, with no deadline given.
	if _, err, _ := matchBounded(t, "/"+strings.Repeat("(|)", 60)+"^$/", "x", 0); !errors.Is(err, ErrTimeout) {
		t.Fatalf("with no deadline the step bound gave %v, want filter_timeout", err)
	}
}

// Package subfilter is the portable subscription filter: the regular-expression
// subset ECMAScript and Python agree on (docs/agents/subscriptions.md#the-shared-behaviour,
// src/agents/shared/subscriptions.md#common-filter-grammar), matched with
// ECMAScript's semantics.
//
// The grammar is checked by the parser below, which refuses everything outside
// the subset; matching is a bounded backtracking search over UTF-16 code units
// (no u flag) or Unicode scalar values (u flag). Go's own regexp package is not
// used: its syntax and its anchors differ (m recognises only LF, no engine
// agrees on legacy ignore-case), and one dialect for every runtime is the point.
package subfilter

import (
	"errors"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	MaxPattern     = 200  // Unicode scalar characters of the pattern
	MaxText        = 8192 // UTF-8 bytes of the text a filter is tested against
	MaxQuantifiers = 20
	MaxRepeat      = 100
	maxFilterBytes = 4096 // the whole stored filter, flags and slashes included
	maxSteps       = 4_000_000
)

var (
	ErrInvalidFilter = errors.New("invalid_filter")
	ErrInvalidText   = errors.New("invalid_text")
	ErrTimeout       = errors.New("filter_timeout")
)

// Filter is a compiled filter; it holds no per-match state and may be shared.
type Filter struct {
	prog    matcher
	unicode bool
	fold    func(rune) rune // nil when the filter is case sensitive
}

type machine struct {
	t        []rune
	steps    int
	deadline time.Time
	abort    bool
}

type matcher func(m *machine, i int, k func(int) bool) bool

func (m *machine) tick() bool {
	if m.abort {
		return false
	}
	m.steps++
	if m.steps > maxSteps || m.steps&1023 == 0 && !m.deadline.IsZero() && time.Now().After(m.deadline) {
		m.abort = true
		return false
	}
	return true
}

func isTerminator(r rune) bool { return r == '\n' || r == '\r' || r == 0x2028 || r == 0x2029 }

// units is s as UTF-16 code units, each held in a rune: the no-u view of text.
func units(s string) []rune {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= 0x10000 {
			r -= 0x10000
			out = append(out, 0xD800+r>>10, 0xDC00+r&0x3FF)
		} else {
			out = append(out, r)
		}
	}
	return out
}

// legacyFold is ECMAScript's Canonicalize without u: the single-character
// uppercase, never a multi-character one, never a non-ASCII unit folded into
// ASCII (so U+212A and U+017F stay apart from k and s).
func legacyFold(c rune) rune {
	if c < 128 {
		if c >= 'a' && c <= 'z' {
			return c - 32
		}
		return c
	}
	if multiUpper[c] {
		return c
	}
	if u := unicode.ToUpper(c); u >= 128 {
		return u
	}
	return c
}

// unicodeFold is simple case folding (Unicode C and S mappings): one
// representative of the character's orbit, so two characters fold alike
// exactly when simple folding makes them equal.
func unicodeFold(c rune) rune {
	low := c
	for f := unicode.SimpleFold(c); f != c; f = unicode.SimpleFold(f) {
		if f < low {
			low = f
		}
	}
	return low
}

// Compile parses a stored filter: a bare pattern, or /pattern/flags with the
// flags i m s u once each. The empty filter, and //, match everything.
func Compile(value string) (*Filter, error) {
	if len(value) > maxFilterBytes || !utf8.ValidString(value) {
		return nil, ErrInvalidFilter
	}
	pattern, flags, err := split([]rune(value))
	if err != nil {
		return nil, err
	}
	f := &Filter{unicode: flags['u']}
	if flags['i'] {
		f.fold = legacyFold
		if f.unicode {
			f.fold = unicodeFold
		}
	}
	p := pattern
	if !f.unicode {
		p = units(string(pattern))
	}
	prog, err := parse(p, flags, f.fold)
	if err != nil {
		return nil, err
	}
	f.prog = prog
	return f, nil
}

func split(v []rune) ([]rune, map[rune]bool, error) {
	flags := map[rune]bool{}
	if len(v) > 0 && v[0] == '/' {
		var slashes []int
		escaped := false
		for i := 1; i < len(v); i++ {
			switch {
			case escaped:
				escaped = false
			case v[i] == '\\':
				escaped = true
			case v[i] == '/':
				slashes = append(slashes, i)
			}
		}
		if len(slashes) == 0 {
			return nil, nil, ErrInvalidFilter
		}
		end := slashes[len(slashes)-1]
		for _, c := range v[end+1:] {
			if flags[c] || c != 'i' && c != 'm' && c != 's' && c != 'u' {
				return nil, nil, ErrInvalidFilter
			}
			flags[c] = true
		}
		v = v[1:end]
	}
	if len(v) > MaxPattern {
		return nil, nil, ErrInvalidFilter
	}
	for _, c := range v {
		if unicode.Is(unicode.Cc, c) {
			return nil, nil, ErrInvalidFilter
		}
	}
	return v, flags, nil
}

var (
	jsSpace = func() map[rune]bool {
		out := map[rune]bool{}
		for _, c := range []rune{0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x20, 0xA0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF} {
			out[c] = true
		}
		for c := rune(0x2000); c <= 0x200a; c++ {
			out[c] = true
		}
		return out
	}()
	jsWord = func() map[rune]bool {
		out := map[rune]bool{'_': true}
		for c := 'a'; c <= 'z'; c++ {
			out[c], out[c-32] = true, true
		}
		for c := '0'; c <= '9'; c++ {
			out[c] = true
		}
		return out
	}()
	jsDigit = func() map[rune]bool {
		out := map[rune]bool{}
		for c := '0'; c <= '9'; c++ {
			out[c] = true
		}
		return out
	}()
)

type frame struct {
	alts [][]matcher
	cur  []matcher
	had  bool // a quantifier was seen inside this group
}

func (f *frame) alt() matcher {
	alts := append(append([][]matcher{}, f.alts...), f.cur)
	branches := make([]matcher, len(alts))
	for i, seq := range alts {
		branches[i] = sequence(seq)
	}
	if len(branches) == 1 {
		return branches[0]
	}
	return func(m *machine, i int, k func(int) bool) bool {
		for _, b := range branches {
			if b(m, i, k) {
				return true
			}
			if m.abort {
				return false
			}
		}
		return false
	}
}

func sequence(ms []matcher) matcher {
	return func(m *machine, i int, k func(int) bool) bool {
		var run func(idx, pos int) bool
		run = func(idx, pos int) bool {
			if idx == len(ms) {
				return k(pos)
			}
			return ms[idx](m, pos, func(j int) bool { return run(idx+1, j) })
		}
		return run(0, i)
	}
}

func repeat(x matcher, lo, hi int) matcher { // hi < 0 is unbounded
	return func(m *machine, i int, k func(int) bool) bool {
		var try func(pos, count int) bool
		try = func(pos, count int) bool {
			if !m.tick() {
				return false
			}
			if hi < 0 || count < hi {
				if x(m, pos, func(j int) bool {
					if j == pos && count >= lo {
						return false // an empty iteration adds nothing
					}
					return try(j, count+1)
				}) {
					return true
				}
			}
			return count >= lo && k(pos)
		}
		return try(i, 0)
	}
}

func one(test func(r rune) bool) matcher {
	return func(m *machine, i int, k func(int) bool) bool {
		if !m.tick() || i >= len(m.t) || !test(m.t[i]) {
			return false
		}
		return k(i + 1)
	}
}

func parse(p []rune, flags map[rune]bool, fold func(rune) rune) (matcher, error) {
	canon := func(c rune) rune {
		if fold != nil {
			return fold(c)
		}
		return c
	}
	literal := func(c rune) matcher {
		want := canon(c)
		return one(func(r rune) bool { return r == want })
	}
	class := func(values map[rune]bool, negative bool) matcher {
		set := map[rune]bool{}
		for c := range values {
			set[canon(c)] = true
		}
		return one(func(r rune) bool { return set[r] != negative })
	}
	frames := []*frame{{}}
	top := func() *frame { return frames[len(frames)-1] }
	add := func(m matcher) { top().cur = append(top().cur, m) }
	last := ""
	quantifiers := 0
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch {
		case c == '\\':
			i++
			if i == len(p) {
				return nil, ErrInvalidFilter
			}
			e := p[i]
			switch {
			case e == 'd' || e == 'D' || e == 'w' || e == 'W' || e == 's' || e == 'S':
				values := jsDigit
				switch e | 0x20 {
				case 'w':
					values = jsWord
				case 's':
					values = jsSpace
				}
				add(class(values, e < 'a'))
			case e == 'n':
				add(literal('\n'))
			case e == 'r':
				add(literal('\r'))
			case e == 't':
				add(literal('\t'))
			case e < 128 && containsRune(`\./-^$|?*+(){}[]`, e):
				add(literal(e))
			default:
				return nil, ErrInvalidFilter
			}
			last = "atom"
		case c == '[':
			end := -1
			for j := i + 1; j < len(p); j++ {
				if p[j] == ']' {
					end = j
					break
				}
			}
			if end < 0 {
				return nil, ErrInvalidFilter
			}
			negative, values, err := characterClass(p[i+1 : end])
			if err != nil {
				return nil, err
			}
			add(class(values, negative))
			i = end
			last = "atom"
		case c == '(':
			if i+1 < len(p) && p[i+1] == '?' {
				if i+2 >= len(p) || p[i+2] != ':' {
					return nil, ErrInvalidFilter
				}
				i += 2
			}
			frames = append(frames, &frame{})
			last = ""
		case c == ')':
			if len(frames) == 1 {
				return nil, ErrInvalidFilter
			}
			f := top()
			frames = frames[:len(frames)-1]
			add(f.alt())
			last = "atom"
			if f.had {
				last = "quantified_group"
			}
		case c == '*' || c == '+' || c == '?' || c == '{':
			if last != "atom" {
				return nil, ErrInvalidFilter
			}
			lo, hi := 0, -1
			switch c {
			case '+':
				lo = 1
			case '?':
				hi = 1
			case '{':
				var n int
				var ok bool
				lo, hi, n, ok = bound(p[i:])
				if !ok {
					return nil, ErrInvalidFilter
				}
				i += n - 1
			}
			quantifiers++
			if quantifiers > MaxQuantifiers {
				return nil, ErrInvalidFilter
			}
			cur := top().cur
			cur[len(cur)-1] = repeat(cur[len(cur)-1], lo, hi)
			for _, f := range frames[1:] {
				f.had = true
			}
			last = "quantifier"
		case c == '}' || c == ']':
			return nil, ErrInvalidFilter
		case c == '|':
			f := top()
			f.alts = append(f.alts, f.cur)
			f.cur = nil
			last = ""
		case c == '.':
			if flags['s'] {
				add(one(func(rune) bool { return true }))
			} else {
				add(one(func(r rune) bool { return !isTerminator(r) }))
			}
			last = "atom"
		case c == '^':
			if flags['m'] {
				add(func(m *machine, i int, k func(int) bool) bool { return (i == 0 || isTerminator(m.t[i-1])) && k(i) })
			} else {
				add(func(m *machine, i int, k func(int) bool) bool { return i == 0 && k(i) })
			}
			last = "anchor"
		case c == '$':
			if flags['m'] {
				add(func(m *machine, i int, k func(int) bool) bool { return (i == len(m.t) || isTerminator(m.t[i])) && k(i) })
			} else {
				add(func(m *machine, i int, k func(int) bool) bool { return i == len(m.t) && k(i) })
			}
			last = "anchor"
		default:
			add(literal(c))
			last = "atom"
		}
	}
	if len(frames) != 1 {
		return nil, ErrInvalidFilter
	}
	return frames[0].alt(), nil
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}

// bound reads {n}, {n,} or {n,m} at the start of p: its bounds (hi < 0 is
// unbounded), its length, and whether it was one. Every explicit bound is at
// most MaxRepeat and lo is at most hi.
func bound(p []rune) (lo, hi, n int, ok bool) {
	i := 1
	number := func() (int, bool) {
		start, v := i, 0
		for i < len(p) && p[i] >= '0' && p[i] <= '9' {
			if i-start >= 9 {
				return 0, false
			}
			v = v*10 + int(p[i]-'0')
			i++
		}
		return v, i > start
	}
	lo, found := number()
	if !found {
		return 0, 0, 0, false
	}
	hi = lo
	if i < len(p) && p[i] == ',' {
		i++
		hi = -1
		if v, f := number(); f {
			hi = v
		}
	}
	if i >= len(p) || p[i] != '}' {
		return 0, 0, 0, false
	}
	if lo > MaxRepeat || hi > MaxRepeat || hi >= 0 && lo > hi {
		return 0, 0, 0, false
	}
	return lo, hi, i + 1, true
}

// characterClass is the body of [...]: ASCII literals and ranges only.
func characterClass(body []rune) (negative bool, values map[rune]bool, err error) {
	if len(body) == 0 {
		return false, nil, ErrInvalidFilter
	}
	for i, c := range body {
		if c >= 128 || c == '[' || c == '\\' {
			return false, nil, ErrInvalidFilter
		}
		if i+1 < len(body) {
			switch string([]rune{c, body[i+1]}) {
			case "&&", "--", "~~", "||":
				return false, nil, ErrInvalidFilter
			}
		}
	}
	if body[0] == '^' {
		negative, body = true, body[1:]
	}
	if len(body) == 0 {
		return false, nil, ErrInvalidFilter
	}
	hyphens := 0
	for _, c := range body {
		if c == '-' {
			hyphens++
		}
	}
	if hyphens > 1 {
		return false, nil, ErrInvalidFilter
	}
	values = map[rune]bool{}
	for i := 0; i < len(body); {
		if i+2 < len(body) && body[i+1] == '-' {
			if body[i] > body[i+2] {
				return false, nil, ErrInvalidFilter
			}
			for c := body[i]; c <= body[i+2]; c++ {
				values[c] = true
			}
			i += 3
		} else {
			values[body[i]] = true
			i++
		}
	}
	return negative, values, nil
}

// Match reports whether the filter finds a match anywhere in text, within the
// budget (zero is the step bound alone). Text over MaxText bytes or not valid
// UTF-8 is ErrInvalidText for every filter, the empty one included; a search
// that runs out of budget is ErrTimeout.
func (f *Filter) Match(text string, budget time.Duration) (bool, error) {
	if len(text) > MaxText || !utf8.ValidString(text) {
		return false, ErrInvalidText
	}
	var t []rune
	if f.unicode {
		t = []rune(text)
	} else {
		t = units(text)
	}
	if f.fold != nil {
		for i, c := range t {
			t[i] = f.fold(c)
		}
	}
	m := &machine{t: t}
	if budget > 0 {
		m.deadline = time.Now().Add(budget)
	}
	for start := 0; start <= len(t); start++ {
		if f.prog(m, start, func(int) bool { return true }) {
			return true, nil
		}
		if m.abort {
			return false, ErrTimeout
		}
	}
	return false, nil
}

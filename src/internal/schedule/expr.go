// Package schedule holds the daemon's schedule rules that need no bus: the
// cron expression and the time forms (Plans/R1.0-Release/cron-in-core.md).
package schedule

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Expr is a parsed cron expression: five fields (minute hour day-of-month
// month day-of-week) firing at second 0, or six with a leading seconds field.
// Day of week 0 and 7 are Sunday; Vixie's rule joins the two day fields.
// Every time is UTC.
type Expr struct {
	Text                           string
	second, minute, hour, dom, mon set
	dow                            set
	domStar, dowStar               bool
	seconds                        bool
}

type set map[int]bool

func (s set) sorted() []int {
	out := make([]int, 0, len(s))
	for v := range s {
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}

type spec struct {
	name      string
	low, high int
}

var (
	fieldSpecs = []spec{{"minute", 0, 59}, {"hour", 0, 23}, {"day of month", 1, 31}, {"month", 1, 12}, {"day of week", 0, 7}}
	secondSpec = spec{"second", 0, 59}
	element    = regexp.MustCompile(`^(\*|([0-9]{1,2})(?:-([0-9]{1,2}))?)(?:/([0-9]{1,2}))?$`)
	shape      = regexp.MustCompile(`^[^ ]+( [^ ]+){4,5}$`)
	monthDays  = []int{31, 29, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
)

// horizonDays bounds a search: a February 29 schedule fires within eight years.
const horizonDays = 8 * 366

// Error is a refused expression, worded as #cron worded it.
type Error string

func (e Error) Error() string { return string(e) }

func field(text string, position int, s spec) (set, error) {
	values := set{}
	for _, item := range strings.Split(text, ",") {
		m := element.FindStringSubmatch(item)
		if m == nil {
			return nil, Error(fmt.Sprintf("field %d (%s): '%s' is not *, n, a-b or a step of them", position, s.name, item))
		}
		star, first, last, step := m[1] == "*", m[2], m[3], m[4]
		a, b := s.low, s.high
		if !star {
			a, _ = strconv.Atoi(first)
			b = a
			if last != "" {
				b, _ = strconv.Atoi(last)
			}
		}
		if step != "" && !star && last == "" {
			return nil, Error(fmt.Sprintf("field %d (%s): a step needs * or a range", position, s.name))
		}
		if a < s.low || a > s.high || b < s.low || b > s.high {
			return nil, Error(fmt.Sprintf("field %d (%s): values are %d..%d", position, s.name, s.low, s.high))
		}
		if a > b {
			return nil, Error(fmt.Sprintf("field %d (%s): range %d-%d runs backwards", position, s.name, a, b))
		}
		n := 1
		if step != "" {
			n, _ = strconv.Atoi(step)
		}
		if n == 0 {
			return nil, Error(fmt.Sprintf("field %d (%s): step 0", position, s.name))
		}
		for v := a; v <= b; v += n {
			values[v] = true
		}
	}
	if s.name == "day of week" && values[7] {
		delete(values, 7)
		values[0] = true
	}
	return values, nil
}

// Parse reads five or six fields separated by single spaces.
func Parse(text string) (*Expr, error) {
	if !shape.MatchString(text) {
		return nil, Error("a schedule is five fields separated by single spaces: minute hour day-of-month month day-of-week, or six with a leading second")
	}
	parts := strings.Split(text, " ")
	e := &Expr{Text: text, second: set{0: true}}
	offset := 0
	if len(parts) == 6 {
		sec, err := field(parts[0], 1, secondSpec)
		if err != nil {
			return nil, err
		}
		e.second, e.seconds, offset = sec, true, 1
	}
	sets := make([]set, 5)
	for i := 0; i < 5; i++ {
		v, err := field(parts[i+offset], i+1+offset, fieldSpecs[i])
		if err != nil {
			return nil, err
		}
		sets[i] = v
	}
	e.minute, e.hour, e.dom, e.mon, e.dow = sets[0], sets[1], sets[2], sets[3], sets[4]
	e.domStar, e.dowStar = strings.HasPrefix(parts[2+offset], "*"), strings.HasPrefix(parts[4+offset], "*")
	if !e.canFire() {
		return nil, Error("the schedule can never fire")
	}
	return e, nil
}

// day applies Vixie's rule: either day field starting with * makes the two
// an AND, else an OR.
func (e *Expr) day(d time.Time) bool {
	dom, dow := e.dom[d.Day()], e.dow[int(d.Weekday())]
	if e.domStar || e.dowStar {
		return dom && dow
	}
	return dom || dow
}

// canFire tries every allowed month and day across the 28-year calendar cycle.
func (e *Expr) canFire() bool {
	for _, m := range e.mon.sorted() {
		for d := 1; d <= monthDays[m-1]; d++ {
			for y := 2024; y < 2052; y++ {
				if m == 2 && d == 29 && y%4 != 0 {
					continue
				}
				if e.day(time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)) {
					return true
				}
			}
		}
	}
	return false
}

// Seconds says whether the expression names seconds.
func (e *Expr) Seconds() bool { return e.seconds }

// Next is the first slot strictly after t, in UTC, or false within the horizon.
func (e *Expr) Next(t time.Time) (time.Time, bool) {
	t = t.UTC().Truncate(time.Second)
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	hours, minutes, seconds := e.hour.sorted(), e.minute.sorted(), e.second.sorted()
	for i := 0; i < horizonDays; i++ {
		if e.mon[int(day.Month())] && e.day(day) {
			for _, h := range hours {
				if day.Equal(t.Truncate(24*time.Hour)) && h < t.Hour() {
					continue
				}
				for _, m := range minutes {
					for _, s := range seconds {
						c := time.Date(day.Year(), day.Month(), day.Day(), h, m, s, 0, time.UTC)
						if c.After(t) {
							return c, true
						}
					}
				}
			}
		}
		day = day.AddDate(0, 0, 1)
	}
	return time.Time{}, false
}

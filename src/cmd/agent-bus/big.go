// The big-key verbs: named values too big for a message, held by the daemon in
// a file or in memory (Plans/R1.0-Release/big-keys.md#help-text). This file
// checks what it can before any request and speaks the wire only through
// bigWire; the daemon decides access, size and expiry.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	bigDefaultTTL = 24 * time.Hour
	bigMaxTTL     = 30 * 24 * time.Hour
)

// bigPutPlan is a put with every local check done and nothing sent yet.
type bigPutPlan struct {
	Record, Name string
	Source       string // a file path, or "" for stdin
	TTL          time.Duration
	Memory       bool
}

// bigGetPlan is a get: Out is a file path, or "" for stdout.
type bigGetPlan struct{ Record, Name, Out string }

// parseBigTTL reads h, m and s durations and whole days: 90m, 24h, 30d.
func parseBigTTL(s string) (time.Duration, error) {
	var d time.Duration
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("--ttl is a duration such as 90m, 24h or 30d, not %q", s)
		}
		d = time.Duration(n) * 24 * time.Hour
	} else {
		var err error
		if d, err = time.ParseDuration(s); err != nil || d <= 0 {
			return 0, fmt.Errorf("--ttl is a duration such as 90m, 24h or 30d, not %q", s)
		}
	}
	if d > bigMaxTTL {
		return 0, fmt.Errorf("--ttl is at most 30d, not %q", s)
	}
	return d, nil
}

// bigEmpty is the one refusal for a zero-byte file and for empty stdin.
var bigEmpty = errors.New("big put wants a value with at least one byte; nothing was sent")

// takeShortO removes "-o <file>" from args: the option parser knows only --name.
func takeShortO(args []string) ([]string, string, bool, error) {
	var rest []string
	out, seen := "", false
	for i := 0; i < len(args); i++ {
		if args[i] != "-o" {
			rest = append(rest, args[i])
			continue
		}
		if seen || i+1 >= len(args) {
			return nil, "", false, fmt.Errorf("-o wants one file")
		}
		out, seen = args[i+1], true
		i++
	}
	return rest, out, seen, nil
}

func planBigPut(args []string, stdin io.Reader) (bigPutPlan, []byte, error) {
	pos, f, err := split(args)
	if err := only(f, err, "ttl", "memory"); err != nil {
		return bigPutPlan{}, nil, err
	}
	if len(pos) < 2 || len(pos) > 3 {
		return bigPutPlan{}, nil, fmt.Errorf("big put wants <record> <name> [<file>|-]")
	}
	p := bigPutPlan{Record: pos[0], Name: pos[1], TTL: bigDefaultTTL, Memory: has(f, "memory")}
	if v, ok := f["ttl"]; ok {
		if p.TTL, err = parseBigTTL(v); err != nil {
			return p, nil, err
		}
	}
	if len(pos) == 3 && pos[2] != "-" {
		p.Source = pos[2]
	}
	return p, nil, nil
}

func planBigGet(args []string) (bigGetPlan, error) {
	rest, out, seen, err := takeShortO(args)
	if err != nil {
		return bigGetPlan{}, err
	}
	pos, f, err := split(rest)
	if err := only(f, err); err != nil {
		return bigGetPlan{}, err
	}
	if len(pos) != 2 {
		return bigGetPlan{}, fmt.Errorf("big get wants <record> <name> [-o <file>]")
	}
	if seen && out == "" {
		return bigGetPlan{}, fmt.Errorf("-o wants one file")
	}
	return bigGetPlan{Record: pos[0], Name: pos[1], Out: out}, nil
}

// bigSource opens the put's bytes: a file, or stdin. A zero-byte file and
// empty stdin are refused alike, before any request.
func bigSource(p bigPutPlan, stdin io.Reader) (io.Reader, int64, error) {
	if p.Source == "" {
		b, err := io.ReadAll(stdin)
		if err != nil {
			return nil, 0, err
		}
		if len(b) == 0 {
			return nil, 0, bigEmpty
		}
		return strings.NewReader(string(b)), int64(len(b)), nil
	}
	st, err := os.Stat(p.Source)
	if err != nil {
		return nil, 0, err
	}
	if !st.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("%s is not a regular file", p.Source)
	}
	if st.Size() == 0 {
		return nil, 0, bigEmpty
	}
	f, err := os.Open(p.Source)
	return f, st.Size(), err
}

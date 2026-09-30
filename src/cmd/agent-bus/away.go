package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"time"
)

// A foreground runner outlives the daemon going away: a restart, a socket
// not there yet, a dropped forward or network. It waits and reconnects with
// backoff; only a refusal ends it. See docs/08-runner-role.md#script-agents.
var (
	awayFirst  = time.Second      // the first wait after a failed read
	awayMost   = 30 * time.Second // the longest wait between tries
	awayRemind = 5 * time.Minute  // how often a long absence is said again
	answerFor  = 2 * time.Minute  // how long an answer is retried without a caller deadline
	readGrace  = 15 * time.Second // a long poll with no answer this long past its wait is a dead link
)

// away is a request that never got an answer from the daemon: nothing
// listening, the connection cut, the body cut short. A refusal is not one.
type away struct{ err error }

func (a away) Error() string { return a.err.Error() }
func (a away) Unwrap() error { return a.err }

// isAway says whether a failure is the daemon being unreachable rather than
// the daemon saying no. A proxy or forward answering for a daemon it cannot
// reach says so with 502, 503 or 504.
func isAway(err error, code int) bool {
	var a away
	if errors.As(err, &a) {
		return true
	}
	return code == http.StatusBadGateway || code == http.StatusServiceUnavailable || code == http.StatusGatewayTimeout
}

// twoReads is the daemon refusing a read because the inbox already has one.
type twoReads struct{ err error }

func (t twoReads) Error() string { return t.err.Error() }
func (t twoReads) Unwrap() error { return t.err }

func isTwoReads(err error) bool {
	var t twoReads
	return errors.As(err, &t)
}

// link is the runner's view of the daemon: up, or away since when. The
// consume loop reports into it; the process title and the log read it.
type link struct {
	say  io.Writer
	name string

	mu    sync.Mutex
	since time.Time // zero while up
	told  time.Time
	wait  time.Duration
}

// note is the process title's addition: "away" while the daemon is.
func (l *link) note() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.since.IsZero() {
		return ""
	}
	return "away"
}

// recovering says the daemon went away less than d ago and has not been
// read from since.
func (l *link) recovering(d time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return !l.since.IsZero() && time.Since(l.since) < d
}

// down records a failed read and sleeps the backoff. It returns false when
// ctx ends first, which is a stop, not a failure.
func (l *link) down(ctx context.Context, err error) bool {
	l.mu.Lock()
	now := time.Now()
	switch {
	case l.since.IsZero():
		l.since, l.told, l.wait = now, now, awayFirst
		fmt.Fprintf(l.say, "%s: daemon away: %v; retrying\n", l.name, err)
	case now.Sub(l.told) >= awayRemind:
		l.told = now
		fmt.Fprintf(l.say, "%s: daemon still away after %s: %v\n", l.name, now.Sub(l.since).Round(time.Second), err)
	}
	wait := l.wait
	l.wait = min(l.wait*2, awayMost)
	l.mu.Unlock()
	return sleep(ctx, jitter(wait))
}

// up records a read that reached the daemon.
func (l *link) up() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.since.IsZero() {
		return
	}
	fmt.Fprintf(l.say, "%s: reconnected after %s\n", l.name, time.Since(l.since).Round(time.Second))
	l.since = time.Time{}
}

// jitter spreads the tries of many runners after one restart: 75–125 %.
func jitter(d time.Duration) time.Duration {
	return d*3/4 + rand.N(d/2+1)
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// deliver posts one answer or receipt, retrying while the daemon is away
// until until passes or ctx ends. A refusal is not retried.
func deliver(ctx context.Context, until time.Time, path string, body any) error {
	wait := awayFirst
	for {
		out, code, err := call("POST", path, nil, body)
		if err == nil && code < 400 {
			return nil
		}
		if !isAway(err, code) {
			if err != nil {
				return err
			}
			return fmt.Errorf("%s", strings.TrimSpace(string(out)))
		}
		left := time.Until(until)
		if left <= 0 {
			return fmt.Errorf("daemon away: %v", orStatus(err, code))
		}
		if !sleep(ctx, min(jitter(wait), left)) {
			return fmt.Errorf("stopped while the daemon was away: %v", orStatus(err, code))
		}
		wait = min(wait*2, awayMost)
	}
}

func orStatus(err error, code int) any {
	if err != nil {
		return err
	}
	return http.StatusText(code)
}

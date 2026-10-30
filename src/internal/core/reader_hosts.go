package core

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Where a pool member says it is (Plans/R1.0-Release/runner-implementation.md#where-a-member-says-it-is).
// A read may carry the reader's hostname; the daemon keeps, per name, when each
// host last read. It is a label for the person reading a listing: stated, never
// observed, never an input to routing, authority or counting, and held in
// memory only — never written to the database or a backup.

const (
	// hostWindow is how long after a host's last read began or ended it stays
	// listed: longer than the runner's 55-second read, so a full pool whose
	// places are all busy keeps its members.
	hostWindow = 2 * time.Minute
	// maxHosts bounds the hosts kept per name; past it the oldest is forgotten
	// and the listing says it is cut.
	maxHosts = 32
	// maxHostLen is the longest hostname a read may state.
	maxHostLen = 64
)

type hostKey struct{}

// WithHost says a read comes from the named host.
func WithHost(ctx context.Context, host string) context.Context {
	return context.WithValue(ctx, hostKey{}, host)
}

// CheckHost refuses a hostname that is not 1 to 64 characters of letters,
// digits, '.' and '-': refused by name, never silently dropped.
func CheckHost(host string) error {
	if host == "" || len(host) > maxHostLen {
		return fmt.Errorf("%w: host must be 1 to %d characters of letters, digits, '.' and '-'", ErrBadName, maxHostLen)
	}
	for _, c := range host {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '-') {
			return fmt.Errorf("%w: host %q has a character other than a letter, digit, '.' or '-'", ErrBadName, host)
		}
	}
	return nil
}

// touchHost notes that host read name's inbox now. Caller does not hold b.mu.
func (b *Bus) touchHost(name, host string) {
	if host == "" {
		return
	}
	b.mu.Lock()
	defer b.unlock()
	if in := b.inboxes[name]; in != nil {
		in.noteHost(host, time.Now())
	}
}

// noteHost records a read by host at now. Caller holds b.mu.
func (in *inbox) noteHost(host string, now time.Time) {
	in.pruneHosts(now)
	if in.hosts == nil {
		in.hosts = map[string]time.Time{}
	}
	in.hosts[host] = now
	for len(in.hosts) > maxHosts {
		oldest, at := "", time.Time{}
		for h, t := range in.hosts {
			if oldest == "" || t.Before(at) {
				oldest, at = h, t
			}
		}
		delete(in.hosts, oldest)
		in.hostsCut = at
	}
}

// pruneHosts forgets the hosts whose last read is older than the window.
func (in *inbox) pruneHosts(now time.Time) {
	for h, t := range in.hosts {
		if now.Sub(t) > hostWindow {
			delete(in.hosts, h)
		}
	}
}

// listHosts is the hosts of in for a listing: each with its last read, by
// name, and whether more than the bound read in the window.
func (in *inbox) listHosts(now time.Time) ([]protocol.ReaderHost, bool) {
	in.pruneHosts(now)
	out := make([]protocol.ReaderHost, 0, len(in.hosts))
	for h, t := range in.hosts {
		out = append(out, protocol.ReaderHost{Name: h, LastRead: t})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, !in.hostsCut.IsZero() && now.Sub(in.hostsCut) <= hostWindow
}

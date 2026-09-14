// The views the MVP owes, from docs/05-discovery.md#what-it-shows. Every one
// of them is a **reshape of what the bus already answered this caller** — the
// records, the feed, `status` and the caller's own credentials. Nothing here
// asks a second question, keeps anything between requests, or works out
// something the daemon could have been asked for: these diagnostic views are read-only
// and the filtering already happened in the bus (docs/05-discovery.md#dashboard).
//
// What *is* the page's is ordering, grouping and the late mark, which is
// exactly the split the design draws.
package main

import (
	"sort"
	"time"

	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

type view struct {
	You    string
	Status core.Status

	Records   []protocol.Record // registry, as this caller may see it
	Backlogs  []protocol.Record // stuck inboxes, worst first
	Losses    []protocol.Record // loss by name
	Refusals  []refusal
	Exchanges []exchange
	Names     []auth.Held

	NoFeed  string // this caller may not read the feed
	NoNames string // nor its own credentials
}

// stuck is the backlogs, **oldest first**: a count alone cannot say whether a
// queue is busy or stalled, and the message that has been waiting longest is
// the one an incident is about. Depth is only the tie-break — a deep queue
// that started a second ago is a burst, and a single message nobody has taken
// for an hour is the outage.
//
// Every backlog here is one nobody is reading, which is not a filter but a
// property of the daemon: an unfiltered reader is handed a message as it
// arrives, so a queue only grows when there is nobody on the other end
// (docs/04-messaging.md#one-reader-per-inbox). The reader column says so
// rather than being taken on trust.
// See docs/05-discovery.md#what-it-shows.
func stuck(rs []protocol.Record) []protocol.Record {
	out := make([]protocol.Record, 0, len(rs))
	for _, r := range rs {
		if r.Queued > 0 {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if x, y := waited(a.Oldest), waited(b.Oldest); x != y {
			return x > y
		}
		if a.Queued != b.Queued {
			return a.Queued > b.Queued
		}
		return a.Name < b.Name
	})
	return out
}

// waited reads the age the daemon stated. An age it cannot parse sorts as no
// age at all rather than as the oldest: a page that promoted a name because
// its own parsing failed would point an incident at the wrong service.
func waited(d string) time.Duration {
	v, err := time.ParseDuration(d)
	if err != nil {
		return 0
	}
	return v
}

// lost is loss by name: what each inbox dropped to overflow and what expired
// in it. The node's total is on `status` and says only that something is
// losing work (docs/05-discovery.md#what-a-listing-answers).
func lost(rs []protocol.Record) []protocol.Record {
	out := make([]protocol.Record, 0, len(rs))
	for _, r := range rs {
		if r.Dropped+r.Expired > 0 {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Dropped+out[i].Expired, out[j].Dropped+out[j].Expired
		if a != b {
			return a > b
		}
		return out[i].Name < out[j].Name
	})
	return out
}

type refusal struct {
	Reason string
	Count  int
}

// refusals lists only the reasons that have happened: a reason with a zero
// beside it is noise on every other node (docs/05-discovery.md#refusals). The
// map the daemon sends already holds only those, so this orders them.
func refusals(m map[string]int) []refusal {
	out := make([]refusal, 0, len(m))
	for reason, n := range m {
		out = append(out, refusal{reason, n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Reason < out[j].Reason
	})
	return out
}

// exchange is a request and everything that came back for it, on one row.
type exchange struct {
	At    time.Time
	Topic string
	Tag   string
	From  string
	To    string
	N     int
	Ack   bool
	Done  bool
	Reply bool
	Late  bool

	// The caller's deadline, off the request. Kept so that a later message
	// can be compared with it; never rendered, because what a reader needs
	// is the mark, not the arithmetic.
	deadline time.Time
}

// exchanges groups the feed by topic and tag, so a request, its `ack`, its
// reply and its `done` are one row instead of four lines to read in the right
// order. An answer that arrived after the caller's deadline is marked late —
// the deadline travels on the envelope, so the page can see it went by
// (docs/04-messaging.md#request-and-reply).
//
// Grouping and the late mark are the page's; the feed was filtered per caller
// in the bus, and this never widens it.
func exchanges(feed []protocol.Envelope) []exchange {
	// The feed arrives newest first; an exchange is read oldest first,
	// because its first message is the request everything else answers.
	by := map[string]*exchange{}
	order := []*exchange{}
	for i := len(feed) - 1; i >= 0; i-- {
		e := feed[i]
		// A message with neither topic nor tag belongs to no exchange but
		// its own: grouping those together would put unrelated traffic on
		// one row.
		key := e.Topic + "\x00" + e.Tag
		if e.Topic == "" && e.Tag == "" {
			key = e.ID
		}
		x, seen := by[key]
		if !seen {
			x = &exchange{
				At: e.At, Topic: e.Topic, Tag: e.Tag,
				From: e.From, To: e.To, deadline: e.Deadline,
			}
			by[key], order = x, append(order, x)
		}
		x.N++
		switch e.Receipt {
		case protocol.ReceiptAck:
			x.Ack = true
		case protocol.ReceiptDone:
			x.Done = true
		default:
			// An ordinary message coming back the other way is the answer.
			if seen && e.From == x.To {
				x.Reply = true
			}
		}
		// The deadline is the request's, so only what came after it can be
		// late — and a feed that has already forgotten the request carries
		// no deadline to judge by, which is the honest answer rather than a
		// guess (docs/04-messaging.md#request-and-reply).
		if seen && !x.deadline.IsZero() && e.At.After(x.deadline) {
			x.Late = true
		}
	}
	out := make([]exchange, 0, len(order))
	for i := len(order) - 1; i >= 0; i-- { // newest exchange first, like the feed
		out = append(out, *order[i])
	}
	return out
}

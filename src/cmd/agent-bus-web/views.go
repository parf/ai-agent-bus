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
	"html/template"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/display"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// titleMark returns only fixed, repository-owned markup. Callers may select a
// category, including one stated by the daemon, but no caller text enters the
// result. The adjacent h1 text names the page, so every mark is decorative.
func titleMark(category string) template.HTML {
	const (
		channel     = `<svg class=page-title-mark width="26" height="26" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M4 8h5l7-4v16l-7-4H4Z" fill="#3d718e" stroke="#172b3a" stroke-width="1.5"/><path d="M18 8c2 2 2 6 0 8" fill="none" stroke="#e83b32" stroke-width="2" stroke-linecap="round"/></svg>`
		activity    = `<svg class=page-title-mark width="26" height="26" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M3 20V4M3 20h18" fill="none" stroke="#172b3a" stroke-width="1.5"/><path d="m5 16 4-5 4 2 6-7" fill="none" stroke="#2255aa" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"/></svg>`
		diagnostics = `<svg class=page-title-mark width="26" height="26" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><circle cx="10" cy="10" r="6" fill="none" stroke="#3d718e" stroke-width="2"/><path d="m15 15 6 6" stroke="#172b3a" stroke-width="2.5" stroke-linecap="round"/><path d="M7 10h6M10 7v6" stroke="#e83b32" stroke-width="1.5"/></svg>`
		problem     = `<svg class=page-title-mark width="26" height="26" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M12 3 22 21H2Z" fill="#ffdf83" stroke="#b00" stroke-width="1.5"/><path d="M12 8v6m0 3v.2" stroke="#172b3a" stroke-width="2" stroke-linecap="round"/></svg>`
	)
	switch category {
	case "overview":
		// Not the bus mark: the header already carries it, so the page title
		// repeated the logo rather than naming the page.
		return `<span class=page-title-mark aria-hidden=true>🏠</span>`
	case "credentials":
		return `<span class=page-title-mark aria-hidden=true>🔑</span>`
	case "agent", "agents", "personal":
		return `<span class=page-title-mark aria-hidden=true>👾</span>`
	case "channels", protocol.KindQueue, protocol.KindPubSub:
		return channel
	case "activity":
		return activity
	case "services", protocol.KindService:
		// The same mark the rows carry: a title that said something else about
		// the record under it would be the page disagreeing with its own table.
		return `<span class=page-title-mark aria-hidden=true>📡</span>`
	case "users", "user":
		return `<span class=page-title-mark aria-hidden=true>👤</span>`
	case "groups":
		return `<span class=page-title-mark aria-hidden=true>👥</span>`
	case "identity":
		return `<span class=page-title-mark aria-hidden=true>🪪</span>`
	case "diagnostics":
		return diagnostics
	case "problem":
		return problem
	default:
		return `<span class=page-title-mark aria-hidden=true>⚙️</span>`
	}
}

func readerCount(readers *int) string {
	if readers == nil {
		return "unavailable"
	}
	return number(*readers)
}

func matchesReaderFilter(readers *int, filter string) bool {
	switch filter {
	case "present":
		return readers != nil && *readers > 0
	case "none":
		return readers != nil && *readers == 0
	case "unavailable":
		return readers == nil
	default:
		return true
	}
}

// number keeps dense operational figures readable without changing any
// machine-facing value. HTML is the only consumer: JSON, URLs and form values
// remain plain decimal strings.
func number(value any) string {
	var raw string
	switch n := value.(type) {
	case int:
		raw = strconv.Itoa(n)
	case int64:
		raw = strconv.FormatInt(n, 10)
	case uint:
		raw = strconv.FormatUint(uint64(n), 10)
	case uint64:
		raw = strconv.FormatUint(n, 10)
	default:
		return ""
	}
	negative := strings.HasPrefix(raw, "-")
	digits := raw
	if negative {
		digits = raw[1:]
	}
	for i := len(digits) - 3; i > 0; i -= 3 {
		digits = digits[:i] + "," + digits[i:]
	}
	if negative {
		return "-" + digits
	}
	return digits
}

// registrationUpdated keeps recent list entries scannable without pretending
// their write time is an observation or health signal. Detail pages retain the
// full timestamp. Future clock skew is rendered as now rather than a negative
// age.
func registrationUpdatedAt(at, now time.Time) string {
	if at.IsZero() {
		return ""
	}
	at = at.In(now.Location())
	age := now.Sub(at)
	if age < 0 {
		age = 0
	}
	switch {
	case age < time.Minute:
		return "now"
	case age < time.Hour:
		return strconv.Itoa(int(age/time.Minute)) + "m ago"
	case age < 24*time.Hour:
		return strconv.Itoa(int(age/time.Hour)) + "h ago"
	case age < 30*24*time.Hour:
		return strconv.Itoa(int(age/(24*time.Hour))) + "d ago"
	case at.Year() == now.Year():
		return at.Format("Jan 2")
	default:
		return at.Format("Jan 2, 2006")
	}
}

func registrationUpdated(at time.Time) string {
	return registrationUpdatedAt(at, time.Now())
}

// authorityLabel is display.Authority for templates, which cannot call a
// two-argument function on fields of different structs any other way.
func authorityLabel(daemonOwner, administrator bool) string {
	return display.Authority(daemonOwner, administrator)
}

func entityLabel(kind string) string { return display.Entity(kind) }

// identityKind is entityLabel for a row that names a principal, so that the
// node's daemon owner reads as the authority rather than as one more user. The
// owner's name comes from the frame, which every page already carries; an
// unavailable node publishes no name and marks nobody.
func identityKind(kind, name, daemonOwner string) string {
	return display.Identity(kind, name != "" && name == daemonOwner)
}

// channelRecord reports whether a record belongs with the channels rather than
// the services. Four of the five kinds are names on this bus that something is
// delivered to; only a service is external, and it is the one thing nothing is
// served from here.
// See docs/03-records.md#five-record-kinds.
func channelRecord(kind string) bool {
	return kind != protocol.KindService && kind != protocol.KindAgent
}

// agentRecord reports whether a record belongs on the agents page. An agent is
// the thing this bus exists to carry messages between, so it is listed first
// and on its own, not mixed in with the queues it reads or with the external
// services it calls.
func agentRecord(kind string) bool { return kind == protocol.KindAgent }

// detailPathFor and listPathFor are the one place a kind becomes a URL. Three
// listings, one per thing a record can be: an agent on this bus, an external
// service, or a channel something is delivered through.
func detailPathFor(kind string) string {
	switch {
	case agentRecord(kind):
		return "/agent"
	case channelRecord(kind):
		return "/channel"
	default:
		return "/service"
	}
}

// listPathForRecord is listPathFor asked of a whole record. A Personal agent is
// excluded from /agents, so a Back link or a post-removal redirect that went
// there would name a listing which cannot show the record it came from.
func listPathForRecord(kind string, personal bool) string {
	if personal && kind != protocol.KindUser {
		return "/personal"
	}
	return listPathFor(kind)
}

func listPathFor(kind string) string {
	switch {
	case agentRecord(kind):
		return "/agents"
	case channelRecord(kind):
		return "/channels"
	default:
		return "/services"
	}
}

// deliveryMode says how a record delivers, in words rather than in a kind. Only
// a pub/sub topic copies; everything else on this bus holds one queue, and a
// service holds nothing at all.
func deliveryMode(record protocol.Record) string {
	switch record.Kind {
	case protocol.KindPubSub:
		return "a copy to each subscriber"
	case protocol.KindService:
		return ""
	default:
		return "one at a time"
	}
}

// external reports whether a record is reached off this bus. Only a service is,
// and it is the kind that must carry an address and a protocol; a populated
// endpoint on anything else is not evidence that the thing is external, so the
// kind is what the page reads.
// See docs/03-records.md#five-record-kinds.
func external(record protocol.Record) bool { return record.Kind == protocol.KindService }

// copies reports whether a kind delivers a copy to every subscriber. Only
// pub/sub does; it is the one kind that keeps no queue of its own, so the page
// shows subscribers where the others show held work.
func copies(kind string) bool { return kind == protocol.KindPubSub }

// deliveryLabel is the one-line delivery fact for a listing column, where
// deliveryMode writes the same fact as a sentence on a detail page.
func deliveryLabel(record protocol.Record) string {
	if copies(record.Kind) {
		return "Pub/sub · copy to each"
	}
	return "Queue · one at a time"
}

// recordNoun is the plain word for a kind, used where a document title needs
// a noun rather than a labelled glyph.
func recordNoun(kind string) string {
	switch kind {
	case protocol.KindUser:
		return "User"
	case protocol.KindAgent:
		return "Agent"
	case protocol.KindQueue:
		return "Queue"
	case protocol.KindPubSub:
		return "PubSub"
	case protocol.KindService:
		return "Service"
	default:
		return "Record"
	}
}

func entityGlyph(kind string) string {
	return display.EntityGlyph(kind)
}

func groupGlyph() string {
	return display.GroupGlyph
}

// kindTotal is one cell of the node strip: what a record can be, and how many
// of them this node holds.
type kindTotal struct {
	Label string
	Count int
}

// kindTotals names the four things a record can be, in the order the navigation
// lists them, and gives each its own node-wide count. A queue and a pub/sub
// topic are both channels here, which is the only place the four differ from
// the five kinds (docs/03-records.md#five-record-kinds).
//
// A daemon that stated no kinds gets nothing rather than four zeros: the strip
// would otherwise read as an empty node beside a total that says eleven.
func kindTotals(kinds map[string]int) []kindTotal {
	if len(kinds) == 0 {
		return nil
	}
	return []kindTotal{
		{"Agents", kinds[protocol.KindAgent]},
		{"Services", kinds[protocol.KindService]},
		{"Channels", kinds[protocol.KindQueue] + kinds[protocol.KindPubSub]},
		{"Users", kinds[protocol.KindUser]},
	}
}

type view struct {
	pageInfo
	You    string
	Status core.Status
	Totals []kindTotal // node-wide record counts, one cell per thing a record can be

	Records   []protocol.Record // registry, as this caller may see it
	Backlogs  []protocol.Record // inboxes holding messages, longest wait first
	Losses    []protocol.Record // loss by name
	Attention []attention       // enumerated conditions for the short Overview
	Refusals  []refusal
	Exchanges []exchange
	NoFeed    string // this caller may not read the feed
}

// attention is an observed condition, never a health verdict. A record gets at
// most one item even when several facts apply; the item keeps every supporting
// value so Overview does not hide a loss behind a current queue condition.
type attention struct {
	Kind, Level, Title string
	Href, Link         string
	Name, Reason       string
	Count              int
	Queued             int
	Oldest             string
	Overflow           string // the configured policy, stated as a setting
	Dropped, Expired   int
	Disabled, AtBound  bool
}

func recordHref(r protocol.Record) string {
	return detailPathFor(r.Kind) + "?name=" + url.QueryEscape(r.Name)
}

// detailPath is the row link's path alone, for a template that writes the
// query itself. Only the path comes from here: the name and the return state
// stay separate interpolations so html/template escapes each in its own
// context, which is what makes the return value a readable round trip.
func detailPath(r protocol.Record) string { return detailPathFor(r.Kind) }

// attentionItems enumerates only conditions the daemon answer supports. A
// backlog on its own is ordinary work and therefore stays out of this list.
// whenFull states the configured overflow policy in the words the record page
// already uses. It is a setting, never a prediction about the next send: the
// observation path does not prune, so at capacity now says nothing about what
// the send path will do (Plans/MVP/web/glyphs.md#what-an-observation-is-worth).
func whenFull(r protocol.Record) string {
	if r.Full == protocol.OverflowRing {
		return "when full: drop the oldest"
	}
	return "when full: refuse"
}

func attentionItems(status core.Status, records []protocol.Record) []attention {
	var out []attention
	if status.Unclean {
		// Overview owns the node strip, and uptime there is the one further
		// fact about this restart the daemon actually answers. Diagnostics
		// has no node section to send them to.
		out = append(out, attention{Kind: "unclean", Level: "red", Title: "The previous stop was not clean", Href: "/#node", Link: "View node totals"})
	}
	for _, item := range refusals(status.Refused) {
		if item.Count == 0 {
			continue
		}
		// Informational: a lifetime total cannot say anything is wrong now,
		// and it cannot be climbing (Plans/MVP/web/pages.md#overview).
		out = append(out, attention{Kind: "refusal", Level: "blue", Title: "Requests were refused", Reason: item.Reason, Count: item.Count, Href: "/diagnostics#refusals", Link: "View refusal reasons"})
	}
	for _, r := range records {
		if !r.AtBound && r.Dropped+r.Expired == 0 && !(r.Disabled && r.Queued > 0) {
			continue
		}
		item := attention{
			Kind: "record", Level: "orange", Name: r.Name, Href: recordHref(r), Link: "View record",
			Queued: r.Queued, Oldest: r.Oldest, Dropped: r.Dropped, Expired: r.Expired,
			Disabled: r.Disabled, AtBound: r.AtBound,
		}
		// Precedence is the spec's, and it is not severity order: a disabled
		// record at its bound is not urgent, because nothing is being
		// accepted and capacity is not what is wrong with it. The condition
		// that explains the other wins, and the rest stay as supporting
		// facts (Plans/MVP/web/glyphs.md#attention-levels).
		switch {
		case r.Disabled && r.Queued > 0:
			item.Title = "Delivery is off and work is held"
			if r.AtBound {
				item.Overflow = whenFull(r)
			}
		case r.AtBound:
			item.Level, item.Title, item.Overflow = "red", "Queue at capacity when observed", whenFull(r)
		default:
			item.Title = "Messages were lost from this inbox"
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		// red, then orange, then informational: one ordered enum, the order
		// the level vocabulary defines.
		weight := func(level string) int {
			switch level {
			case "red":
				return 0
			case "orange":
				return 1
			}
			return 2
		}
		if a, b := weight(out[i].Level), weight(out[j].Level); a != b {
			return a < b
		}
		if out[i].Title != out[j].Title {
			return out[i].Title < out[j].Title
		}
		return out[i].Name+out[i].Reason < out[j].Name+out[j].Reason
	})
	return out
}

// stuck is the backlogs, **oldest first**: a count alone cannot say whether a
// queue is busy or stalled, and the message that has been waiting longest is
// the one an incident is about. Depth is only the tie-break — a deep queue
// that started a second ago is a burst, and a single message nobody has taken
// for an hour is the outage.
//
// A backlog can coexist with filtered readers: they take matching messages
// while everything else waits. Readers is therefore an observation beside
// queue depth rather than a health verdict about it.
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

// refusals lists **every** supported reason, including the ones at zero
// (docs/05-discovery.md#refusals). The daemon's map is sparse — `refused`
// starts empty and `Refuse` increments per reason — but the reason set is
// closed and named, so a reason missing from a status that answered is a
// measured zero rather than an absence of observation. Leaving it out made
// the page unable to say the difference, which is the distinction this whole
// layer is for (Plans/MVP/web/data-dictionary.md#absence).
//
// api.Reasons is the daemon's own table, so a reason added there appears here
// without anybody remembering to add it.
func refusals(m map[string]int) []refusal {
	out := make([]refusal, 0, len(m))
	seen := map[string]bool{}
	for _, reason := range api.Reasons() {
		seen[reason] = true
		out = append(out, refusal{reason, m[reason]})
	}
	// A reason the daemon counted that this face does not know about is still
	// a refusal that happened, and dropping it would be worse than not
	// recognising it.
	for reason, n := range m {
		if !seen[reason] {
			out = append(out, refusal{reason, n})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Reason < out[j].Reason
	})
	return out
}

// An ordinary message stays a row of its own: replies carry a route, not a
// request ID. Only a receipt with an explicit reference and the original's
// expected route joins that original. A route match is a clue, not completion.
type exchange struct {
	protocol.Envelope
	Receipts []receiptObservation
	Matches  []string
	Notice   string
	Ack      bool
	Done     bool
	Late     bool // ordinary message after its sole retained route match's deadline
}

type receiptObservation struct {
	protocol.Envelope
	Late bool
}

func (x exchange) N() int { return 1 + len(x.Receipts) }

func (x exchange) latest() time.Time {
	at := x.At
	for _, r := range x.Receipts {
		if r.At.After(at) {
			at = r.At
		}
	}
	return at
}

// responseRoute includes all routing fields, including a third-party reply
// destination. A same-tag message from some other participant is unrelated.
func responseRoute(request, response protocol.Envelope) bool {
	back := protocol.ReplyTo{Name: request.From, Topic: request.Topic, Tag: request.Tag}
	if request.ReplyTo != nil {
		back = *request.ReplyTo
	}
	return response.From == request.To && response.To == back.Name &&
		response.Topic == back.Topic && response.Tag == back.Tag
}

func exchanges(feed []protocol.Envelope) []exchange {
	out := make([]exchange, 0, len(feed))
	byID := map[string][]int{}
	receiptIDs := map[string]bool{}
	for _, e := range feed {
		if e.Receipt == "" {
			byID[e.ID] = append(byID[e.ID], len(out))
			out = append(out, exchange{Envelope: e})
		} else {
			receiptIDs[e.ID] = true
		}
	}
	for _, e := range feed {
		if e.Receipt == "" {
			continue
		}
		notice := "Original message not in this visible history."
		if e.Re == "" {
			notice = "Receipt has no original message reference."
		} else if targets := byID[e.Re]; len(targets) == 1 {
			x := &out[targets[0]]
			if responseRoute(x.Envelope, e) && !e.At.Before(x.At) {
				x.Receipts = append(x.Receipts, receiptObservation{
					Envelope: e, Late: !x.Deadline.IsZero() && e.At.After(x.Deadline),
				})
				x.Ack = x.Ack || e.Receipt == protocol.ReceiptAck
				x.Done = x.Done || e.Receipt == protocol.ReceiptDone
				continue
			}
			notice = "Reference found; route or time differs from the original. Kept separate."
		} else if len(targets) > 1 {
			notice = "Reference is ambiguous in this history. Kept separate."
		} else if receiptIDs[e.Re] {
			notice = "The referenced message is itself a receipt. Kept separate."
		}
		out = append(out, exchange{Envelope: e, Notice: notice})
	}
	for i := range out {
		x := &out[i]
		if x.Receipt != "" || x.Tag == "" {
			continue
		}
		var deadline time.Time
		for j := range out {
			r := &out[j]
			if r.Receipt == "" && r.At.Before(x.At) && responseRoute(r.Envelope, x.Envelope) {
				x.Matches = append(x.Matches, r.ID)
				deadline = r.Deadline
			}
		}
		// Multiple retained candidates cannot supply one meaningful deadline.
		x.Late = len(x.Matches) == 1 && !deadline.IsZero() && x.At.After(deadline)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if a, b := out[i].latest(), out[j].latest(); !a.Equal(b) {
			return a.After(b)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Used by the diagnostics page; no template access to message bodies.
const exchangesTemplate = `
<style>
 .exchanges td{vertical-align:top;overflow-wrap:anywhere}
 @media (max-width:40rem){
  .exchanges,.exchanges tbody,.exchanges tr,.exchanges td{display:block}
  .exchanges caption{display:block;text-align:left;margin:.6rem 0}
  .exchanges thead{position:absolute;width:1px;height:1px;overflow:hidden;clip-path:inset(50%)}
  .exchanges tr{margin-bottom:1rem;padding:.8rem;border:1px solid #ddd;border-radius:.3rem}
  .exchanges td{border:0;padding:.25rem 0}
  .exchanges td::before{content:attr(data-label);display:block;font-weight:600;color:#555}
  .exchanges td.num::before{text-align:left}
 }
</style>
<section class=dashboard-section><div class=page-title><h2 id=exchanges>Exchanges in retained history</h2><button type=button class=help-button popovertarget=exchanges-help aria-label="About retained exchanges" data-tooltip="Visible envelopes from this daemon run only. Missing history or receipts prove neither failure nor unfinished work; bodies are never shown.">ⓘ</button></div>
<div popover id=exchanges-help class=context-help><h2>Retained exchanges</h2><ul><li>Only envelopes visible to you in this daemon run appear here; message bodies never do.</li><li>History is bounded, so missing messages or receipts prove neither failure nor unfinished work.</li><li>Times include their UTC offset.</li><li>Receipts name their original message. Route matches are clues, not proof of response or completion.</li><li>Untagged messages get no inferred links, and receipts from a different subscriber or worker stay separate.</li></ul></div>
{{if .NoFeed}}<p class=warn>Envelope history unavailable: {{.NoFeed}}</p>{{else}}
{{if .Exchanges}}
<table class=exchanges><caption>Messages and explicitly referenced receipts</caption>
<thead><tr><th scope=col>Observed message<th scope=col>Route and conversation<th scope=col class=num>Envelopes<th scope=col>Evidence</tr></thead>
<tbody>
{{range .Exchanges}}<tr id="message-{{.ID}}"><td data-label="Observed message"><time>{{.At.Format "2006-01-02 15:04:05Z07:00"}}</time><br><code>{{.ID}}</code><td data-label="Route and conversation"><code>{{.From}}</code> → <code>{{.To}}</code><br>Topic: <code>{{if .Topic}}{{.Topic}}{{else}}not supplied{{end}}</code> · Tag: <code>{{if .Tag}}{{.Tag}}{{else}}not supplied{{end}}</code>{{with .ReplyTo}}<br>Reply route: <code>{{.Service}}</code> · Topic: <code>{{.Topic}}</code> · Tag: <code>{{.Tag}}</code>{{end}}<td class=num data-label="Envelopes">{{number .N}}</td><td data-label="Evidence">
{{if .Receipt}}<strong>{{.Receipt}} receipt</strong>{{with .Re}} about <code>{{.}}</code>{{end}}<p>{{.Notice}}</p>{{else}}
{{if .Ack}}<p>Acknowledgement observed.</p>{{end}}
{{if .Done}}<p>Completion receipt observed.</p>{{else}}<p class=muted>No completion receipt observed in retained history.</p>{{end}}
{{with .Matches}}<p>Possible response — matching earlier routes: {{range .}}<a href="#message-{{.}}"><code>{{.}}</code></a> {{end}}</p>{{end}}
{{if .Late}}<p class=warn>After the matching message's deadline.</p>{{end}}
{{with .Receipts}}<details><summary>Receipt evidence</summary><ul>{{range .}}<li><strong>{{.Receipt}}</strong> <code>{{.ID}}</code> about <code>{{.Re}}</code><br><code>{{.From}}</code> → <code>{{.To}}</code><br><time>{{.At.Format "2006-01-02 15:04:05Z07:00"}}</time>{{if .Late}} · <span class=warn>after the request's deadline</span>{{end}}</li>{{end}}</ul></details>{{end}}
{{end}}</td></tr>{{end}}
</tbody></table>
{{else}}<p class=muted>No envelopes in your retained history. This is not a count of all traffic.</p>{{end}}
{{end}}
</section>
`

// formSecret is a textarea's value as the bytes the person typed. A browser
// submits a textarea with CRLF line endings whatever the page was served with,
// and a service secret is opaque bytes the daemon stores exactly as sent — so
// without this, a two-line credential is stored with a carriage return in it
// that nobody put there. See docs/06-services.md#secrets.
func formSecret(value string) string {
	return strings.ReplaceAll(value, "\r\n", "\n")
}

// figure is a count as the node strip states it: the grouped decimal, or a
// dash when there is none. Zero is a true answer to "how many", and a strip of
// them made a quiet node look like a page of readings to check; the dash says
// the same thing without asking to be read. Only the strip uses it — a table
// column stays a column of numbers, where a dash would break the alignment
// that makes it scannable.
func figure(value any) template.HTML {
	switch n := value.(type) {
	case int:
		if n == 0 {
			return noFigure
		}
	case uint64:
		if n == 0 {
			return noFigure
		}
	}
	return template.HTML(template.HTMLEscapeString(number(value)))
}

const noFigure = template.HTML(`<span class=muted>&mdash;</span>`)

// onList reports whether a name is on a stored list under its own name. The
// Deliver-To control that takes an inbox off the list is offered only then:
// a name that receives through a group cannot be removed by removing itself,
// so a button that looked the same would do nothing.
// See docs/04-messaging.md#subscribers.
func onList(list []string, who string) bool {
	for _, s := range list {
		if s == who {
			return true
		}
	}
	return false
}

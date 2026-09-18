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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/display"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

var titleBusLogo = template.HTML(strings.NewReplacer(
	"class=node-logo", "class=page-title-mark",
	`width="80" height="58"`, `width="32" height="24"`,
).Replace(nodeLogo))

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
		return titleBusLogo
	case "credentials":
		return `<span class=page-title-mark aria-hidden=true>🔑</span>`
	case "agent":
		return `<span class=page-title-mark aria-hidden=true>👾</span>`
	case "channels", protocol.KindTopic:
		return channel
	case "activity":
		return activity
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

func entityLabel(kind string) string {
	if kind == protocol.KindTopic {
		return "Channel"
	}
	return display.Entity(kind)
}

func entityGlyph(kind string) string {
	return display.EntityGlyph(kind)
}

func groupGlyph() string {
	return display.GroupGlyph
}

type view struct {
	pageInfo
	You string
	// At is when this page was built. The page no longer refreshes itself, so
	// it has to say how old what you are reading is.
	At     string
	Status core.Status

	Records   []protocol.Record // registry, as this caller may see it
	Backlogs  []protocol.Record // inboxes holding messages, longest wait first
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
	back := protocol.ReplyTo{Service: request.From, Topic: request.Topic, Tag: request.Tag}
	if request.ReplyTo != nil {
		back = *request.ReplyTo
	}
	return response.From == request.To && response.To == back.Service &&
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

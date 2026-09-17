package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

type busError struct {
	code    int
	message string
}

func (e *busError) Error() string { return e.message }

// problem is a failed page, shown in place of the one asked for. It carries the
// shell, because a refusal is the moment somebody most needs the navigation and
// their own name: http.Error writes text/plain with no way back, no title and
// nothing saying who it was refusing (Plans/MVP/done/web-review.md W12).
type problem struct {
	pageInfo
	You             string
	Title           string
	Detail          string
	Advice          string
	Back, BackLabel string
}

// fail turns a bus refusal into the page for that refusal. The recoveries are
// deliberately different from each other: an expired session, a permission
// refusal and a bus that is not answering are three different things to do
// next, and one status line told a reader none of them apart.
// Codes are the closed set in docs/05-discovery.md#refusals.
func fail(w http.ResponseWriter, r *http.Request, you string, err error) {
	var bad *busError
	if !errors.As(err, &bad) {
		// This page rendered, so the dashboard is up; the daemon it asks is a
		// separate process (docs/11-processes.md#the-processes) and is not
		// answering. Saying "sign in" here would be a lie about whose fault it
		// is, and is what the anonymous page used to say.
		show(w, http.StatusBadGateway, problem{You: you,
			Title:  "The bus is not answering",
			Detail: err.Error(),
			Advice: "The dashboard is running; the daemon behind it is not reachable, so there is nothing to show and nothing was changed. It comes back on its own when the daemon does.",
		}, r)
		return
	}
	switch bad.code {
	case http.StatusUnauthorized:
		// Not an error page at all: the sign-in form, at the address they
		// asked for, so signing in lands them where they were going.
		signIn(w, r, "that session has ended \u2014 sign in to carry on")
	case http.StatusForbidden:
		show(w, bad.code, problem{You: you,
			Title:  "Not yours to see",
			Detail: bad.message,
			Advice: "You are signed in, and refused for lack of permission rather than for want of a credential \u2014 signing in again would change nothing. Its owner, or a maintainer of it, is who can grant this.",
		}, r)
	case http.StatusNotFound:
		// Hidden and absent are one answer on purpose: telling them apart
		// would let anybody enumerate the registry a name at a time.
		// See docs/05-discovery.md#refusals.
		show(w, bad.code, problem{You: you,
			Title:  "No such name",
			Advice: "Either nothing is registered under that name or it is not one you may see. Those are deliberately the same answer, so this does not tell you which.",
		}, r)
	case http.StatusServiceUnavailable:
		show(w, bad.code, problem{You: you,
			Title:  "The bus is busy",
			Detail: bad.message,
			Advice: "The daemon is there and briefly cannot answer. Nothing was changed; the same request is worth making again.",
		}, r)
	case http.StatusInternalServerError:
		show(w, bad.code, problem{You: you,
			Title:  "Something went wrong in the daemon",
			Detail: bad.message,
			Advice: "This is a fault, not a rule: repeating it is unlikely to help, and the daemon's log on this node is where it is recorded.",
		}, r)
	default:
		// 400, 409, 412 and 429: the bus understood and would not.
		show(w, bad.code, problem{You: you,
			Title:  "That was refused",
			Detail: bad.message,
			Advice: "Nothing was changed. The reason above is the daemon's own.",
		}, r)
	}
}

// show renders a problem with the way back filled in: retrying a page that
// failed is following the same link again, while a form that was refused wants
// the form, not a repeat of the submission.
func show(w http.ResponseWriter, code int, p problem, r *http.Request) {
	if r.Method == http.MethodGet {
		p.Back, p.BackLabel = r.URL.RequestURI(), "Try again"
	} else if back := referrer(r); back != "/" {
		p.Back, p.BackLabel = back, "Back to the page"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	p.pageInfo = requestInfo(r)
	render(w, problemPage, p)
}

var problemPage = template.Must(template.New("problem").Parse(shell("", "Problem") + `<h1>{{.Title}}</h1>
{{with .Detail}}<p class=warn>{{.}}</p>{{end}}
<p>{{.Advice}}</p>
{{with .Back}}<p><a href="{{.}}">{{$.BackLabel}}</a></p>{{end}}
`))

type adminView struct {
	pageInfo
	You           string `json:"you"`
	DaemonOwner   bool   `json:"daemon_owner"`
	Administrator bool   `json:"administrator"`
	Records       []protocol.Record
	Record        protocol.Record
	Groups        map[string][]string
	Mine, State   string
	OwnerFilter   string
	Current       string
	Owners        []string
	Channels      bool
	PersonalPage  bool
}

func (c *caller) signedIn(w http.ResponseWriter, r *http.Request) (adminView, bool) {
	var v adminView
	if cookie(r) == "" {
		// A deep link is where somebody meant to go, and a plain 401 left them
		// at a dead end with no form on it. The form is served here instead,
		// at that address, and carries it back (web-review.md W12).
		signIn(w, r, "sign in to open this page")
		return v, false
	}
	node, err := c.status(r)
	if err != nil {
		fail(w, r, "", err)
		return v, false
	}
	v.pageInfo = requestInfo(r)
	v.You, v.Administrator, v.DaemonOwner = node.You, node.Administrator, node.DaemonOwner
	return v, true
}

// Mutating browser requests require the exact origin, including scheme/port.
// No process-local CSRF state, and no credential rendered as a form field.
func sameOrigin(r *http.Request, tls bool) bool {
	origin, err := url.Parse(r.Header.Get("Origin"))
	scheme := "http"
	if tls {
		scheme = "https"
	}
	return err == nil && origin.Scheme == scheme && origin.Host == r.Host && origin.User == nil && origin.Path == "" && origin.RawQuery == "" && origin.Fragment == "" && (r.Header.Get("Sec-Fetch-Site") == "" || r.Header.Get("Sec-Fetch-Site") == "same-origin")
}

func (c *caller) post(cred, path string, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.request(cred, "POST", path, bytes.NewReader(body), nil)
}

func (c *caller) adminRoutes(mux *http.ServeMux, tls bool) {
	c.activityRoutes(mux)
	c.userRoutes(mux, tls)
	listing := func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		var records []protocol.Record
		if err := c.get(cookie(r), "/ls", &records); err != nil {
			fail(w, r, v.You, err)
			return
		}
		v.Mine, v.State = r.URL.Query().Get("scope"), r.URL.Query().Get("state")
		v.Channels, v.PersonalPage = r.URL.Path == "/channels", r.URL.Path == "/personal"
		switch {
		case v.Channels:
			v.Current = "channels"
		case v.PersonalPage:
			v.Current = "personal"
		default:
			v.Current = "services"
		}
		if v.PersonalPage && !v.DaemonOwner && r.URL.Query().Has("owner") {
			// The owner selector belongs to the daemon owner's view. Do not
			// silently give the same shared URL a caller-dependent meaning.
			query := r.URL.Query()
			query.Del("owner")
			to := r.URL.Path
			if encoded := query.Encode(); encoded != "" {
				to += "?" + encoded
			}
			http.Redirect(w, r, to, http.StatusSeeOther)
			return
		}
		if v.PersonalPage {
			if v.DaemonOwner {
				v.OwnerFilter = r.URL.Query().Get("owner")
			} else {
				v.OwnerFilter = v.You
			}
		}
		owners := map[string]bool{}
		for _, record := range records {
			if v.PersonalPage {
				if record.Kind == protocol.KindTopic || !record.Personal {
					continue
				}
				owners[record.Owner] = true
				if v.OwnerFilter != "" && record.Owner != v.OwnerFilter {
					continue
				}
			} else if (record.Kind == protocol.KindTopic) != v.Channels || (!v.Channels && record.Personal) {
				continue
			}
			if v.Mine == "my" && record.Owner != v.You {
				continue
			}
			if v.State == "active" && record.Disabled || v.State == "inactive" && !record.Disabled {
				continue
			}
			v.Records = append(v.Records, record)
		}
		for owner := range owners {
			v.Owners = append(v.Owners, owner)
		}
		sort.Strings(v.Owners)
		sort.Slice(v.Records, func(i, j int) bool { return v.Records[i].Name < v.Records[j].Name })
		render(w, serviceList, v)
	}
	mux.HandleFunc("GET /services", listing)
	mux.HandleFunc("GET /channels", listing)
	mux.HandleFunc("GET /personal", listing)
	mux.HandleFunc("GET /service", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		if err := c.get(cookie(r), "/lookup?name="+url.QueryEscape(r.URL.Query().Get("name")), &v.Record); err != nil {
			fail(w, r, v.You, err)
			return
		}
		if err := c.get(cookie(r), "/groups", &v.Groups); err != nil {
			fail(w, r, v.You, err)
			return
		}
		switch {
		case v.Record.Kind == protocol.KindTopic:
			v.Current = "channels"
		case v.Record.Personal:
			v.Current = "personal"
		default:
			v.Current = "services"
		}
		render(w, serviceDetail, v)
	})
	mux.HandleFunc("GET /groups", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		if err := c.get(cookie(r), "/groups", &v.Groups); err != nil {
			fail(w, r, v.You, err)
			return
		}
		render(w, groupList, v)
	})
	// The view goes through to the handler so a refused submission can be
	// presented as the person who was refused, on the shell, rather than as a
	// bare status line (web-review.md W12).
	mutate := func(next func(http.ResponseWriter, *http.Request, adminView)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !sameOrigin(r, tls) {
				http.Error(w, "same-origin form required", http.StatusForbidden)
				return
			}
			v, ok := c.signedIn(w, r)
			if !ok {
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			next(w, r, v)
		}
	}
	mux.HandleFunc("POST /service", mutate(func(w http.ResponseWriter, r *http.Request, v adminView) {
		name := r.PostForm.Get("name")
		change := core.Management{Name: name}
		var err error
		switch r.PostForm.Get("action") {
		case "save":
			descr, addr, proto, ttl, overflow := r.PostForm.Get("descr"), r.PostForm.Get("addr"), r.PostForm.Get("protocol"), r.PostForm.Get("ttl"), r.PostForm.Get("overflow")
			bound, parseErr := strconv.Atoi(r.PostForm.Get("bound"))
			if parseErr != nil {
				http.Error(w, "invalid queue capacity", 400)
				return
			}
			noMaster := r.PostForm.Get("no_master") == "on"
			change.Descr, change.Addr, change.Proto, change.TTL, change.Full, change.Bound, change.NoMaster = &descr, &addr, &proto, &ttl, &overflow, &bound, &noMaster
			if r.PostForm.Has("edit_allow") {
				allow := strings.Fields(r.PostForm.Get("allow"))
				change.Allow = &allow
			}
			err = c.post(cookie(r), "/manage", change)
		case "enable", "disable":
			disabled := r.PostForm.Get("action") == "disable"
			change.Disabled = &disabled
			err = c.post(cookie(r), "/manage", change)
		case "maintainers":
			group := r.PostForm.Get("maintainers")
			change.Maintainers = &group
			err = c.post(cookie(r), "/manage", change)
		case "personal":
			personal := r.PostForm.Get("personal") == "on"
			allow := strings.Fields(r.PostForm.Get("allow"))
			maintainers := r.PostForm.Get("maintainers")
			change.Personal, change.Allow, change.Maintainers = &personal, &allow, &maintainers
			err = c.post(cookie(r), "/manage", change)
		case "transfer":
			owner := r.PostForm.Get("owner")
			change.Owner = &owner
			err = c.post(cookie(r), "/manage", change)
		case "configure":
			cfg := json.RawMessage(r.PostForm.Get("config"))
			if !json.Valid(cfg) {
				http.Error(w, "configuration must be valid JSON", 400)
				return
			}
			err = c.post(cookie(r), "/configure", struct {
				Name   string
				Config json.RawMessage
			}{name, cfg})
		case "create":
			kind, mode := r.PostForm.Get("kind"), r.PostForm.Get("mode")
			if kind != "generic" && kind != "agent" && kind != "topic" {
				http.Error(w, "invalid record kind", 400)
				return
			}
			err = c.post(cookie(r), "/register", protocol.Record{Name: name, Kind: kind, Mode: mode, Descr: r.PostForm.Get("descr"), Allow: strings.Fields(r.PostForm.Get("allow")), Personal: r.PostForm.Get("personal") == "on"})
		case "subscribe", "unsubscribe":
			err = c.post(cookie(r), "/subscribe", struct {
				Topic string
				Off   bool
			}{name, r.PostForm.Get("action") == "unsubscribe"})
		case "remove-subscriber":
			err = c.post(cookie(r), "/subscriber/remove", map[string]string{"topic": name, "subscriber": r.PostForm.Get("subscriber")})
		case "delete":
			err = c.post(cookie(r), "/unregister", map[string]string{"name": name})
		default:
			http.Error(w, "unknown action", 400)
			return
		}
		if err != nil {
			fail(w, r, v.You, err)
			return
		}
		next := "/services?scope=my"
		if r.PostForm.Get("action") == "personal" || r.PostForm.Get("action") == "create" && r.PostForm.Get("personal") == "on" {
			next = "/personal"
		}
		http.Redirect(w, r, next, http.StatusSeeOther)
	}))
	mux.HandleFunc("POST /groups", mutate(func(w http.ResponseWriter, r *http.Request, v adminView) {
		// "delete" was an action here and is not one now: a group is retired
		// by emptying its membership. It is named rather than falling into the
		// default so that an old bookmark is refused instead of silently
		// saving whatever members the form carried.
		if action := r.PostForm.Get("action"); action != "save" {
			http.Error(w, "unknown action", 400)
			return
		}
		err := c.post(cookie(r), "/group", struct {
			Name    string
			Members []string
		}{r.PostForm.Get("name"), strings.Fields(r.PostForm.Get("members"))})
		if err != nil {
			fail(w, r, v.You, err)
			return
		}
		http.Redirect(w, r, "/groups", http.StatusSeeOther)
	}))
}

var serviceList = template.Must(template.New("services").Funcs(template.FuncMap{"readerCount": readerCount}).Parse(shell("records", "Registered services") + `
<h1>{{if .Channels}}Registered channels{{else if .PersonalPage}}Personal services{{else}}Registered services{{end}}</h1>
<form method=get>{{if .PersonalPage}}{{if .DaemonOwner}}<label>Owner <select name=owner><option value="">All visible owners</option>{{range .Owners}}<option {{if eq . $.OwnerFilter}}selected{{end}}>{{.}}</option>{{end}}</select></label>{{else}}<span>Owned by <code>{{.You}}</code></span>{{end}}{{else}}<label>Scope <select name=scope><option value=all>All visible</option><option value=my {{if eq .Mine "my"}}selected{{end}}>My</option></select></label>{{end}}
<label>Delivery <select name=state><option value=all>All</option><option value=active {{if eq .State "active"}}selected{{end}}>Enabled</option><option value=inactive {{if eq .State "inactive"}}selected{{end}}>Disabled</option></select></label> <button>Filter</button></form>
{{if .PersonalPage}}<p class=muted>Personal is an owner-set grouping tag. It changes where a service appears here, not who may call it. {{if .DaemonOwner}}This per-owner view contains only Personal services visible through your normal access; it is not a node-wide inventory.{{else}}This page shows your Personal services.{{end}}</p>{{end}}
<p class=muted>Three separate facts, and none of them is health: the record&rsquo;s
 delivery setting, what the daemon <em>observed</em> about a read on its inbox,
 and whether the caller said it is reached some other way. <em>Enabled</em> does
 not establish that a send will be accepted — the owner&rsquo;s access and the
 allow list are checked as well. None of it is
 health: the daemon does not observe whether a process is alive, so this page does
 not say it. <em>Readers</em> counts every outstanding consume request,
 filtered and unfiltered together. Zero can mean a process is between reads;
 a positive count promises neither a matching message nor completed work.</p>
<table><caption>Records visible to you — not a count of this node</caption>
<thead><tr><th scope=col>Name<th scope=col>Owner<th scope=col>Delivery<th scope=col>Readers<th scope=col>Reached<th scope=col>Queued<th scope=col>Registration updated<th scope=col>Controls</tr></thead>
<tbody>
{{range .Records}}<tr><td><a href="/service?name={{.Name}}">{{.Name}}</a><td>{{.Owner}}<td>{{if .Disabled}}Disabled{{else}}Enabled{{end}}<td>{{readerCount .Readers}}<td>{{if .Proto}}external{{else}}<span class=muted>&mdash;</span>{{end}}<td>{{.Queued}}{{if .AtBound}} <span class=warn>at capacity when observed</span>{{end}}<td>{{if .At.IsZero}}<span class=muted>&iquest;</span>{{else}}{{.At.Format "2006-01-02 15:04"}}{{end}}<td>{{if .CanManage}}Manage{{else}}View{{end}}</tr>{{else}}<tr><td colspan=8>No matching records</tr>{{end}}
</tbody></table>
<h2>Register {{if .Channels}}channel{{else}}service{{end}}</h2>
<form method=post action=/service><input type=hidden name=action value=create>
<label>Name <input name=name required placeholder="name@realm"></label><p><label>Description <input name=descr></label></p>
{{if .Channels}}<input type=hidden name=kind value=topic><label>Delivery <select name=mode><option value=pubsub>Pub/sub</option><option value=queue>Queue</option></select></label>{{else if .PersonalPage}}<input type=hidden name=kind value=generic><input type=hidden name=personal value=on>{{else}}<label>Kind <select name=kind><option value=generic>Service</option><option value=agent>Agent</option></select></label> <label>Personal <input type=checkbox name=personal></label>{{end}}
<p><label>Allow <input name=allow></label> Empty allows only the owner and assigned Maintainers. Add names or * to share.</p>{{if not .Channels}}<p class=muted>A Personal service may name only other registered services directly. Users, groups, <code>*</code>, itself and Maintainers are refused.</p>{{end}}<button>Register</button></form>`))
var serviceDetail = template.Must(template.New("service").Funcs(template.FuncMap{"join": strings.Join, "readerCount": readerCount}).Parse(shell("records", "Service") + `
{{with .Record}}<h1>{{.Name}}{{if .Personal}} <span class=muted>· Personal</span>{{end}}</h1><p>Owner: {{.Owner}}{{with .Maintainers}} · Maintainers: {{.}}{{end}}{{if .Mode}} · Delivery: {{if eq .Mode "pubsub"}}a copy to each subscriber{{else}}one at a time{{end}}{{end}}</p>
<h2>Delivery setting</h2>
<p>Delivery: <strong>{{if .Disabled}}Disabled{{else}}Enabled{{end}}</strong>{{if .Disabled}} <span class=muted>— the bit does not say whether the owner turned it off or the name stopped being active</span>{{end}}</p>
<p class=muted>Not under either heading below, because it is neither: the daemon
 answers the stored bit OR the name having stopped being active, so the page
 cannot take it apart. And <em>Enabled</em> does not establish that a send will be
 accepted — the owner's access may be suspended, and the allow list and the queue
 bound are checked as well. It is the setting, not an answer about the next
 send.</p>
<h2>What the record declares</h2>
<p>Reached: {{if .Proto}}<strong>external</strong> <span class=muted>— a caller-supplied hint, not proof of anything</span>{{else}}this bus{{end}}</p>
<p>Queue bound: {{if .Bound}}{{.Bound}}{{else}}<span class=muted>unset — uses the daemon default, which is not readable here</span>{{end}}
 · Retention: {{if .TTL}}{{.TTL}}{{else}}<span class=muted>unset — no queue-imposed expiry; a message may still specify its own</span>{{end}}
 · When full: {{if eq .Full "ring"}}drop the oldest{{else}}refuse{{end}}</p>
<p class=muted>One of those three inherits. A bound the record leaves
 unset is resolved by the daemon and that resolved number is not something this
 page may read, so it is not shown. Retention has no default to inherit at all.
 The overflow policy is always the record&rsquo;s own: an empty one is normalised
 to <em>refuse</em> when the record is registered, so there is no unset to report.</p>
<h2>What the daemon observed</h2>
<p>Readers: <strong>{{readerCount .Readers}}</strong>
 <span class=muted>— outstanding consume requests, filtered and unfiltered together</span></p>
<p class=muted>Zero can mean a process is between reads and is not an offline
 signal. A positive count promises neither that a particular message matches
 nor that any work has finished.</p>
<p>Held now: {{.Queued}}{{if .AtBound}} · <span class=warn>at capacity when observed</span>{{end}}
 · Oldest held: {{if .Oldest}}{{.Oldest}}{{else}}<span class=muted>&mdash;</span>{{end}}
 <span class=muted>— the read does not prune first, so some of these may already have outlived their TTL</span></p>
<p>Accepted: {{.In}} · Dequeued: {{.Out}} · Dropped: {{.Dropped}} · Expired: {{.Expired}}
 <span class=muted>— cumulative across restarts, restored from the snapshot. Dequeued is handed to a reader, which is not completed</span></p>
<p>Registration updated: {{if .At.IsZero}}<span class=muted>&iquest;</span>{{else}}{{.At.Format "2006-01-02 15:04:05"}}{{end}}</p>
<p>Configuration digest: <code>{{.ConfigSHA}}</code></p>
{{if eq .Mode "pubsub"}}<h2>Subscriptions</h2>
{{range .Subs}}<p>{{.}}{{if $.Record.CanManage}}<form method=post action=/service><input type=hidden name=name value="{{$.Record.Name}}"><input type=hidden name=subscriber value="{{.}}"><button name=action value=remove-subscriber>Remove subscription</button></form>{{end}}</p>{{else}}<p>No subscribers</p>{{end}}
<form method=post action=/service><input type=hidden name=name value="{{.Name}}"><button name=action value=subscribe>Subscribe my inbox</button><button name=action value=unsubscribe>Unsubscribe my inbox</button></form>{{end}}
{{if .CanManage}}
<form method=post action=/service><input type=hidden name=name value="{{.Name}}"><input type=hidden name=action value=save>
<p><label>Description <input name=descr value="{{.Descr}}"></label></p>
<p><label>Address <input name=addr value="{{.Addr}}"></label></p>
<p><label>Protocol <input name=protocol value="{{.Proto}}"></label></p>
{{if .Personal}}<p class=muted>Personal classification, Allow and Maintainers are changed together in the owner form below.</p>{{else}}<input type=hidden name=edit_allow value=1><p><label>Allow (principals, groups or *) <input name=allow value="{{join .Allow " "}}"></label></p><p>Empty allows only the owner and assigned Maintainers. Add names or * to share.</p>{{end}}
<p><label>Refuse master access <input type=checkbox name=no_master {{if .NoMaster}}checked{{end}}></label></p>
<p><label>Queue TTL <input name=ttl value="{{.TTL}}" placeholder="default"></label></p>
<p><label>Queue capacity (0 uses default) <input type=number min=0 name=bound value="{{.Bound}}"></label></p>
<p><label>Overflow <select name=overflow><option value=strict>Refuse</option><option value=ring {{if eq .Full "ring"}}selected{{end}}>Drop oldest</option></select></label></p><button>Save settings</button></form>
<form method=post action=/service><input type=hidden name=name value="{{.Name}}"><button name=action value="{{if .Disabled}}enable{{else}}disable{{end}}">{{if .Disabled}}Enable{{else}}Disable{{end}}</button></form>
<h2>Replace configuration</h2><form method=post action=/service><input type=hidden name=name value="{{.Name}}"><input type=hidden name=action value=configure><label>New configuration <textarea name=config rows=6 cols=60 required autocomplete=off></textarea></label><p>Existing private configuration is never displayed.</p><button>Replace configuration</button></form>
{{if .CanTransfer}}{{if eq .Kind "generic"}}<h2>Classification and sharing</h2><form method=post action=/service><input type=hidden name=name value="{{.Name}}"><input type=hidden name=action value=personal><p><label>Personal <input type=checkbox name=personal {{if .Personal}}checked{{end}}></label> Groups this service in the owner&rsquo;s Personal services page; it does not change access.</p><p><label>Allow <input name=allow value="{{join .Allow " "}}"></label></p><p><label>Maintainers group <select name=maintainers><option value="">None</option>{{range $group,$members := $.Groups}}<option {{if eq $group $.Record.Maintainers}}selected{{end}}>{{$group}}</option>{{end}}</select></label></p><p class=muted>When Personal is checked, Allow may name only other registered services directly. Users, groups, <code>*</code>, this service and Maintainers are refused. Clear Personal in this same form before adding any of them.</p><button>Save classification and sharing</button></form>{{else}}<h2>Maintainers</h2><form method=post action=/service><input type=hidden name=name value="{{.Name}}"><input type=hidden name=action value=maintainers><label>Group <select name=maintainers><option value="">None</option>{{range $group,$members := $.Groups}}<option {{if eq $group $.Record.Maintainers}}selected{{end}}>{{$group}}</option>{{end}}</select></label><button>Assign</button></form>{{end}}
{{if ne .Name .Owner}}<h2>Transfer ownership</h2><form method=post action=/service><input type=hidden name=name value="{{.Name}}"><input type=hidden name=action value=transfer><label>New owner <input name=owner required></label><p>The new owner must have a registered identity. Existing service credentials remain valid; transfer does not revoke copies already held.</p><button>Transfer ownership</button></form>{{end}}{{end}}
<h2>Remove registration</h2><form method=post action=/service><input type=hidden name=name value="{{.Name}}"><input type=hidden name=action value=delete><p>Drain the queue and stop readers first. <strong>No registration, no access:</strong> the credential goes with the address and nothing answers to this name afterwards. A person's own credential stays, because it is not a record's to drop.</p><button>Remove idle service</button></form>
{{else}}<p>{{.Descr}}</p><p>You can view this record; its owner and assigned maintainers can manage it.</p>{{end}}{{end}}`))
var groupList = template.Must(template.New("groups").Funcs(template.FuncMap{"join": strings.Join}).Parse(shell("groups", "Groups") + `
<h1>Groups</h1>{{range $name,$members := .Groups}}<h2>{{$name}}</h2>{{if and $.Administrator (or $.DaemonOwner (ne $name "@administrators"))}}<form method=post action=/groups><input type=hidden name=name value="{{$name}}"><label>Members <input name=members value="{{join $members " "}}"></label><button name=action value=save>Save members</button></form>{{if eq $name "@administrators"}}<p class=muted>The daemon owner stays in this group.</p>{{end}}{{end}}{{else}}<p>No groups registered.</p>{{end}}
{{if .Administrator}}<h2>Create group</h2><form method=post action=/groups><label>Name <input name=name placeholder="@operators" required></label><label>Members <input name=members placeholder="user@realm"></label><button name=action value=save>Create</button></form>{{else}}<p>Daemon administrators manage group membership. Only the daemon owner changes the maintainers group. Service owners can assign an existing group to their own services.</p>{{end}}`))

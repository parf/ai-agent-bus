package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"log"
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
		log.Printf("dashboard bus unavailable: %v", err)
		show(w, http.StatusBadGateway, problem{You: you,
			Title:  "The bus is not answering",
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

// localProblem is for browser input rejected by the web face before it calls
// the daemon. Keeping that distinction out of fail avoids claiming that the
// daemon refused a request it never received.
func localProblem(w http.ResponseWriter, r *http.Request, you string, code int, detail string) {
	show(w, code, problem{
		You:    you,
		Title:  "That request was not understood",
		Detail: detail,
		Advice: "Nothing was sent to the daemon. Return to the page and use the action shown there.",
	}, r)
}

var problemPage = template.Must(template.New("problem").Funcs(template.FuncMap{"titleMark": titleMark}).Parse(shellTitle("", `{{.Title}}`) + `<div class=page-title><h1>{{titleMark "problem"}} {{.Title}}</h1></div>
{{with .Detail}}<p class=warn>{{.}}</p>{{end}}
<p>{{.Advice}}</p>
{{with .Back}}<p><a href="{{.}}">{{$.BackLabel}}</a></p>{{end}}
`))

type adminView struct {
	pageInfo
	You             string `json:"you"`
	DaemonOwner     bool   `json:"daemon_owner"`
	Administrator   bool   `json:"administrator"`
	Records         []protocol.Record
	Record          protocol.Record
	OwnerUser       *protocol.User
	Groups          map[string][]string
	GroupName       string
	GroupMembers    []string
	GroupReferences []groupReference
	CanEditGroup    bool
	Mine, State     string
	Query, Kind     string
	Readers         string
	Work            string
	Sort            string
	OwnerFilter     string
	Current         string
	Return          string
	Previous        string
	Next            string
	ClearFilters    string
	Matched         int
	CategoryTotal   int
	Start           int
	End             int
	HasFilters      bool
	Owners          []string
	Channels        bool
	Agents          bool
	PersonalPage    bool
	Status          core.Status
	Activity        activityPresentation
	SectionLinks    []viewLink
	FilterLinks     []viewLink
	KindLinks       []viewLink
	ReaderLinks     []viewLink
	WorkLinks       []viewLink
	Form            formState
}

type groupReference struct {
	Name, Kind string
	Uses       []string
}

func groupReaches(groups map[string][]string, outer, target string, seen map[string]bool) bool {
	if outer == target {
		return true
	}
	if seen[outer] {
		return false
	}
	seen[outer] = true
	for _, member := range groups[outer] {
		if strings.HasPrefix(member, "@") && groupReaches(groups, member, target, seen) {
			return true
		}
	}
	return false
}

func groupUses(terms []string, groups map[string][]string, target, label string) []string {
	var uses []string
	for _, term := range terms {
		if !strings.HasPrefix(term, "@") || !groupReaches(groups, term, target, map[string]bool{}) {
			continue
		}
		if term == target {
			uses = append(uses, label)
		} else {
			uses = append(uses, label+" via "+term)
		}
	}
	return uses
}

func referencesToGroup(records []protocol.Record, groups map[string][]string, target string) []groupReference {
	var refs []groupReference
	for _, record := range records {
		uses := groupUses(record.Allow, groups, target, "ACL")
		uses = append(uses, groupUses(record.Maintainers, groups, target, "Maintainers")...)
		if len(uses) != 0 {
			refs = append(refs, groupReference{Name: record.Name, Kind: record.Kind, Uses: uses})
		}
	}
	return refs
}

// formState carries only fields that are safe to render back after a refused
// submission. Tokens and private configuration never enter it. The daemon is
// still the validator; this is presentation state for the one response only.
type formState struct {
	Action string
	Target string
	Field  string
	Error  string
	Values map[string]string
}

func (f formState) Is(action string) bool { return f.Action == action && f.Error != "" }
func (f formState) Value(name string) string {
	if f.Values == nil {
		return ""
	}
	return f.Values[name]
}
func (f formState) Checked(name string) bool { return f.Value(name) == "on" }
func (f formState) Invalid(name string) bool { return f.Error != "" && f.Field == name }
func (f formState) Matches(action, name, value string) bool {
	return f.Is(action) && f.Value(name) == value
}
func retainedForm(action, message string, values url.Values, names ...string) formState {
	f := formState{Action: action, Target: action, Error: message, Values: make(map[string]string, len(names))}
	for _, name := range names {
		f.Values[name] = values.Get(name)
	}
	return f
}

// busMessage is the daemon's own wording for a refusal, unwrapped from the
// JSON body it answers with. It is the only part of a failed call that may be
// shown: everything else is transport text naming the backend address.
func busMessage(bad *busError) string {
	message := strings.TrimSpace(bad.message)
	var payload struct {
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(message), &payload) == nil && payload.Error != "" {
		return payload.Error
	}
	return message
}

// sectionProblem is what a page may say when one section of it failed and the
// rest is still true. A refusal the daemon worded is caller-facing and is
// shown; anything else is a transport failure whose text carries the socket or
// TCP address this child talks to, so it is logged here and replaced. The
// whole-page path does the same in fail(). Every degraded section goes through
// here: rendering err.Error() directly put the backend address on the record
// and Account pages, and a section that fails must still say so rather than
// read as an answer of nothing.
func sectionProblem(what string, err error) string {
	var bad *busError
	if !errors.As(err, &bad) {
		log.Printf("dashboard %s unavailable: %v", what, err)
		return "the daemon did not answer"
	}
	return busMessage(bad)
}

func formRefusal(err error) (int, string, bool) {
	var bad *busError
	if !errors.As(err, &bad) {
		return 0, "", false
	}
	message := busMessage(bad)
	return bad.code, message, bad.code == http.StatusBadRequest || bad.code == http.StatusNotFound || bad.code == http.StatusConflict || bad.code == http.StatusPreconditionFailed || bad.code == http.StatusTooManyRequests
}

func renderForm(w http.ResponseWriter, code int, t *template.Template, value any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	render(w, t, value)
}

const formErrorSummary = `{{if $.Form.Error}}<section class=form-error role=alert aria-labelledby=form-error-title><h2 id=form-error-title>Check this form</h2><p>{{$.Form.Error}}</p><p><a href="#form-{{$.Form.Target}}">Review the submitted fields</a></p></section>{{end}}`

type viewLink struct {
	Href, Label string
	Class       string
	Count       int
	Counted     bool
	Current     bool
}

func pageURL(path string, q url.Values) string {
	if encoded := q.Encode(); encoded != "" {
		return path + "?" + encoded
	}
	return path
}

func cloneValues(in url.Values) url.Values {
	out := make(url.Values, len(in))
	for key, values := range in {
		out[key] = append([]string(nil), values...)
	}
	return out
}

func queryWith(path string, in url.Values, key, value string) string {
	q := cloneValues(in)
	if value == "" {
		q.Del(key)
	} else {
		q.Set(key, value)
	}
	return pageURL(path, q)
}

func recordReturn(raw, fallback string) string {
	u, err := url.Parse(local(raw))
	if err != nil || u.Path != "/services" && u.Path != "/personal" && u.Path != "/channels" && u.Path != "/agents" {
		return fallback
	}
	u.Fragment = ""
	return u.RequestURI()
}

type serviceConfirmation struct {
	adminView
	Action, NewOwner string
}

func readerSnapshot(readers *int) string {
	if readers == nil {
		return "unavailable"
	}
	return strconv.Itoa(*readers)
}

func conditionsChanged(w http.ResponseWriter, r *http.Request, v adminView, detail string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusConflict)
	render(w, problemPage, problem{
		pageInfo:  requestInfo(r),
		You:       v.You,
		Title:     "The conditions changed",
		Detail:    detail,
		Advice:    "The daemon was re-read before the action. Review the current record and confirm again if the action still applies.",
		Back:      "/service-danger?name=" + url.QueryEscape(v.Record.Name),
		BackLabel: "Review the current Danger Zone",
	})
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
	v.You, v.Administrator, v.DaemonOwner, v.Status = node.You, node.Administrator, node.DaemonOwner, node.Status
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
	loadRecord := func(w http.ResponseWriter, r *http.Request, v *adminView, name string) bool {
		if err := c.get(cookie(r), "/lookup?name="+url.QueryEscape(name), &v.Record); err != nil {
			fail(w, r, v.You, err)
			return false
		}
		switch {
		case v.Record.Personal:
			v.Current = "personal"
		case channelRecord(v.Record.Kind):
			v.Current = "channels"
		case agentRecord(v.Record.Kind):
			v.Current = "agents"
		default:
			v.Current = "services"
		}
		return true
	}
	loadServiceDetails := func(w http.ResponseWriter, r *http.Request, v *adminView, name string) bool {
		if !loadRecord(w, r, v, name) {
			return false
		}
		if err := c.get(cookie(r), "/groups", &v.Groups); err != nil {
			fail(w, r, v.You, err)
			return false
		}
		// One directory answer supplies the caller-visible owner profile and its
		// local thumbnail. A hidden owner stays absent, and no page render ever
		// performs a provider or per-photo daemon lookup.
		var users []protocol.User
		if err := c.get(cookie(r), "/users", &users); err == nil {
			for i := range users {
				if users[i].Kind == protocol.DirectoryUser && users[i].Name == v.Record.Owner {
					owner := users[i]
					v.OwnerUser = &owner
					break
				}
			}
		}
		var points []core.ActivityPoint
		if err := c.get(cookie(r), "/activity?name="+url.QueryEscape(v.Record.Name), &points); err != nil {
			v.Activity = activityView(nil, v.Record.Name, v.Status.Up, false)
			v.Activity.Unavailable = sectionProblem("record activity", err)
		} else {
			v.Activity = activityView(points, v.Record.Name, v.Status.Up, false)
		}
		return true
	}
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
		v.Query = strings.TrimSpace(r.URL.Query().Get("q"))
		v.Kind = r.URL.Query().Get("kind")
		v.Readers, v.Work, v.Sort = r.URL.Query().Get("readers"), r.URL.Query().Get("work"), r.URL.Query().Get("sort")
		if v.Mine != "my" {
			v.Mine = ""
		}
		if v.State != "active" && v.State != "inactive" {
			v.State = ""
		}
		if !protocol.ValidKind(v.Kind) || v.Kind == protocol.KindService {
			v.Kind = ""
		}
		if v.Readers != "present" && v.Readers != "none" && v.Readers != "unavailable" {
			v.Readers = ""
		}
		if v.Work != "held" {
			v.Work = ""
		}
		if v.Sort != "updated" && v.Sort != "queued" {
			v.Sort = ""
		}
		v.Channels, v.Agents = r.URL.Path == "/channels", r.URL.Path == "/agents"
		v.PersonalPage = r.URL.Path == "/personal"
		if !v.Channels {
			// Agents and services are each one kind, so a kind filter would
			// narrow nothing; only the channels page lists more than one.
			v.Kind = ""
		}
		switch {
		case v.Channels:
			v.Current = "channels"
		case v.Agents:
			v.Current = "agents"
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
		allServices, myServices, personalServices, allChannels, allAgents, myAgents := 0, 0, 0, 0, 0, 0
		for _, record := range records {
			switch {
			case channelRecord(record.Kind):
				allChannels++
			case agentRecord(record.Kind) && !record.Personal:
				allAgents++
				if record.Owner == v.You {
					myAgents++
				}
			case record.Personal:
				if v.DaemonOwner || record.Owner == v.You {
					personalServices++
				}
			default:
				allServices++
				if record.Owner == v.You {
					myServices++
				}
			}
			if v.PersonalPage {
				if !agentRecord(record.Kind) || !record.Personal {
					continue
				}
				owners[record.Owner] = true
				if v.OwnerFilter != "" && record.Owner != v.OwnerFilter {
					continue
				}
			} else if listPathFor(record.Kind) != r.URL.Path || (v.Agents && record.Personal) {
				continue
			}
			if v.Mine == "my" && record.Owner != v.You {
				continue
			}
			// CategoryTotal is the selected view before toolbar filters. It lets
			// the face distinguish an empty registry category from a filter that
			// happens to match nothing.
			v.CategoryTotal++
			if v.State == "active" && record.Disabled || v.State == "inactive" && !record.Disabled {
				continue
			}
			if !matchesReaderFilter(record.Readers, v.Readers) {
				continue
			}
			if v.Work == "held" && record.Queued == 0 {
				continue
			}
			if v.Channels {
				if v.Kind != "" && record.Kind != v.Kind {
					continue
				}
			}
			if v.Query != "" {
				needle := strings.ToLower(v.Query)
				if !strings.Contains(strings.ToLower(record.Name), needle) &&
					!strings.Contains(strings.ToLower(record.Descr), needle) &&
					!strings.Contains(strings.ToLower(record.Owner), needle) {
					continue
				}
			}
			v.Records = append(v.Records, record)
		}
		stateQuery := url.Values{}
		if v.State != "" {
			stateQuery.Set("state", v.State)
		}
		if v.Query != "" {
			stateQuery.Set("q", v.Query)
		}
		if v.Kind != "" && v.Channels {
			stateQuery.Set("kind", v.Kind)
		}
		if v.Readers != "" {
			stateQuery.Set("readers", v.Readers)
		}
		if v.Work != "" {
			stateQuery.Set("work", v.Work)
		}
		if v.Sort != "" {
			stateQuery.Set("sort", v.Sort)
		}
		switch {
		case v.Channels:
			v.SectionLinks = []viewLink{
				{Href: pageURL("/channels", stateQuery), Label: "All", Count: allChannels, Counted: true, Current: true},
				{Href: "/channels/new", Label: "Register channel"},
			}
		case v.Agents, v.PersonalPage:
			agentQuery, myAgentQuery, personalQuery := cloneValues(stateQuery), cloneValues(stateQuery), cloneValues(stateQuery)
			myAgentQuery.Set("scope", "my")
			if v.DaemonOwner && v.OwnerFilter != "" {
				personalQuery.Set("owner", v.OwnerFilter)
			}
			v.SectionLinks = []viewLink{
				{Href: pageURL("/agents", agentQuery), Label: "All", Count: allAgents, Counted: true, Current: !v.PersonalPage && v.Mine == ""},
				{Href: pageURL("/agents", myAgentQuery), Label: "My", Class: "my-view", Count: myAgents, Counted: true, Current: !v.PersonalPage && v.Mine == "my"},
				{Href: pageURL("/personal", personalQuery), Label: "Personal", Class: "personal-view", Count: personalServices, Counted: true, Current: v.PersonalPage},
				{Href: "/agents/new", Label: "Register agent"},
			}
		default:
			// Services, which is the only page left here: Personal is agent-only
			// and was taken by the branch above, so nothing on this one can be it.
			allQuery, myQuery := cloneValues(stateQuery), cloneValues(stateQuery)
			myQuery.Set("scope", "my")
			v.SectionLinks = []viewLink{
				{Href: pageURL("/services", allQuery), Label: "All", Count: allServices, Counted: true, Current: v.Mine == ""},
				{Href: pageURL("/services", myQuery), Label: "My", Class: "my-view", Count: myServices, Counted: true, Current: v.Mine == "my"},
				{Href: "/services/new", Label: "Register service"},
			}
		}
		filterBase := url.Values{}
		if v.Mine == "my" && !v.PersonalPage {
			filterBase.Set("scope", "my")
		}
		if v.PersonalPage && v.DaemonOwner && v.OwnerFilter != "" {
			filterBase.Set("owner", v.OwnerFilter)
		}
		if v.Query != "" {
			filterBase.Set("q", v.Query)
		}
		if v.Kind != "" && v.Channels {
			filterBase.Set("kind", v.Kind)
		}
		if v.Readers != "" {
			filterBase.Set("readers", v.Readers)
		}
		if v.Work != "" {
			filterBase.Set("work", v.Work)
		}
		if v.Sort != "" {
			filterBase.Set("sort", v.Sort)
		}
		filterPath := r.URL.Path
		v.FilterLinks = []viewLink{
			{Href: pageURL(filterPath, cloneValues(filterBase)), Label: "All", Current: v.State == ""},
			{Href: queryWith(filterPath, filterBase, "state", "active"), Label: "Enabled", Current: v.State == "active"},
			{Href: queryWith(filterPath, filterBase, "state", "inactive"), Label: "Disabled", Current: v.State == "inactive"},
		}
		if v.Channels {
			// The three kinds this page lists, and no more: an agent is on its
			// own page now, so a filter for one here would only ever empty the
			// table. See docs/03-services-and-topics.md#five-record-kinds.
			kindBase := cloneValues(filterBase)
			kindBase.Del("kind")
			if v.State != "" {
				kindBase.Set("state", v.State)
			}
			v.KindLinks = []viewLink{
				{Href: pageURL(filterPath, cloneValues(kindBase)), Label: "All", Current: v.Kind == ""},
				{Href: queryWith(filterPath, kindBase, "kind", protocol.KindUser), Label: entityLabel(protocol.KindUser), Current: v.Kind == protocol.KindUser},
				{Href: queryWith(filterPath, kindBase, "kind", protocol.KindQueue), Label: entityLabel(protocol.KindQueue), Current: v.Kind == protocol.KindQueue},
				{Href: queryWith(filterPath, kindBase, "kind", protocol.KindPubSub), Label: entityLabel(protocol.KindPubSub), Current: v.Kind == protocol.KindPubSub},
			}
		}
		readerBase := cloneValues(filterBase)
		readerBase.Del("readers")
		if v.State != "" {
			readerBase.Set("state", v.State)
		}
		v.ReaderLinks = []viewLink{
			{Href: pageURL(filterPath, cloneValues(readerBase)), Label: "All", Current: v.Readers == ""},
			{Href: queryWith(filterPath, readerBase, "readers", "present"), Label: "Reading now", Current: v.Readers == "present"},
			{Href: queryWith(filterPath, readerBase, "readers", "none"), Label: "No reader now", Current: v.Readers == "none"},
			{Href: queryWith(filterPath, readerBase, "readers", "unavailable"), Label: "Unavailable", Current: v.Readers == "unavailable"},
		}
		workBase := cloneValues(filterBase)
		workBase.Del("work")
		if v.State != "" {
			workBase.Set("state", v.State)
		}
		v.WorkLinks = []viewLink{
			{Href: pageURL(filterPath, cloneValues(workBase)), Label: "All", Current: v.Work == ""},
			{Href: queryWith(filterPath, workBase, "work", "held"), Label: "Holding work", Current: v.Work == "held"},
		}
		for owner := range owners {
			v.Owners = append(v.Owners, owner)
		}
		sort.Strings(v.Owners)
		sort.SliceStable(v.Records, func(i, j int) bool {
			a, b := v.Records[i], v.Records[j]
			switch v.Sort {
			case "updated":
				if !a.At.Equal(b.At) {
					return a.At.After(b.At)
				}
			case "queued":
				aWork, bWork := a.Queued, b.Queued
				if v.Channels {
					if a.Kind == protocol.KindPubSub {
						aWork = a.In
					}
					if b.Kind == protocol.KindPubSub {
						bWork = b.In
					}
				}
				if aWork != bWork {
					return aWork > bWork
				}
			}
			return a.Name < b.Name
		})
		v.Matched = len(v.Records)
		const pageSize = 25
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		last := max(1, (v.Matched+pageSize-1)/pageSize)
		page = min(max(1, page), last)
		listingQuery := cloneValues(stateQuery)
		if v.Mine == "my" && !v.PersonalPage {
			listingQuery.Set("scope", "my")
		}
		if v.PersonalPage && v.DaemonOwner && v.OwnerFilter != "" {
			listingQuery.Set("owner", v.OwnerFilter)
		}
		link := func(n int) string {
			q := cloneValues(listingQuery)
			q.Set("page", strconv.Itoa(n))
			return pageURL(filterPath, q)
		}
		v.Return = link(page)
		if page > 1 {
			v.Previous = link(page - 1)
		}
		if page < last {
			v.Next = link(page + 1)
		}
		start, end := (page-1)*pageSize, min(page*pageSize, v.Matched)
		if v.Matched > 0 {
			v.Start, v.End = start+1, end
		}
		v.Records = v.Records[start:end]
		clearQuery := url.Values{}
		if v.Mine == "my" && !v.PersonalPage {
			clearQuery.Set("scope", "my")
		}
		if v.PersonalPage && v.DaemonOwner && v.OwnerFilter != "" {
			clearQuery.Set("owner", v.OwnerFilter)
		}
		v.ClearFilters = pageURL(filterPath, clearQuery)
		v.HasFilters = v.Query != "" || v.State != "" || v.Kind != "" || v.Readers != "" || v.Work != "" || v.Sort != ""
		render(w, serviceList, v)
	}
	mux.HandleFunc("GET /agents", listing)
	mux.HandleFunc("GET /services", listing)
	mux.HandleFunc("GET /channels", listing)
	mux.HandleFunc("GET /personal", listing)
	mux.HandleFunc("GET /services/new", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		v.Current = "services"
		render(w, serviceNew, v)
	})
	mux.HandleFunc("GET /agents/new", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		v.Current, v.Agents = "agents", true
		render(w, serviceNew, v)
	})
	mux.HandleFunc("GET /channels/new", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		v.Current, v.Channels = "channels", true
		render(w, serviceNew, v)
	})
	recordDetail := func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		if !loadServiceDetails(w, r, &v, r.URL.Query().Get("name")) {
			return
		}
		fallback := listPathFor(v.Record.Kind)
		if v.Record.Personal {
			fallback = "/personal"
		}
		v.Return = recordReturn(r.URL.Query().Get("return"), fallback)
		render(w, serviceDetail, v)
	}
	mux.HandleFunc("GET /service", recordDetail) // one page, reached by the name of each listing
	mux.HandleFunc("GET /channel", recordDetail)
	mux.HandleFunc("GET /agent", recordDetail)
	mux.HandleFunc("GET /service-danger", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		if !loadRecord(w, r, &v, r.URL.Query().Get("name")) {
			return
		}
		if !v.Record.CanManage {
			fail(w, r, v.You, &busError{code: http.StatusForbidden, message: "only the owner or an assigned Maintainer can manage this record"})
			return
		}
		render(w, serviceDanger, v)
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
		v.SectionLinks = []viewLink{{Href: "/groups", Label: "All groups", Count: len(v.Groups), Counted: true, Current: true}}
		if v.Administrator {
			v.SectionLinks = append(v.SectionLinks, viewLink{Href: "/groups/new", Label: "Register group"})
		}
		render(w, groupList, v)
	})
	mux.HandleFunc("GET /group", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		if err := c.get(cookie(r), "/groups", &v.Groups); err != nil {
			fail(w, r, v.You, err)
			return
		}
		name := r.URL.Query().Get("name")
		members, exists := v.Groups[name]
		if !exists {
			fail(w, r, v.You, &busError{code: http.StatusNotFound, message: "no such group"})
			return
		}
		v.Current = "groups"
		v.GroupName = name
		v.GroupMembers = append([]string(nil), members...)
		v.CanEditGroup = v.Administrator && (v.DaemonOwner || name != core.AdministratorsGroup)
		if err := c.get(cookie(r), "/ls", &v.Records); err != nil {
			fail(w, r, v.You, err)
			return
		}
		v.GroupReferences = referencesToGroup(v.Records, v.Groups, name)
		render(w, groupDetail, v)
	})
	mux.HandleFunc("GET /groups/new", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		v.Current = "groups"
		if !v.Administrator {
			fail(w, r, v.You, &busError{code: http.StatusForbidden, message: "only a daemon Administrator can register a group"})
			return
		}
		render(w, groupNew, v)
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
				localProblem(w, r, v.You, http.StatusBadRequest, "The submitted form could not be read.")
				return
			}
			next(w, r, v)
		}
	}
	renderServiceFormError := func(w http.ResponseWriter, r *http.Request, v adminView, action string, code int, message string, field ...string) bool {
		setField := func(form formState) formState {
			if len(field) != 0 {
				form.Field = field[0]
			}
			return form
		}
		switch action {
		case "create":
			kind := r.PostForm.Get("kind")
			v.Channels, v.Agents = channelRecord(kind), agentRecord(kind)
			switch {
			case v.Channels:
				v.Current = "channels"
			case v.Agents:
				v.Current = "agents"
			default:
				v.Current = "services"
			}
			// Everything the three forms between them ask for: a refusal on one
			// field must not empty the others, and the service form asks for an
			// address and a protocol the daemon will not do without.
			v.Form = setField(retainedForm(action, message, r.PostForm, "name", "descr", "kind", "addr", "protocol", "personal", "allow"))
			renderForm(w, code, serviceNew, v)
			return true
		case "save":
			if !loadServiceDetails(w, r, &v, r.PostForm.Get("name")) {
				return true
			}
			v.Form = setField(retainedForm(action, message, r.PostForm, "descr", "addr", "protocol", "ttl", "overflow", "bound", "allow", "edit_allow"))
			renderForm(w, code, serviceDetail, v)
			return true
		case "maintainers":
			if !loadServiceDetails(w, r, &v, r.PostForm.Get("name")) {
				return true
			}
			if agentRecord(v.Record.Kind) && v.Record.CanTransfer {
				// The agent editor changes classification,
				// access and Maintainers atomically. A refusal from the former
				// Maintainers-only action returns to that real form with the
				// other two values filled from the fresh record, so retrying
				// cannot clear either by accident.
				values := url.Values{
					"allow":       {strings.Join(v.Record.Allow, "\n")},
					"maintainers": {r.PostForm.Get("maintainers")},
				}
				if v.Record.Personal {
					values.Set("personal", "on")
				}
				v.Form = setField(retainedForm("personal", message, values, "personal", "allow", "maintainers"))
			} else {
				v.Form = setField(retainedForm(action, message, r.PostForm, "maintainers"))
			}
			renderForm(w, code, serviceDetail, v)
			return true
		case "personal":
			if !loadServiceDetails(w, r, &v, r.PostForm.Get("name")) {
				return true
			}
			v.Form = setField(retainedForm(action, message, r.PostForm, "personal", "allow", "maintainers"))
			renderForm(w, code, serviceDetail, v)
			return true
		case "configure", "transfer":
			if !loadRecord(w, r, &v, r.PostForm.Get("name")) {
				return true
			}
			if !v.Record.CanManage {
				return false
			}
			// Configuration is deliberately absent: a rejected replacement is
			// still a secret and must not return in HTML.
			keys := []string(nil)
			if action == "transfer" {
				keys = []string{"owner"}
			}
			v.Form = setField(retainedForm(action, message, r.PostForm, keys...))
			renderForm(w, code, serviceDanger, v)
			return true
		}
		return false
	}
	mux.HandleFunc("POST /service-confirm", mutate(func(w http.ResponseWriter, r *http.Request, v adminView) {
		name, action := r.PostForm.Get("name"), r.PostForm.Get("action")
		if !loadRecord(w, r, &v, name) {
			return
		}
		confirm := serviceConfirmation{adminView: v, Action: action}
		switch action {
		case "transfer":
			if !v.Record.CanTransfer || v.Record.Name == v.Record.Owner {
				fail(w, r, v.You, &busError{code: http.StatusForbidden, message: "only this record's owner or the daemon owner can transfer it"})
				return
			}
			confirm.NewOwner = strings.TrimSpace(r.PostForm.Get("owner"))
			if confirm.NewOwner == "" {
				renderServiceFormError(w, r, v, action, http.StatusBadRequest, "New owner is required.", "owner")
				return
			}
		case "delete":
			if !v.Record.CanManage {
				fail(w, r, v.You, &busError{code: http.StatusForbidden, message: "only the owner or an assigned Maintainer can remove this record"})
				return
			}
		default:
			localProblem(w, r, v.You, http.StatusBadRequest, "That confirmation action is not available.")
			return
		}
		render(w, serviceConfirm, confirm)
	}))
	mux.HandleFunc("POST /service", mutate(func(w http.ResponseWriter, r *http.Request, v adminView) {
		name := r.PostForm.Get("name")
		action := r.PostForm.Get("action")
		change := core.Management{Name: name}
		var err error
		targetKind := ""
		confirmed := r.PostForm.Get("confirmed") == "1"
		if confirmed && (action == "transfer" || action == "delete") {
			if !loadRecord(w, r, &v, name) {
				return
			}
			targetKind = v.Record.Kind
			changed := v.Record.Owner != r.PostForm.Get("expected_owner")
			if action == "transfer" {
				changed = changed || !v.Record.CanTransfer || v.Record.Name == v.Record.Owner
			} else {
				expectedQueued, parseErr := strconv.Atoi(r.PostForm.Get("expected_queued"))
				changed = changed || parseErr != nil || v.Record.Queued != expectedQueued || readerSnapshot(v.Record.Readers) != r.PostForm.Get("expected_readers") || !v.Record.CanManage
			}
			if changed {
				conditionsChanged(w, r, v, "The record no longer has the owner, queue or reader state shown on the confirmation page.")
				return
			}
		}
		if action == "transfer" && !confirmed {
			var record protocol.Record
			if lookupErr := c.get(cookie(r), "/lookup?name="+url.QueryEscape(name), &record); lookupErr != nil {
				fail(w, r, v.You, lookupErr)
				return
			}
			targetKind = record.Kind
		}
		switch action {
		case "save":
			descr, addr, proto, ttl, overflow := r.PostForm.Get("descr"), r.PostForm.Get("addr"), r.PostForm.Get("protocol"), r.PostForm.Get("ttl"), r.PostForm.Get("overflow")
			bound, parseErr := strconv.Atoi(r.PostForm.Get("bound"))
			if parseErr != nil {
				renderServiceFormError(w, r, v, action, http.StatusBadRequest, "Queue capacity must be a whole number.", "bound")
				return
			}
			change.Descr, change.Addr, change.Proto, change.TTL, change.Full, change.Bound = &descr, &addr, &proto, &ttl, &overflow, &bound
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
			maintainers := protocol.MaintainerList(strings.Fields(r.PostForm.Get("maintainers")))
			change.Maintainers = &maintainers
			err = c.post(cookie(r), "/manage", change)
		case "personal":
			personal := r.PostForm.Get("personal") == "on"
			allow := strings.Fields(r.PostForm.Get("allow"))
			maintainers := protocol.MaintainerList(strings.Fields(r.PostForm.Get("maintainers")))
			change.Personal, change.Allow, change.Maintainers = &personal, &allow, &maintainers
			err = c.post(cookie(r), "/manage", change)
		case "transfer":
			owner := r.PostForm.Get("owner")
			change.Owner = &owner
			err = c.post(cookie(r), "/manage", change)
		case "configure":
			cfg := json.RawMessage(r.PostForm.Get("config"))
			if !json.Valid(cfg) {
				renderServiceFormError(w, r, v, action, http.StatusBadRequest, "Configuration must be valid JSON. The submitted configuration is not shown again.", "config")
				return
			}
			err = c.post(cookie(r), "/configure", struct {
				Name   string
				Config json.RawMessage
			}{name, cfg})
		case "create":
			kind := r.PostForm.Get("kind")
			if !protocol.ValidKind(kind) {
				renderServiceFormError(w, r, v, action, http.StatusBadRequest, "Choose a valid record kind.", "kind")
				return
			}
			targetKind = kind
			err = c.post(cookie(r), "/register", protocol.Record{Name: name, Kind: kind, Addr: r.PostForm.Get("addr"), Proto: r.PostForm.Get("protocol"), Descr: r.PostForm.Get("descr"), Allow: strings.Fields(r.PostForm.Get("allow")), Personal: r.PostForm.Get("personal") == "on"})
		case "subscribe", "unsubscribe":
			err = c.post(cookie(r), "/subscribe", struct {
				Topic string
				Off   bool
			}{name, r.PostForm.Get("action") == "unsubscribe"})
		case "remove-subscriber":
			err = c.post(cookie(r), "/subscriber/remove", map[string]string{"topic": name, "subscriber": r.PostForm.Get("subscriber")})
		case "delete":
			var record protocol.Record
			if lookupErr := c.get(cookie(r), "/lookup?name="+url.QueryEscape(name), &record); lookupErr != nil {
				fail(w, r, v.You, lookupErr)
				return
			}
			targetKind = record.Kind
			err = c.post(cookie(r), "/unregister", map[string]string{"name": name})
		default:
			localProblem(w, r, v.You, http.StatusBadRequest, "That service action is not available.")
			return
		}
		if err != nil {
			if code, message, preserve := formRefusal(err); preserve {
				field := ""
				switch {
				case action == "create" && code == http.StatusPreconditionFailed:
					field = "name"
				case action == "maintainers":
					field = "maintainers"
				}
				if renderServiceFormError(w, r, v, action, code, message, field) {
					return
				}
			}
			fail(w, r, v.You, err)
			return
		}
		// A mutation posted through the shared /service action returns to the
		// listing its record actually belongs to. The daemon's fresh answer,
		// rather than a hidden field, chooses the destination.
		if targetKind == "" && action != "create" && action != "delete" && action != "transfer" {
			var current protocol.Record
			if lookupErr := c.get(cookie(r), "/lookup?name="+url.QueryEscape(name), &current); lookupErr == nil {
				targetKind = current.Kind
			}
		}
		next := detailPathFor(targetKind) + "?name=" + url.QueryEscape(name)
		switch {
		case action == "delete":
			next = listPathFor(targetKind)
		case action == "transfer":
			var current protocol.Record
			// The redirect below must follow this exact daemon visibility answer;
			// do not replace it with cached or face-inferred authority.
			if lookupErr := c.get(cookie(r), "/lookup?name="+url.QueryEscape(name), &current); lookupErr != nil {
				next = listPathFor(targetKind)
			}
		}
		http.Redirect(w, r, next, http.StatusSeeOther)
	}))
	mux.HandleFunc("POST /groups", mutate(func(w http.ResponseWriter, r *http.Request, v adminView) {
		// "delete" was an action here and is not one now: a group is retired
		// by emptying its membership. It is named rather than falling into the
		// default so that an old bookmark is refused instead of silently
		// saving whatever members the form carried.
		if action := r.PostForm.Get("action"); action != "save" {
			localProblem(w, r, v.You, http.StatusBadRequest, "That group action is not available.")
			return
		}
		err := c.post(cookie(r), "/group", struct {
			Name    string
			Members []string
		}{r.PostForm.Get("name"), strings.Fields(r.PostForm.Get("members"))})
		if err != nil {
			if code, message, preserve := formRefusal(err); preserve {
				v.Form = retainedForm("save", message, r.PostForm, "name", "members", "new")
				if r.PostForm.Get("new") == "1" {
					v.Current = "groups"
					renderForm(w, code, groupNew, v)
					return
				}
				v.Form.Field = "members"
				v.GroupName = r.PostForm.Get("name")
				if groupsErr := c.get(cookie(r), "/groups", &v.Groups); groupsErr != nil {
					fail(w, r, v.You, groupsErr)
					return
				}
				if _, exists := v.Groups[v.GroupName]; !exists {
					fail(w, r, v.You, err)
					return
				}
				v.GroupMembers = strings.Fields(r.PostForm.Get("members"))
				v.CanEditGroup = v.Administrator && (v.DaemonOwner || v.GroupName != core.AdministratorsGroup)
				if !v.CanEditGroup {
					fail(w, r, v.You, err)
					return
				}
				renderForm(w, code, groupDetail, v)
				return
			}
			fail(w, r, v.You, err)
			return
		}
		http.Redirect(w, r, "/group?name="+url.QueryEscape(r.PostForm.Get("name")), http.StatusSeeOther)
	}))
}

var serviceList = template.Must(template.New("services").Funcs(template.FuncMap{"readerCount": readerCount, "registrationUpdated": registrationUpdated, "entityLabel": entityLabel, "copies": copies, "deliveryLabel": deliveryLabel, "titleMark": titleMark, "number": number, "href": detailPath}).Parse(shellTitle("records", `{{if .Channels}}Channels{{else if .Agents}}Agents{{else if .PersonalPage}}Personal agents{{else}}Services{{end}}`) + `
<div class=page-title><h1>{{if .Channels}}{{titleMark "channels"}} Channels{{else if .Agents}}{{titleMark "agent"}} Agents{{else if .PersonalPage}}{{titleMark "agent"}} Personal agents{{else}}{{titleMark "services"}} Services{{end}}</h1><button type=button class=help-button popovertarget=service-views-help aria-label="About service views">ⓘ</button></div>
<div popover id=service-views-help class=context-help><h2>About these records</h2><ul>
{{if .Channels}}<li>All counts the caller-visible queue and pub/sub records.</li><li>A channel carries messages without a service process of its own.</li>{{else if or .Agents .PersonalPage}}<li>An agent is a name on this bus with a queue something reads. All and My omit Personal agents; My is the caller-owned subset. Personal is an owner-set grouping tag and does not change access.</li>{{else}}<li>A service is external: it is reached at its own address, by its own protocol, and nothing on this bus answers for it.</li>{{end}}
<li>Category counts use the records visible to you before toolbar filters; they are not node-wide totals or page counts.</li>
{{if .Channels}}<li>A queue holds work for one reader; a pub/sub topic copies each accepted message to subscribers and holds no backlog of its own.</li>{{end}}
<li>Enabled is a stored delivery setting. It does not establish that a send will be accepted; the owner&rsquo;s access and ACL are checked separately.</li>
<li>Readers counts outstanding filtered and unfiltered reads. Zero may be between reads; a positive count proves neither a matching message nor completed work.</li>
<li>Accepted and Dequeued are cumulative across restarts. Dequeued means handed to a reader, not completed.</li>
<li>Reached external is a caller-supplied hint. None of it is health; the daemon does not observe whether a process is alive.</li>
</ul></div>
<nav class=section-nav aria-label="{{if .Channels}}Channel{{else if or .Agents .PersonalPage}}Agent{{else}}Service{{end}} views">{{range .SectionLinks}}{{if .Current}}<a href="{{.Href}}" aria-current=true class="{{.Class}}">{{else}}<a href="{{.Href}}" class="{{.Class}}">{{end}}{{.Label}}{{if .Counted}} ({{number .Count}}){{end}}</a>{{end}}</nav>
{{if .PersonalPage}}{{if .DaemonOwner}}<form method=get><label>Owner <select name=owner><option value="">All visible owners</option>{{range .Owners}}<option {{if eq . $.OwnerFilter}}selected{{end}}>{{.}}</option>{{end}}</select></label>{{with .State}}<input type=hidden name=state value="{{.}}">{{end}}{{with .Readers}}<input type=hidden name=readers value="{{.}}">{{end}}{{with .Work}}<input type=hidden name=work value="{{.}}">{{end}}{{with .Query}}<input type=hidden name=q value="{{.}}">{{end}}{{with .Kind}}<input type=hidden name=kind value="{{.}}">{{end}}{{with .Sort}}<input type=hidden name=sort value="{{.}}">{{end}} <button>Choose owner</button></form>{{else}}<p>Owned by <code>{{.You}}</code></p>{{end}}{{end}}
<div class=record-toolbar>
<form class=record-search method=get action="{{if .Channels}}/channels{{else if .Agents}}/agents{{else if .PersonalPage}}/personal{{else}}/services{{end}}">
<label for=record-query class=visually-hidden>Search records</label><input id=record-query type=search name=q value="{{.Query}}" placeholder="Search by name, owner, or description">
{{with .Mine}}<input type=hidden name=scope value="{{.}}">{{end}}{{with .State}}<input type=hidden name=state value="{{.}}">{{end}}{{with .Readers}}<input type=hidden name=readers value="{{.}}">{{end}}{{with .Work}}<input type=hidden name=work value="{{.}}">{{end}}{{with .Kind}}<input type=hidden name=kind value="{{.}}">{{end}}{{with .OwnerFilter}}<input type=hidden name=owner value="{{.}}">{{end}}
<div class=record-choices><nav class=filter-nav aria-label="Delivery filter"><span><span aria-hidden=true>🔛</span> Delivery</span>{{range .FilterLinks}}{{if .Current}}<a href="{{.Href}}" aria-current=true>{{else}}<a href="{{.Href}}">{{end}}{{.Label}}</a>{{end}}</nav><nav class=filter-nav aria-label="Reader filter"><span>Readers</span>{{range .ReaderLinks}}{{if .Current}}<a href="{{.Href}}" aria-current=true>{{else}}<a href="{{.Href}}">{{end}}{{.Label}}</a>{{end}}</nav><nav class=filter-nav aria-label="Queue filter"><span>Queue</span>{{range .WorkLinks}}{{if .Current}}<a href="{{.Href}}" aria-current=true>{{else}}<a href="{{.Href}}">{{end}}{{.Label}}</a>{{end}}</nav>{{with .KindLinks}}<nav class=filter-nav aria-label="Kind filter"><span>Kind</span>{{range .}}{{if .Current}}<a href="{{.Href}}" aria-current=true>{{else}}<a href="{{.Href}}">{{end}}{{.Label}}</a>{{end}}</nav>{{end}}</div>
<label for=record-sort>Sort</label><select id=record-sort name=sort data-submit-on-change><option value="" {{if eq .Sort ""}}selected{{end}}>Name (A&ndash;Z)</option><option value=updated {{if eq .Sort "updated"}}selected{{end}}>Recently updated</option><option value=queued {{if eq .Sort "queued"}}selected{{end}}>{{if .Channels}}Work{{else}}Queued{{end}} (high&ndash;low)</option></select><noscript><button>Apply</button></noscript>
</form>
</div>
{{if eq .CategoryTotal 0}}
<section class="empty-state editor-card"><h2>No {{if .Channels}}channels{{else if .Agents}}agents{{else if .PersonalPage}}Personal agents{{else}}services{{end}} yet</h2>
{{if .Channels}}<p>A channel routes messages without a separate service process. A queue hands each message to one reader; a pub/sub topic copies each message to its subscribers.</p><p><a href=/channels/new>Register a channel</a></p>
{{else if .Agents}}<p>An agent is a name on this bus with a queue something reads. It carries no address of its own: callers send to the name and the daemon delivers.</p><p><a href=/agents/new>Register an agent</a></p>
{{else if .PersonalPage}}<p>Personal is an owner-set grouping for agents; it does not change access.</p><p><a href=/agents/new>Register an agent</a></p>
{{else}}<p>A service is external: it is reached at its own address, by its own protocol, and nothing on this bus answers for it.</p><p><a href=/services/new>Register a service</a></p>{{end}}</section>
{{else}}
{{if .PersonalPage}}<p class=muted>{{if .DaemonOwner}}This per-owner view contains only Personal agents visible through your normal access; it is not a node-wide inventory.{{else}}Your Personal agents.{{end}}</p>{{end}}
{{if eq .Matched 0}}<section class="empty-state editor-card"><h2>No records match these filters</h2><p>Change the active filters above or <a href="{{.ClearFilters}}">clear filters</a>.</p></section>{{else}}
<table class=record-table><caption>Showing {{number .Start}}&ndash;{{number .End}} of {{number .Matched}} matching records, caller-visible on this page and not a count of this node.{{if .HasFilters}} <a href="{{.ClearFilters}}">Clear filters</a>{{end}}</caption>
{{if .Channels}}<thead><tr><th scope=col>Channel<th scope=col>Type<th scope=col>Delivery mode<th scope=col>Owner<th scope=col>Delivery<th scope=col class=num>Readers<th scope=col>Reached<th scope=col class=num>Held<th scope=col class=num>Accepted<th scope=col class=num>Dequeued<th scope=col class=num>Subscribers<th scope=col>Updated</tr></thead>
{{else}}<thead><tr><th scope=col>{{if or .Agents .PersonalPage}}Agent{{else}}Service{{end}}<th scope=col>Type<th scope=col>Owner<th scope=col>Delivery<th scope=col class=num>Readers<th scope=col>Reached<th scope=col class=num>Queued<th scope=col class=num>Accepted<th scope=col class=num>Dequeued<th scope=col>Updated</tr></thead>{{end}}
<tbody>
{{range .Records}}<tr><td class="record-name-cell{{if eq .Owner $.You}} owned-record{{end}}{{if .Personal}} personal-record{{end}}"><a class=record-name href="{{href .}}?name={{.Name}}&return={{$.Return}}">{{if .Descr}}<span class=record-description>{{.Descr}}</span><code>{{.Name}}</code>{{else}}<code class=record-description>{{.Name}}</code>{{end}}</a>{{if .Personal}} <span class=personal-marker>Personal</span>{{end}}
{{if $.Channels}}<td data-label=Type>{{entityLabel .Kind}}<td data-label="Delivery mode">{{deliveryLabel .}}<td data-label=Owner><code>{{.Owner}}</code><td data-label=Delivery>{{if .Disabled}}<span class=status-glyph role=img aria-label=Disabled title=Disabled>🚫</span>{{else}}<span class=status-glyph role=img aria-label=Enabled title=Enabled>🔛</span>{{end}}<td class=num data-label=Readers>{{readerCount .Readers}}<td data-label=Reached>{{if .Proto}}external{{else}}<span class=muted>&mdash;</span>{{end}}<td class=num data-label=Held>{{if copies .Kind}}<span class=muted>&mdash;</span>{{else}}{{number .Queued}}{{if .AtBound}} <span class=warn>at capacity when observed</span>{{end}}{{end}}<td class=num data-label=Accepted>{{number .In}}<td class=num data-label=Dequeued>{{number .Out}}<td class=num data-label=Subscribers>{{if copies .Kind}}{{number (len .Subs)}}{{else}}<span class=muted>&mdash;</span>{{end}}<td data-label=Updated>{{if .At.IsZero}}<span class=muted>&iquest;</span>{{else}}{{registrationUpdated .At}}{{end}}
{{else}}<td data-label=Type>{{entityLabel .Kind}}<td data-label=Owner><code>{{.Owner}}</code><td data-label=Delivery>{{if .Disabled}}<span class=status-glyph role=img aria-label=Disabled title=Disabled>🚫</span>{{else}}<span class=status-glyph role=img aria-label=Enabled title=Enabled>🔛</span>{{end}}<td class=num data-label=Readers>{{readerCount .Readers}}<td data-label=Reached>{{if .Proto}}external{{else}}<span class=muted>&mdash;</span>{{end}}<td class=num data-label=Queued>{{number .Queued}}{{if .AtBound}} <span class=warn>at capacity when observed</span>{{end}}<td class=num data-label=Accepted>{{number .In}}<td class=num data-label=Dequeued>{{number .Out}}<td data-label=Updated>{{if .At.IsZero}}<span class=muted>&iquest;</span>{{else}}{{registrationUpdated .At}}{{end}}{{end}}</tr>{{end}}
</tbody></table>
{{end}}
<nav aria-label="Record pages">{{with .Previous}}<a href="{{.}}">Previous page</a>{{end}} {{with .Next}}<a href="{{.}}">Next page</a>{{end}}</nav>
{{end}}
`))
var serviceNew = template.Must(template.New("service-new").Funcs(template.FuncMap{"entityLabel": entityLabel, "titleMark": titleMark}).Parse(shellTitle("records", `Register {{if .Agents}}agent{{else if .Channels}}channel{{else}}service{{end}}`) + `
<p><a href="{{if .Agents}}/agents{{else if .Channels}}/channels{{else}}/services{{end}}">Back to {{if .Agents}}Agents{{else if .Channels}}Channels{{else}}Services{{end}}</a></p>
<div class=page-title><h1>{{if .Agents}}{{titleMark "agent"}}{{else if .Channels}}{{titleMark "channels"}}{{else}}{{titleMark "services"}}{{end}} Register {{if .Agents}}agent{{else if .Channels}}channel{{else}}service{{end}}</h1></div>
` + formErrorSummary + `<form id=form-create class="editor-card task-card" method=post action=/service><input type=hidden name=action value=create><div class=form-grid>
<label class="form-field form-field-wide">Name <input id=create-name name=name required placeholder="name@realm" value="{{.Form.Value "name"}}" aria-invalid="{{if .Form.Invalid "name"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Invalid "name"}}create-error{{end}}"><small>The routing identity callers use.</small></label>
<label class="form-field form-field-wide">Description <input name=descr value="{{.Form.Value "descr"}}" placeholder="What this {{if .Agents}}agent{{else if .Channels}}channel{{else}}service{{end}} does"><small>Shown first in the registry.</small></label>
{{if .Channels}}<fieldset aria-invalid="{{if .Form.Invalid "kind"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Invalid "kind"}}create-error{{end}}" class=form-field-wide><legend>What to register</legend><div class=choice-row><label><input type=radio name=kind value=queue {{if or (not (.Form.Is "create")) (ne (.Form.Value "kind") "pubsub")}}checked{{end}}> {{entityLabel "queue"}}</label><label><input type=radio name=kind value=pubsub {{if eq (.Form.Value "kind") "pubsub"}}checked{{end}}> {{entityLabel "pubsub"}}</label></div><small>A queue holds work and hands each message to one reader. A pub/sub topic copies every accepted message to its subscribers and holds nothing.</small></fieldset>
{{else if .Agents}}<input type=hidden name=kind value=agent><fieldset><legend>Visibility</legend><div class=choice-row><label><input type=checkbox name=personal {{if .Form.Checked "personal"}}checked{{end}}> Personal</label></div><small>A Personal agent is grouped in its owner&rsquo;s view and may share with other agents only.</small></fieldset>
{{else}}<input type=hidden name=kind value=service>
<label class="form-field">Address <input name=addr required value="{{.Form.Value "addr"}}" placeholder="host:port, a path, or a URL" aria-invalid="{{if .Form.Invalid "addr"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Invalid "addr"}}create-error{{end}}"><small>Where the caller reaches it. Required: a service is not on this bus.</small></label>
<label class="form-field">Protocol <input name=protocol required value="{{.Form.Value "protocol"}}" placeholder="https, postgresql, smtp" aria-invalid="{{if .Form.Invalid "protocol"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Invalid "protocol"}}create-error{{end}}"><small>How the caller speaks to it. A hint in the registry; the daemon neither implements nor checks it.</small></label>{{end}}
<label class="form-field form-field-wide"><span class=field-heading>Allow list <button type=button class=help-button popovertarget=create-access-help aria-label="About initial access" data-tooltip="One identity per line. Empty is owner and Maintainers only; @owner adds the records the Owner directly owns; * shares with every admitted principal. Personal agents may list agents only.">ⓘ</button></span><textarea name=allow rows=5 placeholder="agent@realm&#10;@group&#10;@owner&#10;*">{{.Form.Value "allow"}}</textarea><small>One identity, group, <code>@owner</code>, or <code>*</code> per line.</small></label></div>
<div popover id=create-access-help class=context-help><h2>Initial access</h2><ul><li>Empty allows only the owner and assigned Maintainers.</li><li><code>@owner</code> adds records directly owned by this record&rsquo;s Owner. It is runtime ACL syntax, not an editable group.</li><li><code>*</code> shares with every admitted principal.</li>{{if .Agents}}<li>A Personal agent may name only other registered agents directly; users, groups, <code>@owner</code>, <code>*</code>, itself and Maintainers are refused.</li>{{end}}</ul></div>
{{if .Form.Is "create"}}<p class=warn id=create-error>{{.Form.Error}}</p>{{end}}<div class=form-actions><button>Register {{if .Agents}}agent{{else if .Channels}}channel{{else}}service{{end}}</button></div></form>`))
var serviceDetail = template.Must(template.New("service").Funcs(template.FuncMap{"join": strings.Join, "readerCount": readerCount, "entityLabel": entityLabel, "copies": copies, "deliveryMode": deliveryMode, "recordNoun": recordNoun, "titleMark": titleMark, "photoData": photoData, "profileInitial": profileInitial, "number": number}).Parse(shellTitle("records", `{{recordNoun .Record.Kind}} {{.Record.Name}}`) + `
<p><a href="{{.Return}}">Back to records</a></p>{{with .Record}}<div class=page-title><h1>{{titleMark .Kind}} {{.Name}}{{if .Personal}} <span class=muted>· Personal</span>{{end}}</h1></div>` + formErrorSummary + `<div class=detail-meta><span class=fact-pill>{{entityLabel .Kind}}</span><span>Owner: {{with $.OwnerUser}}<span class=identity-with-photo>{{with photoData .}}<img class=profile-photo src="{{.}}" alt="">{{else}}<span class=profile-initial aria-hidden=true>{{profileInitial .}}</span>{{end}}<a href="/user?name={{.Name}}"><code>{{.Name}}</code></a></span>{{else}}<code>{{.Owner}}</code>{{end}}</span>{{with .Maintainers}}<span>👮 Maintainers: {{join . ", "}}</span>{{end}}{{with deliveryMode .}}<span>Delivery: {{.}}</span>{{end}}</div>
<div class=service-dashboard><section class=fact-card><div class=page-title><h2>Delivery</h2><button type=button class=help-button popovertarget=delivery-help aria-label="About delivery state" data-tooltip="Stored setting only. The daemon also checks owner access, ACL and queue capacity; Disabled does not reveal why it is off.">ⓘ</button></div>
<p><strong><span class=status-glyph role=img aria-label="{{if .Disabled}}Disabled{{else}}Enabled{{end}}" title="{{if .Disabled}}Disabled{{else}}Enabled{{end}}">{{if .Disabled}}🚫{{else}}🔛{{end}}</span></strong> {{if .Disabled}}Disabled{{else}}Enabled{{end}}</p>{{if .CanManage}}<form class=record-state-action method=post action=/service><input type=hidden name=name value="{{.Name}}"><button name=action value="{{if .Disabled}}enable{{else}}disable{{end}}">{{if .Disabled}}Enable{{else}}Disable{{end}} delivery</button></form>{{end}}</section>
<div popover id=delivery-help class=context-help><h2>About delivery state</h2><ul><li>This stored setting does not establish that a send will be accepted.</li><li>The daemon also checks owner access, the allow list and queue capacity.</li><li>The disabled bit does not say whether the owner turned it off or the name stopped being active.</li></ul></div>
<section class=fact-card><div class=page-title><h2>Policy</h2><button type=button class=help-button popovertarget=policy-help aria-label="About record policy" data-tooltip="Queue values are this record's stored policy. External remains a caller hint, and unset values are described without guessing resolved defaults.">ⓘ</button></div>
<dl><dt>Reached<dd><strong>{{if .Proto}}external{{else}}this bus{{end}}</strong><dt>Queue bound<dd>{{if .Bound}}{{number .Bound}}{{else}}default{{end}}<dt>Retention<dd>{{if .TTL}}{{.TTL}}{{else}}none{{end}}<dt>When full<dd>{{if eq .Full "ring"}}drop the oldest{{else}}refuse{{end}}</dl></section>
<div popover id=policy-help class=context-help><h2>About record policy</h2><ul>{{if .Proto}}<li>External is a caller-supplied hint, not proof of anything.</li>{{end}}{{if not .Bound}}<li>An unset queue bound uses the daemon default; its resolved value is not readable here.</li>{{end}}{{if not .TTL}}<li>Unset retention means no queue-imposed expiry; a message may still specify its own.</li>{{end}}<li>The overflow policy always belongs to this record.</li></ul></div>
<section class=fact-card><div class=page-title><h2>Queue &amp; counters</h2><button type=button class=help-button popovertarget=observed-help aria-label="About live counters" data-tooltip="Readers are outstanding requests, not health. Queue reads may precede pruning. Counters survive restart; Dequeued is not completion.">ⓘ</button></div>
<dl><dt>Readers<dd><strong>{{readerCount .Readers}}</strong><dt>Held now<dd><strong>{{number .Queued}}</strong>{{if .AtBound}} <span class=warn>at capacity when observed</span>{{end}}<dt>Oldest held<dd>{{if .Oldest}}{{.Oldest}}{{else}}<span class=muted>&mdash;</span>{{end}}<dt>Accepted<dd>{{number .In}}<dt>Dequeued<dd>{{number .Out}}<dt>Dropped / expired<dd>{{number .Dropped}} / {{number .Expired}}</dl>
<p class=muted>Updated {{if .At.IsZero}}&iquest;{{else}}{{.At.Format "2006-01-02 15:04:05"}}{{end}} · Config {{if .ConfigSHA}}<code>{{.ConfigSHA}}</code>{{else}}&mdash;{{end}}</p></section>
<div popover id=observed-help class=context-help><h2>About observed counters</h2><ul><li>Readers counts outstanding filtered and unfiltered reads; zero is not an offline signal.</li><li>The read does not prune first, so Held and Oldest may include work already past its TTL.</li><li>Counters are cumulative across restarts. Dequeued is handed to a reader, which is not completed.</li></ul></div>
{{template "activity-view" $}}
<p class=activity-fact><a href="/activity?name={{.Name}}">View all activity and sample values</a></p></div>
{{if copies .Kind}}<details><summary>Subscriptions</summary>
{{range .Subs}}<p>{{.}}{{if $.Record.CanManage}}<form method=post action=/service><input type=hidden name=name value="{{$.Record.Name}}"><input type=hidden name=subscriber value="{{.}}"><button name=action value=remove-subscriber>Remove subscription</button></form>{{end}}</p>{{else}}<p>No subscribers</p>{{end}}
<form method=post action=/service><input type=hidden name=name value="{{.Name}}"><button name=action value=subscribe>Subscribe my inbox</button><button name=action value=unsubscribe>Unsubscribe my inbox</button></form></details>{{end}}
{{if .CanManage}}
<details class=editor-card {{if $.Form.Is "save"}}open{{end}}><summary id=settings>Edit settings</summary><form id=form-save method=post action=/service><input type=hidden name=name value="{{.Name}}"><input type=hidden name=action value=save><div class=form-grid>
<label class=form-field>Description <input name=descr value="{{if $.Form.Is "save"}}{{$.Form.Value "descr"}}{{else}}{{.Descr}}{{end}}"></label>
<label class=form-field>Address <input name=addr value="{{if $.Form.Is "save"}}{{$.Form.Value "addr"}}{{else}}{{.Addr}}{{end}}"></label>
<label class=form-field>Protocol <input name=protocol value="{{if $.Form.Is "save"}}{{$.Form.Value "protocol"}}{{else}}{{.Proto}}{{end}}"></label>
<label class=form-field>Queue TTL <input name=ttl value="{{if $.Form.Is "save"}}{{$.Form.Value "ttl"}}{{else}}{{.TTL}}{{end}}" placeholder="default"></label>
<label class=form-field>Queue capacity <input type=number min=0 name=bound value="{{if $.Form.Is "save"}}{{$.Form.Value "bound"}}{{else}}{{.Bound}}{{end}}" aria-invalid="{{if $.Form.Invalid "bound"}}true{{else}}false{{end}}" aria-describedby="{{if $.Form.Invalid "bound"}}save-error{{end}}"><small>Enter 0 to select the default.</small></label>
<label class=form-field>Overflow <select name=overflow><option value=strict {{if and ($.Form.Is "save") (eq ($.Form.Value "overflow") "strict")}}selected{{end}}>Refuse</option><option value=ring {{if $.Form.Is "save"}}{{if eq ($.Form.Value "overflow") "ring"}}selected{{end}}{{else}}{{if eq .Full "ring"}}selected{{end}}{{end}}>Drop oldest</option></select></label>
{{if .Personal}}<p class="muted form-field-wide">Personal classification, Allow and Maintainers are changed together below.</p>{{else}}<input type=hidden name=edit_allow value=1><label class="form-field form-field-wide"><span class=field-heading>Allow list <button type=button class=help-button popovertarget=settings-access-help aria-label="About the allow list" data-tooltip="One identity per line. Empty is owner and Maintainers only; @owner adds the records the Owner directly owns; * shares with every admitted principal.">ⓘ</button></span><textarea name=allow rows=5>{{if $.Form.Is "save"}}{{$.Form.Value "allow"}}{{else}}{{join .Allow "\n"}}{{end}}</textarea><small>One identity, group, <code>@owner</code>, or <code>*</code> per line.</small></label>{{end}}</div>
{{if $.Form.Is "save"}}<p class=warn id=save-error>{{$.Form.Error}}</p>{{end}}<div class=form-actions><button>Save settings</button></div></form></details>
{{if not .Personal}}<div popover id=settings-access-help class=context-help><h2>Allow list</h2><ul><li>Empty allows only the owner and assigned Maintainers.</li><li><code>@owner</code> adds records directly owned by this record&rsquo;s Owner. It is runtime ACL syntax, not an editable group.</li><li><code>*</code> shares with every admitted principal.</li></ul></div>{{end}}
{{if .CanTransfer}}{{if eq .Kind "agent"}}<details class=editor-card {{if $.Form.Is "personal"}}open{{end}}><summary>Classification and sharing</summary><form id=form-personal method=post action=/service><input type=hidden name=name value="{{.Name}}"><input type=hidden name=action value=personal><div class=form-grid><fieldset class=form-field-wide><legend>Classification</legend><div class=choice-row><label><input type=checkbox name=personal {{if $.Form.Is "personal"}}{{if $.Form.Checked "personal"}}checked{{end}}{{else}}{{if .Personal}}checked{{end}}{{end}}> Personal</label><span class=muted>Groups this service in the owner&rsquo;s Personal view; access is unchanged.</span></div></fieldset><label class=form-field><span class=field-heading>Allow list <button type=button class=help-button popovertarget=personal-help aria-label="About Personal sharing" data-tooltip="Personal services may allow only other registered services directly; users, groups, @owner, wildcard, self and Maintainers are refused.">ⓘ</button></span><textarea name=allow rows=5>{{if $.Form.Is "personal"}}{{$.Form.Value "allow"}}{{else}}{{join .Allow "\n"}}{{end}}</textarea><small>One service identity per line when Personal.</small></label><label class=form-field>Maintainers<textarea name=maintainers rows=5 aria-invalid="{{if $.Form.Is "personal"}}true{{else}}false{{end}}" aria-describedby="{{if $.Form.Is "personal"}}personal-error{{end}}">{{if $.Form.Is "personal"}}{{$.Form.Value "maintainers"}}{{else}}{{join .Maintainers "\n"}}{{end}}</textarea><small>One user, group, agent or service per line.</small></label></div>{{if $.Form.Is "personal"}}<p class=warn id=personal-error>{{$.Form.Error}}</p>{{end}}<div class=form-actions><button>Save classification and sharing</button></div></form></details><div popover id=personal-help class=context-help><h2>Personal sharing</h2><p>When Personal is checked, Allow may name only other registered services directly. Users, groups, <code>@owner</code>, <code>*</code>, this service and Maintainers are refused. Clear Personal in this same form before adding any of them.</p></div>{{else}}<details class=editor-card {{if $.Form.Is "maintainers"}}open{{end}}><summary>Maintainers</summary><form id=form-maintainers method=post action=/service><input type=hidden name=name value="{{.Name}}"><input type=hidden name=action value=maintainers><label class=form-field>Maintainers<textarea name=maintainers rows=5 aria-invalid="{{if $.Form.Is "maintainers"}}true{{else}}false{{end}}" aria-describedby="{{if $.Form.Is "maintainers"}}maintainers-error{{end}}">{{if $.Form.Is "maintainers"}}{{$.Form.Value "maintainers"}}{{else}}{{join .Maintainers "\n"}}{{end}}</textarea><small>One user, group, agent or service per line; <code>@owner</code> is ACL-only.</small></label>{{if $.Form.Is "maintainers"}}<p class=warn id=maintainers-error>{{$.Form.Error}}</p>{{end}}<div class=form-actions><button>Assign maintainers</button></div></form></details>{{end}}
{{end}}
<p><a class=danger href="/service-danger?name={{.Name}}">Danger Zone</a></p>
{{else}}<p>{{.Descr}}</p><p>You can view this record; its owner and assigned maintainers can manage it.</p>{{end}}{{end}}` + activityViewTemplate))
var serviceDanger = template.Must(template.New("service-danger").Funcs(template.FuncMap{"readerCount": readerCount, "href": detailPath, "titleMark": titleMark}).Parse(shellTitle("records", `Danger Zone · {{.Record.Name}}`) + `
{{with .Record}}<p><a href="{{href .}}?name={{.Name}}">Back to {{.Name}}</a></p>
<div class=page-title><h1>{{titleMark "problem"}} Danger Zone · {{.Name}}</h1></div>` + formErrorSummary + `
<h2>Replace configuration</h2><form id=form-configure method=post action=/service><input type=hidden name=name value="{{.Name}}"><input type=hidden name=action value=configure><label>New configuration <textarea name=config rows=6 cols=60 required autocomplete=off aria-invalid="{{if $.Form.Invalid "config"}}true{{else}}false{{end}}" aria-describedby="{{if $.Form.Invalid "config"}}configure-error{{end}}"></textarea></label><p>Existing private configuration and a refused replacement are never displayed.</p>{{if $.Form.Is "configure"}}<p class=warn id=configure-error>{{$.Form.Error}}</p>{{end}}<button>Replace configuration</button></form>
{{if .CanTransfer}}{{if ne .Name .Owner}}<h2>Transfer ownership</h2><form id=form-transfer method=post action=/service-confirm><input type=hidden name=name value="{{.Name}}"><input type=hidden name=action value=transfer><label>New owner <input name=owner required value="{{if $.Form.Is "transfer"}}{{$.Form.Value "owner"}}{{end}}" aria-invalid="{{if $.Form.Invalid "owner"}}true{{else}}false{{end}}" aria-describedby="{{if $.Form.Invalid "owner"}}transfer-error{{end}}"></label>{{if $.Form.Is "transfer"}}<p class=warn id=transfer-error>{{$.Form.Error}}</p>{{end}}<p>The new owner must have a registered identity. Existing service credentials remain valid; transfer does not revoke copies already held.</p><button>Continue to confirmation</button></form>{{end}}{{end}}
<h2>Remove registration</h2><form method=post action=/service-confirm><input type=hidden name=name value="{{.Name}}"><input type=hidden name=action value=delete><p><strong>No registration, no access:</strong> the credential goes with the address. Drain the queue and stop readers first; the confirmation page re-reads both before describing the consequence.</p><button>Continue to confirmation</button></form>{{end}}`))
var serviceConfirm = template.Must(template.New("service-confirm").Funcs(template.FuncMap{"readerSnapshot": readerSnapshot, "readerCount": readerCount, "titleMark": titleMark}).Parse(shellTitle("records", `{{if eq .Action "transfer"}}Confirm ownership transfer{{else}}Confirm removal{{end}} · {{.Record.Name}}`) + `
{{with .Record}}<p><a href="/service-danger?name={{.Name}}">Back to the Danger Zone</a></p>
{{if eq $.Action "transfer"}}<div class=page-title><h1>{{titleMark "problem"}} Confirm ownership transfer</h1></div><p>Transfer <code>{{.Name}}</code> from <code>{{.Owner}}</code> to <code>{{$.NewOwner}}</code>?</p><p>The new owner must still be registered and active when the daemon applies this. Credentials already held are not revoked.</p><form method=post action=/service><input type=hidden name=action value=transfer><input type=hidden name=name value="{{.Name}}"><input type=hidden name=owner value="{{$.NewOwner}}"><input type=hidden name=expected_owner value="{{.Owner}}"><input type=hidden name=confirmed value=1><button>Transfer ownership</button></form>
{{else}}<div class=page-title><h1>{{titleMark "problem"}} Confirm removal</h1></div><p>Remove <code>{{.Name}}</code>? It currently holds <strong>{{.Queued}}</strong> messages and has <strong>{{readerCount .Readers}}</strong> outstanding reads.</p><p>The address and its credential go with it; nothing answers to this name afterwards. A person&rsquo;s own credential stays because it is not this record&rsquo;s to remove.</p><form method=post action=/service><input type=hidden name=action value=delete><input type=hidden name=name value="{{.Name}}"><input type=hidden name=expected_owner value="{{.Owner}}"><input type=hidden name=expected_queued value="{{.Queued}}"><input type=hidden name=expected_readers value="{{readerSnapshot .Readers}}"><input type=hidden name=confirmed value=1><button>Remove registration</button></form>{{end}}{{end}}`))
var groupList = template.Must(template.New("groups").Funcs(template.FuncMap{"groupGlyph": groupGlyph, "titleMark": titleMark, "number": number}).Parse(shell("groups", "Groups") + `
<div class=page-title><h1>{{titleMark "groups"}} Groups</h1><button type=button class=help-button popovertarget=groups-help aria-label="About group membership" data-tooltip="Groups are reusable authority lists. Select a group to inspect its members and edit them when your daemon authority permits.">ⓘ</button></div><div popover id=groups-help class=context-help><h2>Group membership</h2><ul><li>Select a group to inspect or edit its membership.</li><li><code>@owner</code> is runtime ACL syntax and cannot be created or nested as a group.</li><li>Administrators may edit ordinary groups; only the daemon owner changes <code>@administrators</code>.</li></ul></div><nav class=section-nav aria-label="Group views">{{range .SectionLinks}}{{if .Current}}<a href="{{.Href}}" aria-current=true>{{else}}<a href="{{.Href}}">{{end}}{{.Label}}{{if .Counted}} ({{number .Count}}){{end}}</a>{{end}}</nav>
<table class="record-table group-table"><caption>{{number (len .Groups)}} groups</caption><thead><tr><th scope=col>Group</th><th scope=col>Members</th></tr></thead><tbody>{{range $name,$members := .Groups}}<tr><td data-label=Group><span role=img aria-label="Group">{{groupGlyph}}</span> <a href="/group?name={{urlquery $name}}"><code>{{$name}}</code></a>{{if eq $name "@administrators"}} <span class=fact-pill>protected</span>{{end}}</td><td data-label=Members>{{if not $.Administrator}}<span class=muted>Not visible to you</span>{{else}}{{range $i,$member := $members}}{{if $i}}, {{end}}<code>{{$member}}</code>{{else}}<span class=muted>No members</span>{{end}}{{end}}</td></tr>{{else}}<tr><td colspan=2>No groups registered.</td></tr>{{end}}</tbody></table>`))
var groupDetail = template.Must(template.New("group-detail").Funcs(template.FuncMap{"join": strings.Join, "groupGlyph": groupGlyph, "titleMark": titleMark, "recordKindPath": recordKindPath, "entityLabel": entityLabel}).Parse(shellTitle("groups", `Group {{.GroupName}}`) + `
<p><a href=/groups>Back to Groups</a></p><div class=page-title><h1><span role=img aria-label="Group">{{groupGlyph}}</span> <code>{{.GroupName}}</code></h1>{{if eq .GroupName "@administrators"}}<span class=fact-pill>protected</span><button type=button class=help-button popovertarget=administrators-help aria-label="About the Administrators group" data-tooltip="Members administer users and ordinary groups. They do not automatically manage every Service or Channel; assign this group as a resource Maintainer when that is wanted. Only the daemon Owner changes membership.">ⓘ</button>{{end}}</div>{{if eq .GroupName "@administrators"}}<p class=muted>Daemon administration group. What membership grants is stated below; every other group on this node confers only what a resource assigns it.</p><div popover id=administrators-help class=context-help><h2>How this group behaves</h2><ul><li>It accepts direct user identities only. A snapshot that nests a group inside it is refused at startup.</li><li>It cannot be emptied, and the daemon Owner is always a member.</li><li>Adding an Administrator creates a user profile; removing the membership keeps that profile.</li><li>An ordinary group may name <code>@administrators</code>. Its direct members then receive that ordinary group&rsquo;s access or Maintainer grant, and no Administrator authority.</li></ul></div>
<section class="dashboard-section admin-rights"><h2>What membership grants</h2><ul><li><strong>Ordinary users.</strong> See the whole user directory, register new users, and edit, pause, ban or reactivate users below your own level.</li><li><strong>Ordinary groups.</strong> Change the membership of any ordinary group, including one assigned as a resource&rsquo;s Maintainer &mdash; you may add yourself, or a user you created, without asking that resource&rsquo;s owner.</li><li><strong>Membership lists.</strong> Read the full membership of every group.</li></ul>
<h2>What it does not grant</h2><ul><li><strong>The Owner, or each other.</strong> An Administrator cannot edit the daemon Owner or another Administrator, and cannot grant either position.</li><li><strong>Services and Channels.</strong> Administering the node is not managing its resources. That comes only from being named in a resource&rsquo;s Maintainers list; put <code>@administrators</code> there when every Administrator should manage it.</li><li><strong>This group.</strong> Only the daemon Owner changes who is in it.</li></ul></section>{{end}}` + formErrorSummary + `
{{if .CanEditGroup}}<section class="editor-card group-card"><form id=form-group method=post action=/groups><input type=hidden name=name value="{{.GroupName}}"><label class=form-field>Members<textarea name=members rows=8 placeholder="user@realm&#10;@nested-group" aria-invalid="{{if .Form.Invalid "members"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Invalid "members"}}group-error{{end}}">{{if .Form.Is "save"}}{{.Form.Value "members"}}{{else}}{{join .GroupMembers "\n"}}{{end}}</textarea><small>One identity or group per line; <code>@owner</code> is reserved for ACLs.</small></label>{{if .Form.Is "save"}}<p class=warn id=group-error>{{.Form.Error}}</p>{{end}}<div class=form-actions><button name=action value=save>Save members</button></div></form></section>{{else}}<section class="editor-card compact-card"><h2>Members</h2><div class=member-list>{{if not .Administrator}}<span class=muted>Membership is not visible to you.</span>{{else}}{{range .GroupMembers}}<code class=member-line>{{.}}</code>{{else}}<span class=muted>No members</span>{{end}}{{end}}</div>{{if eq .GroupName "@administrators"}}<p class=muted>Only the daemon owner changes this protected group.</p>{{else}}<p class=muted>Daemon administrators manage this group.</p>{{end}}</section>{{end}}
<section class=dashboard-section><h2>Used by visible records</h2>{{if .GroupReferences}}<table><thead><tr><th scope=col>Record</th><th scope=col>Kind</th><th scope=col>Uses this group</th></tr></thead><tbody>{{range .GroupReferences}}<tr><td><a href="{{recordKindPath .Name .Kind}}"><code>{{.Name}}</code></a></td><td>{{entityLabel .Kind}}</td><td>{{range $i,$use := .Uses}}{{if $i}} · {{end}}{{$use}}{{end}}</td></tr>{{end}}</tbody></table>{{else}}<p class=muted>No caller-visible record refers to this group.</p>{{end}}</section>`))
var groupNew = template.Must(template.New("group-new").Funcs(template.FuncMap{"titleMark": titleMark}).Parse(shell("groups", "Register group") + `
<p><a href=/groups>Back to Groups</a></p><div class=page-title><h1>{{titleMark "groups"}} Register group</h1></div>
` + formErrorSummary + `<form id=form-save class="editor-card task-card" method=post action=/groups><input type=hidden name=action value=save><input type=hidden name=new value=1><div class=form-grid>
<label class="form-field form-field-wide">Name <input name=name placeholder="@operators" required value="{{.Form.Value "name"}}" aria-invalid="{{if .Form.Is "save"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Is "save"}}group-new-error{{end}}">{{if eq (.Form.Value "name") "@owner"}}<small><code>@owner</code> is runtime ACL syntax and cannot be registered as a group.</small>{{end}}</label>
<label class="form-field form-field-wide">Members <textarea name=members rows=6 placeholder="user@realm&#10;@nested-group">{{.Form.Value "members"}}</textarea><small>One identity or group per line.</small></label></div>
{{if .Form.Is "save"}}<p class=warn id=group-new-error>{{.Form.Error}}</p>{{end}}<div class=form-actions><button>Register group</button></div></form>`))

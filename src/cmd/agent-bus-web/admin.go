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

var problemPage = template.Must(template.New("problem").Funcs(template.FuncMap{"titleMark": titleMark}).Parse(shell("", "Problem") + `<div class=page-title><h1>{{titleMark "problem"}} {{.Title}}</h1></div>
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
	OwnerUser     *protocol.User
	Groups        map[string][]string
	Mine, State   string
	Query, Kind   string
	Sort          string
	OwnerFilter   string
	Current       string
	Owners        []string
	Channels      bool
	PersonalPage  bool
	Status        core.Status
	Activity      activityPresentation
	SectionLinks  []viewLink
	FilterLinks   []viewLink
	KindLinks     []viewLink
	Form          formState
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

func formRefusal(err error) (int, string, bool) {
	var bad *busError
	if !errors.As(err, &bad) {
		return 0, "", false
	}
	message := strings.TrimSpace(bad.message)
	var payload struct {
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(message), &payload) == nil && payload.Error != "" {
		message = payload.Error
	}
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
		case v.Record.Kind == protocol.KindTopic:
			v.Current = "channels"
		case v.Record.Personal:
			v.Current = "personal"
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
			v.Activity.Unavailable = err.Error()
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
		v.Kind, v.Sort = r.URL.Query().Get("kind"), r.URL.Query().Get("sort")
		if v.Mine != "my" {
			v.Mine = ""
		}
		if v.State != "active" && v.State != "inactive" {
			v.State = ""
		}
		if v.Kind != "agent" && v.Kind != "service" {
			v.Kind = ""
		}
		if v.Sort != "updated" && v.Sort != "queued" {
			v.Sort = ""
		}
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
		allServices, myServices, personalServices, allChannels := 0, 0, 0, 0
		for _, record := range records {
			switch {
			case record.Kind == protocol.KindTopic:
				allChannels++
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
			if !v.Channels {
				if v.Kind == "agent" && record.Kind != "agent" || v.Kind == "service" && record.Kind != "generic" {
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
		if v.Kind != "" && !v.Channels {
			stateQuery.Set("kind", v.Kind)
		}
		if v.Sort != "" {
			stateQuery.Set("sort", v.Sort)
		}
		if v.Channels {
			v.SectionLinks = []viewLink{
				{Href: pageURL("/channels", stateQuery), Label: "All channels", Count: allChannels, Counted: true, Current: true},
				{Href: "/channels/new", Label: "Register channel"},
			}
		} else {
			allQuery, myQuery, personalQuery := cloneValues(stateQuery), cloneValues(stateQuery), cloneValues(stateQuery)
			myQuery.Set("scope", "my")
			if v.PersonalPage && v.DaemonOwner && v.OwnerFilter != "" {
				personalQuery.Set("owner", v.OwnerFilter)
			}
			v.SectionLinks = []viewLink{
				{Href: pageURL("/services", allQuery), Label: "All", Count: allServices, Counted: true, Current: !v.PersonalPage && v.Mine == ""},
				{Href: pageURL("/services", myQuery), Label: "My", Class: "my-view", Count: myServices, Counted: true, Current: !v.PersonalPage && v.Mine == "my"},
				{Href: pageURL("/personal", personalQuery), Label: "Personal", Class: "personal-view", Count: personalServices, Counted: true, Current: v.PersonalPage},
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
		if v.Kind != "" && !v.Channels {
			filterBase.Set("kind", v.Kind)
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
		if !v.Channels {
			kindBase := cloneValues(filterBase)
			kindBase.Del("kind")
			if v.State != "" {
				kindBase.Set("state", v.State)
			}
			v.KindLinks = []viewLink{
				{Href: pageURL(filterPath, cloneValues(kindBase)), Label: "All", Current: v.Kind == ""},
				{Href: queryWith(filterPath, kindBase, "kind", "agent"), Label: entityLabel("agent"), Current: v.Kind == "agent"},
				{Href: queryWith(filterPath, kindBase, "kind", "service"), Label: entityLabel("generic"), Current: v.Kind == "service"},
			}
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
				if a.Queued != b.Queued {
					return a.Queued > b.Queued
				}
			}
			return a.Name < b.Name
		})
		render(w, serviceList, v)
	}
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
	mux.HandleFunc("GET /channels/new", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		v.Current, v.Channels = "channels", true
		render(w, serviceNew, v)
	})
	mux.HandleFunc("GET /service", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		if !loadServiceDetails(w, r, &v, r.URL.Query().Get("name")) {
			return
		}
		render(w, serviceDetail, v)
	})
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
			v.Channels = r.PostForm.Get("kind") == protocol.KindTopic
			if v.Channels {
				v.Current = "channels"
			} else {
				v.Current = "services"
			}
			v.Form = setField(retainedForm(action, message, r.PostForm, "name", "descr", "kind", "mode", "personal", "allow"))
			renderForm(w, code, serviceNew, v)
			return true
		case "save":
			if !loadServiceDetails(w, r, &v, r.PostForm.Get("name")) {
				return true
			}
			v.Form = setField(retainedForm(action, message, r.PostForm, "descr", "addr", "protocol", "ttl", "overflow", "bound", "no_master", "allow", "edit_allow"))
			renderForm(w, code, serviceDetail, v)
			return true
		case "maintainers":
			if !loadServiceDetails(w, r, &v, r.PostForm.Get("name")) {
				return true
			}
			if v.Record.Kind == "generic" && v.Record.CanTransfer {
				// The current Generic-service editor changes classification,
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
		targetChannel := false
		confirmed := r.PostForm.Get("confirmed") == "1"
		if confirmed && (action == "transfer" || action == "delete") {
			if !loadRecord(w, r, &v, name) {
				return
			}
			targetChannel = v.Record.Kind == protocol.KindTopic
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
			targetChannel = record.Kind == protocol.KindTopic
		}
		switch action {
		case "save":
			descr, addr, proto, ttl, overflow := r.PostForm.Get("descr"), r.PostForm.Get("addr"), r.PostForm.Get("protocol"), r.PostForm.Get("ttl"), r.PostForm.Get("overflow")
			bound, parseErr := strconv.Atoi(r.PostForm.Get("bound"))
			if parseErr != nil {
				renderServiceFormError(w, r, v, action, http.StatusBadRequest, "Queue capacity must be a whole number.", "bound")
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
			kind, mode := r.PostForm.Get("kind"), r.PostForm.Get("mode")
			if kind != "generic" && kind != "agent" && kind != "topic" {
				renderServiceFormError(w, r, v, action, http.StatusBadRequest, "Choose a valid record kind.", "kind")
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
			var record protocol.Record
			if lookupErr := c.get(cookie(r), "/lookup?name="+url.QueryEscape(name), &record); lookupErr != nil {
				fail(w, r, v.You, lookupErr)
				return
			}
			targetChannel = record.Kind == protocol.KindTopic
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
		next := "/service?name=" + url.QueryEscape(name)
		switch {
		case action == "delete" && targetChannel:
			next = "/channels"
		case action == "delete":
			next = "/services"
		case action == "transfer":
			var current protocol.Record
			// The redirect below must follow this exact daemon visibility answer;
			// do not replace it with cached or face-inferred authority.
			if lookupErr := c.get(cookie(r), "/lookup?name="+url.QueryEscape(name), &current); lookupErr != nil {
				if targetChannel {
					next = "/channels"
				} else {
					next = "/services"
				}
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
				if groupsErr := c.get(cookie(r), "/groups", &v.Groups); groupsErr != nil {
					fail(w, r, v.You, groupsErr)
					return
				}
				v.SectionLinks = []viewLink{{Href: "/groups", Label: "All groups", Count: len(v.Groups), Counted: true, Current: true}}
				if v.Administrator {
					v.SectionLinks = append(v.SectionLinks, viewLink{Href: "/groups/new", Label: "Register group"})
				}
				v.Form.Target = "group-" + r.PostForm.Get("name")
				renderForm(w, code, groupList, v)
				return
			}
			fail(w, r, v.You, err)
			return
		}
		http.Redirect(w, r, "/groups", http.StatusSeeOther)
	}))
}

var serviceList = template.Must(template.New("services").Funcs(template.FuncMap{"readerCount": readerCount, "registrationUpdated": registrationUpdated, "entityLabel": entityLabel, "titleMark": titleMark}).Parse(shell("records", "Services") + `
<div class=page-title><h1>{{if .Channels}}{{titleMark "channels"}} Channels{{else}}{{titleMark "services"}} {{if .PersonalPage}}Personal services{{else}}Services{{end}}{{end}}</h1><button type=button class=help-button popovertarget=service-views-help aria-label="About service views">ⓘ</button></div>
<div popover id=service-views-help class=context-help><h2>About these records</h2><ul>
{{if .Channels}}<li>All channels counts the caller-visible Channel records.</li>{{else}}<li>All and My omit Personal services; My is the caller-owned subset. Personal is an owner-set grouping tag and does not change access.</li>{{end}}
<li>The count uses the records visible to you before the Delivery filter; it is not a node-wide total.</li>
<li>Enabled is a stored delivery setting. It does not establish that a send will be accepted; the owner&rsquo;s access and ACL are checked separately.</li>
<li>Readers counts outstanding filtered and unfiltered reads. Zero may be between reads; a positive count proves neither a matching message nor completed work.</li>
<li>Reached external is a caller-supplied hint. None of it is health; the daemon does not observe whether a process is alive.</li>
</ul></div>
<nav class=section-nav aria-label="{{if .Channels}}Channel{{else}}Service{{end}} views">{{range .SectionLinks}}{{if .Current}}<a href="{{.Href}}" aria-current=true class="{{.Class}}">{{else}}<a href="{{.Href}}" class="{{.Class}}">{{end}}{{.Label}}{{if .Counted}} ({{.Count}}){{end}}</a>{{end}}</nav>
{{if .PersonalPage}}{{if .DaemonOwner}}<form method=get><label>Owner <select name=owner><option value="">All visible owners</option>{{range .Owners}}<option {{if eq . $.OwnerFilter}}selected{{end}}>{{.}}</option>{{end}}</select></label>{{with .State}}<input type=hidden name=state value="{{.}}">{{end}} <button>Choose owner</button></form>{{else}}<p>Owned by <code>{{.You}}</code></p>{{end}}{{end}}
<div class=record-toolbar>
<form class=record-search method=get action="{{if .Channels}}/channels{{else if .PersonalPage}}/personal{{else}}/services{{end}}">
<label for=record-query class=visually-hidden>Search records</label><input id=record-query type=search name=q value="{{.Query}}" placeholder="Search by name, owner, or description">
{{with .Mine}}<input type=hidden name=scope value="{{.}}">{{end}}{{with .State}}<input type=hidden name=state value="{{.}}">{{end}}{{with .Kind}}<input type=hidden name=kind value="{{.}}">{{end}}{{with .OwnerFilter}}<input type=hidden name=owner value="{{.}}">{{end}}
<div class=record-choices><nav class=filter-nav aria-label="Delivery filter"><span><span aria-hidden=true>🔛</span> Delivery</span>{{range .FilterLinks}}{{if .Current}}<a href="{{.Href}}" aria-current=true>{{else}}<a href="{{.Href}}">{{end}}{{.Label}}</a>{{end}}</nav>{{with .KindLinks}}<nav class=filter-nav aria-label="Kind filter"><span>Kind</span>{{range .}}{{if .Current}}<a href="{{.Href}}" aria-current=true>{{else}}<a href="{{.Href}}">{{end}}{{.Label}}</a>{{end}}</nav>{{end}}</div>
<label for=record-sort>Sort</label><select id=record-sort name=sort data-submit-on-change><option value="" {{if eq .Sort ""}}selected{{end}}>Name (A&ndash;Z)</option><option value=updated {{if eq .Sort "updated"}}selected{{end}}>Recently updated</option><option value=queued {{if eq .Sort "queued"}}selected{{end}}>Queued (high&ndash;low)</option></select><noscript><button>Apply</button></noscript>
</form>
</div>
{{if .PersonalPage}}<p class=muted>{{if .DaemonOwner}}This per-owner view contains only Personal services visible through your normal access; it is not a node-wide inventory.{{else}}Your Personal services.{{end}}</p>{{end}}
<table class=record-table><caption>{{len .Records}} records shown &mdash; visible to you, not a count of this node</caption>
<thead><tr><th scope=col>Service<th scope=col>Type<th scope=col>Owner<th scope=col>Delivery<th scope=col class=num>Readers<th scope=col>Reached<th scope=col class=num>Queued<th scope=col>Updated</tr></thead>
<tbody>
{{range .Records}}<tr><td class="record-name-cell{{if eq .Owner $.You}} owned-record{{end}}{{if .Personal}} personal-record{{end}}"><a class=record-name href="/service?name={{.Name}}">{{if .Descr}}<span class=record-description>{{.Descr}}</span><code>{{.Name}}</code>{{else}}<code class=record-description>{{.Name}}</code>{{end}}</a>{{if .Personal}} <span class=personal-marker>Personal</span>{{end}}<td data-label=Type>{{entityLabel .Kind}}<td data-label=Owner><code>{{.Owner}}</code><td data-label=Delivery>{{if .Disabled}}<span class=status-glyph role=img aria-label=Disabled title=Disabled>🚫</span>{{else}}<span class=status-glyph role=img aria-label=Enabled title=Enabled>🔛</span>{{end}}<td class=num data-label=Readers>{{readerCount .Readers}}<td data-label=Reached>{{if .Proto}}external{{else}}<span class=muted>&mdash;</span>{{end}}<td class=num data-label=Queued>{{.Queued}}{{if .AtBound}} <span class=warn>at capacity when observed</span>{{end}}<td data-label=Updated>{{if .At.IsZero}}<span class=muted>&iquest;</span>{{else}}{{registrationUpdated .At}}{{end}}</tr>{{else}}<tr><td colspan=8>No matching records</tr>{{end}}
</tbody></table>
`))
var serviceNew = template.Must(template.New("service-new").Funcs(template.FuncMap{"entityLabel": entityLabel, "titleMark": titleMark}).Parse(shell("records", "Register") + `
<p><a href="{{if .Channels}}/channels{{else}}/services{{end}}">Back to {{if .Channels}}Channels{{else}}Services{{end}}</a></p>
<div class=page-title><h1>{{if .Channels}}{{titleMark "channels"}}{{else}}{{titleMark "services"}}{{end}} Register {{if .Channels}}channel{{else}}service{{end}}</h1></div>
` + formErrorSummary + `<form id=form-create method=post action=/service><input type=hidden name=action value=create>
<label>Name <input id=create-name name=name required placeholder="name@realm" value="{{.Form.Value "name"}}" aria-invalid="{{if .Form.Invalid "name"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Invalid "name"}}create-error{{end}}"></label><p><label>Description <input name=descr value="{{.Form.Value "descr"}}"></label></p>
{{if .Channels}}<input type=hidden name=kind value=topic><fieldset><legend>Delivery</legend><label><input type=radio name=mode value=pubsub {{if or (not (.Form.Is "create")) (eq (.Form.Value "mode") "pubsub")}}checked{{end}}> Pub/sub</label> <label><input type=radio name=mode value=queue {{if eq (.Form.Value "mode") "queue"}}checked{{end}}> Queue</label></fieldset>{{else}}<fieldset aria-invalid="{{if .Form.Invalid "kind"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Invalid "kind"}}create-error{{end}}"><legend>Kind</legend><label><input type=radio name=kind value=generic {{if or (not (.Form.Is "create")) (eq (.Form.Value "kind") "generic")}}checked{{end}}> {{entityLabel "generic"}}</label> <label><input type=radio name=kind value=agent {{if eq (.Form.Value "kind") "agent"}}checked{{end}}> {{entityLabel "agent"}}</label></fieldset><p><label>Personal <input type=checkbox name=personal {{if .Form.Checked "personal"}}checked{{end}}></label></p>{{end}}
<p><label>Allow, one name or <code>*</code> per line <textarea name=allow rows=5>{{.Form.Value "allow"}}</textarea></label> Empty allows only the owner and assigned Maintainers. Add names or * to share.</p>{{if .Form.Is "create"}}<p class=warn id=create-error>{{.Form.Error}}</p>{{end}}{{if not .Channels}}<p class=muted>A Personal service may name only other registered services directly. Users, groups, <code>*</code>, itself and Maintainers are refused.</p>{{end}}<button>Register</button></form>`))
var serviceDetail = template.Must(template.New("service").Funcs(template.FuncMap{"join": strings.Join, "readerCount": readerCount, "entityLabel": entityLabel, "titleMark": titleMark, "photoData": photoData, "profileInitial": profileInitial}).Parse(shell("records", "Service") + `
{{with .Record}}<div class=page-title><h1>{{titleMark .Kind}} {{.Name}}{{if .Personal}} <span class=muted>· Personal</span>{{end}}</h1></div>` + formErrorSummary + `<p>Type: <strong>{{entityLabel .Kind}}</strong> · Owner: {{with $.OwnerUser}}<span class=identity-with-photo>{{with photoData .}}<img class=profile-photo src="{{.}}" alt="">{{else}}<span class=profile-initial aria-hidden=true>{{profileInitial .}}</span>{{end}}<a href="/user?name={{.Name}}"><code>{{.Name}}</code></a></span>{{else}}{{.Owner}}{{end}}{{with .Maintainers}} · Maintainers: {{join . ", "}}{{end}}{{if .Mode}} · Delivery: {{if eq .Mode "pubsub"}}a copy to each subscriber{{else}}one at a time{{end}}{{end}}</p>
<div class=page-title><h2>Delivery</h2><button type=button class=help-button popovertarget=delivery-help aria-label="About delivery state" data-tooltip="Stored setting only. The daemon also checks owner access, ACL and queue capacity; Disabled does not reveal why it is off.">ⓘ</button></div>
<p>Delivery: <strong><span class=status-glyph role=img aria-label="{{if .Disabled}}Disabled{{else}}Enabled{{end}}" title="{{if .Disabled}}Disabled{{else}}Enabled{{end}}">{{if .Disabled}}🚫{{else}}🔛{{end}}</span></strong></p>
<div popover id=delivery-help class=context-help><h2>About delivery state</h2><ul><li>This stored setting does not establish that a send will be accepted.</li><li>The daemon also checks owner access, the allow list and queue capacity.</li><li>The disabled bit does not say whether the owner turned it off or the name stopped being active.</li></ul></div>
<div class=page-title><h2>Policy</h2><button type=button class=help-button popovertarget=policy-help aria-label="About record policy" data-tooltip="External is a caller hint. An unset bound uses an unreadable daemon default; unset retention adds no queue expiry; overflow belongs to this record.">ⓘ</button></div>
<p>Reached: <strong>{{if .Proto}}external{{else}}this bus{{end}}</strong> · Queue bound: {{if .Bound}}{{.Bound}}{{else}}default{{end}} · Retention: {{if .TTL}}{{.TTL}}{{else}}none{{end}} · When full: {{if eq .Full "ring"}}drop the oldest{{else}}refuse{{end}}</p>
<div popover id=policy-help class=context-help><h2>About record policy</h2><ul>{{if .Proto}}<li>External is a caller-supplied hint, not proof of anything.</li>{{end}}{{if not .Bound}}<li>An unset queue bound uses the daemon default; its resolved value is not readable here.</li>{{end}}{{if not .TTL}}<li>Unset retention means no queue-imposed expiry; a message may still specify its own.</li>{{end}}<li>The overflow policy always belongs to this record.</li></ul></div>
<div class=page-title><h2>Observed</h2><button type=button class=help-button popovertarget=observed-help aria-label="About observed counters" data-tooltip="Readers are outstanding requests, not health. Queue reads may precede pruning. Counters survive restart; Dequeued is not completion.">ⓘ</button></div>
<p>Readers: <strong>{{readerCount .Readers}}</strong> · Held now: <strong>{{.Queued}}</strong>{{if .AtBound}} · <span class=warn>at capacity when observed</span>{{end}} · Oldest held: {{if .Oldest}}{{.Oldest}}{{else}}<span class=muted>&mdash;</span>{{end}}</p>
<p>Accepted: {{.In}} · Dequeued: {{.Out}} · Dropped: {{.Dropped}} · Expired: {{.Expired}}</p>
<div popover id=observed-help class=context-help><h2>About observed counters</h2><ul><li>Readers counts outstanding filtered and unfiltered reads; zero is not an offline signal.</li><li>The read does not prune first, so Held and Oldest may include work already past its TTL.</li><li>Counters are cumulative across restarts. Dequeued is handed to a reader, which is not completed.</li></ul></div>
<p>Registration updated: {{if .At.IsZero}}<span class=muted>&iquest;</span>{{else}}{{.At.Format "2006-01-02 15:04:05"}}{{end}}</p>
<p>Configuration digest: {{if .ConfigSHA}}<code>{{.ConfigSHA}}</code>{{else}}<span class=muted>&mdash;</span>{{end}}</p>
{{template "activity-view" $}}
<p><a href="/activity?name={{.Name}}">View all activity and sample values</a></p>
{{if eq .Mode "pubsub"}}<details><summary>Subscriptions</summary>
{{range .Subs}}<p>{{.}}{{if $.Record.CanManage}}<form method=post action=/service><input type=hidden name=name value="{{$.Record.Name}}"><input type=hidden name=subscriber value="{{.}}"><button name=action value=remove-subscriber>Remove subscription</button></form>{{end}}</p>{{else}}<p>No subscribers</p>{{end}}
<form method=post action=/service><input type=hidden name=name value="{{.Name}}"><button name=action value=subscribe>Subscribe my inbox</button><button name=action value=unsubscribe>Unsubscribe my inbox</button></form></details>{{end}}
{{if .CanManage}}
<details {{if $.Form.Is "save"}}open{{end}}><summary id=settings>Edit settings</summary><form id=form-save method=post action=/service><input type=hidden name=name value="{{.Name}}"><input type=hidden name=action value=save>
<p><label>Description <input name=descr value="{{if $.Form.Is "save"}}{{$.Form.Value "descr"}}{{else}}{{.Descr}}{{end}}"></label></p>
<p><label>Address <input name=addr value="{{if $.Form.Is "save"}}{{$.Form.Value "addr"}}{{else}}{{.Addr}}{{end}}"></label></p>
<p><label>Protocol <input name=protocol value="{{if $.Form.Is "save"}}{{$.Form.Value "protocol"}}{{else}}{{.Proto}}{{end}}"></label></p>
{{if .Personal}}<p class=muted>Personal classification, Allow and Maintainers are changed together in the owner form below.</p>{{else}}<input type=hidden name=edit_allow value=1><p><label>Allow, one name or <code>*</code> per line <textarea name=allow rows=5>{{if $.Form.Is "save"}}{{$.Form.Value "allow"}}{{else}}{{join .Allow "\n"}}{{end}}</textarea></label></p><p>Empty allows only the owner and assigned Maintainers. Add names or * to share.</p>{{end}}
<p><label>Refuse master access <input type=checkbox name=no_master {{if $.Form.Is "save"}}{{if $.Form.Checked "no_master"}}checked{{end}}{{else}}{{if .NoMaster}}checked{{end}}{{end}}></label></p>
<p><label>Queue TTL <input name=ttl value="{{if $.Form.Is "save"}}{{$.Form.Value "ttl"}}{{else}}{{.TTL}}{{end}}" placeholder="default"></label></p>
<p><label>Queue capacity (0 uses default) <input type=number min=0 name=bound value="{{if $.Form.Is "save"}}{{$.Form.Value "bound"}}{{else}}{{.Bound}}{{end}}" aria-invalid="{{if $.Form.Invalid "bound"}}true{{else}}false{{end}}" aria-describedby="{{if $.Form.Invalid "bound"}}save-error{{end}}"></label></p>
<p><label>Overflow <select name=overflow><option value=strict {{if and ($.Form.Is "save") (eq ($.Form.Value "overflow") "strict")}}selected{{end}}>Refuse</option><option value=ring {{if $.Form.Is "save"}}{{if eq ($.Form.Value "overflow") "ring"}}selected{{end}}{{else}}{{if eq .Full "ring"}}selected{{end}}{{end}}>Drop oldest</option></select></label></p>{{if $.Form.Is "save"}}<p class=warn id=save-error>{{$.Form.Error}}</p>{{end}}<button>Save settings</button></form></details>
<form method=post action=/service><input type=hidden name=name value="{{.Name}}"><button name=action value="{{if .Disabled}}enable{{else}}disable{{end}}">{{if .Disabled}}Enable{{else}}Disable{{end}}</button></form>
{{if .CanTransfer}}{{if eq .Kind "generic"}}<details {{if $.Form.Is "personal"}}open{{end}}><summary>Classification and sharing</summary><form id=form-personal method=post action=/service><input type=hidden name=name value="{{.Name}}"><input type=hidden name=action value=personal><p><label>Personal <input type=checkbox name=personal {{if $.Form.Is "personal"}}{{if $.Form.Checked "personal"}}checked{{end}}{{else}}{{if .Personal}}checked{{end}}{{end}}></label> Groups this service in the owner&rsquo;s Personal services page; it does not change access.</p><p><label>Allow, one name or <code>*</code> per line <textarea name=allow rows=5>{{if $.Form.Is "personal"}}{{$.Form.Value "allow"}}{{else}}{{join .Allow "\n"}}{{end}}</textarea></label></p><p><label>Maintainers, one user, group, agent or service per line <textarea name=maintainers rows=5 aria-invalid="{{if $.Form.Is "personal"}}true{{else}}false{{end}}" aria-describedby="{{if $.Form.Is "personal"}}personal-error{{end}}">{{if $.Form.Is "personal"}}{{$.Form.Value "maintainers"}}{{else}}{{join .Maintainers "\n"}}{{end}}</textarea></label></p>{{if $.Form.Is "personal"}}<p class=warn id=personal-error>{{$.Form.Error}}</p>{{end}}<p class=muted>When Personal is checked, Allow may name only other registered services directly. Users, groups, <code>*</code>, this service and Maintainers are refused. Clear Personal in this same form before adding any of them.</p><button>Save classification and sharing</button></form></details>{{else}}<details {{if $.Form.Is "maintainers"}}open{{end}}><summary>Maintainers</summary><form id=form-maintainers method=post action=/service><input type=hidden name=name value="{{.Name}}"><input type=hidden name=action value=maintainers><label>One user, group, agent or service per line <textarea name=maintainers rows=5 aria-invalid="{{if $.Form.Is "maintainers"}}true{{else}}false{{end}}" aria-describedby="{{if $.Form.Is "maintainers"}}maintainers-error{{end}}">{{if $.Form.Is "maintainers"}}{{$.Form.Value "maintainers"}}{{else}}{{join .Maintainers "\n"}}{{end}}</textarea></label>{{if $.Form.Is "maintainers"}}<p class=warn id=maintainers-error>{{$.Form.Error}}</p>{{end}}<button>Assign</button></form></details>{{end}}
{{end}}
<p><a class=danger href="/service-danger?name={{.Name}}">Danger Zone</a></p>
{{else}}<p>{{.Descr}}</p><p>You can view this record; its owner and assigned maintainers can manage it.</p>{{end}}{{end}}` + activityViewTemplate))
var serviceDanger = template.Must(template.New("service-danger").Funcs(template.FuncMap{"readerCount": readerCount, "titleMark": titleMark}).Parse(shell("records", "Danger Zone") + `
{{with .Record}}<p><a href="/service?name={{.Name}}">Back to {{.Name}}</a></p>
<div class=page-title><h1>{{titleMark "problem"}} Danger Zone · {{.Name}}</h1></div>` + formErrorSummary + `
<h2>Replace configuration</h2><form id=form-configure method=post action=/service><input type=hidden name=name value="{{.Name}}"><input type=hidden name=action value=configure><label>New configuration <textarea name=config rows=6 cols=60 required autocomplete=off aria-invalid="{{if $.Form.Invalid "config"}}true{{else}}false{{end}}" aria-describedby="{{if $.Form.Invalid "config"}}configure-error{{end}}"></textarea></label><p>Existing private configuration and a refused replacement are never displayed.</p>{{if $.Form.Is "configure"}}<p class=warn id=configure-error>{{$.Form.Error}}</p>{{end}}<button>Replace configuration</button></form>
{{if .CanTransfer}}{{if ne .Name .Owner}}<h2>Transfer ownership</h2><form id=form-transfer method=post action=/service-confirm><input type=hidden name=name value="{{.Name}}"><input type=hidden name=action value=transfer><label>New owner <input name=owner required value="{{if $.Form.Is "transfer"}}{{$.Form.Value "owner"}}{{end}}" aria-invalid="{{if $.Form.Invalid "owner"}}true{{else}}false{{end}}" aria-describedby="{{if $.Form.Invalid "owner"}}transfer-error{{end}}"></label>{{if $.Form.Is "transfer"}}<p class=warn id=transfer-error>{{$.Form.Error}}</p>{{end}}<p>The new owner must have a registered identity. Existing service credentials remain valid; transfer does not revoke copies already held.</p><button>Continue to confirmation</button></form>{{end}}{{end}}
<h2>Remove registration</h2><form method=post action=/service-confirm><input type=hidden name=name value="{{.Name}}"><input type=hidden name=action value=delete><p><strong>No registration, no access:</strong> the credential goes with the address. Drain the queue and stop readers first; the confirmation page re-reads both before describing the consequence.</p><button>Continue to confirmation</button></form>{{end}}`))
var serviceConfirm = template.Must(template.New("service-confirm").Funcs(template.FuncMap{"readerSnapshot": readerSnapshot, "readerCount": readerCount, "titleMark": titleMark}).Parse(shell("records", "Confirm action") + `
{{with .Record}}<p><a href="/service-danger?name={{.Name}}">Back to the Danger Zone</a></p>
{{if eq $.Action "transfer"}}<div class=page-title><h1>{{titleMark "problem"}} Confirm ownership transfer</h1></div><p>Transfer <code>{{.Name}}</code> from <code>{{.Owner}}</code> to <code>{{$.NewOwner}}</code>?</p><p>The new owner must still be registered and active when the daemon applies this. Credentials already held are not revoked.</p><form method=post action=/service><input type=hidden name=action value=transfer><input type=hidden name=name value="{{.Name}}"><input type=hidden name=owner value="{{$.NewOwner}}"><input type=hidden name=expected_owner value="{{.Owner}}"><input type=hidden name=confirmed value=1><button>Transfer ownership</button></form>
{{else}}<div class=page-title><h1>{{titleMark "problem"}} Confirm removal</h1></div><p>Remove <code>{{.Name}}</code>? It currently holds <strong>{{.Queued}}</strong> messages and has <strong>{{readerCount .Readers}}</strong> outstanding reads.</p><p>The address and its credential go with it; nothing answers to this name afterwards. A person&rsquo;s own credential stays because it is not this record&rsquo;s to remove.</p><form method=post action=/service><input type=hidden name=action value=delete><input type=hidden name=name value="{{.Name}}"><input type=hidden name=expected_owner value="{{.Owner}}"><input type=hidden name=expected_queued value="{{.Queued}}"><input type=hidden name=expected_readers value="{{readerSnapshot .Readers}}"><input type=hidden name=confirmed value=1><button>Remove registration</button></form>{{end}}{{end}}`))
var groupList = template.Must(template.New("groups").Funcs(template.FuncMap{"join": strings.Join, "groupGlyph": groupGlyph, "titleMark": titleMark}).Parse(shell("groups", "Groups") + `
<div class=page-title><h1>{{titleMark "groups"}} Groups</h1></div>` + formErrorSummary + `<nav class=section-nav aria-label="Group views">{{range .SectionLinks}}{{if .Current}}<a href="{{.Href}}" aria-current=true>{{else}}<a href="{{.Href}}">{{end}}{{.Label}}{{if .Counted}} ({{.Count}}){{end}}</a>{{end}}</nav>{{range $name,$members := .Groups}}<h2><span role=img aria-label="Group">{{groupGlyph}}</span> <code>{{$name}}</code></h2>{{if and $.Administrator (or $.DaemonOwner (ne $name "@administrators"))}}<form id="form-group-{{$name}}" method=post action=/groups><input type=hidden name=name value="{{$name}}"><p><label>Members, one identity or group per line <textarea name=members rows=6 aria-invalid="{{if and ($.Form.Matches "save" "name" $name) ($.Form.Invalid "members")}}true{{else}}false{{end}}" aria-describedby="{{if and ($.Form.Matches "save" "name" $name) ($.Form.Invalid "members")}}group-error{{end}}">{{if $.Form.Matches "save" "name" $name}}{{$.Form.Value "members"}}{{else}}{{join $members "\n"}}{{end}}</textarea></label></p>{{if $.Form.Matches "save" "name" $name}}<p class=warn id=group-error>{{$.Form.Error}}</p>{{end}}<button name=action value=save>Save members</button></form>{{if eq $name "@administrators"}}<p class=muted>The daemon owner stays in this group.</p>{{end}}{{end}}{{else}}<p>No groups registered.</p>{{end}}
{{if not .Administrator}}<p>Daemon administrators manage group membership. Only the daemon owner changes the administrators group. Service owners can assign an existing group to their own services.</p>{{end}}`))
var groupNew = template.Must(template.New("group-new").Funcs(template.FuncMap{"titleMark": titleMark}).Parse(shell("groups", "Register group") + `
<p><a href=/groups>Back to Groups</a></p><div class=page-title><h1>{{titleMark "groups"}} Register group</h1></div>
` + formErrorSummary + `<form id=form-save method=post action=/groups><input type=hidden name=action value=save><input type=hidden name=new value=1><label>Name <input name=name placeholder="@operators" required value="{{.Form.Value "name"}}" aria-invalid="{{if .Form.Is "save"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Is "save"}}group-new-error{{end}}"></label><p><label>Members, one identity or group per line <textarea name=members rows=6 placeholder="user@realm">{{.Form.Value "members"}}</textarea></label></p>{{if .Form.Is "save"}}<p class=warn id=group-new-error>{{.Form.Error}}</p>{{end}}<button>Register group</button></form>`))

package main

import (
	"encoding/base64"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/parf/ai-agent-bus/internal/display"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func profileInitial(u protocol.User) string {
	label := u.PersonName
	if label == "" {
		label = u.Name
	}
	initial, _ := utf8.DecodeRuneInString(label)
	return strings.ToUpper(string(initial))
}

// photoData is safe template.URL only because the scheme and media type are
// fixed here and the payload is base64 over adapter-normalized local PNG.
// No provider URL or caller text enters the result.
func photoData(u protocol.User) template.URL {
	if len(u.PhotoPNG) == 0 {
		return ""
	}
	return template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(u.PhotoPNG))
}

func githubProfileURL(login string) string { return "https://github.com/" + url.PathEscape(login) }
func twitterProfileURL(name string) string { return "https://x.com/" + url.PathEscape(name) }

func profileFromForm(r *http.Request) protocol.User {
	return protocol.User{
		Name: r.PostForm.Get("name"), PersonName: r.PostForm.Get("person_name"),
		Email: r.PostForm.Get("email"), GithubUser: r.PostForm.Get("github_user"),
		GithubCompany: r.PostForm.Get("company"), GithubLocation: r.PostForm.Get("location"),
		GithubTwitterUsername: r.PostForm.Get("twitter"),
	}
}

func recordPath(name string, kinds map[string]string) string {
	return recordKindPath(name, kinds[name])
}

func recordKindPath(name, kind string) string {
	return detailPathFor(kind) + "?name=" + url.QueryEscape(name)
}

// The directory answer may name every record a user owns. Detail navigation
// follows the separately authorized listing, so a directory-visible person
// never becomes an oracle for record names hidden from this caller.
//
// The daemon's own list names only live records, so an inactive user's page
// takes the records it owns from the caller-visible listings, /inactive
// included: they are the one place an inactive record shows.
func visibleUserResources(u protocol.User, kinds, owners map[string]string) protocol.User {
	seen := map[string]bool{}
	services := make([]string, 0, len(u.Services))
	for _, name := range u.Services {
		if _, visible := kinds[name]; visible && !seen[name] {
			seen[name] = true
			services = append(services, name)
		}
	}
	for name, owner := range owners {
		if owner == u.Name && name != u.Name && !seen[name] {
			seen[name] = true
			services = append(services, name)
		}
	}
	sort.Strings(services)
	u.Services = services
	return u
}

type peopleView struct {
	adminView
	Users                                []protocol.User
	User                                 protocol.User
	New                                  bool
	People                               []protocol.User
	Query, State, Return, Previous, Next string
	PeopleCount, Matched, Start, End     int
	ActiveCount, InactiveCount           int
	StateLinks                           []viewLink
	RecordKinds                          map[string]string
	RecordOwners                         map[string]string
	// Agents counts each User's agents; LastUsed is when a User's own
	// credential last made a call, from the listing (docs/05-discovery.md#what-a-listing-answers).
	Agents   map[string]int
	LastUsed map[string]time.Time
	Now      time.Time
}

// identityLabel and identityGlyph mark the node's daemon owner as the
// authority rather than as one more person: a directory row carries only a
// name, so the entity type is the row's least useful fact about the one
// identity that cannot be delegated. The daemon states it per user, so a page
// never infers it from a name.
func identityLabel(u protocol.User, recordKinds map[string]string) string {
	if u.Kind == protocol.DirectoryUser {
		return display.Identity(protocol.DirectoryUser, u.DaemonOwner)
	}
	if u.Kind == protocol.DirectoryRecord {
		if kind := recordKinds[u.Name]; protocol.ValidKind(kind) {
			return display.Identity(kind, u.DaemonOwner)
		}
	}
	return ""
}

func identityGlyph(u protocol.User, recordKinds map[string]string) string {
	if u.Kind == protocol.DirectoryUser {
		return display.IdentityGlyph(protocol.DirectoryUser, u.DaemonOwner)
	}
	if u.Kind == protocol.DirectoryRecord {
		return display.IdentityGlyph(recordKinds[u.Name], u.DaemonOwner)
	}
	return ""
}

// directoryReturn keeps a way back to the page that led here: the user
// directory, or Diagnostics, which lists leftover credentials.
func directoryReturn(raw string) string {
	u, err := url.Parse(local(raw))
	if err != nil || u.Path != "/users" && u.Path != "/diagnostics" {
		return "/users"
	}
	u.Fragment = ""
	return u.RequestURI()
}

// userState normalises a directory entry's status. A blank value is an
// active user, which is how the detail page reads it too.
func userState(u protocol.User) string {
	if u.Status == "" {
		return protocol.StatusActive
	}
	return u.Status
}

// stateBadge is the marker shown after a name that is not active. Active users
// get nothing: marking the ordinary majority marks nothing
// (Plans/MVP/web/glyphs.md#the-rule-that-matters-most), and the directory has
// no state column for the same reason (docs/decisions.md#dashboard-implementation-defaults).
func stateBadge(u protocol.User) template.HTML {
	if userState(u) == protocol.StatusInactive {
		return `<span class="state-badge state-inactive">INACTIVE</span>`
	}
	return ""
}

func (p *peopleView) directory(r *http.Request) {
	p.Query = strings.TrimSpace(r.URL.Query().Get("q"))
	// Active is the default view. Only Users are listed: every name is a
	// User or an Agent, and Agents have their own page. A credential left
	// with no record is Diagnostics' to show (docs/05-discovery.md#overview-and-diagnostics).
	p.State = r.URL.Query().Get("state")
	switch p.State {
	case protocol.StatusInactive, "all":
	default:
		p.State = "active"
	}
	var matched []protocol.User
	for _, u := range p.Users {
		if u.Kind != protocol.DirectoryUser {
			continue
		}
		p.PeopleCount++
		if userState(u) == protocol.StatusInactive {
			p.InactiveCount++
		} else {
			p.ActiveCount++
		}
		if p.State != "all" && userState(u) != p.State {
			continue
		}
		if !strings.Contains(strings.ToLower(u.Name+" "+u.PersonName+" "+u.Email+" "+u.GithubUser+" "+u.GithubCompany+" "+u.GithubLocation+" "+u.GithubTwitterUsername), strings.ToLower(p.Query)) {
			continue
		}
		matched = append(matched, u)
	}
	p.Matched = len(matched)
	const pageSize = 25
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	last := max(1, (len(matched)+pageSize-1)/pageSize)
	page = min(max(1, page), last)
	link := func(n int) string {
		q := url.Values{}
		if p.Query != "" {
			q.Set("q", p.Query)
		}
		if p.State != "active" {
			q.Set("state", p.State)
		}
		q.Set("page", strconv.Itoa(n))
		return "/users?" + q.Encode()
	}
	p.Return = link(page)
	if page > 1 {
		p.Previous = link(page - 1)
	}
	if page < last {
		p.Next = link(page + 1)
	}
	start, end := (page-1)*pageSize, min(page*pageSize, len(matched))
	if len(matched) > 0 {
		p.Start, p.End = start+1, end
	}
	p.People = matched[start:end]
}

func (c *caller) userRoutes(mux *http.ServeMux, tls bool) {
	c.accountRoute(mux)
	load := func(w http.ResponseWriter, r *http.Request) (peopleView, bool) {
		v, ok := c.signedIn(w, r)
		p := peopleView{adminView: v}
		if !ok {
			return p, false
		}
		if err := c.get(cookie(r), "/users", &p.Users); err != nil {
			fail(w, r, v.You, err)
			return p, false
		}
		// Inactive records too: an inactive user's records are all inactive,
		// and their page still names what they own.
		records, err := c.allRecords(cookie(r))
		if err != nil {
			fail(w, r, v.You, err)
			return p, false
		}
		p.RecordKinds = make(map[string]string, len(records))
		p.RecordOwners = make(map[string]string, len(records))
		p.Agents, p.LastUsed, p.Now = map[string]int{}, map[string]time.Time{}, time.Now()
		for _, record := range records {
			p.RecordKinds[record.Name] = record.Kind
			if record.Kind != protocol.KindGroup {
				p.RecordOwners[record.Name] = record.Owner
			}
			if record.Kind == protocol.KindAgent {
				p.Agents[record.Owner]++
			}
			if record.LastUsed != nil {
				p.LastUsed[record.Name] = *record.LastUsed
			}
		}
		return p, true
	}
	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		// The Other filter is gone: what it listed is Diagnostics' now, and
		// an old link lands there. kind=users was every row, and is ignored.
		if r.URL.Query().Get("kind") == "other" {
			http.Redirect(w, r, "/diagnostics#leftovers", http.StatusSeeOther)
			return
		}
		p, ok := load(w, r)
		if !ok {
			return
		}
		p.directory(r)
		p.SectionLinks = []viewLink{{Href: "/users", Label: "All", Count: p.PeopleCount, Counted: true, Current: true}}
		if p.Administrator {
			p.SectionLinks = append(p.SectionLinks, viewLink{Href: "/users/new", Label: "Register user"})
		}
		filterBase := url.Values{}
		if p.Query != "" {
			filterBase.Set("q", p.Query)
		}
		stateBase := cloneValues(filterBase)
		p.StateLinks = []viewLink{
			{Href: pageURL("/users", cloneValues(stateBase)), Label: "Active", Count: p.ActiveCount, Counted: true, Current: p.State == "active"},
			{Href: queryWith("/users", stateBase, "state", "inactive"), Label: "Inactive", Count: p.InactiveCount, Counted: true, Current: p.State == "inactive"},
			{Href: queryWith("/users", stateBase, "state", "all"), Label: "All states", Count: p.PeopleCount, Counted: true, Current: p.State == "all"},
		}
		render(w, peoplePage, p)
	})
	renderNew := func(w http.ResponseWriter, r *http.Request, p peopleView) {
		if !p.Administrator {
			fail(w, r, p.You, &busError{code: http.StatusForbidden, message: "only a daemon Administrator can register a user"})
			return
		}
		p.New, p.Return = true, "/users"
		render(w, personPage, p)
	}
	mux.HandleFunc("GET /users/new", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		renderNew(w, r, peopleView{adminView: v})
	})
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		p, ok := load(w, r)
		if !ok {
			return
		}
		name := r.URL.Query().Get("name")
		p.Return = directoryReturn(r.URL.Query().Get("return"))
		if name == "" && p.Administrator {
			renderNew(w, r, p)
			return
		}
		for _, u := range p.Users {
			if u.Name == name {
				p.User = visibleUserResources(u, p.RecordKinds, p.RecordOwners)
				render(w, personPage, p)
				return
			}
		}
		fail(w, r, p.You, &busError{code: http.StatusNotFound})
	})
	// The profile form is a page, not a panel: it is the form that adds a
	// person with the person already in it.
	mux.HandleFunc("GET /user/edit", func(w http.ResponseWriter, r *http.Request) {
		p, ok := load(w, r)
		if !ok {
			return
		}
		p.Return = directoryReturn(r.URL.Query().Get("return"))
		for _, u := range p.Users {
			if u.Name != r.URL.Query().Get("name") {
				continue
			}
			if u.Kind != protocol.DirectoryUser || !u.CanEdit {
				fail(w, r, p.You, &busError{code: http.StatusForbidden, message: "that profile cannot be edited by you"})
				return
			}
			p.User = visibleUserResources(u, p.RecordKinds, p.RecordOwners)
			render(w, userEdit, p)
			return
		}
		fail(w, r, p.You, &busError{code: http.StatusNotFound})
	})
	mux.HandleFunc("GET /user-deactivate", func(w http.ResponseWriter, r *http.Request) {
		p, ok := load(w, r)
		if !ok {
			return
		}
		p.Return = directoryReturn(r.URL.Query().Get("return"))
		for _, u := range p.Users {
			if u.Name != r.URL.Query().Get("name") {
				continue
			}
			if u.Kind != protocol.DirectoryUser || u.DaemonOwner || !u.CanActivate || userState(u) == protocol.StatusInactive {
				fail(w, r, p.You, &busError{code: http.StatusForbidden, message: "that user cannot be deactivated by you in their current state"})
				return
			}
			p.User = u
			render(w, userDeactivatePage, p)
			return
		}
		fail(w, r, p.You, &busError{code: http.StatusNotFound})
	})
	mux.HandleFunc("GET /credential-remove", func(w http.ResponseWriter, r *http.Request) {
		p, ok := load(w, r)
		if !ok {
			return
		}
		p.Return = directoryReturn(r.URL.Query().Get("return"))
		for _, u := range p.Users {
			if u.Name != r.URL.Query().Get("name") {
				continue
			}
			if u.Kind == protocol.DirectoryUser || !u.CanRemove {
				fail(w, r, p.You, &busError{code: http.StatusForbidden, message: "that credential cannot be removed by you"})
				return
			}
			p.User = visibleUserResources(u, p.RecordKinds, p.RecordOwners)
			render(w, credentialRemovePage, p)
			return
		}
		fail(w, r, p.You, &busError{code: http.StatusNotFound})
	})
	mux.HandleFunc("GET /avatar", func(w http.ResponseWriter, r *http.Request) {
		p, ok := load(w, r)
		if !ok {
			return
		}
		for _, u := range p.Users {
			if u.Name == r.URL.Query().Get("name") {
				if len(u.PhotoPNG) != 0 {
					w.Header().Set("Content-Type", "image/png")
					w.Write(u.PhotoPNG)
					return
				}
				w.Header().Set("Content-Type", "image/svg+xml")
				render(w, avatarPage, profileInitial(u))
				return
			}
		}
		http.Error(w, "no such user", 404)
	})
	renderUserFormError := func(w http.ResponseWriter, r *http.Request, action string, code int, message string, field ...string) bool {
		p, ok := load(w, r)
		if !ok {
			return true
		}
		p.Return = directoryReturn(r.PostForm.Get("return"))
		p.Form = retainedForm(action, message, r.PostForm, "name", "person_name", "email", "github_user", "company", "location", "twitter", "return")
		if len(field) != 0 {
			p.Form.Field = field[0]
		}
		if action == "create" {
			if !p.Administrator {
				return false
			}
			p.New = true
			p.User = profileFromForm(r)
			p.User.Kind = protocol.DirectoryUser
			renderForm(w, code, personPage, p)
			return true
		}
		if action == "save" {
			for _, u := range p.Users {
				if u.Name != r.PostForm.Get("name") {
					continue
				}
				form := profileFromForm(r)
				u.PersonName, u.Email, u.GithubUser = form.PersonName, form.Email, form.GithubUser
				u.GithubCompany, u.GithubLocation, u.GithubTwitterUsername = form.GithubCompany, form.GithubLocation, form.GithubTwitterUsername
				p.User = visibleUserResources(u, p.RecordKinds, p.RecordOwners)
				renderForm(w, code, userEdit, p)
				return true
			}
		}
		return false
	}
	mux.HandleFunc("POST /user", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r, tls) {
			http.Error(w, "same-origin form required", 403)
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
		var err error
		action := r.PostForm.Get("action")
		switch action {
		case "remove-credential":
			err = c.post(cookie(r), "/identity/remove", map[string]string{"name": r.PostForm.Get("name")})
		case "save", "create":
			u := profileFromForm(r)
			err = c.post(cookie(r), "/user", struct {
				protocol.User
				Create            bool `json:"create,omitempty"`
				ProfileDetailsSet bool `json:"profile_details_set"`
			}{u, r.PostForm.Get("action") == "create", true})
		case "refresh-github":
			err = c.post(cookie(r), "/user/github-refresh", map[string]string{"name": r.PostForm.Get("name")})
		case protocol.StatusActive, protocol.StatusInactive:
			err = c.post(cookie(r), "/user/state", map[string]string{"name": r.PostForm.Get("name"), "status": r.PostForm.Get("action")})
		default:
			localProblem(w, r, v.You, http.StatusBadRequest, "That user action is not available.")
			return
		}
		if err != nil {
			if code, message, preserve := formRefusal(err); preserve {
				field := ""
				if action == "create" && code == http.StatusPreconditionFailed {
					field = "name"
				}
				if action == "refresh-github" {
					p, loaded := load(w, r)
					if loaded {
						p.Return = directoryReturn(r.PostForm.Get("return"))
						for _, u := range p.Users {
							if u.Name == r.PostForm.Get("name") {
								p.User, p.Form = visibleUserResources(u, p.RecordKinds, p.RecordOwners), formState{Action: action, Target: action, Error: message}
								renderForm(w, code, personPage, p)
								return
							}
						}
					}
				} else if renderUserFormError(w, r, action, code, message, field) {
					return
				}
			}
			fail(w, r, v.You, err)
			return
		}
		next := directoryReturn(r.PostForm.Get("return"))
		if action == "create" {
			next = "/user?name=" + url.QueryEscape(r.PostForm.Get("name"))
		}
		http.Redirect(w, r, next, http.StatusSeeOther)
	})
}

var peoplePage = template.Must(template.New("people").Funcs(template.FuncMap{"authorityLabel": authorityLabel, "identityGlyph": identityGlyph, "identityLabel": identityLabel, "userState": userState, "stateBadge": stateBadge, "titleMark": titleMark, "photoData": photoData, "profileInitial": profileInitial, "number": number, "ago": registrationUpdatedAt}).Parse(shell("users", "Users") + `
<div class=page-title><h1>{{titleMark "users"}} Users</h1><button type=button class=help-button popovertarget=users-help aria-label="About users">ⓘ</button></div>
<div popover id=users-help class=context-help><h2>About users</h2><ul>
<li>A User is a person on this node, and owns every record: agents, services, queues, topics and groups. Agents are listed under Agents.</li>
<li>A User is Active or Inactive; an inactive one is struck and marked INACTIVE beside the name. An inactive User, and everything they own, answers as unknown until reactivated.</li>
<li>Last used is when the User's own credential last made a call, as the daemon reports it.</li>
<li>Agents counts the agents each User owns that are visible to you. That figure, and the counts beside the tabs and filters, are worked out by this page, not figures the daemon reported.</li>
</ul></div>
<nav class=section-nav aria-label="User views">{{range .SectionLinks}}{{if .Current}}<a href="{{.Href}}" aria-current=true>{{else}}<a href="{{.Href}}">{{end}}{{.Label}}{{if .Counted}} ({{number .Count}}){{end}}</a>{{end}}</nav>
<div class=record-toolbar>
<form class=record-search method=get action=/users>
<label for=record-query class=visually-hidden>Search users</label><input id=record-query type=search name=q value="{{.Query}}" placeholder="Search by name, identity, email or GitHub login">
{{if ne .State "active"}}<input type=hidden name=state value="{{.State}}">{{end}}
<div class=record-choices><nav class=filter-nav aria-label="Status filter"><span>Status</span>{{range .StateLinks}}{{if .Current}}<a href="{{.Href}}" aria-current=true>{{else}}<a href="{{.Href}}">{{end}}{{.Label}} ({{number .Count}})</a>{{end}}</nav></div>
<noscript><button>Search</button></noscript>
</form>
</div>
{{if .People}}
<table class="record-table users-table"><caption>Showing {{number .Start}}&ndash;{{number .End}} of {{number .Matched}} matching users.</caption><thead><tr><th scope=col>User</th><th scope=col>Authority</th><th scope=col>Contact</th><th scope=col class=num>Agents</th><th scope=col>Last used</th></tr></thead><tbody>
{{range .People}}{{$struck := ne (userState .) "active"}}<tr><td class=record-name-cell><span class=identity-with-photo>{{with photoData .}}<img class=profile-photo src="{{.}}" alt="">{{else}}<span class=profile-initial aria-hidden=true>{{profileInitial .}}</span>{{end}}<span>{{with .PersonName}}<strong>{{if $struck}}<s>{{.}}</s>{{else}}{{.}}{{end}}</strong><br>{{end}}{{if identityGlyph . $.RecordKinds}}<span role=img aria-label="{{identityLabel . $.RecordKinds}}">{{identityGlyph . $.RecordKinds}}</span> {{end}}<a href="/user?name={{.Name}}&return={{$.Return}}"><code>{{if $struck}}<s>{{.Name}}</s>{{else}}{{.Name}}{{end}}</code></a> {{stateBadge .}}{{with .GithubCompany}}<br><span class=muted>{{.}}</span>{{end}}</span></span></td>
<td data-label=Authority>{{authorityLabel .DaemonOwner .Administrator}}</td>
<td data-label=Contact>{{with .Email}}<span class=contact-line>{{.}}</span>{{end}}{{with .GithubUser}}<span class=contact-line><span class=muted>GitHub</span> <code>{{.}}</code></span>{{end}}{{if not (or .Email .GithubUser)}}<span class=muted>&mdash;</span>{{end}}</td>
<td class=num data-label=Agents>{{with index $.Agents .Name}}{{number .}}{{else}}<span class=muted>0</span>{{end}}</td>
<td data-label="Last used">{{$at := index $.LastUsed .Name}}{{if not $at.IsZero}}<time datetime="{{$at.Format "2006-01-02T15:04:05Z07:00"}}" title="{{$at.Format "2006-01-02 15:04"}}">{{ago $at $.Now}}</time>{{else if $struck}}<span class=muted title="Not reported while inactive">&mdash;</span>{{else}}<span class=muted>never</span>{{end}}</td></tr>
{{end}}</tbody></table>
<nav aria-label="Directory pages">{{with .Previous}}<a href="{{.}}">Previous page</a>{{end}} {{with .Next}}<a href="{{.}}">Next page</a>{{end}}</nav>
{{else}}
<section class="empty-state editor-card"><h2>{{if .Query}}No user matches this search{{else if eq .State "inactive"}}No inactive users{{else}}No users yet{{end}}</h2>
<p>A User is a person on this node; every record belongs to one.</p>{{if .Query}}<p><a href=/users>Clear the search</a></p>{{else if .Administrator}}<p><a href=/users/new>Register a user</a></p>{{end}}
</section>
{{end}}
`))

// A user has one profile form, and adding a person and editing one are the
// same form. The two used to be separate markup in the one template and had
// already drifted in what they said about the GitHub login.
// See Plans/MVP/web/forms.md#rules.
const userFields = `{{define "user-fields"}}<div class=form-grid>
{{if .New}}<label class="form-field form-field-wide">Identity <input name=name required placeholder="user@realm" value="{{.User.Name}}" aria-invalid="{{if .Form.Invalid "name"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Invalid "name"}}profile-error{{end}}"><small>The principal name used by AgentBus. It cannot be changed afterwards.</small></label>
{{else}}<input type=hidden name=name value="{{.User.Name}}">
{{end}}
<label class=form-field>Person name <input name=person_name value="{{.User.PersonName}}"><small>The name shown to people.</small></label>
<label class=form-field>Email <input type=email name=email value="{{.User.Email}}" aria-invalid="{{if .Form.Invalid "email"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Invalid "email"}}profile-error{{end}}"></label>
<label class=form-field>GitHub login <input name=github_user value="{{.User.GithubUser}}"><small>Setting or changing it attempts to import public values.</small></label>
<label class=form-field>Company <input name=company value="{{.User.GithubCompany}}"></label>
<label class=form-field>Location <input name=location value="{{.User.GithubLocation}}"></label>
<label class=form-field>Twitter/X <input name=twitter value="{{.User.GithubTwitterUsername}}" placeholder="handle"></label></div>
{{if or (.Form.Is "create") (.Form.Is "save")}}<p class=warn id=profile-error>{{.Form.Error}}</p>{{end}}{{end}}`

// userEdit is the profile form on a page of its own, the same one that adds a
// person with the person already in it.
var userEdit = template.Must(template.New("user-edit").Funcs(template.FuncMap{"titleMark": titleMark, "photoData": photoData, "profileInitial": profileInitial}).Parse(shellTitle("users", `Edit {{.User.Name}}`) + `
<p><a href="/user?name={{.User.Name}}{{with .Return}}&amp;return={{urlquery .}}{{end}}">Back to {{.User.Name}}</a></p>
<div class=page-title><h1 style="overflow-wrap:anywhere">{{with photoData .User}}<img class=profile-photo-large src="{{.}}" alt="">{{else}}<span class=profile-initial-large aria-hidden=true>{{profileInitial .User}}</span>{{end}} Edit {{.User.Name}}</h1></div>` + formErrorSummary + `
<form id=form-save class="editor-card task-card" method=post action=/user><input type=hidden name=action value=save>{{with .Return}}<input type=hidden name=return value="{{.}}">{{end}}` +
	`{{template "user-fields" .}}<div class=form-actions><button>Save profile</button></div></form>
` + userFields))

var personPage = template.Must(template.New("person").Funcs(template.FuncMap{"authorityLabel": authorityLabel, "identityLabel": identityLabel, "titleMark": titleMark, "photoData": photoData, "profileInitial": profileInitial, "registrationUpdated": registrationUpdated, "githubProfileURL": githubProfileURL, "twitterProfileURL": twitterProfileURL, "recordPath": recordPath, "userState": userState}).Parse(shellTitle("users", `{{if .New}}Add user{{else}}{{.User.Name}}{{end}}`) + `
<p><a href="{{.Return}}">Back to directory</a></p>
<div class=page-title><h1 style="overflow-wrap:anywhere">{{if .New}}{{titleMark "user"}}{{else if eq .User.Kind "user"}}{{with photoData .User}}<img class=profile-photo-large src="{{.}}" alt="">{{else}}<span class=profile-initial-large aria-hidden=true>{{profileInitial .User}}</span>{{end}}{{else}}{{titleMark "identity"}}{{end}} {{if .New}}Add user{{else}}{{.User.Name}}{{end}}</h1></div>` + formErrorSummary + `
{{if .New}}
<section class="editor-card task-card" aria-labelledby=profile-heading><h2 id=profile-heading>Profile</h2><form id=form-create method=post action=/user><input type=hidden name=action value=create><input type=hidden name=return value="{{.Return}}">{{template "user-fields" .}}<div class=form-actions><button>Save profile</button></div></form></section>
<aside class=credential-note aria-labelledby=ssh-access-heading><div class=page-title><h2 id=ssh-access-heading><span aria-hidden=true>🔑</span> SSH access</h2><button type=button class=help-button popovertarget=ssh-access-help aria-label="How to add an SSH public key" data-tooltip="Public keys are installed on the host after the profile is saved; they are not profile fields.">ⓘ</button></div><p>Public keys are added on the host after the profile is saved.</p></aside><div popover id=ssh-access-help class=context-help><h2>Add an SSH public key</h2><ul><li>The key belongs to host onboarding, not to the user profile.</li><li>An Administrator runs <code>agent-bus-admin user add &lt;user@realm&gt; &lt;key.pub&gt;</code>.</li><li>The command installs a forced SSH command and provisions the AgentBus user together.</li></ul></div>
{{else if ne .User.Kind "user"}}
<div class=detail-meta>{{with identityLabel .User .RecordKinds}}<span class=fact-pill>{{.}}</span>{{end}}<span>No user lifecycle state</span></div>
<section class="editor-card compact-card"><div class=page-title><h2>{{if eq .User.Kind "record"}}Registered name{{else}}Credential only{{end}}</h2><button type=button class=help-button popovertarget=non-user-help aria-label="About this identity" data-tooltip="This name has no user profile. Inspect any self-owned record before removing a credential.">ⓘ</button></div>{{if eq .User.Kind "record"}}<p>A self-owned record exists. <a href="{{recordPath .User.Name .RecordKinds}}">Inspect its registration and queues</a>.</p>{{range .User.Services}}<p><a href="{{recordPath . $.RecordKinds}}">{{.}}</a></p>{{end}}{{else}}<p>No registered record remains for this credential.</p>{{end}}{{if .User.CanRemove}}<form method=get action=/credential-remove><input type=hidden name=name value="{{.User.Name}}"><input type=hidden name=return value="{{.Return}}"><button class=danger-action>Review credential removal…</button></form>{{end}}</section>
<div popover id=non-user-help class=context-help><h2>About this identity</h2><ul><li>This is not a registered user, so no user lifecycle state is assigned.</li><li>Credential removal ends the current token, previous token and browser sessions.</li><li>It never deletes a user or service, and is refused if the name becomes registered first.</li></ul></div>
{{else}}
<div class=person-layout><div class=person-main>
{{if .User.CanEdit}}<section class="editor-card compact-card" aria-labelledby=profile-heading><div class=page-title><h2 id=profile-heading>Profile</h2></div><dl>{{with .User.PersonName}}<dt>Person name</dt><dd>{{.}}</dd>{{end}}{{with .User.Email}}<dt>Email</dt><dd>{{.}}</dd>{{end}}{{with .User.GithubUser}}<dt>GitHub login</dt><dd><a href="{{githubProfileURL .}}">@{{.}}</a></dd>{{end}}{{with .User.GithubCompany}}<dt>Company</dt><dd>{{.}}</dd>{{end}}{{with .User.GithubLocation}}<dt>Location</dt><dd>{{.}}</dd>{{end}}{{with .User.GithubTwitterUsername}}<dt>Twitter/X</dt><dd><a href="{{twitterProfileURL .}}">@{{.}}</a></dd>{{end}}</dl>
<p><a id=profile-edit class=editor-link href="/user/edit?name={{.User.Name}}{{with .Return}}&amp;return={{urlquery .}}{{end}}">Edit profile</a></p></section>{{else}}<section class="editor-card compact-card"><div class=page-title><h2>Profile</h2><button type=button class=help-button popovertarget=profile-source-help aria-label="About profile fields" data-tooltip="These are AgentBus User fields. GitHub may supply initial or refreshed values; authorized profile edits can change them.">ⓘ</button></div><dl>{{with .User.PersonName}}<dt>Person name</dt><dd>{{.}}</dd>{{end}}{{with .User.Email}}<dt>Email</dt><dd>{{.}}</dd>{{end}}{{with .User.GithubUser}}<dt>GitHub login</dt><dd><a href="{{githubProfileURL .}}">@{{.}}</a></dd>{{end}}{{with .User.GithubCompany}}<dt>Company</dt><dd>{{.}}</dd>{{end}}{{with .User.GithubLocation}}<dt>Location</dt><dd>{{.}}</dd>{{end}}{{with .User.GithubTwitterUsername}}<dt>Twitter/X</dt><dd><a href="{{twitterProfileURL .}}">@{{.}}</a></dd>{{end}}</dl><p class=muted>Trusted profile fields are edited by a daemon administrator.</p></section><div popover id=profile-source-help class=context-help><h2>Profile fields</h2><p>These values belong to the AgentBus User profile. Public GitHub data may fill or refresh them, but GitHub is not a separate profile on this page.</p></div>{{end}}
</div><aside class=person-sidebar>
<section class="editor-card compact-card"><h2>Identity</h2><div class=detail-meta>{{with identityLabel .User .RecordKinds}}<span class=fact-pill>{{.}}</span>{{end}}<span class=fact-pill>{{authorityLabel .User.DaemonOwner .User.Administrator}}</span></div><h2>Groups</h2><div class=choice-row>{{range .User.Groups}}<a class=group-chip href="/group?name={{.}}">{{.}}</a>{{else}}<span class=muted>No memberships</span>{{end}}</div></section>
<section class="editor-card compact-card"><div class=page-title><h2>Access</h2><button type=button class=help-button popovertarget=user-access-help aria-label="About user status" data-tooltip="Inactive blocks bus access and makes every record the user owns inactive; queued work and tokens are kept and running processes are not stopped.">ⓘ</button></div><p>Current: {{if eq (userState .User) "inactive"}}<span class="user-state user-state-inactive"><span aria-hidden=true>●</span> Inactive</span>{{else}}<span class="user-state user-state-active"><span aria-hidden=true>●</span> Active</span>{{end}}</p>{{if and (not .User.DaemonOwner) .User.CanActivate}}<details class=access-change><summary>Change</summary><div class=access-actions>{{if eq (userState .User) "inactive"}}<form method=post action=/user><input type=hidden name=name value="{{.User.Name}}"><input type=hidden name=return value="{{.Return}}"><button name=action value=active>Reactivate</button></form>{{else}}<form method=get action=/user-deactivate><input type=hidden name=name value="{{.User.Name}}"><input type=hidden name=return value="{{.Return}}"><button class=danger-action>Deactivate…</button></form>{{end}}</div></details>{{end}}</section><div popover id=user-access-help class=context-help><h2>User status</h2><ul><li>An inactive user has no bus access, and every record the user owns is inactive: hidden from listings and answered as unknown.</li><li>Queued work and tokens are kept, and running service processes are not stopped.</li><li>An Administrator may change an ordinary user; only the daemon Owner may change an Administrator. The daemon Owner stays active.</li></ul></div>
<section class="editor-card compact-card"><h2>Owned records</h2>{{range .User.Services}}<p><a href="{{recordPath . $.RecordKinds}}">{{.}}</a></p>{{else}}<p class=muted>None</p>{{end}}</section>
</aside></div>
{{end}}
` + userFields))

var userDeactivatePage = template.Must(template.New("user-deactivate").Funcs(template.FuncMap{"titleMark": titleMark}).Parse(shellTitle("users", `Confirm deactivation · {{.User.Name}}`) + `
<p><a href="/user?name={{.User.Name}}&return={{.Return}}">Back to user</a></p>
<div class=page-title><h1>{{titleMark "problem"}} Confirm deactivation</h1></div>
<section class="editor-card compact-card"><p>Deactivate <code>{{.User.Name}}</code>?</p><ul><li>Bus access stops, and every record this user owns becomes inactive: hidden from listings and answered as unknown.</li><li>Queued work and tokens are kept, and running processes are not stopped.</li></ul>
{{if .User.Administrator}}<p>Only the daemon Owner can reactivate this Administrator later.</p>{{else}}<p>An authorized Administrator or the daemon Owner can reactivate this user later.</p>{{end}}
<form method=post action=/user><input type=hidden name=name value="{{.User.Name}}"><input type=hidden name=return value="{{.Return}}"><button class=danger-action name=action value=inactive>Deactivate user</button> <a href="/user?name={{.User.Name}}&return={{.Return}}">Cancel</a></form></section>
`))

var credentialRemovePage = template.Must(template.New("credential-remove").Funcs(template.FuncMap{"titleMark": titleMark}).Parse(shellTitle("users", `Confirm credential removal · {{.User.Name}}`) + `
<p><a href="/user?name={{.User.Name}}&return={{.Return}}">Back to identity</a></p>
<div class=page-title><h1>{{titleMark "problem"}} Confirm credential removal</h1></div>
<section class="editor-card compact-card"><p>Remove the credential for <code>{{.User.Name}}</code>?</p><ul><li>The current and previous credentials stop authenticating.</li><li>Every browser session for this identity ends.</li><li>No user or registered record is removed by this action.</li></ul>
<form method=post action=/user><input type=hidden name=name value="{{.User.Name}}"><input type=hidden name=return value="{{.Return}}"><button class=danger-action name=action value=remove-credential>Remove credential</button> <a href="/user?name={{.User.Name}}&return={{.Return}}">Cancel</a></form></section>
`))
var avatarPage = template.Must(template.New("avatar").Parse(`<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32"><rect width="32" height="32" rx="6" fill="#e5eaf4"/><text x="16" y="22" text-anchor="middle" font-family="sans-serif" font-size="20" fill="#253c66">{{.}}</text></svg>`))

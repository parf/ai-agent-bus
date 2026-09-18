package main

import (
	"encoding/base64"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

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

type peopleView struct {
	adminView
	Users                                        []protocol.User
	User                                         protocol.User
	New                                          bool
	People, Other                                []protocol.User
	Query, Kind, Return, Previous, Next          string
	PeopleCount, OtherCount, Matched, Start, End int
	RecordKinds                                  map[string]string
}

func identityLabel(u protocol.User, recordKinds map[string]string) string {
	if u.Kind == protocol.DirectoryUser {
		return entityLabel(protocol.DirectoryUser)
	}
	if u.Kind == protocol.DirectoryRecord {
		kind := recordKinds[u.Name]
		if kind == "agent" || kind == "generic" || kind == protocol.KindTopic {
			return entityLabel(kind)
		}
	}
	return ""
}

func identityGlyph(u protocol.User, recordKinds map[string]string) string {
	if u.Kind == protocol.DirectoryUser {
		return entityGlyph(protocol.DirectoryUser)
	}
	if u.Kind == protocol.DirectoryRecord {
		return entityGlyph(recordKinds[u.Name])
	}
	return ""
}

func directoryReturn(raw string) string {
	u, err := url.Parse(local(raw))
	if err != nil || u.Path != "/users" {
		return "/users"
	}
	u.Fragment = ""
	return u.RequestURI()
}

func (p *peopleView) directory(r *http.Request) {
	p.Query = strings.TrimSpace(r.URL.Query().Get("q"))
	p.Kind = r.URL.Query().Get("kind")
	if p.Kind != "users" && p.Kind != "other" {
		p.Kind = ""
	}
	var matched []protocol.User
	for _, u := range p.Users {
		person := u.Kind == protocol.DirectoryUser
		if person {
			p.PeopleCount++
		} else {
			p.OtherCount++
		}
		if p.Kind == "users" && !person || p.Kind == "other" && person {
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
		if p.Kind != "" {
			q.Set("kind", p.Kind)
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
	for _, u := range matched[start:end] {
		if u.Kind == protocol.DirectoryUser {
			p.People = append(p.People, u)
		} else {
			p.Other = append(p.Other, u)
		}
	}
}

func (c *caller) userRoutes(mux *http.ServeMux, tls bool) {
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
		var records []protocol.Record
		if err := c.get(cookie(r), "/ls", &records); err != nil {
			fail(w, r, v.You, err)
			return p, false
		}
		p.RecordKinds = make(map[string]string, len(records))
		for _, record := range records {
			p.RecordKinds[record.Name] = record.Kind
		}
		return p, true
	}
	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		p, ok := load(w, r)
		if !ok {
			return
		}
		p.directory(r)
		p.SectionLinks = []viewLink{{Href: "/users", Label: "All identities", Count: p.PeopleCount + p.OtherCount, Counted: true, Current: true}}
		if p.Administrator {
			p.SectionLinks = append(p.SectionLinks, viewLink{Href: "/users/new", Label: "Register user"})
		}
		filterBase := url.Values{}
		if p.Query != "" {
			filterBase.Set("q", p.Query)
		}
		p.FilterLinks = []viewLink{
			{Href: pageURL("/users", cloneValues(filterBase)), Label: "All", Count: p.PeopleCount + p.OtherCount, Counted: true, Current: p.Kind == ""},
			{Href: queryWith("/users", filterBase, "kind", "users"), Label: "👤 Users", Count: p.PeopleCount, Counted: true, Current: p.Kind == "users"},
			{Href: queryWith("/users", filterBase, "kind", "other"), Label: "Other", Count: p.OtherCount, Counted: true, Current: p.Kind == "other"},
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
				p.User = u
				render(w, personPage, p)
				return
			}
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
		p.Form = retainedForm(action, message, r.PostForm, "name", "person_name", "email", "github_user", "return")
		if len(field) != 0 {
			p.Form.Field = field[0]
		}
		if action == "create" {
			if !p.Administrator {
				return false
			}
			p.New = true
			p.User = protocol.User{Name: r.PostForm.Get("name"), Kind: protocol.DirectoryUser, PersonName: r.PostForm.Get("person_name"), Email: r.PostForm.Get("email"), GithubUser: r.PostForm.Get("github_user")}
			renderForm(w, code, personPage, p)
			return true
		}
		if action == "save" {
			for _, u := range p.Users {
				if u.Name != r.PostForm.Get("name") {
					continue
				}
				u.PersonName, u.Email, u.GithubUser = r.PostForm.Get("person_name"), r.PostForm.Get("email"), r.PostForm.Get("github_user")
				p.User = u
				renderForm(w, code, personPage, p)
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
			u := protocol.User{Name: r.PostForm.Get("name"), PersonName: r.PostForm.Get("person_name"), Email: r.PostForm.Get("email"), GithubUser: r.PostForm.Get("github_user")}
			err = c.post(cookie(r), "/user", struct {
				protocol.User
				Create bool
			}{u, r.PostForm.Get("action") == "create"})
		case "refresh-github":
			err = c.post(cookie(r), "/user/github-refresh", map[string]string{"name": r.PostForm.Get("name")})
		case "active", "paused", "banned":
			err = c.post(cookie(r), "/user/state", map[string]string{"name": r.PostForm.Get("name"), "state": r.PostForm.Get("action")})
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
								p.User, p.Form = u, formState{Action: action, Target: action, Error: message}
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

var peoplePage = template.Must(template.New("people").Funcs(template.FuncMap{"identityGlyph": identityGlyph, "identityLabel": identityLabel, "titleMark": titleMark, "photoData": photoData, "profileInitial": profileInitial, "number": number}).Parse(shell("users", "Users") + `
<div class=page-title><h1>{{titleMark "users"}} Users and other identities</h1><button type=button class=help-button popovertarget=identity-types-help aria-label="About identity types">ⓘ</button></div>
<div popover id=identity-types-help class=context-help><h2>About identity types</h2><ul>
<li>Registered users have a profile.</li>
<li>Registered names have a record of their own and no user profile.</li>
<li>A credential with no registered name has neither profile nor registered record.</li>
<li>Those counts are computed by this page over identities visible to you before search and kind filters. They are not figures the daemon reported and not a count of the credential store.</li>
<li>Nothing is inferred from how a name is spelled.</li>
</ul></div>
<nav class=section-nav aria-label="User views">{{range .SectionLinks}}{{if .Current}}<a href="{{.Href}}" aria-current=true>{{else}}<a href="{{.Href}}">{{end}}{{.Label}}{{if .Counted}} ({{number .Count}}){{end}}</a>{{end}}</nav>
<p>{{number .PeopleCount}} registered users · {{number .OtherCount}} other identities visible to you.</p>
<form method=get action=/users>
<label>Search <input type=search name=q value="{{.Query}}" placeholder="Name, identity, email or GitHub login"></label>
{{with .Kind}}<input type=hidden name=kind value="{{.}}">{{end}}<button>Search</button> <a href=/users>Clear filters</a></form>
<nav class=filter-nav aria-label="Identity type filter">Show: {{range .FilterLinks}}{{if .Current}}<a href="{{.Href}}" aria-current=true>{{else}}<a href="{{.Href}}">{{end}}{{.Label}} ({{number .Count}})</a>{{end}}</nav>
<p>Showing {{number .Start}}–{{number .End}} of {{number .Matched}} matching identities.</p>
<section aria-labelledby=people-heading><h2 id=people-heading>Registered users</h2>
<table><thead><tr><th scope=col>Person / identity</th><th scope=col>Authority</th><th scope=col>State</th></tr></thead><tbody>
{{range .People}}<tr><td><span class=identity-with-photo>{{with photoData .}}<img class=profile-photo src="{{.}}" alt="">{{else}}<span class=profile-initial aria-hidden=true>{{profileInitial .}}</span>{{end}}<span>{{with .PersonName}}<strong>{{.}}</strong><br>{{end}}{{if identityGlyph . $.RecordKinds}}<span role=img aria-label="{{identityLabel . $.RecordKinds}}">{{identityGlyph . $.RecordKinds}}</span> {{end}}<a href="/user?name={{.Name}}&return={{$.Return}}"><code>{{.Name}}</code></a>{{with .GithubCompany}}<br><span class=muted>{{.}}</span>{{end}}</span></span></td><td>{{if .DaemonOwner}}Daemon owner{{else if .Administrator}}Daemon administrator{{else}}User{{end}}</td><td>{{.State}}</td></tr>
{{else}}<tr><td colspan=3>No registered users on this page.</td></tr>{{end}}</tbody></table></section>
<section aria-labelledby=other-heading><h2 id=other-heading>Other identities — review and cleanup</h2>
<table><thead><tr><th scope=col>Identity</th><th scope=col>What it is</th><th scope=col>Why it is here / next step</th></tr></thead><tbody>
{{range .Other}}<tr><td>{{if identityGlyph . $.RecordKinds}}<span role=img aria-label="{{identityLabel . $.RecordKinds}}">{{identityGlyph . $.RecordKinds}}</span> {{end}}<a href="/user?name={{.Name}}&return={{$.Return}}"><code>{{.Name}}</code></a></td>
<td>{{if eq .Kind "record"}}Registered name{{else}}Credential with no registered name{{end}}</td>
<td>{{if eq .Kind "record"}}A self-owned record, not a user profile. <a href="/service?name={{.Name}}">Inspect the record</a> before deciding whether it is needed.
{{else}}No user profile and no registered record. {{if .CanRemove}}<a href="/user?name={{.Name}}&return={{$.Return}}">Review credential removal</a>{{else}}An authorized administrator can review removal.{{end}}{{end}}</td></tr>
{{else}}<tr><td colspan=3>No other identities on this page.</td></tr>{{end}}</tbody></table></section>
<nav aria-label="Directory pages">{{with .Previous}}<a href="{{.}}">Previous page</a>{{end}} {{with .Next}}<a href="{{.}}">Next page</a>{{end}}</nav>
`))
var personPage = template.Must(template.New("person").Funcs(template.FuncMap{"identityLabel": identityLabel, "titleMark": titleMark, "photoData": photoData, "profileInitial": profileInitial, "registrationUpdated": registrationUpdated, "githubProfileURL": githubProfileURL, "twitterProfileURL": twitterProfileURL}).Parse(shell("users", "Identity details") + `
<p><a href="{{.Return}}">Back to directory</a></p>
<div class=page-title><h1 style="overflow-wrap:anywhere">{{if .New}}{{titleMark "user"}}{{else if eq .User.Kind "user"}}{{with photoData .User}}<img class=profile-photo-large src="{{.}}" alt="">{{else}}<span class=profile-initial-large aria-hidden=true>{{profileInitial .User}}</span>{{end}}{{else}}{{titleMark "identity"}}{{end}} {{if .New}}Add user{{else}}{{.User.Name}}{{end}}</h1></div>` + formErrorSummary + `
{{if .New}}
<section class="editor-card task-card" aria-labelledby=profile-heading><h2 id=profile-heading>Profile</h2><form id=form-create method=post action=/user><input type=hidden name=return value="{{.Return}}"><div class=form-grid>
{{if .New}}<label class="form-field form-field-wide">Identity <input name=name required placeholder="user@realm" value="{{.User.Name}}" aria-invalid="{{if .Form.Invalid "name"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Invalid "name"}}profile-error{{end}}"><small>The principal name used by AgentBus.</small></label><input type=hidden name=action value=create>{{else}}<input type=hidden name=name value="{{.User.Name}}"><input type=hidden name=action value=save>{{end}}
<label class=form-field>Person name <input name=person_name value="{{.User.PersonName}}"><small>The name shown to people.</small></label>
<label class=form-field>Email <input type=email name=email value="{{.User.Email}}" aria-invalid="{{if .Form.Invalid "email"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Invalid "email"}}profile-error{{end}}"><small>A public GitHub email fills this only when it is blank.</small></label>
<label class=form-field>GitHub login <input name=github_user value="{{.User.GithubUser}}"><small>Setting or changing it attempts to refresh public profile data.</small></label></div>{{if or (.Form.Is "create") (.Form.Is "save")}}<p class=warn id=profile-error>{{.Form.Error}}</p>{{end}}<div class=form-actions><button>Save profile</button></div></form></section>
<aside class=credential-note aria-labelledby=ssh-access-heading><div class=page-title><h2 id=ssh-access-heading><span aria-hidden=true>🔑</span> SSH access</h2><button type=button class=help-button popovertarget=ssh-access-help aria-label="How to add an SSH public key" data-tooltip="Public keys are installed on the host after the profile is saved; they are not profile fields.">ⓘ</button></div><p>Public keys are added on the host after the profile is saved.</p></aside><div popover id=ssh-access-help class=context-help><h2>Add an SSH public key</h2><ul><li>The key belongs to host onboarding, not to the user profile.</li><li>An Administrator runs <code>agent-bus-admin user add &lt;user@realm&gt; &lt;key.pub&gt;</code>.</li><li>The command installs a forced SSH command and provisions the AgentBus user together.</li></ul></div>
{{else if ne .User.Kind "user"}}
<div class=detail-meta>{{with identityLabel .User .RecordKinds}}<span class=fact-pill>{{.}}</span>{{end}}<span>No user lifecycle state</span></div>
<section class="editor-card compact-card"><div class=page-title><h2>{{if eq .User.Kind "record"}}Registered name{{else}}Credential only{{end}}</h2><button type=button class=help-button popovertarget=non-user-help aria-label="About this identity" data-tooltip="This name has no user profile. Inspect any self-owned record before removing a credential.">ⓘ</button></div>{{if eq .User.Kind "record"}}<p>A self-owned record exists. <a href="/service?name={{.User.Name}}">Inspect its registration and queues</a>.</p>{{range .User.Services}}<p><a href="/service?name={{.}}">{{.}}</a></p>{{end}}{{else}}<p>No registered record remains for this credential.</p>{{end}}{{if .User.CanRemove}}<form method=post action=/user><input type=hidden name=name value="{{.User.Name}}"><input type=hidden name=return value="{{.Return}}"><button class=danger-action name=action value=remove-credential>Remove credential for {{.User.Name}}</button></form>{{end}}</section>
<div popover id=non-user-help class=context-help><h2>About this identity</h2><ul><li>This is not a registered user, so no user lifecycle state is assigned.</li><li>Credential removal ends the current token, previous token and browser sessions.</li><li>It never deletes a user or service, and is refused if the name becomes registered first.</li></ul></div>
{{else}}
<div class=person-layout><div class=person-main>
{{if .User.CanEdit}}<section class=editor-card aria-labelledby=profile-heading><h2 id=profile-heading>Profile</h2><form id=form-save method=post action=/user><input type=hidden name=return value="{{.Return}}"><input type=hidden name=name value="{{.User.Name}}"><input type=hidden name=action value=save><div class=form-grid>
<label class=form-field>Person name <input name=person_name value="{{.User.PersonName}}"><small>The name shown to people.</small></label><label class=form-field>Email <input type=email name=email value="{{.User.Email}}" aria-invalid="{{if .Form.Invalid "email"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Invalid "email"}}profile-error{{end}}"><small>A public GitHub email fills this only when blank.</small></label><label class=form-field>GitHub login <input name=github_user value="{{.User.GithubUser}}"><small>Changing it attempts to refresh public profile data.</small></label></div>{{if .Form.Is "save"}}<p class=warn id=profile-error>{{.Form.Error}}</p>{{end}}<div class=form-actions><button>Save profile</button></div></form></section>{{else}}<section class="editor-card compact-card"><h2>Profile</h2><dl>{{with .User.PersonName}}<dt>Person name</dt><dd>{{.}}</dd>{{end}}{{with .User.Email}}<dt>Email</dt><dd>{{.}}</dd>{{end}}{{with .User.GithubUser}}<dt>GitHub login</dt><dd>{{.}}</dd>{{end}}</dl><p class=muted>Trusted profile fields are edited by a daemon administrator.</p></section>{{end}}
{{if .User.GithubUser}}<section class="editor-card compact-card"><div class=page-title><h2>GitHub profile</h2><button type=button class=help-button popovertarget=github-profile-help aria-label="About imported GitHub data" data-tooltip="Public profile data is optional. It is imported when available after the login is set, changed, or explicitly refreshed.">ⓘ</button></div><dl><dt>GitHub</dt><dd><a href="{{githubProfileURL .User.GithubUser}}">@{{.User.GithubUser}}</a></dd>{{with .User.GithubCompany}}<dt>Company</dt><dd>{{.}}</dd>{{end}}{{with .User.GithubLocation}}<dt>Location</dt><dd>{{.}}</dd>{{end}}{{with .User.GithubTwitterUsername}}<dt>Twitter/X</dt><dd><a href="{{twitterProfileURL .}}">@{{.}}</a></dd>{{end}}{{with .User.GithubGravatarID}}<dt>Gravatar ID</dt><dd><code>{{.}}</code></dd>{{end}}{{with .User.PhotoSource}}<dt>Photo source</dt><dd>{{if eq . "github"}}GitHub{{else}}Gravatar{{end}}</dd>{{end}}{{if not .User.GithubProfileAt.IsZero}}<dt>Fetched</dt><dd><time datetime="{{.User.GithubProfileAt.Format "2006-01-02T15:04:05Z07:00"}}">{{registrationUpdated .User.GithubProfileAt}}</time></dd>{{end}}</dl>{{if .User.GithubProfileAt.IsZero}}<p class=muted>Public profile data has not been fetched yet.</p>{{end}}{{if .User.CanEdit}}<form id=form-refresh-github method=post action=/user><input type=hidden name=name value="{{.User.Name}}"><input type=hidden name=return value="{{.Return}}"><button name=action value=refresh-github>Refresh GitHub profile</button>{{if .Form.Is "refresh-github"}}<p class=warn>{{.Form.Error}}</p>{{end}}</form>{{end}}</section><div popover id=github-profile-help class=context-help><h2>Imported GitHub data</h2><p>Company, location, Twitter/X, photo source and fetched time come from the trusted daemon adapter when it is available. Page rendering never fetches GitHub.</p></div>{{end}}
</div><aside class=person-sidebar>
<section class="editor-card compact-card"><h2>Identity</h2><p>State: {{.User.State}}</p><div class=detail-meta>{{with identityLabel .User .RecordKinds}}<span class=fact-pill>{{.}}</span>{{end}}<span class=fact-pill>{{if .User.DaemonOwner}}Daemon owner{{else if .User.Administrator}}Daemon administrator{{else}}User{{end}}</span></div><h2>Groups</h2><div class=choice-row>{{range .User.Groups}}<span class=group-chip>{{.}}</span>{{else}}<span class=muted>No memberships</span>{{end}}</div></section>
{{if not .User.DaemonOwner}}<section class="editor-card compact-card"><div class=page-title><h2>Access</h2><button type=button class=help-button popovertarget=user-access-help aria-label="About user access states" data-tooltip="Pause and ban block bus access and new inbox deliveries; queued work stays and running processes are not stopped.">ⓘ</button></div><form class=access-actions method=post action=/user><input type=hidden name=name value="{{.User.Name}}"><input type=hidden name=return value="{{.Return}}">{{if .User.CanActivate}}<button name=action value=active>Activate</button>{{end}}<button name=action value=paused>Pause</button><button class=danger-action name=action value=banned>Ban</button></form></section><div popover id=user-access-help class=context-help><h2>User access states</h2><ul><li>Pause and ban block bus access and new deliveries to this user's inbox.</li><li>Queued work is retained and running service processes are not stopped.</li><li>Administrators may lift a ban on an ordinary user; the daemon Owner controls protected authority levels.</li></ul></div>{{end}}
<section class="editor-card compact-card"><h2>Owned services</h2>{{range .User.Services}}<p><a href="/service?name={{.}}">{{.}}</a></p>{{else}}<p class=muted>None</p>{{end}}</section>
</aside></div>
{{end}}
`))
var avatarPage = template.Must(template.New("avatar").Parse(`<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32"><rect width="32" height="32" rx="6" fill="#e5eaf4"/><text x="16" y="22" text-anchor="middle" font-family="sans-serif" font-size="20" fill="#253c66">{{.}}</text></svg>`))

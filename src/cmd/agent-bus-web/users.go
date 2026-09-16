package main

import (
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

type peopleView struct {
	adminView
	Users                                        []protocol.User
	User                                         protocol.User
	New                                          bool
	People, Other                                []protocol.User
	Query, Kind, Return, Previous, Next          string
	PeopleCount, OtherCount, Matched, Start, End int
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
		if !strings.Contains(strings.ToLower(u.Name+" "+u.PersonName+" "+u.Email+" "+u.GithubUser), strings.ToLower(p.Query)) {
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
		return p, true
	}
	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		p, ok := load(w, r)
		if !ok {
			return
		}
		p.directory(r)
		render(w, peoplePage, p)
	})
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		p, ok := load(w, r)
		if !ok {
			return
		}
		name := r.URL.Query().Get("name")
		p.Return = directoryReturn(r.URL.Query().Get("return"))
		if name == "" && p.Administrator {
			p.New = true
			render(w, personPage, p)
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
				label := u.PersonName
				if label == "" {
					label = u.Name
				}
				initial, _ := utf8.DecodeRuneInString(label)
				w.Header().Set("Content-Type", "image/svg+xml")
				render(w, avatarPage, strings.ToUpper(string(initial)))
				return
			}
		}
		http.Error(w, "no such user", 404)
	})
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
			http.Error(w, "invalid form", 400)
			return
		}
		var err error
		switch r.PostForm.Get("action") {
		case "remove-credential":
			err = c.post(cookie(r), "/identity/remove", map[string]string{"name": r.PostForm.Get("name")})
		case "save", "create":
			u := protocol.User{Name: r.PostForm.Get("name"), PersonName: r.PostForm.Get("person_name"), Email: r.PostForm.Get("email"), GithubUser: r.PostForm.Get("github_user")}
			err = c.post(cookie(r), "/user", struct {
				protocol.User
				Create bool
			}{u, r.PostForm.Get("action") == "create"})
		case "active", "paused", "banned":
			err = c.post(cookie(r), "/user/state", map[string]string{"name": r.PostForm.Get("name"), "state": r.PostForm.Get("action")})
		default:
			http.Error(w, "unknown action", 400)
			return
		}
		if err != nil {
			fail(w, r, v.You, err)
			return
		}
		http.Redirect(w, r, directoryReturn(r.PostForm.Get("return")), http.StatusSeeOther)
	})
}

var peoplePage = template.Must(template.New("people").Parse(shell("users", "Users") + `
<h1>Users and other identities</h1>
<p>{{.PeopleCount}} registered users · {{.OtherCount}} other identities visible to you.</p>
<p>Credentials and registered names are shown separately from user profiles, so unused identities remain visible for review.</p>
{{if .Administrator}}<p><a href=/user>Add user</a></p>{{end}}
<form method=get action=/users>
<label>Search <input type=search name=q value="{{.Query}}" placeholder="Name, identity, email or GitHub login"></label>
<label>Show <select name=kind><option value="">All identities</option><option value=users {{if eq .Kind "users"}}selected{{end}}>Registered users</option><option value=other {{if eq .Kind "other"}}selected{{end}}>Other identities</option></select></label>
<button>Apply</button> <a href=/users>Clear filters</a></form>
<p>Showing {{.Start}}–{{.End}} of {{.Matched}} matching identities.</p>
<section aria-labelledby=people-heading><h2 id=people-heading>Registered users</h2>
<table><thead><tr><th scope=col>Person / identity</th><th scope=col>Authority</th><th scope=col>State</th></tr></thead><tbody>
{{range .People}}<tr><td>{{with .PersonName}}<strong>{{.}}</strong><br>{{end}}<a href="/user?name={{.Name}}&return={{$.Return}}"><code>{{.Name}}</code></a></td><td>{{if .DaemonOwner}}Daemon owner{{else if .Maintainer}}Daemon maintainer{{else}}User{{end}}</td><td>{{.State}}</td></tr>
{{else}}<tr><td colspan=3>No registered users on this page.</td></tr>{{end}}</tbody></table></section>
<section aria-labelledby=other-heading><h2 id=other-heading>Other identities — review and cleanup</h2>
<table><thead><tr><th scope=col>Identity</th><th scope=col>What it is</th><th scope=col>Why it is here / next step</th></tr></thead><tbody>
{{range .Other}}<tr><td><a href="/user?name={{.Name}}&return={{$.Return}}"><code>{{.Name}}</code></a></td>
<td>{{if eq .Kind "record"}}Registered name{{else}}Credential without a registered name{{end}}</td>
<td>{{if eq .Kind "record"}}A self-owned record, not a user profile. <a href="/service?name={{.Name}}">Inspect the record</a> before deciding whether it is needed.
{{else if .Services}}No user profile or record of its own. It still owns services; resolve those services first. Credential removal is held until orphan-service cleanup is available.
{{else}}No user profile and no registered record; it owns no services. {{if .CanRemove}}<a href="/user?name={{.Name}}&return={{$.Return}}">Review credential removal</a>{{else}}An authorized administrator can review removal.{{end}}{{end}}</td></tr>
{{else}}<tr><td colspan=3>No other identities on this page.</td></tr>{{end}}</tbody></table></section>
<nav aria-label="Directory pages">{{with .Previous}}<a href="{{.}}">Previous page</a>{{end}} {{with .Next}}<a href="{{.}}">Next page</a>{{end}}</nav>
</main>`))
var personPage = template.Must(template.New("person").Parse(shell("users", "Identity details") + `
<p><a href="{{.Return}}">Back to directory</a></p>
<h1 style="overflow-wrap:anywhere">{{if .New}}Add user{{else}}{{.User.Name}}{{end}}</h1>
{{if and (not .New) (ne .User.Kind "user")}}
<h2>{{if eq .User.Kind "record"}}Registered name{{else}}Credential-only identity{{end}}</h2>
<p>This is not a registered user. No user lifecycle state has been assigned.</p>
{{if eq .User.Kind "record"}}<p>A self-owned record exists. <a href="/service?name={{.User.Name}}">Inspect its registration and queues</a> before removing the record.</p>
{{else if .User.Services}}<p>This name owns services but has no user profile or record of its own. Resolve the services below first; credential removal is held until orphan-service cleanup is available.</p>
{{else}}<p>No user profile, registered record or owned service remains for this name. Review and remove the unused credential.</p>{{end}}
{{if .User.CanRemove}}<h2>Remove unused credential</h2><p>Removal ends access through this name’s current token, previous token and browser sessions. It does not delete a user or service. If the name becomes registered before submission, removal will be refused.</p>
<form method=post action=/user><input type=hidden name=name value="{{.User.Name}}"><input type=hidden name=return value="{{.Return}}"><button name=action value=remove-credential style="max-width:100%;overflow-wrap:anywhere">Remove credential for {{.User.Name}}</button></form>{{end}}
{{else}}
{{if not .New}}<p>State: {{.User.State}} · {{if .User.DaemonOwner}}Daemon owner{{else if .User.Maintainer}}Daemon maintainer{{else}}User{{end}}</p>
<h2>Groups</h2>{{range .User.Groups}}<p>{{.}}</p>{{else}}<p>No group memberships</p>{{end}}{{end}}
{{if or .New .User.CanEdit}}<h2>Profile</h2><form method=post action=/user><input type=hidden name=return value="{{.Return}}">
{{if .New}}<label>Identity <input name=name required placeholder="user@realm"></label><input type=hidden name=action value=create>{{else}}<input type=hidden name=name value="{{.User.Name}}"><input type=hidden name=action value=save>{{end}}
<p><label>Person name <input name=person_name value="{{.User.PersonName}}"></label></p>
<p><label>Email <input type=email name=email value="{{.User.Email}}"></label></p>
<p><label>GitHub login <input name=github_user value="{{.User.GithubUser}}"></label></p><button>Save profile</button></form>
{{if and (not .New) (not .User.DaemonOwner)}}<h2>Access</h2><p>Pause and ban block this user's bus access and new deliveries to their inbox. Queued work is retained. Their running service processes are not stopped. Only the daemon owner can lift a ban.</p><form method=post action=/user><input type=hidden name=name value="{{.User.Name}}"><input type=hidden name=return value="{{.Return}}">{{if .User.CanActivate}}<button name=action value=active>Activate</button>{{end}}<button name=action value=paused>Pause</button><button name=action value=banned>Ban</button></form>{{end}}
{{else}}<p>{{.User.PersonName}}</p><p>{{.User.Email}}</p><p>{{.User.GithubUser}}</p><p>Trusted profile fields are edited by a daemon administrator.</p>{{end}}
{{end}}
{{if not .New}}<h2>Owned services</h2>{{range .User.Services}}<p><a href="/service?name={{.}}">{{.}}</a></p>{{else}}<p>No owned services</p>{{end}}{{end}}
</main>`))
var avatarPage = template.Must(template.New("avatar").Parse(`<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32"><rect width="32" height="32" rx="6" fill="#e5eaf4"/><text x="16" y="22" text-anchor="middle" font-family="sans-serif" font-size="20" fill="#253c66">{{.}}</text></svg>`))

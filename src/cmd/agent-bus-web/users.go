package main

import (
	"html/template"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

type peopleView struct {
	adminView
	Users []protocol.User
	User  protocol.User
	New   bool
}

func (c *caller) userRoutes(mux *http.ServeMux, tls bool) {
	load := func(w http.ResponseWriter, r *http.Request) (peopleView, bool) {
		v, ok := c.signedIn(w, r)
		p := peopleView{adminView: v}
		if !ok {
			return p, false
		}
		if err := c.get(cookie(r), "/users", &p.Users); err != nil {
			adminError(w, err)
			return p, false
		}
		return p, true
	}
	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		p, ok := load(w, r)
		if !ok {
			return
		}
		render(w, peoplePage, p)
	})
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		p, ok := load(w, r)
		if !ok {
			return
		}
		name := r.URL.Query().Get("name")
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
		http.Error(w, "no such user", 404)
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
		if _, ok := c.signedIn(w, r); !ok {
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", 400)
			return
		}
		var err error
		switch r.PostForm.Get("action") {
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
			adminError(w, err)
			return
		}
		http.Redirect(w, r, "/users", http.StatusSeeOther)
	})
}

var peoplePage = template.Must(template.New("people").Parse(shell("users", "Users") + `
<h1>Users</h1>{{if .Administrator}}<p><a href=/user>Add user</a></p>{{end}}
<table><tr><th>Person<th>Identity<th>Authority<th>State</tr>{{range .Users}}<tr><td><img src="/avatar?name={{.Name}}" width=32 height=32 alt=""> {{.PersonName}}<td><a href="/user?name={{.Name}}">{{.Name}}</a><td>{{if .DaemonOwner}}Daemon owner{{else if .Maintainer}}Daemon maintainer{{else}}User{{end}}<td>{{.State}}</tr>{{else}}<tr><td colspan=4>No users visible</tr>{{end}}</table>`))
var personPage = template.Must(template.New("person").Parse(shell("users", "Person") + `
<h1>{{if .New}}Add user{{else}}{{.User.Name}}{{end}}</h1>
{{if not .New}}<p>State: {{.User.State}} · {{if .User.DaemonOwner}}Daemon owner{{else if .User.Maintainer}}Daemon maintainer{{else}}User{{end}}</p>
<h2>Groups</h2>{{range .User.Groups}}<p>{{.}}</p>{{else}}<p>No group memberships</p>{{end}}
<h2>Owned services</h2>{{range .User.Services}}<p><a href="/service?name={{.}}">{{.}}</a></p>{{else}}<p>No owned services</p>{{end}}{{end}}
{{if or .New .User.CanEdit}}<h2>Profile</h2><form method=post action=/user>
{{if .New}}<label>Identity <input name=name required placeholder="user@realm"></label><input type=hidden name=action value=create>{{else}}<input type=hidden name=name value="{{.User.Name}}"><input type=hidden name=action value=save>{{end}}
<p><label>Person name <input name=person_name value="{{.User.PersonName}}"></label></p>
<p><label>Email <input type=email name=email value="{{.User.Email}}"></label></p>
<p><label>GitHub login <input name=github_user value="{{.User.GithubUser}}"></label></p><button>Save profile</button></form>
{{if and (not .New) (not .User.DaemonOwner)}}<h2>Access</h2><p>Pause and ban block this user's bus access and new deliveries to their inbox. Queued work is retained. Their running service processes are not stopped. Only the daemon owner can lift a ban.</p><form method=post action=/user><input type=hidden name=name value="{{.User.Name}}">{{if .User.CanActivate}}<button name=action value=active>Activate</button>{{end}}<button name=action value=paused>Pause</button><button name=action value=banned>Ban</button></form>{{end}}
{{else}}<p>{{.User.PersonName}}</p><p>{{.User.Email}}</p><p>{{.User.GithubUser}}</p><p>Trusted profile fields are edited by a daemon administrator.</p>{{end}}`))
var avatarPage = template.Must(template.New("avatar").Parse(`<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32"><rect width="32" height="32" rx="6" fill="#e5eaf4"/><text x="16" y="22" text-anchor="middle" font-family="sans-serif" font-size="20" fill="#253c66">{{.}}</text></svg>`))

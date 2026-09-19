package main

import (
	"html/template"
	"net/http"

	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

type accountView struct {
	adminView
	Profile        *protocol.User
	IdentityRecord *protocol.Record
	Owned          []protocol.Record
	Names          []auth.Held
	NoNames        string
	RecordKinds    map[string]string
	ForeignOwner   bool
}

func credentialKind(kind string) string {
	switch kind {
	case "person":
		return "User"
	case "unregistered":
		return "Unregistered"
	default:
		return entityLabel(kind)
	}
}

func (c *caller) accountRoute(mux *http.ServeMux) {
	mux.HandleFunc("GET /account", func(w http.ResponseWriter, r *http.Request) {
		base, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		v := accountView{adminView: base, RecordKinds: map[string]string{}}
		var users []protocol.User
		if err := c.get(cookie(r), "/users", &users); err != nil {
			fail(w, r, base.You, err)
			return
		}
		for i := range users {
			if users[i].Name == base.You {
				v.Profile = &users[i]
				break
			}
		}
		var records []protocol.Record
		if err := c.get(cookie(r), "/ls", &records); err != nil {
			fail(w, r, base.You, err)
			return
		}
		for i := range records {
			record := records[i]
			v.RecordKinds[record.Name] = record.Kind
			if record.Name == base.You {
				v.IdentityRecord = &records[i]
			}
			if record.Owner == base.You && record.Name != base.You {
				v.Owned = append(v.Owned, record)
			}
		}
		if err := c.get(cookie(r), "/names", &v.Names); err != nil {
			v.NoNames = sectionProblem("held credentials", err)
		}
		for _, held := range v.Names {
			if held.Owner != "" && held.Owner != base.You {
				v.ForeignOwner = true
			}
		}
		render(w, accountPage, v)
	})
}

var accountPage = template.Must(template.New("account").Funcs(template.FuncMap{"authorityLabel": authorityLabel, 
	"credentialKind": credentialKind,
	"entityLabel":    entityLabel,
	"profileInitial": profileInitial,
	"photoData":      photoData,
	"recordPath":     recordPath,
	"titleMark":      titleMark,
}).Parse(shell("account", "Account") + `
<div class=page-title><h1>{{titleMark "identity"}} Account</h1><button type=button class=help-button popovertarget=account-help aria-label="About this account" data-tooltip="Your identity, owned records and credential fingerprints. A fingerprint identifies a credential without revealing it.">ⓘ</button></div>
<div popover id=account-help class=context-help><h2>Account scope</h2><ul><li>This page contains only facts the daemon returned for the signed-in identity.</li><li>A fingerprint identifies a credential without revealing the credential.</li><li>Rotation replaces the current credential; the replaced credential remains valid until the next rotation.</li></ul></div>
<div class=account-summary>
<section class="editor-card compact-card"><h2>Identity</h2>
{{with .Profile}}<div class=identity-with-photo>{{with photoData .}}<img class=profile-photo-large src="{{.}}" alt="">{{else}}<span class=profile-initial-large aria-hidden=true>{{profileInitial .}}</span>{{end}}<div><strong>{{with .PersonName}}{{.}}{{else}}{{.Name}}{{end}}</strong><br><code>{{.Name}}</code></div></div>
<div class=detail-meta><span class=fact-pill>👤 User</span><span class=fact-pill>{{authorityLabel .DaemonOwner .Administrator}}</span><span class=fact-pill>{{.State}}</span></div>
{{with .Groups}}<h3>Groups</h3><div class=choice-row>{{range .}}<a class=group-chip href="/group?name={{.}}">{{.}}</a>{{end}}</div>{{end}}
{{else}}{{with .IdentityRecord}}<p><strong>{{entityLabel .Kind}}</strong> <code>{{.Name}}</code></p><p class=muted>This identity has a registered record and no person profile visible here.</p>{{else}}<p><code>{{$.You}}</code></p><p class=muted>No person profile or caller-visible identity record was returned. The authenticated daemon status still identifies this account.</p>{{end}}{{end}}
</section>
<section class="editor-card compact-card"><h2>Owned records</h2>{{range .Owned}}<p><a href="{{recordPath .Name $.RecordKinds}}"><code>{{.Name}}</code></a> <span class=fact-pill>{{entityLabel .Kind}}</span></p>{{else}}<p class=muted>No records owned by this identity are visible.</p>{{end}}</section>
</div>
<section class=dashboard-section><h2>Credentials</h2>
{{if .NoNames}}<p class=muted>{{.NoNames}}</p>{{else}}<table><thead><tr><th scope=col>Name</th><th scope=col>Kind</th>{{if .ForeignOwner}}<th scope=col>Owner</th>{{end}}<th scope=col>Fingerprint</th><th scope=col>Issued</th><th scope=col>Last used</th></tr></thead><tbody>{{range .Names}}<tr><td><code>{{.Name}}</code></td><td>{{credentialKind .Kind}}</td>{{if $.ForeignOwner}}<td>{{if ne .Owner $.You}}<code>{{.Owner}}</code>{{else}}<span class=muted>&mdash;</span>{{end}}</td>{{end}}<td><code>{{.Fingerprint}}</code></td><td>{{if .Issued.IsZero}}<span class=muted>unavailable</span>{{else}}{{.Issued.Format "2006-01-02 15:04"}}{{end}}</td><td>{{if .Used.IsZero}}<span class=muted>not this run</span>{{else}}{{.Used.Format "2006-01-02 15:04"}}{{end}}</td></tr>{{else}}<tr><td colspan={{if .ForeignOwner}}6{{else}}5{{end}} class=muted>You hold no credential.</td></tr>{{end}}</tbody></table>{{end}}
<div class=credential-note><strong>Rotate your identity credential</strong><p><code>agent-bus-token {{.You}} --rotate</code></p><p class=muted>The replaced credential remains valid until the next rotation.</p></div></section>
`))

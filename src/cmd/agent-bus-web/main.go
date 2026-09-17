// agent-bus-web: the dashboard. A separate process that holds no state and
// speaks the API like any other client, because that is what it is in the
// design (docs/11-processes.md#the-processes) — building it inside the
// daemon would mean building it twice.
//
// It shows the envelope and nothing else: bodies are struck out in the bus,
// before they could reach this. See docs/05-discovery.md#dashboard.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	dash "github.com/parf/ai-agent-bus/internal/dashboard"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/version"
)

func main() {
	if version.Print() {
		return
	}
	addr := flag.String("addr", env("AGENT_BUS_WEB_ADDR", dash.Addr), "where the dashboard listens")
	certF := flag.String("cert", env("AGENT_BUS_WEB_CERT", ""), "TLS certificate; without one the dashboard is plain HTTP on loopback")
	keyF := flag.String("key", env("AGENT_BUS_WEB_KEY", ""), "the certificate's private key")
	flag.Parse()

	client, base := api.Dial(os.Getenv("AGENT_BUS_ADDR"))
	// No credential of its own, deliberately. The child reaches the bus over
	// the shared socket, where a caller has to say who it is, so every page
	// is rendered with the credential of whoever asked for it and the child
	// has no authority to lend out. See docs/05-discovery.md#signing-in.
	bus := &caller{client: client, base: base}
	tls := have(*certF) && have(*keyF)

	handler := dashboard(bus, tls)

	// Asked for and not there is a refusal, not a downgrade. A dashboard on
	// plain HTTP is a different thing from one on HTTPS, and somebody who
	// asked for a certificate would be told by a log line nobody reads while
	// the page they open is not encrypted. Refusing is the only answer they
	// cannot miss. See docs/05-discovery.md#where-it-listens.
	if !tls && (*certF != "" || *keyF != "") {
		log.Fatalf("asked for a certificate at %q with a key at %q, and HTTPS needs both: refusing to start rather than serve plain HTTP", *certF, *keyF)
	}
	l, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	scheme := "http"
	if tls {
		scheme = "https"
	}
	log.Printf("agent-bus-web on %s://%s", scheme, l.Addr())
	if tls {
		log.Fatal(srv.ServeTLS(l, *certF, *keyF))
	}
	log.Fatal(srv.Serve(l))
}

// dashboard has no application state or credential of its own.
func dashboard(bus *caller, tls bool) http.Handler {
	mux := http.NewServeMux()
	// Health still answers nothing; the node deliberately publishes its
	// identity separately.
	// See docs/05-discovery.md#rules-it-is-built-to.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		cred := cookie(r)
		// Whose page this is comes from the bus, not from the child: the
		// cookie is checked by being used. An expired or forged one is a
		// visitor who is not signed in, which is the anonymous page.

		if cred == "" {
			signIn(w, r, "")
			return
		}
		// And a bus that is not answering is a different fact, which this
		// used to swallow: every failed status request became the anonymous
		// page, so a stopped daemon read as "you are not signed in" and the
		// one recovery offered was the one that could not work.
		// See Plans/MVP/done/web-review.md W12.
		node, err := bus.status(r)
		if err != nil {
			fail(w, r, "", err)
			return
		}
		v := view{pageInfo: requestInfo(r), You: node.You, At: time.Now().Format("2006-01-02 15:04:05"), Status: node.Status, Refusals: refusals(node.Refused)}
		if err := bus.get(cred, "/ls", &v.Records); err != nil {
			fail(w, r, node.You, err)
			return
		}
		// Three views off one listing, each answering a different question
		// of the same records — and each one the caller's own, because the
		// bus filtered the listing before it got here.
		v.Backlogs, v.Losses = stuck(v.Records), lost(v.Records)
		// A caller who may not read the feed, or has no credential of its
		// own to be told about, still gets the rest of the page.
		var feed []protocol.Envelope
		if err := bus.get(cred, "/recent", &feed); err != nil {
			v.NoFeed = err.Error()
		} else {
			v.Exchanges = exchanges(feed)
		}
		if err := bus.get(cred, "/names", &v.Names); err != nil {
			v.NoNames = err.Error()
		}
		render(w, page, v)
	})

	// Signing in is the one moment a token is handled here, and it is not
	// kept: it is spent on a session the bus holds and then forgotten.
	mux.HandleFunc("POST /signin", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		var got struct {
			Session string `json:"session"`
		}
		// Belt and braces: an empty credential must never be forwarded. On a
		// socket that supplies the identity the bus would answer it, and
		// signing in with nothing would make every visitor that socket's
		// owner — which is the mint this child must not be.
		// See docs/05-discovery.md#signing-in.
		typed := r.FormValue("token")
		// Where they were going before they were asked to sign in, kept
		// across the refusal too so a mistyped token does not lose it.
		to := local(r.FormValue("return"))
		if typed == "" {
			render(w, anon, signin{pageInfo: requestInfo(r), Refused: "that credential was not accepted", Return: to})
			return
		}
		if err := bus.send(typed, "POST", "/session", &got); err != nil {
			// One message for every way it can fail, because telling a bad
			// token from an unknown name is an oracle on an open form.
			// See docs/05-discovery.md#rules-it-is-built-to.
			render(w, anon, signin{pageInfo: requestInfo(r), Refused: "that credential was not accepted", Return: to})
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name: cookieName, Value: got.Session, Path: "/",
			HttpOnly: true, Secure: tls, SameSite: http.SameSiteStrictMode,
			MaxAge: int(auth.IdleLife / time.Second),
		})
		http.Redirect(w, r, to, http.StatusSeeOther)
	})

	mux.HandleFunc("POST /signout", func(w http.ResponseWriter, r *http.Request) {
		if cred := cookie(r); cred != "" {
			bus.send(cred, "DELETE", "/session", nil)
		}
		http.SetCookie(w, &http.Cookie{Name: cookieName, Path: "/", MaxAge: -1})
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	bus.adminRoutes(mux, tls)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
		if r.Method == http.MethodPost {
			if r.Header.Get("Origin") != "" && !sameOrigin(r, tls) {
				http.Error(w, "same-origin form required", http.StatusForbidden)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		}
		if r.URL.Path != "/healthz" && r.URL.Path != "/avatar" && r.URL.Path != "/signout" {
			r = bus.pageRequest(r)
		}
		mux.ServeHTTP(w, r)
	})
}

func have(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// cookieName carries the session and nothing else — never a token, and never
// in a URL. See docs/05-discovery.md#signing-in.
const cookieName = "agent_bus_session"

func cookie(r *http.Request) string {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

// signIn serves the form at the address that needed it, rather than sending an
// anonymous visitor a status code with no form on it. The page keeps its URL,
// so signing in returns them to what they asked for.
// See Plans/MVP/done/web-review.md W12.
func signIn(w http.ResponseWriter, r *http.Request, refused string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	to := ""
	if r.Method == http.MethodGet && r.URL.Path != "/" {
		to = r.URL.RequestURI()
	} else if r.Method != http.MethodGet {
		to = referrer(r)
	}
	if to == "/" {
		to = ""
	}
	if refused != "" {
		// Something was wrong with the request, and a browser that is told 200
		// will cache and re-present this as though it were the page.
		w.WriteHeader(http.StatusUnauthorized)
	}
	render(w, anon, signin{pageInfo: requestInfo(r), Refused: refused, Return: to})
}

// referrer is where a submission came from, when that was a page of ours. A
// browser sends Referer as an absolute URL, so it is ours only if the host
// matches the one this request arrived on.
func referrer(r *http.Request) string {
	u, err := url.Parse(r.Referer())
	if err != nil || u.Host != r.Host {
		return "/"
	}
	return local(u.RequestURI())
}

// local keeps a return address on this dashboard. Anything with a scheme, a
// host or a leading // is somebody else's site, and a form field that took one
// would make the sign-in page an open redirect — the one page a stranger can
// always reach. Everything that is not plainly ours becomes the front page.
func local(raw string) string {
	if raw == "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return "/"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil {
		return "/"
	}
	return u.RequestURI()
}

func render(w http.ResponseWriter, t *template.Template, v any) {
	if err := t.Execute(w, v); err != nil {
		log.Printf("render: %v", err)
		return
	}
	if framed, ok := v.(interface{ Frame() pageInfo }); ok {
		if err := frameFooter.Execute(w, framed.Frame()); err != nil {
			log.Printf("render footer: %v", err)
		}
	}
}

// caller is somewhere to send a request. It holds no credential: one is
// given per call, because the only credentials this process sees belong to
// whoever is asking. See docs/05-discovery.md#signing-in.
type caller struct {
	client *http.Client
	base   string
}

func (c *caller) get(cred, path string, into any) error {
	return c.send(cred, "GET", path, into)
}

func (c *caller) send(cred, method, path string, into any) error {
	return c.request(cred, method, path, nil, into)
}

func (c *caller) request(cred, method, path string, body io.Reader, into any) error {
	if cred == "" {
		return fmt.Errorf("sign in required")
	}
	req, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set(api.HeaderToken, cred)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
		if method == "POST" && path == "/register" {
			req.Header.Set("If-None-Match", "*")
		}
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return &busError{code: resp.StatusCode, message: string(payload)}
	}
	if into == nil {
		return nil
	}
	return json.Unmarshal(payload, into)
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// No JavaScript, no assets, nothing fetched from anywhere: a view of the bus
// should not need a build step to read, and the first external asset added
// for a chart would inherit a signed-in master's whole envelope feed.
// See docs/05-discovery.md#rules-it-is-built-to.
const head = `<!doctype html>
<html lang=en>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>
 body{font:14px system-ui,sans-serif;margin:2rem;max-width:60rem}
 table{border-collapse:collapse;width:100%;margin-bottom:2rem}
 th,td{text-align:left;padding:.3rem .6rem;border-bottom:1px solid #ddd}
 th{font-weight:600;color:#555}
 code{font:13px ui-monospace,monospace;overflow-wrap:anywhere}
 .muted{color:#6b6b6b}
 .warn{color:#b00}
 h2{font-size:15px;margin:1.6rem 0 .4rem}
 .who{float:right;font-size:13px}
 input{font:13px ui-monospace,monospace;padding:.3rem;width:26rem;max-width:100%;box-sizing:border-box}
 input[type=checkbox]{width:auto}
 textarea{max-width:100%;box-sizing:border-box;font:13px ui-monospace,monospace}
 button,select{font:inherit;padding:.3rem .5rem}
 form+form{margin-top:1rem}
 .site-header{display:grid;grid-template-columns:80px minmax(0,1fr);column-gap:1rem;align-items:center;overflow-x:auto;white-space:nowrap;border-bottom:1px solid #aaa;padding-bottom:1rem;margin-bottom:1rem}
 .node-logo{grid-column:1;grid-row:1 / span 2}
 .node-summary{grid-column:2;grid-row:1;display:flex;align-items:center;flex-wrap:nowrap;gap:1rem;margin:.4rem 0}
 .node-navigation{grid-column:2;grid-row:2}
 .node-summary>*{flex-shrink:0}
 .site-footer{white-space:nowrap;overflow-x:auto;clear:both;border-top:1px solid #aaa;margin-top:2rem;padding-top:1rem;font-size:12px;line-height:1.6;overflow-wrap:anywhere}
 nav{line-height:2}
 nav a[aria-current=page]{font-weight:700;text-decoration:none}
 :focus-visible{outline:2px solid #253c66;outline-offset:2px}
 /* A phone is not a narrow desktop: the gutter shrinks and only a genuinely
    wide table scrolls, rather than the whole page.
    See docs/05-discovery.md#rules-it-is-built-to. */
 @media (max-width:40rem){
  body{margin:1rem}
  .who{float:none;display:block;margin-bottom:.5rem}
  input{width:100%}
  table{display:block;overflow-x:auto}
 }
</style>
`

// The navigation is one list, in one place. It was two — an inline copy on the
// diagnostics page and adminNav on every other — which is how sign-out came to
// exist on one page only (Plans/MVP/done/web-review.md W01).
var navItems = []struct{ Href, Label, Key string }{
	{"/services", "Registered services", "services"},
	{"/personal", "Personal services", "personal"},
	{"/channels", "Channels", "channels"},
	{"/users", "Users", "users"},
	{"/groups", "Groups", "groups"},
	{"/activity", "Activity graphs", "activity"},
	{"/", "Diagnostics", "diagnostics"},
}

// shell is the head, title and navigation a signed-in page shares. It is built
// per page rather than carried in each handler's data, because which page this
// is, is known when the template is parsed and never changes afterwards. That
// keeps the current entry marked without every view growing a field for it.
func shell(key, title string) string {
	var nav strings.Builder
	nav.WriteString(head)
	nav.WriteString("<title>")
	nav.WriteString(template.HTMLEscapeString(title))
	nav.WriteString(" \u00b7 agent-bus</title>\n")
	// Sign out belongs beside the name it signs out, on every page.
	nav.WriteString(frameHeader)
	nav.WriteString(`<div class=node-navigation><form method=post action=/signout class=who><code>{{.You}}</code> <button type=submit>sign out</button></form>` + "\n")
	nav.WriteString("<nav aria-label=\"sections\">")
	for i, item := range navItems {
		if i > 0 {
			nav.WriteString(" \u00b7 ")
		}
		if key == "records" && (item.Key == "services" || item.Key == "personal" || item.Key == "channels") {
			nav.WriteString(`{{if eq .Current "` + item.Key + `"}}<a href=` + item.Href + ` aria-current=page>{{else}}<a href=` + item.Href + `>{{end}}` + item.Label + `</a>`)
			continue
		}
		nav.WriteString("<a href=" + item.Href)
		if item.Key == key {
			nav.WriteString(" aria-current=page")
		}
		nav.WriteString(">" + item.Label + "</a>")
	}
	nav.WriteString("</nav></div>\n" + frameHeaderEnd + "<main>\n")
	return nav.String()
}

// The sign-in page shares the public node identity, and no private status.
// See docs/05-discovery.md#what-a-node-says-about-itself.
type signin struct {
	pageInfo
	Refused string
	Return  string
}

var anon = template.Must(template.New("anon").Parse(head + `<title>Sign in · agent-bus</title>` + frameHeader + frameHeaderEnd + `
<main>
<h1>agent-bus</h1>
<form method=post action=/signin>
{{with .Return}}<input type=hidden name=return value="{{.}}">{{end}}
 <p><label>token <input type=password name=token autofocus></label>
 <button type=submit>sign in</button>
{{with .Refused}}<p class=muted>{{.}}{{end}}
</form>
<p class=muted>A token is what every call carries. Get one with
 <code>agent-bus-token &lt;user@realm&gt;</code> on the box, or
 <code>ssh agent-busd@&lt;node&gt; token</code> from anywhere your key reaches.
`))

// page is the signed-in view, refreshed by the browser. Seven sections in the
// order an incident wants them: what the node is doing and who it is turning
// away, then the inboxes holding messages, then the traffic, and the registry
// and the credentials last. Every one of them is what the bus answered **this
// caller** (docs/05-discovery.md#what-it-shows).
// The page no longer refreshes itself. A reader has to be able to stop moving
// content, and a whole-page reload every five seconds also threw away whatever
// they were part-way through reading (Plans/MVP/done/web-review.md W11).
var page = template.Must(template.New("dash").Funcs(template.FuncMap{"readerCount": readerCount, "entityLabel": entityLabel}).Parse(shell("diagnostics", "Diagnostics") + `<h1>agent-bus</h1>
<p><a href=/>Refresh</a> <span class=muted>· as of {{.At}}</span></p>

<h2 id=node>node</h2>
<p>uptime {{.Status.Up}} · <b>node-wide:</b> {{.Status.Services}} records ·
 {{.Status.Queued}} queued · {{.Status.Waiting}} waiting · {{.Status.Dropped}} dropped ·
 {{.Status.Expired}} expired</p>
<p class=muted>These count the whole node. Every list below is what <em>you</em> may
 see, so the two never have to agree.</p>
{{if .Status.Unclean}}<p class=warn>the last stop was not clean — what was in
 memory at the time was not written down</p>{{end}}
<table><caption>Refusals since this daemon started, by reason</caption>
<thead><tr><th scope=col>reason<th scope=col>count</tr></thead>
<tbody>{{range .Refusals}}<tr><td><code>{{.Reason}}</code><td>{{.Count}}</tr>{{end}}</tbody></table>
<p class=muted>The reason set is closed, so a <code>0</code> here is a measurement
 and not a gap. Counted since this daemon started; how fast it is rising is not
 something this page can say.</p>
<p class=muted>The totals count API refusals, including bad credentials and malformed
 requests, whatever the caller&rsquo;s standing. What the router rejected before any
 handler ran is not included. Internal failures are not caller refusals and are
 not counted either.</p>

<h2 id=stuck>inboxes holding messages</h2>
<table><caption>Inboxes holding messages, longest wait first — visible to you</caption>
<thead><tr><th scope=col>name<th scope=col>readers<th scope=col>held now<th scope=col>oldest held<th scope=col>capacity</tr></thead>
<tbody>
{{range .Backlogs}}<tr><td><code>{{.Name}}</code><td>{{readerCount .Readers}}<td>{{.Queued}}<td>{{if .Oldest}}{{.Oldest}}{{else}}<span class=muted>&mdash;</span>{{end}}<td>{{if .AtBound}}<b class=warn>at capacity when observed</b>{{else}}<span class=muted>&mdash;</span>{{end}}</tr>
{{else}}<tr><td colspan=5 class=muted>every queue you can see is empty</tr>{{end}}
</tbody></table>
<p class=muted>Holding messages is not being stuck: a reader that pulls on a
 schedule is between pulls here. The observation does not prune first, so some of
 what is held may already have outlived its TTL.
 <em>At capacity</em> is what was true when observed, never a prediction about the next send.
 <em>Readers</em> counts outstanding consume requests, filtered and unfiltered
 together. Zero is not health; a process may be between reads. A positive count
 promises neither a match for a held message nor completed work.</p>

` + exchangesTemplate + `
<h2 id=registry>registry</h2>
<table><caption>Records visible to you — not the node-wide count above</caption>
<thead><tr><th scope=col>name<th scope=col>kind<th scope=col>description<th scope=col>readers<th scope=col>held now<th scope=col>accepted<th scope=col>dequeued<th scope=col>config</tr></thead>
<tbody>
{{range .Records}}<tr><td><code>{{.Name}}</code><td>{{entityLabel .Kind}}<td>{{.Descr}}
 <td>{{readerCount .Readers}}<td>{{.Queued}}<td>{{.In}}<td>{{.Out}}
 <td><code class=muted>{{.ConfigSHA}}</code></tr>
{{else}}<tr><td colspan=8 class=muted>nothing you can see is registered</tr>{{end}}
</tbody></table>
<p class=muted>Accepted and dequeued are cumulative across restarts — they come
 back from the snapshot. Dequeued means handed to a reader, which is not the
 same as the work being done.</p>

<h2 id=loss>loss by name</h2>
<table><tr><th>name<th>dropped<th>expired</tr>
{{range .Losses}}<tr><td><code>{{.Name}}</code><td>{{.Dropped}}<td>{{.Expired}}</tr>
{{else}}<tr><td colspan=3 class=muted>nothing lost</tr>{{end}}</table>

<h2 id=names>my names</h2>
{{if .NoNames}}<p class=muted>{{.NoNames}}</p>{{else}}
<table><tr><th>name<th>kind<th>owner<th>fingerprint<th>issued<th>last used</tr>
{{range .Names}}<tr><td><code>{{.Name}}</code>
 <td>{{if eq .Kind "unregistered"}}<span class=warn>unregistered</span>{{else}}{{entityLabel .Kind}}{{end}}
 <td><code class=muted>{{.Owner}}</code><td><code>{{.Fingerprint}}</code>
 <td>{{if .Issued.IsZero}}{{else}}{{.Issued.Format "2006-01-02 15:04"}}{{end}}
 <td>{{if .Used.IsZero}}<span class=muted>not this run</span>{{else}}{{.Used.Format "15:04:05"}}{{end}}</tr>
{{else}}<tr><td colspan=6 class=muted>you hold no credential</tr>{{end}}</table>
{{end}}
<p class=muted>A credential goes when its address does, so a name here answers
 for something. One marked <span class=warn>unregistered</span> is a leftover from
 before that was true.</p>

<p class=muted>A fingerprint names a credential without being one. Rotate with
 <code>agent-bus-token &lt;user@realm&gt; --rotate</code>; the one it replaces
 keeps working until the next rotation.</p>

<p class=muted>Envelopes only — bodies are never shown.</p>
`))

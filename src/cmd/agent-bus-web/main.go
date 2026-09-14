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
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/version"
)

// Where the dashboard is meant to be reached. `*.localhost.direct` resolves
// to 127.0.0.1 in public DNS, so a browser gets a real hostname and a real
// certificate without an /etc/hosts line and without a warning — and nothing
// leaves the machine.
// See docs/05-discovery.md#dashboard.
const (
	Host      = "agent-bus.localhost.direct"
	httpsPort = "443"
	altPort   = "8443" // where it lands when 443 is not ours to bind
)

func main() {
	if version.Print() {
		return
	}
	addr := flag.String("addr", env("AGENT_BUS_WEB_ADDR", Host+":"+httpsPort), "where the dashboard listens")
	certF := flag.String("cert", env("AGENT_BUS_WEB_CERT", defaultCert(".crt")), "TLS certificate; without it the dashboard is plain HTTP on loopback")
	keyF := flag.String("key", env("AGENT_BUS_WEB_KEY", defaultCert(".key")), "the certificate's private key")
	flag.Parse()

	client, base := api.Dial(os.Getenv("AGENT_BUS_ADDR"))
	// No credential of its own, deliberately. The child reaches the bus over
	// the shared socket, where a caller has to say who it is, so every page
	// is rendered with the credential of whoever asked for it and the child
	// has no authority to lend out. See docs/05-discovery.md#signing-in.
	bus := &caller{client: client, base: base}
	tls := have(*certF) && have(*keyF)

	handler := dashboard(bus, tls)

	if !tls {
		// Never silently: a dashboard on plain HTTP is a different thing
		// from one on HTTPS, and the person running it should know which
		// they have. The public hostname is given up with the certificate —
		// it is only worth having because the certificate matches it — but
		// an address somebody asked for out loud is still honoured.
		if *addr == Host+":"+httpsPort {
			*addr = "127.0.0.1:6780"
		}
		log.Printf("no certificate at %s — plain HTTP. See docs/05-discovery.md#dashboard for where to get one", *certF)
	}
	l, err := listen(*addr, tls)
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
	// The whole public signal. Anything that varies is something an
	// anonymous visitor can watch, so this answers nothing at all.
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
		var node struct {
			core.Status
			You string `json:"you"`
		}
		if cred == "" || bus.get(cred, "/status", &node) != nil {
			render(w, anon, nil)
			return
		}
		v := view{You: node.You, Status: node.Status, Refusals: refusals(node.Refused)}
		if err := bus.get(cred, "/ls", &v.Records); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
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
		if typed == "" {
			render(w, anon, map[string]string{"Refused": "that credential was not accepted"})
			return
		}
		if err := bus.send(typed, "POST", "/session", &got); err != nil {
			// One message for every way it can fail, because telling a bad
			// token from an unknown name is an oracle on an open form.
			// See docs/05-discovery.md#rules-it-is-built-to.
			render(w, anon, map[string]string{"Refused": "that credential was not accepted"})
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name: cookieName, Value: got.Session, Path: "/",
			HttpOnly: true, Secure: tls, SameSite: http.SameSiteStrictMode,
			MaxAge: int(auth.IdleLife / time.Second),
		})
		http.Redirect(w, r, "/", http.StatusSeeOther)
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
		mux.ServeHTTP(w, r)
	})
}

// listen binds addr, and falls back off port 443 rather than dying on it:
// binding a low port needs a capability the dashboard has no other use for,
// and a child that will not start is worse than one on a port it announces.
func listen(addr string, tls bool) (net.Listener, error) {
	l, err := net.Listen("tcp", addr)
	if err == nil || !tls || !errors.Is(err, syscall.EACCES) {
		return l, err
	}
	host, port, _ := net.SplitHostPort(addr)
	if port != httpsPort {
		return nil, err
	}
	log.Printf("port %s needs CAP_NET_BIND_SERVICE, which this has not got — using %s instead", httpsPort, altPort)
	return net.Listen("tcp", net.JoinHostPort(host, altPort))
}

func have(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// defaultCert is beside the daemon's other state, because that is where an
// install puts what the account owns (docs/09-setup.md#the-two-accounts).
func defaultCert(ext string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "agent-bus", Host+ext)
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

func render(w http.ResponseWriter, t *template.Template, v any) {
	if err := t.Execute(w, v); err != nil {
		log.Printf("render: %v", err)
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
<meta charset="utf-8">
<title>agent-bus</title>
<style>
 body{font:14px system-ui,sans-serif;margin:2rem;max-width:60rem}
 table{border-collapse:collapse;width:100%;margin-bottom:2rem}
 th,td{text-align:left;padding:.3rem .6rem;border-bottom:1px solid #ddd}
 th{font-weight:600;color:#555}
 code{font:13px ui-monospace,monospace}
 .muted{color:#888}
 .warn{color:#b00}
 h2{font-size:15px;margin:1.6rem 0 .4rem}
 .who{float:right;font-size:13px}
 input{font:13px ui-monospace,monospace;padding:.3rem;width:26rem;max-width:100%;box-sizing:border-box}
 input[type=checkbox]{width:auto}
 textarea{max-width:100%;box-sizing:border-box;font:13px ui-monospace,monospace}
 button,select{font:inherit;padding:.3rem .5rem}
 form+form{margin-top:1rem}
 nav{line-height:2}
</style>
`

// anon is what the bus would answer a caller it cannot name: nothing. A
// title, the form, and where a credential comes from. No uptime, no counts
// and no names — each of those is something a stranger could sit and watch,
// and nothing on the page can know whether it is exposed.
// See docs/05-discovery.md#rules-it-is-built-to.
var anon = template.Must(template.New("anon").Parse(head + `<h1>agent-bus</h1>
<form method=post action=/signin>
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
// away, then the backlogs nobody is reading, then the traffic, and the registry
// and the credentials last. Every one of them is what the bus answered **this
// caller** (docs/05-discovery.md#what-it-shows).
var page = template.Must(template.New("dash").Parse(head + `<meta http-equiv="refresh" content="5">
<form method=post action=/signout class=who>
 <code>{{.You}}</code> <button type=submit>sign out</button></form>
<h1>agent-bus</h1>
<nav><a href=/services>Registered services</a> · <a href=/channels>Channels</a> · <a href=/users>Users</a> · <a href=/groups>Groups</a> · <a href=/activity>Activity graphs</a> · <a href=/>Diagnostics</a></nav>

<h2 id=node>node</h2>
<p>up {{.Status.Up}} · {{.Status.Services}} records · {{.Status.Queued}} queued ·
 {{.Status.Waiting}} waiting · {{.Status.Dropped}} dropped · {{.Status.Expired}} expired</p>
{{if .Status.Unclean}}<p class=warn>the last stop was not clean — what was in
 memory at the time was not written down</p>{{end}}
{{if .Refusals}}<p>refused:
 {{range .Refusals}}<code>{{.Reason}}</code> {{.Count}} · {{end}}</p>
{{else}}<p class=muted>nothing refused</p>{{end}}

<h2 id=stuck>stuck inboxes</h2>
<table><tr><th>name<th>reader<th>queued<th>oldest<th>at bound</tr>
{{range .Backlogs}}<tr><td><code>{{.Name}}</code><td>{{if .Reading}}reading{{else}}<b class=warn>nobody</b>{{end}}<td>{{.Queued}}<td>{{.Oldest}}<td>{{if .AtBound}}<b class=warn>full</b>{{end}}</tr>
{{else}}<tr><td colspan=5 class=muted>every queue is empty</tr>{{end}}</table>

<h2 id=exchanges>exchanges</h2>
{{if .NoFeed}}<p class=muted>{{.NoFeed}}</p>{{else}}
<table><tr><th>at<th>topic<th>tag<th>from<th>to<th>messages<th>ack<th>reply<th>done<th></tr>
{{range .Exchanges}}<tr><td>{{.At.Format "15:04:05"}}<td>{{.Topic}}<td>{{.Tag}}<td><code>{{.From}}</code><td><code>{{.To}}</code><td>{{.N}}<td>{{if .Ack}}ack{{end}}<td>{{if .Reply}}reply{{end}}<td>{{if .Done}}done{{end}}<td>{{if .Late}}<b class=warn>late</b>{{end}}</tr>
{{else}}<tr><td colspan=10 class=muted>nothing yet</tr>{{end}}</table>
{{end}}

<h2 id=registry>registry</h2>
<table><tr><th>name<th>kind<th>description<th>reading<th>queued<th>in<th>out<th>config</tr>
{{range .Records}}<tr><td><code>{{.Name}}</code><td>{{.Kind}}<td>{{.Descr}}
 <td>{{if .Reading}}yes{{end}}<td>{{.Queued}}<td>{{.In}}<td>{{.Out}}
 <td><code class=muted>{{.ConfigSHA}}</code></tr>
{{else}}<tr><td colspan=8 class=muted>nothing registered</tr>{{end}}</table>

<h2 id=loss>loss by name</h2>
<table><tr><th>name<th>dropped<th>expired</tr>
{{range .Losses}}<tr><td><code>{{.Name}}</code><td>{{.Dropped}}<td>{{.Expired}}</tr>
{{else}}<tr><td colspan=3 class=muted>nothing lost</tr>{{end}}</table>

<h2 id=names>my names</h2>
{{if .NoNames}}<p class=muted>{{.NoNames}}</p>{{else}}
<table><tr><th>name<th>fingerprint<th>issued<th>last used</tr>
{{range .Names}}<tr><td><code>{{.Name}}</code><td><code>{{.Fingerprint}}</code>
 <td>{{if .Issued.IsZero}}{{else}}{{.Issued.Format "2006-01-02 15:04"}}{{end}}
 <td>{{if .Used.IsZero}}<span class=muted>not this run</span>{{else}}{{.Used.Format "15:04:05"}}{{end}}</tr>
{{else}}<tr><td colspan=4 class=muted>you hold no credential</tr>{{end}}</table>
{{end}}
<p class=muted>A fingerprint names a credential without being one. Rotate with
 <code>agent-bus-token &lt;user@realm&gt; --rotate</code>; the one it replaces
 keeps working until the next rotation.</p>

<p class=muted>Envelopes only — bodies are never shown.</p>
`))

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

	// The whole public signal. Anything that varies is something an
	// anonymous visitor can watch, so this answers nothing at all.
	// See docs/05-discovery.md#rules-it-is-built-to.
	http.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		cred := cookie(r)
		var v view
		// Whose page this is comes from the bus, not from the child: the
		// cookie is checked by being used. An expired or forged one is a
		// visitor who is not signed in, which is the anonymous page.
		if cred == "" || bus.get(cred, "/status", &v.Status) != nil {
			render(w, anon, nil)
			return
		}
		var me struct {
			You string `json:"you"`
		}
		bus.get(cred, "/status", &me)
		v.You = me.You
		if err := bus.get(cred, "/ls", &v.Records); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if err := bus.get(cred, "/recent", &v.Recent); err != nil {
			// A caller who may not read the feed still gets the rest of
			// the page.
			v.NoFeed = err.Error()
		}
		render(w, page, v)
	})

	// Signing in is the one moment a token is handled here, and it is not
	// kept: it is spent on a session the bus holds and then forgotten.
	http.HandleFunc("POST /signin", func(w http.ResponseWriter, r *http.Request) {
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

	http.HandleFunc("POST /signout", func(w http.ResponseWriter, r *http.Request) {
		if cred := cookie(r); cred != "" {
			bus.send(cred, "DELETE", "/session", nil)
		}
		http.SetCookie(w, &http.Cookie{Name: cookieName, Path: "/", MaxAge: -1})
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	if !tls {
		// Never silently: a dashboard on plain HTTP is a different thing
		// from one on HTTPS, and the person running it should know which
		// they have. The public hostname is given up with the certificate —
		// it is only worth having because the certificate matches it — but
		// an address somebody asked for out loud is still honoured.
		if *addr == Host+":"+httpsPort {
			*addr = "127.0.0.1:7878"
		}
		log.Printf("no certificate at %s — plain HTTP. See docs/05-discovery.md#dashboard for where to get one", *certF)
	}
	l, err := listen(*addr, tls)
	if err != nil {
		log.Fatal(err)
	}
	srv := &http.Server{Handler: nil, ReadHeaderTimeout: 10 * time.Second}
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

type view struct {
	You     string
	Status  core.Status
	Records []protocol.Record
	Recent  []protocol.Envelope
	NoFeed  string
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
	req, err := http.NewRequest(method, c.base+path, nil)
	if err != nil {
		return err
	}
	if cred != "" {
		req.Header.Set(api.HeaderToken, cred)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s: %s", path, body)
	}
	if into == nil {
		return nil
	}
	return json.Unmarshal(body, into)
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
 .who{float:right;font-size:13px}
 input{font:13px ui-monospace,monospace;padding:.3rem;width:26rem;max-width:100%}
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

// page is the signed-in view, refreshed by the browser.
var page = template.Must(template.New("dash").Parse(head + `<meta http-equiv="refresh" content="5">
<form method=post action=/signout class=who>
 <code>{{.You}}</code> <button type=submit>sign out</button></form>
<h1>agent-bus</h1>
<p>up {{.Status.Up}} · {{.Status.Services}} records · {{.Status.Queued}} queued ·
 {{.Status.Waiting}} waiting · {{.Status.Dropped}} dropped · {{.Status.Expired}} expired</p>

<h2>records</h2>
<table><tr><th>name<th>kind<th>description<th>in<th>out<th>queued</tr>
{{range .Records}}<tr><td><code>{{.Name}}</code><td>{{.Kind}}<td>{{.Descr}}
 <td>{{.In}}<td>{{.Out}}<td>{{.Queued}}</tr>
{{else}}<tr><td colspan=6 class=muted>nothing registered</tr>{{end}}</table>

<h2>recent envelopes</h2>
{{if .NoFeed}}<p class=muted>{{.NoFeed}}</p>{{else}}
<table><tr><th>at<th>message<th>from<th>to<th>topic<th>tag<th>receipt</tr>
{{range .Recent}}<tr><td>{{.At.Format "15:04:05"}}<td><code>{{.ID}}</code>
 <td><code>{{.From}}</code><td><code>{{.To}}</code><td>{{.Topic}}<td>{{.Tag}}<td>{{.Receipt}}</tr>
{{else}}<tr><td colspan=7 class=muted>nothing yet</tr>{{end}}</table>
{{end}}
<p class=muted>Envelopes only — bodies are never shown.</p>
`))

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
	mux.HandleFunc("GET /ui.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		io.WriteString(w, uiScript)
	})
	mux.HandleFunc("GET /favicon.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
		io.WriteString(w, faviconSVG)
	})
	// A browser asks for this one whether or not a page names it, and `GET /`
	// answers every unmatched path, so without this the tab icon request gets
	// the overview or the sign-in page with a 200 on it. Answer what is true:
	// there is no .ico here, and the head names the SVG.
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	// The one bitmap this node serves. It is answered before any credential,
	// because the page that shows it is the one a stranger reaches.
	mux.HandleFunc("GET /agent-bus.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		// Overrides the frame's no-store: this file changes when the binary
		// does, and re-sending 119KB on every visit to the sign-in form is
		// the whole reason it is not the original.
		w.Header().Set("Cache-Control", "max-age=86400")
		w.Write(busPicture)
	})

	loadView := func(w http.ResponseWriter, r *http.Request) (view, bool) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		cred := cookie(r)
		// Whose page this is comes from the bus, not from the child: the
		// cookie is checked by being used. An expired or forged one is a
		// visitor who is not signed in, which is the anonymous page.

		if cred == "" {
			signIn(w, r, "")
			return view{}, false
		}
		// And a bus that is not answering is a different fact, which this
		// used to swallow: every failed status request became the anonymous
		// page, so a stopped daemon read as "you are not signed in" and the
		// one recovery offered was the one that could not work.
		// See Plans/MVP/done/web-review.md W12.
		node, err := bus.status(r)
		if err != nil {
			fail(w, r, "", err)
			return view{}, false
		}
		v := view{pageInfo: requestInfo(r), You: node.You, Status: node.Status, Refusals: refusals(node.Refused)}
		v.Totals = kindTotals(node.Kinds)
		if err := bus.get(cred, "/ls", &v.Records); err != nil {
			fail(w, r, node.You, err)
			return view{}, false
		}
		v.Backlogs, v.Losses = stuck(v.Records), lost(v.Records)
		return v, true
	}

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		v, ok := loadView(w, r)
		if !ok {
			return
		}
		// Inactive records are not in /ls; work they hold is still held, and
		// /inactive is the one place it can be seen from.
		var inactive []protocol.Record
		if err := bus.get(cookie(r), "/inactive", &inactive); err != nil {
			fail(w, r, v.You, err)
			return
		}
		for i := range inactive {
			inactive[i].Status = protocol.StatusInactive
		}
		v.Attention = attentionItems(v.Status, append(append([]protocol.Record{}, v.Records...), inactive...))
		render(w, overviewPage, v)
	})

	mux.HandleFunc("GET /diagnostics", func(w http.ResponseWriter, r *http.Request) {
		v, ok := loadView(w, r)
		if !ok {
			return
		}
		// A caller who may not read the feed, or has no credential of its
		// own to be told about, still gets the rest of the page.
		var feed []protocol.Envelope
		if err := bus.get(cookie(r), "/recent", &feed); err != nil {
			v.NoFeed = sectionProblem("recent envelopes", err)
		} else {
			v.Exchanges = exchanges(feed)
		}
		var identities []protocol.User
		if err := bus.get(cookie(r), "/users", &identities); err == nil {
			for _, u := range identities {
				if u.Kind != protocol.DirectoryUser {
					v.Leftovers = append(v.Leftovers, u)
				}
			}
		}
		render(w, diagnosticsPage, v)
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
		// No Max-Age: the session ends when the bus's idle timeout says so,
		// and a fixed browser clock signed out people still working.
		// See docs/05-discovery.md#signing-in.
		http.SetCookie(w, &http.Cookie{
			Name: cookieName, Value: got.Session, Path: "/",
			HttpOnly: true, Secure: tls, SameSite: http.SameSiteStrictMode,
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
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'unsafe-inline'; img-src 'self' data:; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
		if r.Method == http.MethodPost {
			if r.Header.Get("Origin") != "" && !sameOrigin(r, tls) {
				http.Error(w, "same-origin form required", http.StatusForbidden)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		}
		if r.URL.Path != "/healthz" && r.URL.Path != "/ui.js" && r.URL.Path != "/avatar" && r.URL.Path != "/signout" && r.URL.Path != "/agent-bus.jpg" {
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

// This repository-owned behavior submits compact selectors as soon as their
// value changes. State still lives in ordinary URLs and server-rendered forms.
const uiScript = `document.addEventListener("change", function (event) {
  var control = event.target.closest("[data-submit-on-change]");
  if (control && control.form) control.form.requestSubmit();
});`

const head = `<!doctype html>
<html lang=en>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<link rel=icon href=/favicon.svg type="image/svg+xml">
<script defer src=/ui.js></script>
<style>
 body{font:14px system-ui,sans-serif;margin:2rem;max-width:60rem}
 table{border-collapse:collapse;width:100%;margin-bottom:2rem}
 th,td{text-align:left;padding:.3rem .6rem;border-bottom:1px solid #ddd}
 th.num,td.num{text-align:right;font-variant-numeric:tabular-nums}
 th{font-weight:600;color:#555}
 code{font:13px ui-monospace,monospace;overflow-wrap:anywhere}
 .muted{color:#6b6b6b}
 .warn{color:#b00}
 .skip-link{position:absolute;left:-10000px;top:auto;width:1px;height:1px;overflow:hidden}
 .skip-link:focus{left:1rem;top:1rem;width:auto;height:auto;overflow:visible;z-index:10;background:#fff;padding:.5rem;border:2px solid #253c66}
 .form-error{border-left:4px solid #b00;background:#fff4f2;padding:.75rem 1rem;margin:1rem 0}
 [aria-invalid=true]{border-color:#b00}
 a.danger{color:#b00;font-weight:600}
 .page-title{display:flex;align-items:center;gap:.5rem;flex-wrap:wrap}
 .page-title h1{display:flex;align-items:center;gap:.45rem;flex-wrap:wrap;min-width:0}
 .page-title-mark{display:inline-block;flex:none;vertical-align:middle;font-size:1em;line-height:1}
 .help-button{position:relative;width:auto;border:1px solid #777;border-radius:50%;background:transparent;padding:.1rem .4rem;font-weight:700}
 .help-button:hover::after,.help-button:focus-visible::after{content:attr(aria-label);position:absolute;z-index:20;left:calc(100% + .45rem);top:50%;transform:translateY(-50%);width:max-content;max-width:18rem;padding:.3rem .5rem;border:1px solid var(--border-strong);border-radius:3px;background:var(--text-1);color:var(--surface-1);font-size:.75rem;font-weight:500;line-height:1.3;text-align:left;pointer-events:none}
 .help-button[data-tooltip]:hover::after,.help-button[data-tooltip]:focus-visible::after{content:attr(data-tooltip);width:24rem;max-width:min(24rem,70vw);white-space:normal}
 .context-help{max-width:30rem;border:1px solid #777;padding:1rem;box-shadow:0 .25rem 1rem #0003}
 .context-help h2{margin-top:0}.context-help li+li{margin-top:.45rem}
.section-nav,.filter-nav{display:flex;flex-wrap:wrap;gap:.35rem 1rem;margin:.5rem 0 1rem}
.section-nav a[aria-current],.filter-nav a[aria-current]{font-weight:700;text-decoration:none;border-bottom:2px solid currentColor}
.record-name-cell{border-left:3px solid transparent}
.personal-marker{white-space:nowrap}
 .identity-with-photo{display:flex;align-items:center;gap:.55rem}
 .profile-photo,.profile-initial{width:2rem;height:2rem;border-radius:50%;flex:none}
 .profile-photo{object-fit:cover}
 .profile-initial,.profile-initial-large{display:inline-flex;align-items:center;justify-content:center;background:#e5eaf4;color:#253c66;font-weight:700}
 .profile-photo-large,.profile-initial-large{width:3rem;height:3rem;border-radius:50%;object-fit:cover;flex:none;font-size:1.25rem}
 .activity-chart{display:block;width:100%;height:auto;max-height:14rem}
 .activity-axis{fill:none;stroke:#87847b;stroke-width:1;vector-effect:non-scaling-stroke}
 .activity-line{fill:none;stroke-width:2.5;vector-effect:non-scaling-stroke}
 .activity-tick{fill:none;stroke:#87847b;stroke-width:1;vector-effect:non-scaling-stroke}
 .activity-hour{font-size:11px;fill:#5f5c55}
 .activity-accepted{stroke:#1d5fa8;border-top-color:#1d5fa8}
 .activity-output{stroke:#8a5000;border-top-color:#8a5000;stroke-dasharray:8 4}
 .activity-dropped{stroke:#a8271b;border-top-color:#a8271b}
 .activity-expired{stroke:#a8271b;border-top-color:#a8271b;stroke-dasharray:8 4}
 .activity-refused{stroke:#1d5fa8;border-top-color:#1d5fa8;stroke-dasharray:2 4}
 .activity-legend{display:flex;flex-wrap:wrap;gap:.35rem 1.25rem;padding:0;list-style:none}
 .activity-swatch{display:inline-block;width:1.5rem;border-top:3px solid;margin-right:.35rem;vertical-align:middle}
 .activity-swatch.activity-output,.activity-swatch.activity-expired{border-top-style:dashed}
 .activity-swatch.activity-refused{border-top-style:dotted}
 h2{font-size:15px;margin:1.6rem 0 .4rem}
 .who{float:right;font-size:13px}
 input{font:13px ui-monospace,monospace;padding:.3rem;width:26rem;max-width:100%;box-sizing:border-box}
 input[type=checkbox]{width:auto}
 textarea{max-width:100%;box-sizing:border-box;font:13px ui-monospace,monospace}
 button,select{font:inherit;padding:.3rem .5rem}
 select{max-width:100%}
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
 :root{--surface-1:#fbfbf9;--surface-2:#f2f2ee;--surface-3:#e8e7e2;--border:#d2d0c9;--border-strong:#87847b;--text-1:#1a1a17;--text-2:#56544c;--accent:#1d5fa8;--red:#a8271b;--orange:#8a5000;--green:#2d6a3f}
 *{box-sizing:border-box}
 html{background:var(--surface-1);color:var(--text-1)}
 body{margin:0;max-width:none;background:var(--surface-1);color:var(--text-1);font:14px/1.45 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
 a{color:var(--accent)}
main{max-width:104rem;margin:0 auto;padding:2rem 1rem}
.site-header{grid-template-columns:64px minmax(0,1fr);column-gap:1rem;margin:0;padding:.75rem max(1rem,calc((100vw - 104rem)/2 + 1rem));border-color:var(--border);background:var(--surface-1);overflow:visible}
 .node-logo{width:64px;height:auto}
 .node-summary{gap:.5rem 1rem;margin:0;font-size:.875rem;overflow-x:auto}
 .node-navigation{display:flex;align-items:center;gap:1.5rem;min-width:0}
 /* The sections wrap rather than scroll: a scrolled strip hides every entry
    past the edge on a phone, with nothing to say that more are there. */
 .node-navigation nav{display:flex;flex-wrap:wrap;align-items:center;gap:0 1.5rem;line-height:2.4}
 .node-navigation nav a{color:var(--text-1);text-decoration:none;border-bottom:3px solid transparent}
 .node-navigation nav .page-title-mark{width:1.05em;height:1.05em;margin-right:.3em;vertical-align:-.15em}
 .node-navigation nav a[aria-current=page]{color:var(--accent);border-bottom-color:var(--accent)}
 .who{float:none;order:2;margin:0 0 0 auto;font-size:.875rem}
.who button{margin-left:.5rem}
.account-link[aria-current=page]{font-weight:700;text-decoration:none}
.site-footer{max-width:104rem;margin:2rem auto 0;padding:1rem;border-color:var(--border);color:var(--text-2)}
.footer-node{display:flex;flex-wrap:wrap;gap:.35rem 1.25rem;margin-bottom:.35rem}
.hero{margin:0}
.hero img{max-width:100%;height:auto;border-radius:6px}
.landing{max-width:56rem;margin:0 auto;text-align:center}
.landing h1{margin:1.25rem 0 .5rem;font-size:1.9rem;line-height:1.2}
.landing .lede{max-width:44rem;margin:0 auto .6rem;font-size:1.05rem}
.landing .muted{max-width:44rem;margin:0 auto}
.landing-features{display:grid;grid-template-columns:repeat(auto-fit,minmax(12rem,1fr));gap:.75rem;margin:1.5rem 0;text-align:left}
.landing-feature{padding:.85rem 1rem;border:1px solid var(--border);border-radius:6px;background:var(--surface-2)}
.landing-feature strong{display:block;margin-bottom:.25rem}
.landing-links{margin:1.25rem 0 0}
.landing-links a+a{margin-left:.35rem}
.signin-card{max-width:30rem;margin:2rem auto 0;padding:1.25rem 1.5rem 1.5rem;border:1px solid var(--border);border-radius:6px;background:var(--surface-1)}
.signin-card .page-title{justify-content:center;margin-bottom:.9rem}
.signin-card .page-title h2{margin:0;font-size:1.15rem}
.signin-row{display:flex;gap:.5rem;margin-top:.3rem}
.signin-row input{flex:1 1 auto;min-width:0;min-height:2.45rem;padding:.4rem .55rem;border:1px solid var(--border-strong);border-radius:3px;background:var(--surface-1);font:inherit}
.signin-row button{flex:none;min-height:2.45rem;padding:0 1rem;border:1px solid var(--border-strong);border-radius:3px;background:var(--surface-2);font:inherit;cursor:pointer}
.signin-card label{font-size:.8rem;color:var(--text-2)}
.build-tip{text-decoration:underline dotted;text-underline-offset:.2em;cursor:help}
.page-title{margin-bottom:.5rem}
 .page-title h1{font-size:1.75rem;line-height:1.2;margin:.5rem 0}
 .page-title-mark{font-size:1.35em}
 .page-title h1 code{font-size:1em}
 .help-button{border-color:var(--border-strong);color:var(--text-2);cursor:pointer}
 .context-help{border-color:var(--border-strong);border-radius:6px;background:var(--surface-1);box-shadow:none}
 .section-nav{gap:.5rem 1.5rem;margin:.25rem 0 1.25rem;padding-bottom:.75rem;border-bottom:1px solid var(--border)}
 .section-nav a,.filter-nav a{font-weight:600;text-underline-offset:.2em}
.section-nav a[aria-current],.filter-nav a[aria-current]{border-bottom:3px solid currentColor}
.section-nav .my-view{color:var(--accent);font-weight:600}
.section-nav .personal-view{color:var(--orange);font-weight:600}
.record-name-cell.owned-record{border-left-color:var(--accent)}
.record-name-cell.personal-record{border-left-color:var(--orange)}
.record-toolbar{margin:0 0 1rem;padding:1rem;border-radius:6px;background:var(--surface-2)}
.record-search{display:grid;grid-template-columns:minmax(18rem,1fr) auto auto minmax(10rem,auto) auto;align-items:center;gap:.75rem;margin:0}
 .record-search input[type=search]{width:100%;font:inherit;background:var(--surface-1)}
 .record-search input,.record-search select,.record-search button{min-height:2.35rem;border:1px solid var(--border-strong);border-radius:3px}
 .record-search button{color:#fff;background:var(--accent);border-color:var(--accent);font-weight:600;padding-inline:1rem}
.record-choices{display:flex;flex-wrap:wrap;gap:.35rem 1.5rem;align-items:center}
 .filter-nav{align-items:center;gap:.35rem;margin:0}
 .filter-nav>span{font-weight:600;color:var(--text-2);margin-right:.25rem}
 .filter-nav a{padding:.28rem .65rem;border:1px solid var(--border);border-radius:3px;text-decoration:none;background:var(--surface-1)}
 .filter-nav a[aria-current]{color:#fff;background:var(--accent);border-color:var(--accent)}
 table{margin-bottom:2rem}
 caption{text-align:left;color:var(--text-2);padding:.25rem 0 .65rem}
 th{font-size:.75rem;letter-spacing:.02em;color:var(--text-2);background:var(--surface-3)}
 th,td{padding:.55rem .7rem;border-color:var(--border);vertical-align:middle}
 .record-table{font-size:.875rem}
 .users-table .contact-line{display:inline-block;margin-right:.9rem;overflow-wrap:anywhere}
 .users-table td[data-label="Last used"]{white-space:nowrap}
 .record-table tbody tr:hover{background:var(--surface-2)}
 .record-name-cell{min-width:15rem;padding-left:.9rem}
 .record-name{display:inline-flex;flex-direction:column;gap:.08rem;text-decoration:none}
 .record-name:hover .record-description{text-decoration:underline}
 .record-description{font:600 .9rem/1.3 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;color:inherit}
 .record-name code{font-size:.75rem;line-height:1.3;color:var(--text-2)}
 .record-name-cell.owned-record .record-name{color:var(--accent);font-weight:600}
 .record-name-cell.personal-record .record-name,.personal-marker{color:var(--orange);font-weight:600}
 .personal-marker{display:inline-block;margin-top:.2rem;font-size:.75rem}
 .status-glyph{font-size:1rem;line-height:1;white-space:nowrap}
.editor-card{max-width:64rem;margin:1rem 0;padding:1.25rem;border:1px solid var(--border);border-radius:6px;background:var(--surface-1)}
 .editor-card h2{margin-top:0;font-size:1.1rem}
.task-card{max-width:72rem;background:var(--surface-2)}
.task-card fieldset{margin:0;padding:.8rem 1rem;border:1px solid var(--border);border-radius:4px;background:var(--surface-1)}
.choice-row{display:flex;flex-wrap:wrap;gap:.7rem 1.5rem;align-items:center}
.choice-row label{display:inline-flex;align-items:center;gap:.35rem;font-weight:600}
.choice-row input{width:auto}
.field-heading{display:flex;align-items:center;gap:.45rem;margin-bottom:.3rem;font-weight:600}
.field-heading .help-button{font-weight:700}
.form-field textarea{width:100%;min-height:8rem;padding:.55rem;border:1px solid var(--border-strong);border-radius:3px;background:var(--surface-1)}
.detail-meta{display:flex;flex-wrap:wrap;gap:.4rem .65rem;align-items:center;margin:.4rem 0 1rem;color:var(--text-2)}
.fact-pill,.group-chip{display:inline-flex;align-items:center;padding:.2rem .5rem;border:1px solid var(--border);border-radius:999px;background:var(--surface-2);font-size:.8rem}
.person-layout{display:grid;grid-template-columns:minmax(0,2fr) minmax(18rem,1fr);gap:1rem;max-width:82rem;align-items:start}
.account-summary{display:grid;grid-template-columns:minmax(20rem,1fr) minmax(20rem,1fr);gap:1rem;max-width:82rem;align-items:start}
.account-summary .editor-card{width:100%;max-width:none;margin-top:0}
.person-main,.person-sidebar{min-width:0}
.person-sidebar .editor-card,.person-main .editor-card{margin-top:0;width:100%}
.compact-card{padding:1rem}
.compact-card .page-title h2{margin:0}
.compact-card p:last-child{margin-bottom:0}
.access-actions{display:flex;flex-wrap:wrap;gap:.5rem}
.access-actions form{margin:0}
.access-actions .danger-action{color:#fff;background:var(--red);border-color:var(--red)}
button.danger-action{color:#fff;background:var(--red);border-color:var(--red)}
.user-state{display:inline-flex;align-items:center;gap:.35rem;font-weight:700}
.user-state-active{color:var(--green)}
.user-state-inactive{color:var(--red)}
.access-change{margin-top:.8rem;padding-top:.65rem;border-top:1px solid var(--border)}
.access-change summary{color:var(--red);font-weight:700;cursor:pointer;text-decoration:underline;text-underline-offset:.2em}
.access-change[open] summary{margin-bottom:.75rem}
.group-card{margin:0;max-width:none}
.group-card textarea{display:block;width:100%;min-height:9rem;margin-top:.4rem;padding:.55rem;border:1px solid var(--border-strong);border-radius:3px;background:var(--surface-1)}
.member-line{display:block;line-height:1.6}
.member-list{display:grid;gap:.15rem}
.group-table th:first-child,.group-table td:first-child{width:22rem;max-width:22rem}
.group-table td[data-label=Members]{text-align:left}
.service-dashboard{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:1rem;max-width:82rem;margin:1rem 0;align-items:start}
.fact-card{min-width:0;padding:1rem;border:1px solid var(--border);border-radius:6px;background:var(--surface-1)}
.fact-card .page-title{margin:0}
.fact-card .page-title h2{margin:0;font-size:1rem}
.fact-card p{margin:.65rem 0 0}
.fact-card dl{display:grid;grid-template-columns:auto 1fr;gap:.35rem .7rem;margin:.65rem 0 0}
.fact-card dt{color:var(--text-2)}
.fact-card dd{margin:0;text-align:right;font-variant-numeric:tabular-nums}
.activity-fact{grid-column:1/-1}
.dashboard-section{margin-top:1.5rem}
.dashboard-section>.page-title h2{margin:0}
.dashboard-section table{margin-top:.55rem}
.attention-list{display:grid;gap:.65rem;max-width:64rem;margin:1rem 0}
.attention-item{padding:.8rem 1rem;border:1px solid var(--border);border-left:3px solid var(--orange);border-radius:4px;background:var(--surface-2)}
.attention-item.attention-red{border-left-color:var(--red)}
.attention-item.attention-blue{border-left-color:var(--accent)}
.attention-item.attention-empty{border-left-color:var(--border)}
.attention-item h3{margin:0;font-size:1rem}
.attention-item p{margin:.35rem 0 0}
.node-strip{display:flex;flex-wrap:wrap;gap:1px;max-width:64rem;margin:1rem 0;background:var(--border)}
.node-fact{display:flex;flex-direction:column;flex:1 1 7rem;padding:1rem;background:var(--surface-2)}
.node-fact-note{font-size:.95rem;font-weight:600;color:var(--text-2)}
 /* What the node holds is one row, how it is running is the next. Without an
    explicit break the split is whatever the viewport happens to wrap at. */
 .node-break{flex-basis:100%;height:0;padding:0}
.node-fact span{display:block;color:var(--text-2);font-size:.8rem;font-weight:600}
.node-fact strong{display:block;margin-top:auto;padding-top:.15rem;font-size:1.55rem;font-variant-numeric:tabular-nums;text-align:right}
.state-badge{display:inline-block;padding:0 .35rem;border-radius:2px;font-size:.72rem;font-weight:700;letter-spacing:.04em;vertical-align:.08em}
.state-inactive{color:var(--text-2);border:1px solid var(--border-strong)}
.overview-links{display:flex;flex-wrap:wrap;gap:.5rem 1.5rem;margin-top:1rem}
.form-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:1rem 1.25rem}
.form-field{display:flex;flex-direction:column;gap:.3rem;font-weight:600}
.form-field input{width:100%;min-height:2.45rem;border:1px solid var(--border-strong);border-radius:3px;background:var(--surface-1)}
.form-field select{width:100%;min-height:2.45rem;border:1px solid var(--border-strong);border-radius:3px;background:var(--surface-1)}
.form-field small{font-weight:400;color:var(--text-2)}
.form-field-wide{grid-column:1/-1}
.form-actions{display:flex;align-items:center;gap:.75rem;margin-top:1.25rem;padding-top:1rem;border-top:1px solid var(--border)}
 .form-actions button{color:#fff;background:var(--accent);border:1px solid var(--accent);border-radius:3px;font-weight:600;padding:.5rem 1rem}
.credential-note{max-width:64rem;margin:1rem 0;padding:.9rem 1rem;border-left:4px solid var(--accent);background:var(--surface-2)}
.credential-note p{margin:.25rem 0}
.editor-card>summary{cursor:pointer;font-weight:700;font-size:1rem}
.editor-card[open]>summary{margin-bottom:1rem;padding-bottom:.65rem;border-bottom:1px solid var(--border)}
.record-state-action{max-width:64rem;margin:.75rem 0}
.visually-hidden{position:absolute!important;width:1px!important;height:1px!important;padding:0!important;margin:-1px!important;overflow:hidden!important;clip:rect(0,0,0,0)!important;white-space:nowrap!important;border:0!important}
@media (max-width:70rem){
 .record-search{grid-template-columns:minmax(16rem,1fr) auto minmax(10rem,auto) auto}
 .record-choices{grid-column:1/-1;grid-row:2}
 .person-layout{grid-template-columns:1fr}
 .account-summary{grid-template-columns:1fr}
 .service-dashboard{grid-template-columns:repeat(2,minmax(0,1fr))}
}
/* A phone is not a narrow desktop: the gutter shrinks and only a genuinely
    wide table scrolls, rather than the whole page.
    See docs/05-discovery.md#rules-it-is-built-to. */
 @media (max-width:40rem){
  body{margin:0}
  main{padding:1rem}
  .site-header{grid-template-columns:48px minmax(0,1fr);padding:.75rem 1rem}
  .node-logo{width:48px}
  .node-summary{white-space:nowrap}
  .node-navigation{display:block}
  .node-navigation nav{gap:0 1rem}
  .who{display:block;margin:.25rem 0}
 input{width:100%}
 .record-search{grid-template-columns:1fr auto}
 .record-search input[type=search]{grid-column:1/-1}
 .record-choices{grid-column:1/-1;grid-row:auto}
  .record-search label{justify-self:start}
 .record-table,.record-table tbody,.record-table tr,.record-table td{display:block;width:100%}
	.record-table caption{display:block;width:100%}
  .record-table thead{position:absolute;width:1px;height:1px;overflow:hidden;clip-path:inset(50%)}
  .record-table tr{border-bottom:1px solid var(--border);padding:.5rem 0}
  .record-table td{border:0;padding:.2rem .6rem;text-align:left}
  .group-table td:first-child{width:100%;max-width:none}
  .record-table td[data-label]::before{content:attr(data-label) ": ";font-weight:600;color:var(--text-2)}
  .record-table td.num{text-align:left}
 .record-name-cell{min-width:0}
	.node-fact{flex-basis:calc(50% - 1px)}
  .form-grid{grid-template-columns:1fr}
  .form-field-wide{grid-column:auto}
  .service-dashboard{grid-template-columns:1fr}
  .activity-fact{grid-column:auto}
  .task-card{padding:1rem}
  table:not(.record-table):not(.fit-table){display:block;overflow-x:auto}
 }
</style>
`

// The navigation is one list, in one place. It was two — an inline copy on the
// diagnostics page and adminNav on every other — which is how sign-out came to
// exist on one page only (Plans/MVP/done/web-review.md W01).
var navItems = []struct{ Href, Label, Key string }{
	{"/", "Overview", "overview"},
	{"/agents", "Agents", "agents"},
	{"/services", "Services", "services"},
	{"/queues", "Queues", "queues"},
	{"/pubsub", "PubSub", "pubsub"},
	{"/users", "Users", "users"},
	{"/groups", "Groups", "groups"},
	{"/activity", "Activity", "activity"},
	{"/diagnostics", "Diagnostics", "diagnostics"},
}

// shell is the head, title and navigation a signed-in page shares. Most pages
// know their title when the template is parsed, and that literal is escaped
// here because it is built into the template source. A page whose title
// follows daemon-returned or route state uses shellTitle with a fixed,
// repository-owned template expression: the actions in it are escaped by the
// template in title context, so a record or identity may name itself there
// without the expression itself ever coming from a caller. Two open tabs on
// two services are two different titles because of it.
func shell(key, title string) string {
	return shellTitle(key, template.HTMLEscapeString(title))
}

func shellTitle(key, titleTemplate string) string {
	var nav strings.Builder
	nav.WriteString(head)
	nav.WriteString("<title>")
	nav.WriteString(titleTemplate)
	nav.WriteString(" \u00b7 agent-bus</title>\n")
	// Sign out belongs beside the name it signs out, on every page.
	nav.WriteString(`<a class=skip-link href=#main>Skip to main content</a>` + "\n")
	nav.WriteString(frameHeader)
	accountCurrent := ""
	if key == "account" {
		accountCurrent = " aria-current=page"
	}
	nav.WriteString(`<div class=node-navigation><form method=post action=/signout class=who><a class=account-link href=/account` + accountCurrent + `><code>{{.You}}</code></a> <button type=submit>sign out</button></form>` + "\n")
	nav.WriteString("<nav aria-label=\"sections\">")
	for _, item := range navItems {
		// The section's own title mark, decorative here for the same reason:
		// the link text beside it already names the section.
		mark := string(titleMark(item.Key))
		if key == "records" && (item.Key == "agents" || item.Key == "services" || item.Key == "queues" || item.Key == "pubsub") {
			// Personal is a view of the agents, so it marks that entry.
			if item.Key == "agents" {
				nav.WriteString(`{{if or (eq .Current "agents") (eq .Current "personal")}}<a href=` + item.Href + ` aria-current=page>{{else}}<a href=` + item.Href + `>{{end}}` + mark + item.Label + `</a>`)
			} else {
				nav.WriteString(`{{if eq .Current "` + item.Key + `"}}<a href=` + item.Href + ` aria-current=page>{{else}}<a href=` + item.Href + `>{{end}}` + mark + item.Label + `</a>`)
			}
			continue
		}
		nav.WriteString("<a href=" + item.Href)
		if item.Key == key {
			nav.WriteString(" aria-current=page")
		}
		nav.WriteString(">" + mark + item.Label + "</a>")
	}
	nav.WriteString("</nav></div>\n" + frameHeaderEnd + "<main>\n<a id=main tabindex=-1></a>\n")
	return nav.String()
}

// The sign-in page shares the public node identity, and no private status.
// See docs/05-discovery.md#what-a-node-says-about-itself.
type signin struct {
	pageInfo
	Refused string
	Return  string
}

var anon = template.Must(template.New("anon").Funcs(template.FuncMap{"titleMark": titleMark}).Parse(head + `<title>Sign in · agent-bus</title><a class=skip-link href=#main>Skip to main content</a>` + frameHeader + frameHeaderEnd + `
<main>
<a id=main tabindex=-1></a>
<div class=landing>
<p class=hero><img src=/agent-bus.jpg alt="A red double-decker named Agents Bus, carrying AI and non-AI riders: Claude, OpenAI, Slack, Telegram, Email and a shell" width=648 height=432></p>
<h1>One bus for agents, bots and services</h1>
<p class=lede>Connect AI and NON-AI agents, bots and services so they can find and message each other. One daemon gives you a registry, message queues, an MCP server, dashboard and much more&hellip;</p>
<div class=landing-features>
<div class=landing-feature><strong>{{titleMark "identity"}} Registry</strong>Who and what is on the bus: users, agents, queues, services and groups. Every record has an owner and a list of who may reach it.</div>
<div class=landing-feature><strong>{{titleMark "queue"}} Messages</strong>Queues hold what was sent until somebody reads it. Pub/sub copies one publication to everyone subscribed.</div>
<div class=landing-feature><strong>{{titleMark "agents"}} MCP server</strong>An agent reaches the bus through MCP, so finding a peer and sending it a message are tools the model already knows how to call.</div>
<div class=landing-feature><strong>{{titleMark "overview"}} Dashboard</strong>This web face, once you are signed in: what is registered, what is waiting, and what has gone wrong.</div>
</div>
<p class=landing-links><a href="https://github.com/parf/ai-agent-bus">GitHub &mdash; docs &amp; updates</a> &middot; by <a href="https://parf.dev/">Serg Parf</a></p>
</div>

<div class=signin-card>
<div class=page-title><h2>{{titleMark "credentials"}} Sign in</h2><button type=button class=help-button popovertarget=token-help aria-label="How to get a token" data-tooltip="A token is what every call carries. Run agent-bus-token &lt;name&gt; on the box, or ssh agent-busd@&lt;node&gt; token from anywhere your key reaches.">&#9432;</button></div>
<div popover id=token-help class=context-help><h2>Getting a token</h2><ul>
<li>On this box: <code>agent-bus-token &lt;name&gt;</code></li>
<li>From anywhere your key reaches: <code>ssh agent-busd@&lt;node&gt; token</code></li>
<li>Asking again returns the token you already have. It does not expire on its own.</li>
</ul></div>
<form method=post action=/signin>
{{with .Return}}<input type=hidden name=return value="{{.}}">{{end}}
<label for=token>token</label>
<div class=signin-row><input id=token type=password name=token autofocus><button type=submit>sign in</button></div>
{{with .Refused}}<p class=warn>{{.}}{{end}}
</form>
</div>
</main>
`))

var overviewPage = template.Must(template.New("overview").Funcs(template.FuncMap{"titleMark": titleMark, "number": number, "figure": figure}).Parse(shell("overview", "Overview") + `<div class=page-title><h1>{{titleMark "overview"}} Overview</h1><button type=button class=help-button popovertarget=overview-help aria-label="About Overview" data-tooltip="Only enumerated observations appear. An empty list does not claim the node is healthy.">ⓘ</button></div><div popover id=overview-help class=context-help><h2>Overview scope</h2><ul><li>Attention items cover the conditions the daemon reports over records visible to you, plus node-wide refusals, the previous-stop marker and, for Administrators, records inactive because their owner is.</li><li>A backlog by itself is ordinary work and is not called unhealthy.</li><li>The node totals and your caller-visible lists have different scopes and never have to agree.</li></ul></div>

{{if .Attention}}<section class=dashboard-section aria-labelledby=attention><h2 id=attention>Needs attention</h2>
<div class=attention-list>{{range .Attention}}<article class="attention-item attention-{{.Level}}"><h3>{{.Title}}</h3>
{{if eq .Kind "refusal"}}<p><code>{{.Reason}}</code> · {{number .Count}} since this daemon started</p>
{{else if eq .Kind "owner-inactive"}}<p>{{number .Count}} {{if eq .Count 1}}record{{else}}records{{end}} · {{number .Queued}} {{if eq .Queued 1}}message{{else}}messages{{end}} held · node-wide; each returns when its owner is reactivated</p>
{{else if eq .Kind "record"}}<p><code>{{.Name}}</code> · {{number .Queued}} held now{{with .Oldest}} · oldest {{.}}{{end}}{{if .Inactive}} · inactive{{end}}{{if .AtBound}} · at capacity{{end}}{{with .Overflow}} · {{.}}{{end}}{{if .Dropped}} · {{number .Dropped}} dropped{{end}}{{if .Expired}} · {{number .Expired}} expired{{end}}</p>
{{else}}<p>Memory from the previous run may not have reached the snapshot.</p>{{end}}
<p class=muted><a href="{{.Href}}">{{.Link}}</a></p></article>{{end}}</div>
</section>{{end}}

<section class=dashboard-section aria-labelledby=node><div class=page-title><h2 id=node>This node</h2><button type=button class=help-button popovertarget=node-help aria-label="About node totals" data-tooltip="Whole-node values. Caller-visible lists may show a smaller set.">ⓘ</button></div><div popover id=node-help class=context-help><h2>Node totals</h2><ul><li>These values cover the whole daemon.</li><li>The first row is how the node stands right now; the second is what has happened since it started.</li><li>A dash is none. It is the same answer as zero, written so a quiet node does not read as a page of readings to check.</li><li>Agents, Services, Queues, PubSub, Users and Groups count records by kind, node-wide. The six pages of those names show only what you may see, so their counts never have to agree with this strip.</li><li>Readers counts outstanding consume requests, not processes, sessions or health.</li><li>Calls counts HTTP requests reaching the daemon, node-wide, including refused ones. A window the daemon has not observed yet says so rather than reading zero.</li></ul></div>
<div class=node-strip><div class=node-fact><span>Readers</span><strong>{{figure .Status.Waiting}}</strong></div><div class=node-fact><span>Queued</span><strong>{{figure .Status.Queued}}</strong></div>{{range .Totals}}<div class=node-fact><span>{{.Label}}</span><strong>{{figure .Count}}</strong></div>{{else}}<div class=node-fact><span>Records</span><strong>{{figure .Status.Services}}</strong></div>{{end}}<div class=node-break aria-hidden=true></div><div class=node-fact><span>Uptime</span><strong>{{.Status.Up}}</strong></div>
{{with .Frame.Node}}{{with .Calls}}{{range .Windows}}<div class=node-fact><span>Calls, {{if eq .Window "1m"}}minute{{else}}hour{{end}}</span>{{if .Available}}<strong>{{figure .Count}}</strong>{{else}}<strong class=node-fact-note>collecting history</strong>{{end}}</div>{{end}}<div class=node-fact><span>Calls, total</span><strong>{{figure .Total}}</strong></div>{{else}}<div class=node-fact><span>Calls</span><strong class=node-fact-note>unavailable</strong></div>{{end}}{{else}}<div class=node-fact><span>Calls</span><strong class=node-fact-note>unavailable</strong></div>{{end}}</div>
<p class=muted>Node-wide. The lists linked below contain only records visible to you; the two never have to agree.</p></section>
<nav class=overview-links aria-label="Find records"><strong>Find</strong><a href="/agents?sort=queued&amp;work=held">Agents holding work</a><a href="/queues?sort=queued&amp;work=held">Queues holding work</a><a href="/services">External services</a></nav>
`))

// Diagnostics stays detailed and caller-scoped. It no longer duplicates the
// registry catalogue; Services, Queues and PubSub now carry every record fact that
// table uniquely exposed.
var diagnosticsPage = template.Must(template.New("diagnostics").Funcs(template.FuncMap{"readerCount": readerCount, "recordHref": recordHref, "titleMark": titleMark, "number": number}).Parse(shell("diagnostics", "Diagnostics") + `<div class=page-title><h1>{{titleMark "diagnostics"}} Diagnostics</h1><button type=button class=help-button popovertarget=diagnostics-help aria-label="About diagnostics" data-tooltip="Caller-visible queues, loss and retained envelope evidence. Bodies are never shown.">ⓘ</button></div><div popover id=diagnostics-help class=context-help><h2>Diagnostics scope</h2><ul><li>Refusal counts cover the whole daemon; queue, loss and envelope sections contain only facts visible to you.</li><li>History is bounded and process-local.</li><li>Envelope metadata may be shown, but message bodies never are.</li></ul></div>
<p><a href=/diagnostics>Refresh</a></p>

<section class=dashboard-section><div class=page-title><h2 id=refusals>Refusals</h2><button type=button class=help-button popovertarget=refusals-help aria-label="About refusal counts" data-tooltip="Whole-node handled API refusals since process start. Zero is measured; router misses and internal failures are excluded.">ⓘ</button></div>
<div popover id=refusals-help class=context-help><h2>Refusal counts</h2><ul><li>The reason set is closed, so absence from the sparse daemon map becomes a zero measurement here.</li><li>Counts include handled API refusals whatever the caller&rsquo;s standing, including malformed requests and bad credentials.</li><li>A request the router rejected before any handler ran is not a caller refusal; internal failures are not counted either.</li><li>These lifetime values cannot say how quickly refusals are rising.</li></ul></div>
<table class=fit-table><caption>Refusals since this daemon started, by reason</caption><thead><tr><th scope=col>Reason<th scope=col class=num>Count</tr></thead><tbody>{{range .Refusals}}<tr><td><code>{{.Reason}}</code><td class=num>{{number .Count}}</tr>{{end}}</tbody></table></section>

<section class=dashboard-section><div class=page-title><h2 id=stuck>Inboxes holding messages</h2><button type=button class=help-button popovertarget=backlog-help aria-label="About held messages" data-tooltip="A held message is not automatically stuck. Readers counts current requests, not health; expired work may remain until pruning; capacity is only what was true when observed.">ⓘ</button></div><div popover id=backlog-help class=context-help><h2>Held messages</h2><ul><li>A scheduled reader may simply be between pulls.</li><li>This observation does not prune first, so held work may already have outlived its TTL.</li><li>At capacity records what was true when observed, never the next send.</li><li>Readers counts outstanding reads, filtered and unfiltered together. Zero is not health, and a positive count promises neither a match nor completed work.</li></ul></div>
<table class=fit-table><caption>Inboxes holding messages, longest wait first — visible to you</caption><thead><tr><th scope=col>Name<th scope=col class=num>Readers<th scope=col class=num>Held now<th scope=col class=num>Oldest held<th scope=col>Capacity</tr></thead><tbody>
{{range .Backlogs}}<tr><td><a href="{{recordHref .}}"><code>{{.Name}}</code></a><td class=num>{{readerCount .Readers}}<td class=num>{{number .Queued}}<td class=num>{{if .Oldest}}{{.Oldest}}{{else}}<span class=muted>&mdash;</span>{{end}}<td>{{if .AtBound}}<b class=warn>at capacity when observed</b>{{else}}<span class=muted>&mdash;</span>{{end}}</tr>
{{else}}<tr><td colspan=5 class=muted>every queue you can see is empty</tr>{{end}}</tbody></table></section>

` + exchangesTemplate + `
<section class=dashboard-section><div class=page-title><h2 id=loss>Loss by name</h2><button type=button class=help-button popovertarget=loss-help aria-label="About message loss" data-tooltip="Dropped is queue overflow; Expired is retention. Only caller-visible records appear.">ⓘ</button></div><div popover id=loss-help class=context-help><h2>Message loss</h2><ul><li>Dropped counts overflow decisions for the named inbox.</li><li>Expired counts messages removed by retention.</li><li>Only records visible to you appear here.</li></ul></div>
<table class=fit-table aria-labelledby=loss><thead><tr><th scope=col>Name<th scope=col class=num>Dropped<th scope=col class=num>Expired</tr></thead><tbody>{{range .Losses}}<tr><td><a href="{{recordHref .}}"><code>{{.Name}}</code></a><td class=num>{{number .Dropped}}<td class=num>{{number .Expired}}</tr>{{else}}<tr><td colspan=3 class=muted>nothing lost</tr>{{end}}</tbody></table></section>
{{if .Leftovers}}<section class=dashboard-section aria-labelledby=leftovers><div class=page-title><h2 id=leftovers>Leftover names</h2><button type=button class=help-button popovertarget=leftovers-help aria-label="About leftover names" data-tooltip="Names that are neither a User nor an Agent. Rare; usually left by a record ignored at load.">ⓘ</button></div><div popover id=leftovers-help class=context-help><h2>Leftover names</h2><ul><li>Every name is a User or an Agent. These are neither, and appear here only while one exists.</li><li>A credential with no record is usually left by a record ignored at load, and kept so repairing that record finds its credential.</li><li>A self-owned record with no User profile has no User to answer for it; inspect it before deciding whether it is needed.</li></ul></div>
<table class="leftovers-table fit-table" aria-labelledby=leftovers><thead><tr><th scope=col>Name<th scope=col>What it is<th scope=col>Next step</tr></thead><tbody>{{range .Leftovers}}<tr><td><code>{{.Name}}</code><td>{{if eq .Kind "record"}}Self-owned record, no User profile{{else}}Credential with no record{{end}}<td>{{if eq .Kind "record"}}<a href="/user?name={{.Name}}&return=/diagnostics">Inspect before deciding</a>{{else if .CanRemove}}<a href="/user?name={{.Name}}&return=/diagnostics">Review credential removal</a>{{else}}An authorized administrator can review removal.{{end}}</tr>{{end}}</tbody></table></section>{{end}}
`))

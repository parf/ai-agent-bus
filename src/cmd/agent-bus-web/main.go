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
	bus := &caller{client, base, os.Getenv("AGENT_BUS_TOKEN")}

	http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		var v view
		if err := bus.get("/status", &v.Status); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if err := bus.get("/ls", &v.Records); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if err := bus.get("/recent", &v.Recent); err != nil {
			// The feed is the owner's until it can be filtered per caller,
			// so a dashboard run as anyone else shows the rest and says so.
			v.NoFeed = err.Error()
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := page.Execute(w, v); err != nil {
			log.Printf("render: %v", err)
		}
	})
	tls := have(*certF) && have(*keyF)
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
	Status  core.Status
	Records []protocol.Record
	Recent  []protocol.Envelope
	NoFeed  string
}

// caller is a token and somewhere to send it. An empty one is left off: on
// its own socket the daemon supplies the identity.
// See docs/02-access.md#local-socket.
type caller struct {
	client *http.Client
	base   string
	token  string
}

func (c *caller) get(path string, into any) error {
	req, err := http.NewRequest("GET", c.base+path, nil)
	if err != nil {
		return err
	}
	if c.token != "" {
		req.Header.Set(api.HeaderToken, c.token)
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
	return json.Unmarshal(body, into)
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// One page, refreshed by the browser. No JavaScript, no assets: a view of
// the bus should not need a build step to read.
var page = template.Must(template.New("dash").Parse(`<!doctype html>
<meta charset="utf-8"><meta http-equiv="refresh" content="5">
<title>agent-bus</title>
<style>
 body{font:14px system-ui,sans-serif;margin:2rem;max-width:60rem}
 table{border-collapse:collapse;width:100%;margin-bottom:2rem}
 th,td{text-align:left;padding:.3rem .6rem;border-bottom:1px solid #ddd}
 th{font-weight:600;color:#555}
 code{font:13px ui-monospace,monospace}
 .muted{color:#888}
</style>
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
<p class=muted>Envelopes only — the bus never sees a body.</p>
`))

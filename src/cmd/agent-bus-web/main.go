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
	"net/http"
	"os"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func main() {
	addr := flag.String("addr", env("AGENT_BUS_WEB_ADDR", "127.0.0.1:7878"), "where the dashboard listens — loopback only")
	flag.Parse()

	client, base := api.Dial(os.Getenv("AGENT_BUS_ADDR"))
	bus := &caller{client, base, os.Getenv("AGENT_BUS_NAME"), os.Getenv("AGENT_BUS_TOKEN")}

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
	log.Printf("agent-bus-web on http://%s", *addr)
	srv := &http.Server{Addr: *addr, Handler: nil, ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

type view struct {
	Status  core.Status
	Records []protocol.Record
	Recent  []protocol.Envelope
	NoFeed  string
}

// caller is the two parameters and somewhere to send them. Empty ones are
// left off: on its own socket the daemon supplies both.
// See docs/02-access.md#local-socket.
type caller struct {
	client      *http.Client
	base        string
	name, token string
}

func (c *caller) get(path string, into any) error {
	req, err := http.NewRequest("GET", c.base+path, nil)
	if err != nil {
		return err
	}
	if c.name != "" {
		req.Header.Set(api.HeaderUser, c.name)
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

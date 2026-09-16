package main

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/version"
)

// Authenticated status keeps the visitor's identity separate from the public node identity.
type nodeStatus struct {
	core.Status
	You           string `json:"you"`
	Administrator bool   `json:"administrator"`
	DaemonOwner   bool   `json:"daemon_owner"`
}

type pageInfo struct {
	Node *protocol.NodeIdentity
}

// An embedded method lets every HTML view share the frame without changing
// the SVG avatar response. Local build facts remain explicitly web facts.
func (p pageInfo) Frame() pageInfo    { return p }
func (p pageInfo) WebVersion() string { return version.String }
func (p pageInfo) WebBuild() string   { return version.Build }

type pageInfoKey struct{}

// The node publishes this subset to everyone. No session or privileged
// fallback is sent, including on sign-in and error pages.
func (c *caller) pageRequest(r *http.Request) *http.Request {
	p := &pageInfo{}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", c.base+"/identity", nil)
	if err == nil {
		var response *http.Response
		response, err = c.client.Do(req)
		if err == nil {
			defer response.Body.Close()
			if response.StatusCode == http.StatusOK {
				var node protocol.NodeIdentity
				if err = json.NewDecoder(response.Body).Decode(&node); err == nil {
					p.Node = &node
				}
			}
		}
	}
	// An unavailable or older daemon has no identity to display. Never borrow
	// the web child's version or uptime to fill that gap.
	return r.WithContext(context.WithValue(r.Context(), pageInfoKey{}, p))
}

func requestInfo(r *http.Request) pageInfo {
	if p, ok := r.Context().Value(pageInfoKey{}).(*pageInfo); ok {
		return *p
	}
	return pageInfo{}
}

func (c *caller) status(r *http.Request) (nodeStatus, error) {
	var node nodeStatus
	err := c.get(cookie(r), "/status", &node)
	return node, err
}

const frameHeader = `<header class=site-header aria-label="Node information">
<div class=node-summary>
<strong>AgentBus {{with .Frame.Node}}{{if .Version}}V{{.Version}}{{else}}version unavailable{{end}}{{else}}version unavailable{{end}}</strong>
{{with .Frame.Node}}<span>{{if .Host}}{{.Host}}{{else}}host unavailable{{end}}</span>
<span>{{if .Up}}{{.Up}} up{{else}}uptime unavailable{{end}}</span>
<span>owner: <code>{{if .Owner}}{{.Owner}}{{else}}unavailable{{end}}</code></span>
{{else}}<span>Node information unavailable</span>{{end}}
</div>
{{with .Frame.Node}}<div class=node-load>
<div>Host load <small>(1 / 5 / 15 min)</small>: {{if .Load}}<code>{{printf "%.2f" (index .Load 0)}} / {{printf "%.2f" (index .Load 1)}} / {{printf "%.2f" (index .Load 2)}}</code>{{else}}unavailable{{end}}</div>
<div class=message-windows aria-label="Sampled message counts">{{range .Messages}}<span><strong>~{{.Window}}</strong>: {{if .Available}}{{.Accepted}} accepted / {{.Dequeued}} dequeued <small>(observed {{.Observed}})</small>{{else}}collecting history{{end}}</span>{{else}}<span>Message counts unavailable</span>{{end}}</div>
</div>{{end}}
</header>
`

var frameFooter = template.Must(template.New("footer").Parse(`</main>
<footer class=site-footer aria-label="Build information">
{{with .Node}}<div>Daemon build: <code>{{if .Build}}{{.Build}}{{else}}unavailable{{end}}</code></div>{{end}}
<div>Web <code>v{{.WebVersion}}</code> · build: <code>{{.WebBuild}}</code></div>
<details><summary>About load readings</summary>Host load is the OS load average, not CPU utilisation. Message totals count accepted inbox deliveries (including subscriber copies) and dequeues, not API calls or completed work. Zero deliveries does not mean the node is idle; a publication with no subscribers counts no copies. Dequeues may exceed arrivals when older work is drained. Windows use minute samples plus the current partial interval, from this daemon run only; the observed span is shown for each.</details>
</footer>
</html>`))

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
{{with .Calls}}<span class=call-counts>{{range .Windows}}{{if eq .Window "1m"}}min:{{else}}hr:{{end}} {{if .Available}}<strong>{{.Count}}</strong> <small>(observed {{.Observed}})</small>{{else}}collecting history{{end}} ; {{end}}total: <strong>{{.Total}}</strong></span>{{else}}<span>min: unavailable; hr: unavailable; total: unavailable</span>{{end}}
{{else}}<span>Node information unavailable</span>{{end}}
</div>
</header>
`

var frameFooter = template.Must(template.New("footer").Parse(`</main>
<footer class=site-footer aria-label="Build information">
{{with .Node}}<div>Daemon build: <code>{{if .Build}}{{.Build}}{{else}}unavailable{{end}}</code></div>{{end}}
<div>Web <code>v{{.WebVersion}}</code> · build: <code>{{.WebBuild}}</code></div>
<details><summary>About call counts</summary>
HTTP requests entering the daemon on all listeners, including refused requests, router 404/405 responses, this page's queries (including /identity itself), and long polls still in progress. A long poll counts when it starts, not when it finishes. These are not completed operations or the separate refusal totals. Total is exact since this process started; the minute and hour windows use minute samples and the current total, and each shows its observed span. Loading or reloading a page adds calls; leaving it open does not refresh it.</details>
</footer>
</html>`))

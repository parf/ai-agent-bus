package main

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
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
	// At is when this page was built, stated once in the shared footer. No
	// page refreshes itself, so every page has to say how old what you are
	// reading is; one value per request keeps the body and the footer from
	// disagreeing.
	At string
}

// An embedded method lets every HTML view share the frame without changing
// the SVG avatar response. Local build facts remain explicitly web facts.
func (p pageInfo) Frame() pageInfo { return p }

type pageInfoKey struct{}

// The node publishes this subset to everyone. No session or privileged
// fallback is sent, including on sign-in and error pages.
func (c *caller) pageRequest(r *http.Request) *http.Request {
	p := &pageInfo{At: time.Now().Format("2006-01-02 15:04:05")}
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

// shell closes the header after navigation; the anonymous page closes it directly.
const frameHeader = `<header class=site-header aria-label="Site header">
` + nodeLogo + `
<div class=node-summary>
{{with .Frame.Node}}<strong>AgentBus <span{{with .Build}} class=build-tip tabindex=0 title="Build daemon {{.}}"{{end}}>v{{if .Version}}{{.Version}}{{else}}unavailable{{end}}</span></strong> <span>@ {{if .Host}}{{.Host}}{{else}}host unavailable{{end}}</span>{{else}}<strong>AgentBus</strong> <span>node unavailable</span>{{end}}
</div>
`

const frameHeaderEnd = `</header>
`

var frameFooter = template.Must(template.New("footer").Funcs(template.FuncMap{"number": number}).Parse(`</main>
<footer class=site-footer aria-label="Node and build information">
{{with .Node}}<div class=footer-node><span><strong>Owner</strong> <code>{{if .Owner}}{{.Owner}}{{else}}unavailable{{end}}</code></span><span><strong>Uptime</strong> {{if .Up}}{{.Up}}{{else}}unavailable{{end}}</span>{{with $.At}}<span><strong>Generated</strong> {{.}}</span>{{end}}</div>{{else}}<div class=footer-node><span>Node information unavailable</span>{{with .At}}<span><strong>Generated</strong> {{.}}</span>{{end}}</div>{{end}}
</footer>
</html>`))

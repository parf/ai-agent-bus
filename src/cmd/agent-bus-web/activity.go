package main

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/parf/ai-agent-bus/internal/core"
)

type graph struct {
	Label, Points string
	Max           int
}

func activityGraphs(points []core.ActivityPoint) []graph {
	metrics := []struct {
		label string
		get   func(core.Counts) int
	}{
		{"Accepted", func(c core.Counts) int { return c.In }},
		{"Dequeued", func(c core.Counts) int { return c.Out }},
		{"Dropped", func(c core.Counts) int { return c.Dropped }},
		{"Expired", func(c core.Counts) int { return c.Expired }},
		{"Refused", func(c core.Counts) int { return c.Refused }},
	}
	out := []graph{}
	for _, metric := range metrics {
		g := graph{Label: metric.label}
		for _, p := range points {
			g.Max = max(g.Max, metric.get(p.Counts))
		}
		xy := []string{}
		for i, p := range points {
			xy = append(xy, fmt.Sprintf("%.1f,%.1f", 10+580*float64(i)/float64(max(1, len(points)-1)), 100-90*float64(metric.get(p.Counts))/float64(max(1, g.Max))))
		}
		g.Points = strings.Join(xy, " ")
		out = append(out, g)
	}
	return out
}
func (c *caller) activityRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /activity", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		name := r.URL.Query().Get("name")
		if err := c.get(cookie(r), "/ls", &v.Records); err != nil {
			adminError(w, err)
			return
		}
		sort.Slice(v.Records, func(i, j int) bool { return v.Records[i].Name < v.Records[j].Name })
		var points []core.ActivityPoint
		if err := c.get(cookie(r), "/activity?name="+url.QueryEscape(name), &points); err != nil {
			adminError(w, err)
			return
		}
		render(w, activityPage, struct {
			adminView
			Name   string
			Graphs []graph
			Points []core.ActivityPoint
		}{v, name, activityGraphs(points), points})
	})
}

var activityPage = template.Must(template.New("activity").Parse(head + adminNav + `
<meta http-equiv=refresh content=30><h1>Activity graphs</h1>
<form method=get><label>Service or channel <select name=name><option value="">All visible</option>{{range .Records}}<option value="{{.Name}}" {{if eq .Name $.Name}}selected{{end}}>{{.Name}}</option>{{end}}</select></label><button>Filter</button></form>
<p>Counts per sample, approximately one minute, for the last hour. The final sample is still in progress. History starts when the daemon starts. Dequeued means handed to a reader, not successful execution.</p>
{{if .Points}}{{range .Graphs}}<h2>{{.Label}}</h2><svg viewBox="0 0 600 110" role=img aria-label="{{.Label}}; maximum {{.Max}} per sample" style="width:100%;max-height:160px"><path d="M10 10 V100 H590" fill=none stroke="#888"/><polyline points="{{.Points}}" fill=none stroke="#2255aa" stroke-width=2/></svg><p>Maximum: {{.Max}}</p>{{end}}
<details><summary>Sample values</summary><table><tr><th>Time<th>Accepted<th>Dequeued<th>Dropped<th>Expired<th>Refused</tr>{{range .Points}}<tr><td>{{.At.Format "15:04:05"}}<td>{{.In}}<td>{{.Out}}<td>{{.Dropped}}<td>{{.Expired}}<td>{{.Refused}}</tr>{{end}}</table></details>
{{else}}<p>Collecting the first sample.</p>{{end}}`))

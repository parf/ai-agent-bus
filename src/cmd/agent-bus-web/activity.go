package main

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
)

type activitySeries struct {
	Label, Class, Points string
	Total                int
	Single               bool
	X, Y                 string
}

type activityPresentation struct {
	Scope, Start, End, Observed, Uptime string
	Named, ShowTable                    bool
	Max                                 int
	Series                              []activitySeries
	Zero                                []string
	Points                              []core.ActivityPoint
	Unavailable                         string
}

type activityMetric struct {
	label, class string
	get          func(core.Counts) int
}

var activityMetrics = []activityMetric{
	{"Accepted", "activity-accepted", func(c core.Counts) int { return c.In }},
	{"Dequeued", "activity-output", func(c core.Counts) int { return c.Out }},
	{"Dropped", "activity-dropped", func(c core.Counts) int { return c.Dropped }},
	{"Expired", "activity-expired", func(c core.Counts) int { return c.Expired }},
	{"Refused", "activity-refused", func(c core.Counts) int { return c.Refused }},
}

// activityView gives detail and the complete Activity page the same displayed
// series and therefore the same scale for the same daemon answer. X positions
// come from timestamps rather than sample indexes; irregular samples stay
// irregular on the chart.
func activityView(points []core.ActivityPoint, scope, uptime string, table bool) activityPresentation {
	v := activityPresentation{Scope: scope, Named: scope != "", Uptime: uptime, ShowTable: table, Points: points}
	if len(points) == 0 {
		return v
	}
	start, end := points[0].At, points[len(points)-1].At
	v.Start, v.End = start.Format("Jan 2 15:04:05"), end.Format("Jan 2 15:04:05")
	span := end.Sub(start)
	if span < 0 {
		span = 0
	}
	if len(points) == 1 {
		v.Observed = "one partial sample"
	} else {
		v.Observed = span.Round(time.Second).String()
	}

	maxima := make([]int, len(activityMetrics))
	totals := make([]int, len(activityMetrics))
	for i, metric := range activityMetrics {
		for _, point := range points {
			value := metric.get(point.Counts)
			maxima[i] = max(maxima[i], value)
			totals[i] += value
		}
		v.Max = max(v.Max, maxima[i])
		if maxima[i] == 0 {
			v.Zero = append(v.Zero, metric.label)
		}
	}
	for i, metric := range activityMetrics {
		if maxima[i] == 0 {
			continue
		}
		xy := make([]string, 0, len(points))
		for _, point := range points {
			x := 330.0
			if span > 0 {
				x = 50 + 560*float64(point.At.Sub(start))/float64(span)
			}
			y := 125 - 100*float64(metric.get(point.Counts))/float64(v.Max)
			xy = append(xy, fmt.Sprintf("%.1f,%.1f", x, y))
		}
		series := activitySeries{Label: metric.label, Class: metric.class, Points: strings.Join(xy, " "), Total: totals[i]}
		if len(xy) == 1 {
			series.Single, series.X, series.Y = true, "330.0", strings.SplitN(xy[0], ",", 2)[1]
		}
		v.Series = append(v.Series, series)
	}
	return v
}

const activityViewTemplate = `{{define "activity-help"}}<ul><li>The daemon keeps at most one hour of in-memory history and starts fresh after restart.</li><li>Samples are usually about one minute apart; the visible timestamps are the measured interval.</li><li>Dequeued means handed to a reader, not completed work.</li><li>Zero is measured; collecting history means no interval has been observed yet.</li></ul>{{end}}
{{define "activity-view"}}{{with .Activity}}
{{if not .ShowTable}}<div class=page-title><h2>Activity</h2><button type=button class=help-button popovertarget=record-activity-help aria-label="About this activity history">ⓘ</button></div><div popover id=record-activity-help class=context-help><h2>About activity</h2>{{template "activity-help" .}}</div>{{end}}
{{if .Unavailable}}<p class=muted>Activity unavailable: {{.Unavailable}}</p>
{{else if .Points}}
<p>{{if .Named}}Scope: <code>{{.Scope}}</code>{{else}}Scope: currently visible records. On this unfiltered view, Refused is node-wide for the daemon Owner or a configured master and covers visible records for other callers{{end}}. Observed {{.Start}} to {{.End}} ({{.Observed}}). Counts are per sample; the final sample is partial.</p>
<p class=muted>History starts fresh after a daemon restart; current uptime is {{if .Uptime}}{{.Uptime}}{{else}}unavailable{{end}}. Dequeued means handed to a reader, not completed.</p>
{{if .Series}}<figure class=activity-figure><svg class=activity-chart viewBox="0 0 640 170" role=img aria-label="Activity per sample from {{.Start}} to {{.End}}; shared maximum {{.Max}} over the displayed nonzero series">
<path class=activity-axis d="M50 25 V125 H610"/><text x=10 y=30>{{.Max}}</text><text x=34 y=130>0</text><text x=50 y=152>{{.Start}}</text><text x=610 y=152 text-anchor=end>{{.End}}</text>
{{range .Series}}<polyline class="activity-line {{.Class}}" points="{{.Points}}"/>{{if .Single}}<circle class="activity-point {{.Class}}" cx="{{.X}}" cy="{{.Y}}" r=4/>{{end}}{{end}}</svg>
<figcaption>Shared scale: 0–{{.Max}} per sample over the displayed nonzero series.</figcaption>
<ul class=activity-legend>{{range .Series}}<li><span class="activity-swatch {{.Class}}" aria-hidden=true></span>{{.Label}}: {{.Total}} in the shown samples</li>{{end}}</ul></figure>
{{else}}<p>All five observed activity series are measured zero in this window.</p>{{end}}
{{with .Zero}}<p class=muted>Measured zero throughout: {{join . ", "}}.</p>{{end}}
{{if .ShowTable}}<details><summary>Sample values</summary><table><thead><tr><th scope=col>Time<th scope=col>Accepted<th scope=col>Dequeued<th scope=col>Dropped<th scope=col>Expired<th scope=col>Refused</tr></thead><tbody>{{range .Points}}<tr><td>{{.At.Format "Jan 2 15:04:05"}}<td>{{.In}}<td>{{.Out}}<td>{{.Dropped}}<td>{{.Expired}}<td>{{.Refused}}</tr>{{end}}</tbody></table></details>{{end}}
{{else}}<p>Activity history is not observed yet. Collecting the first sample after this daemon restart{{if .Uptime}} (uptime: {{.Uptime}}){{end}}; no zero series is inferred.</p>{{end}}
{{end}}{{end}}`

func (c *caller) activityRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /activity", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		name := r.URL.Query().Get("name")
		if err := c.get(cookie(r), "/ls", &v.Records); err != nil {
			fail(w, r, v.You, err)
			return
		}
		sort.Slice(v.Records, func(i, j int) bool { return v.Records[i].Name < v.Records[j].Name })
		var points []core.ActivityPoint
		if err := c.get(cookie(r), "/activity?name="+url.QueryEscape(name), &points); err != nil {
			fail(w, r, v.You, err)
			return
		}
		render(w, activityPage, struct {
			adminView
			Name     string
			Activity activityPresentation
		}{v, name, activityView(points, name, v.Status.Up, true)})
	})
}

var activityPage = template.Must(template.New("activity").Funcs(template.FuncMap{"join": strings.Join, "titleMark": titleMark}).Parse(shell("activity", "Activity graphs") + `
<div class=page-title><h1>{{titleMark "activity"}} Activity graphs</h1><button type=button class=help-button popovertarget=activity-help aria-label="About activity history">ⓘ</button></div>
<div popover id=activity-help class=context-help><h2>About activity</h2>{{template "activity-help" .}}</div>
<form method=get><label>Service or channel <select name=name><option value="">All visible</option>{{range .Records}}<option value="{{.Name}}" {{if eq .Name $.Name}}selected{{end}}>{{.Name}}</option>{{end}}</select></label><button>Filter</button></form>
{{template "activity-view" .}}` + activityViewTemplate))

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
}

// activityTick is one hour on the day axis; Label is set on every third.
type activityTick struct {
	X     string
	Label string
}

type activityPresentation struct {
	Scope, Start, End, Uptime string
	Named, ShowTable          bool
	AllZero                   bool
	Max                       int
	Series                    []activitySeries
	Ticks                     []activityTick
	Zero                      []string
	Points                    []core.ActivityPoint
	Unavailable               string
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

// activityX is where slot i of a day sits on the fixed 24-hour axis.
func activityX(i, slots int) float64 {
	if slots < 2 {
		return 330
	}
	return 50 + 560*float64(i)/float64(slots-1)
}

// activityView gives detail and the complete Activity page the same series
// and therefore the same scale for the same daemon answer: the last day, one
// point per ten-minute slot on a fixed axis with a tick at every hour. A slot
// nobody counted is zero like any quiet one, so every line is continuous.
func activityView(points []core.ActivityPoint, scope, uptime string, table bool) activityPresentation {
	v := activityPresentation{Scope: scope, Named: scope != "", Uptime: uptime, ShowTable: table, Points: points}
	if len(points) == 0 {
		return v
	}
	v.Start = points[0].At.Format("Jan 2 15:04")
	v.End = points[len(points)-1].At.Add(10 * time.Minute).Format("Jan 2 15:04")
	for i, p := range points {
		if p.At.Minute() != 0 {
			continue
		}
		tick := activityTick{X: fmt.Sprintf("%.1f", activityX(i, len(points)))}
		if p.At.Hour()%3 == 0 {
			tick.Label = p.At.Format("15:04")
		}
		v.Ticks = append(v.Ticks, tick)
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
	v.AllZero = len(v.Zero) == len(activityMetrics)
	for i, metric := range activityMetrics {
		if maxima[i] == 0 {
			continue
		}
		xy := make([]string, 0, len(points))
		for j, point := range points {
			y := 125 - 100*float64(metric.get(point.Counts))/float64(v.Max)
			xy = append(xy, fmt.Sprintf("%.1f,%.1f", activityX(j, len(points)), y))
		}
		v.Series = append(v.Series, activitySeries{Label: metric.label, Class: metric.class, Points: strings.Join(xy, " "), Total: totals[i]})
	}
	return v
}

const activityViewTemplate = `{{define "activity-help"}}<ul><li>The last 24 hours in ten-minute slots of the node's clock, 00:00, 00:10 … 23:50, saved across restarts.</li><li>A time the daemon was down reads as zero, like a quiet one.</li><li>The last slot is the current one and is still counting.</li><li>On an unfiltered view, Refused is node-wide for the daemon Owner and covers visible records for other callers.</li><li>Dequeued means handed to a reader, not completed.</li></ul>{{end}}
{{define "activity-view"}}{{with .Activity}}
{{if not .ShowTable}}<section class="fact-card activity-fact"><div class=page-title><h2>Activity</h2><button type=button class=help-button popovertarget=record-activity-help aria-label="About this activity history" data-tooltip="The last 24 hours in ten-minute slots of the node's clock, saved across restarts; down time reads as zero. Dequeued is not completion.">ⓘ</button></div><div popover id=record-activity-help class=context-help><h2>About activity</h2>{{template "activity-help" .}}</div>{{end}}
{{if .Unavailable}}<p class=muted>Activity unavailable: {{.Unavailable}}</p>
{{else if .Points}}
<p>{{if .Named}}Scope: <code>{{.Scope}}</code> · {{else}}Scope: visible records · {{end}}{{.Start}} to {{.End}}, ten-minute slots.</p>
{{if .ShowTable}}<p class=muted>Window: the last 24 hours of the node's clock · Uptime: {{if .Uptime}}{{.Uptime}}{{else}}unavailable{{end}}.</p>{{end}}
{{if .Series}}<figure class=activity-figure><svg class=activity-chart viewBox="0 0 640 170" role=img aria-label="Activity per ten-minute slot from {{.Start}} to {{.End}}; shared maximum {{number .Max}} over the displayed nonzero series">
<path class=activity-axis d="M50 25 V125 H610"/><text x=10 y=30>{{number .Max}}</text><text x=34 y=130>0</text>
{{range .Ticks}}<path class=activity-tick d="M{{.X}} 125 v4"/>{{if .Label}}<text class=activity-hour x={{.X}} y=145 text-anchor=middle>{{.Label}}</text>{{end}}{{end}}
{{range .Series}}<polyline class="activity-line {{.Class}}" points="{{.Points}}"/>{{end}}</svg>
<figcaption>Shared scale: 0–{{number .Max}} per ten-minute slot over the displayed nonzero series; a tick at every hour.</figcaption>
<ul class=activity-legend>{{range .Series}}<li><span class="activity-swatch {{.Class}}" aria-hidden=true></span>{{.Label}}: {{number .Total}} in the last day</li>{{end}}</ul></figure>
{{else}}<p>All five series: <strong>0</strong> in the last day.</p>{{end}}
{{if not .AllZero}}{{with .Zero}}<p class=muted>Zero all day: {{join . ", "}}.</p>{{end}}{{end}}
{{if .ShowTable}}<details><summary>Slot values</summary><table><thead><tr><th scope=col>Slot<th scope=col class=num>Accepted<th scope=col class=num>Dequeued<th scope=col class=num>Dropped<th scope=col class=num>Expired<th scope=col class=num>Refused</tr></thead><tbody>{{range .Points}}<tr><td>{{.At.Format "Jan 2 15:04"}}<td class=num>{{number .In}}<td class=num>{{number .Out}}<td class=num>{{number .Dropped}}<td class=num>{{number .Expired}}<td class=num>{{number .Refused}}</tr>{{end}}</tbody></table></details>{{end}}
{{else}}<p class=muted>The daemon answered no activity.</p>{{end}}
{{if not .ShowTable}}</section>{{end}}
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

var activityPage = template.Must(template.New("activity").Funcs(template.FuncMap{"join": strings.Join, "titleMark": titleMark, "number": number}).Parse(shell("activity", "Activity graphs") + `
<div class=page-title><h1>{{titleMark "activity"}} Activity graphs</h1><button type=button class=help-button popovertarget=activity-help aria-label="About activity history">ⓘ</button></div>
<div popover id=activity-help class=context-help><h2>About activity</h2>{{template "activity-help" .}}</div>
<form method=get><label>Service or channel <select name=name data-submit-on-change><option value="">All visible</option>{{range .Records}}<option value="{{.Name}}" {{if eq .Name $.Name}}selected{{end}}>{{.Name}}</option>{{end}}</select></label><noscript><button>Apply</button></noscript></form>
{{template "activity-view" .}}` + activityViewTemplate))

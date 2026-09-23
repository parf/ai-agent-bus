package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

type busError struct {
	code    int
	message string
}

func (e *busError) Error() string { return e.message }

// problem is a failed page, shown in place of the one asked for. It carries the
// shell, because a refusal is the moment somebody most needs the navigation and
// their own name: http.Error writes text/plain with no way back, no title and
// nothing saying who it was refusing (Plans/MVP/done/web-review.md W12).
type problem struct {
	pageInfo
	You             string
	Title           string
	Detail          string
	Advice          string
	Back, BackLabel string
}

// fail turns a bus refusal into the page for that refusal. The recoveries are
// deliberately different from each other: an expired session, a permission
// refusal and a bus that is not answering are three different things to do
// next, and one status line told a reader none of them apart.
// Codes are the closed set in docs/05-discovery.md#refusals.
func fail(w http.ResponseWriter, r *http.Request, you string, err error) {
	var bad *busError
	if !errors.As(err, &bad) {
		// This page rendered, so the dashboard is up; the daemon it asks is a
		// separate process (docs/11-processes.md#the-processes) and is not
		// answering. Saying "sign in" here would be a lie about whose fault it
		// is, and is what the anonymous page used to say.
		log.Printf("dashboard bus unavailable: %v", err)
		show(w, http.StatusBadGateway, problem{You: you,
			Title:  "The bus is not answering",
			Advice: "The dashboard is running; the daemon behind it is not reachable, so there is nothing to show and nothing was changed. It comes back on its own when the daemon does.",
		}, r)
		return
	}
	switch bad.code {
	case http.StatusUnauthorized:
		// Not an error page at all: the sign-in form, at the address they
		// asked for, so signing in lands them where they were going.
		signIn(w, r, "that session has ended \u2014 sign in to carry on")
	case http.StatusForbidden:
		show(w, bad.code, problem{You: you,
			Title:  "Not yours to see",
			Detail: bad.message,
			Advice: "You are signed in, and refused for lack of permission rather than for want of a credential \u2014 signing in again would change nothing. Its owner, or a maintainer of it, is who can grant this.",
		}, r)
	case http.StatusNotFound:
		// Hidden and absent are one answer on purpose: telling them apart
		// would let anybody enumerate the registry a name at a time.
		// See docs/05-discovery.md#refusals.
		show(w, bad.code, problem{You: you,
			Title:  "No such name",
			Advice: "Either nothing is registered under that name or it is not one you may see. Those are deliberately the same answer, so this does not tell you which.",
		}, r)
	case http.StatusServiceUnavailable:
		show(w, bad.code, problem{You: you,
			Title:  "The bus is busy",
			Detail: bad.message,
			Advice: "The daemon is there and briefly cannot answer. Nothing was changed; the same request is worth making again.",
		}, r)
	case http.StatusInternalServerError:
		show(w, bad.code, problem{You: you,
			Title:  "Something went wrong in the daemon",
			Detail: bad.message,
			Advice: "This is a fault, not a rule: repeating it is unlikely to help, and the daemon's log on this node is where it is recorded.",
		}, r)
	default:
		// 400, 409, 412 and 429: the bus understood and would not.
		show(w, bad.code, problem{You: you,
			Title:  "That was refused",
			Detail: bad.message,
			Advice: "Nothing was changed. The reason above is the daemon's own.",
		}, r)
	}
}

// show renders a problem with the way back filled in: retrying a page that
// failed is following the same link again, while a form that was refused wants
// the form, not a repeat of the submission.
func show(w http.ResponseWriter, code int, p problem, r *http.Request) {
	if r.Method == http.MethodGet {
		p.Back, p.BackLabel = r.URL.RequestURI(), "Try again"
	} else if back := referrer(r); back != "/" {
		p.Back, p.BackLabel = back, "Back to the page"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	p.pageInfo = requestInfo(r)
	render(w, problemPage, p)
}

// localProblem is for browser input rejected by the web face before it calls
// the daemon. Keeping that distinction out of fail avoids claiming that the
// daemon refused a request it never received.
func localProblem(w http.ResponseWriter, r *http.Request, you string, code int, detail string) {
	show(w, code, problem{
		You:    you,
		Title:  "That request was not understood",
		Detail: detail,
		Advice: "Nothing was sent to the daemon. Return to the page and use the action shown there.",
	}, r)
}

var problemPage = template.Must(template.New("problem").Funcs(template.FuncMap{"titleMark": titleMark}).Parse(shellTitle("", `{{.Title}}`) + `<div class=page-title><h1>{{titleMark "problem"}} {{.Title}}</h1></div>
{{with .Detail}}<p class=warn>{{.}}</p>{{end}}
<p>{{.Advice}}</p>
{{with .Back}}<p><a href="{{.}}">{{$.BackLabel}}</a></p>{{end}}
`))

type adminView struct {
	pageInfo
	You             string `json:"you"`
	DaemonOwner     bool   `json:"daemon_owner"`
	Administrator   bool   `json:"administrator"`
	Records         []protocol.Record
	Record          protocol.Record
	OwnerUser       *protocol.User
	Groups          map[string][]string
	GroupName       string
	GroupMembers    []string
	GroupReferences []groupReference
	CanEditGroup    bool
	// GroupRecords are the Group records the caller may see, by name: their
	// Owner and Maintainers, and — being visible — their membership
	// (docs/constitution.md#-group). A group listed in Groups and absent here
	// is one the caller may name and not read.
	GroupRecords map[string]protocol.Record
	GroupRecord  protocol.Record
	GroupVisible bool
	// Editing says which side of the one record form this render is: the
	// settings page rather than the registration page. They ask the same
	// questions; see recordform.go.
	Editing       bool
	Mine, State   string
	Query, Kind   string
	Readers       string
	Work          string
	Sort          string
	OwnerFilter   string
	Current       string
	Return        string
	Previous      string
	Next          string
	ClearFilters  string
	Matched       int
	CategoryTotal int
	// PersonalHere counts the Personal records of this page's own kind,
	// which the shared view omits, so its empty state can say where they are.
	PersonalHere int
	// KindLinks filter the Personal page, which holds every kind, to one.
	KindLinks  []viewLink
	Start      int
	End        int
	HasFilters bool
	Owners     []string
	// Channels is either channel section; Queues and PubSub say which. They
	// are two sections, each with its own list, registration and settings.
	Channels     bool
	Queues       bool
	PubSub       bool
	Agents       bool
	Services     bool
	PersonalPage bool
	// NewKind is the kind a registration page is registering, empty on the
	// page that asks which. The fields differ by kind rather than by section:
	// a queue declares TTL, capacity and overflow, and a pub/sub topic holds
	// nothing, so neither form can ask the other's questions.
	NewKind      string
	Status       core.Status
	Activity     activityPresentation
	SectionLinks []viewLink
	FilterLinks  []viewLink
	ReaderLinks  []viewLink
	WorkLinks    []viewLink
	Form         formState
}

type groupReference struct {
	Name, Kind string
	Uses       []string
}

func groupReaches(groups map[string][]string, outer, target string, seen map[string]bool) bool {
	if outer == target {
		return true
	}
	if seen[outer] {
		return false
	}
	seen[outer] = true
	for _, member := range groups[outer] {
		if strings.HasPrefix(member, "@") && groupReaches(groups, member, target, seen) {
			return true
		}
	}
	return false
}

func groupUses(terms []string, groups map[string][]string, target, label string) []string {
	var uses []string
	for _, term := range terms {
		if !strings.HasPrefix(term, "@") || !groupReaches(groups, term, target, map[string]bool{}) {
			continue
		}
		if term == target {
			uses = append(uses, label)
		} else {
			uses = append(uses, label+" via "+term)
		}
	}
	return uses
}

func referencesToGroup(records []protocol.Record, groups map[string][]string, target string) []groupReference {
	var refs []groupReference
	for _, record := range records {
		uses := groupUses(record.Allow, groups, target, "ACL")
		uses = append(uses, groupUses(record.Maintainers, groups, target, "Maintainers")...)
		if len(uses) != 0 {
			refs = append(refs, groupReference{Name: record.Name, Kind: record.Kind, Uses: uses})
		}
	}
	return refs
}

// MembersVisible says whether this caller may read a group's membership:
// Administrators read every group's, and anyone else the ones they may see —
// being in it, or managing it. Otherwise an empty list and a hidden one would
// read the same.
func (v adminView) MembersVisible(name string) bool {
	_, visible := v.GroupRecords[name]
	return v.Administrator || visible
}

// mayEditGroup is who changes a group's membership: its Owner, its
// Maintainers and the daemon's Administrators, and for the protected
// @administrators the daemon Owner alone (docs/constitution.md#-group). The
// daemon decides at submission; this only chooses which controls to offer.
func (v adminView) mayEditGroup(name string) bool {
	if name == core.AdministratorsGroup {
		return v.DaemonOwner
	}
	return v.Administrator || v.GroupRecords[name].CanManage
}

// GroupValue and GroupTicked are what a group form control shows: what was
// submitted when this render is a refused submission coming back, and the
// stored value otherwise.
func (v adminView) GroupValue(name, stored string) string {
	if v.Form.Is("save") {
		return v.Form.Value(name)
	}
	return stored
}

func (v adminView) GroupTicked(name string, stored bool) bool {
	if v.Form.Is("save") {
		return v.Form.Checked(name)
	}
	return stored
}

// GroupMayAssign is whether this caller may set a group's owner-only fields,
// Personal and Maintainers: its Owner or the daemon Owner, and nobody for the
// protected group, which has neither (docs/constitution.md#-group).
// Registering a group makes you its Owner.
func (v adminView) GroupMayAssign() bool {
	if !v.Editing {
		return true
	}
	return v.GroupName != core.AdministratorsGroup && v.GroupVisible && v.GroupRecord.CanTransfer
}

// GroupDescrEditable is whether the form can offer the description back: an
// Administrator may edit a group whose record it cannot read, and a field
// showing an empty value it never saw would clear the stored one on save.
func (v adminView) GroupDescrEditable() bool { return !v.Editing || v.GroupVisible }

// GroupSecretEditable is whether this caller may write a group's secret: its
// managers, and not an Administrator by being one (docs/constitution.md#-private-values).
func (v adminView) GroupSecretEditable() bool {
	return !v.Editing || v.GroupVisible && v.GroupRecord.CanManage
}

// loadGroups reads both answers a group page needs: every group the caller
// may name, with the membership they may read, and the group records they
// may see, which carry Owner and Maintainers.
func (c *caller) loadGroups(r *http.Request, v *adminView) error {
	if err := c.get(cookie(r), "/groups", &v.Groups); err != nil {
		return err
	}
	if err := c.get(cookie(r), "/ls", &v.Records); err != nil {
		return err
	}
	v.GroupRecords = map[string]protocol.Record{}
	for _, record := range v.Records {
		if record.Kind == protocol.KindGroup {
			v.GroupRecords[record.Name] = record
		}
	}
	return nil
}

// formState carries only fields that are safe to render back after a refused
// submission. Tokens and private configuration never enter it. The daemon is
// still the validator; this is presentation state for the one response only.
type formState struct {
	Action string
	Target string
	Field  string
	Error  string
	Values map[string]string
}

func (f formState) Is(action string) bool { return f.Action == action && f.Error != "" }
func (f formState) Value(name string) string {
	if f.Values == nil {
		return ""
	}
	return f.Values[name]
}
func (f formState) Checked(name string) bool { return f.Value(name) == "on" }
func (f formState) Invalid(name string) bool { return f.Error != "" && f.Field == name }
func (f formState) Matches(action, name, value string) bool {
	return f.Is(action) && f.Value(name) == value
}
func retainedForm(action, message string, values url.Values, names ...string) formState {
	f := formState{Action: action, Target: action, Error: message, Values: make(map[string]string, len(names))}
	for _, name := range names {
		f.Values[name] = values.Get(name)
	}
	return f
}

// busMessage is the daemon's own wording for a refusal, unwrapped from the
// JSON body it answers with. It is the only part of a failed call that may be
// shown: everything else is transport text naming the backend address.
func busMessage(bad *busError) string {
	message := strings.TrimSpace(bad.message)
	var payload struct {
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(message), &payload) == nil && payload.Error != "" {
		return payload.Error
	}
	return message
}

// sectionProblem is what a page may say when one section of it failed and the
// rest is still true. A refusal the daemon worded is caller-facing and is
// shown; anything else is a transport failure whose text carries the socket or
// TCP address this child talks to, so it is logged here and replaced. The
// whole-page path does the same in fail(). Every degraded section goes through
// here: rendering err.Error() directly put the backend address on the record
// and Account pages, and a section that fails must still say so rather than
// read as an answer of nothing.
func sectionProblem(what string, err error) string {
	var bad *busError
	if !errors.As(err, &bad) {
		log.Printf("dashboard %s unavailable: %v", what, err)
		return "the daemon did not answer"
	}
	return busMessage(bad)
}

func formRefusal(err error) (int, string, bool) {
	var bad *busError
	if !errors.As(err, &bad) {
		return 0, "", false
	}
	message := busMessage(bad)
	return bad.code, message, bad.code == http.StatusBadRequest || bad.code == http.StatusNotFound || bad.code == http.StatusConflict || bad.code == http.StatusPreconditionFailed || bad.code == http.StatusTooManyRequests
}

func renderForm(w http.ResponseWriter, code int, t *template.Template, value any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	render(w, t, value)
}

const formErrorSummary = `{{if $.Form.Error}}<section class=form-error role=alert aria-labelledby=form-error-title><h2 id=form-error-title>Check this form</h2><p>{{$.Form.Error}}</p><p><a href="#form-{{$.Form.Target}}">Review the submitted fields</a></p></section>{{end}}`

type viewLink struct {
	Href, Label string
	Class       string
	Count       int
	Counted     bool
	Current     bool
}

func pageURL(path string, q url.Values) string {
	if encoded := q.Encode(); encoded != "" {
		return path + "?" + encoded
	}
	return path
}

func cloneValues(in url.Values) url.Values {
	out := make(url.Values, len(in))
	for key, values := range in {
		out[key] = append([]string(nil), values...)
	}
	return out
}

func queryWith(path string, in url.Values, key, value string) string {
	q := cloneValues(in)
	if value == "" {
		q.Del(key)
	} else {
		q.Set(key, value)
	}
	return pageURL(path, q)
}

// recordReturn keeps the exact list state a visitor came from, but only when
// that list is the one this record is on. A local URL naming any other listing
// is a Back link to a page the record cannot appear on, so the record's own
// listing is used instead; its query, which carries the filters and the page,
// is kept whole.
func recordReturn(raw string, kind string, personal bool) string {
	home := listPathForRecord(kind, personal)
	u, err := url.Parse(local(raw))
	if err != nil || u.Path != home {
		return home
	}
	u.Fragment = ""
	return u.RequestURI()
}

type serviceConfirmation struct {
	adminView
	Action, NewOwner string
}

func readerSnapshot(readers *int) string {
	if readers == nil {
		return "unavailable"
	}
	return strconv.Itoa(*readers)
}

func conditionsChanged(w http.ResponseWriter, r *http.Request, v adminView, detail string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusConflict)
	render(w, problemPage, problem{
		pageInfo:  requestInfo(r),
		You:       v.You,
		Title:     "The conditions changed",
		Detail:    detail,
		Advice:    "The daemon was re-read before the action. Review the current record and confirm again if the action still applies.",
		Back:      "/service-danger?name=" + url.QueryEscape(v.Record.Name),
		BackLabel: "Review the current Danger Zone",
	})
}

func (c *caller) signedIn(w http.ResponseWriter, r *http.Request) (adminView, bool) {
	var v adminView
	if cookie(r) == "" {
		// A deep link is where somebody meant to go, and a plain 401 left them
		// at a dead end with no form on it. The form is served here instead,
		// at that address, and carries it back (web-review.md W12).
		signIn(w, r, "sign in to open this page")
		return v, false
	}
	node, err := c.status(r)
	if err != nil {
		fail(w, r, "", err)
		return v, false
	}
	v.pageInfo = requestInfo(r)
	v.You, v.Administrator, v.DaemonOwner, v.Status = node.You, node.Administrator, node.DaemonOwner, node.Status
	return v, true
}

// Mutating browser requests require the exact origin, including scheme/port.
// No process-local CSRF state, and no credential rendered as a form field.
func sameOrigin(r *http.Request, tls bool) bool {
	origin, err := url.Parse(r.Header.Get("Origin"))
	scheme := "http"
	if tls {
		scheme = "https"
	}
	return err == nil && origin.Scheme == scheme && origin.Host == r.Host && origin.User == nil && origin.Path == "" && origin.RawQuery == "" && origin.Fragment == "" && (r.Header.Get("Sec-Fetch-Site") == "" || r.Header.Get("Sec-Fetch-Site") == "same-origin")
}

func (c *caller) post(cred, path string, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.request(cred, "POST", path, bytes.NewReader(body), nil)
}

// recordSection is the navigation section a record's pages belong to.
func recordSection(r protocol.Record) string {
	switch {
	case r.Kind == protocol.KindGroup:
		// A group is on Groups, Personal or not: the Personal view lists none.
		return "groups"
	case r.Personal:
		return "personal"
	case pubsubRecord(r.Kind):
		return "pubsub"
	case channelRecord(r.Kind):
		return "queues"
	case agentRecord(r.Kind):
		return "agents"
	}
	return "services"
}

// recordInactive reads a record's status; empty is active.
func recordInactive(r protocol.Record) bool { return r.Status == protocol.StatusInactive }

// allRecords is every record the caller may see in a listing: /ls answers
// only active ones, and /inactive is the one read of the rest.
func (c *caller) allRecords(cred string) ([]protocol.Record, error) {
	var active, inactive []protocol.Record
	if err := c.get(cred, "/ls", &active); err != nil {
		return nil, err
	}
	if err := c.get(cred, "/inactive", &inactive); err != nil {
		return nil, err
	}
	for i := range inactive {
		inactive[i].Status = protocol.StatusInactive
	}
	return append(active, inactive...), nil
}

// inactiveRecord finds name among the caller's inactive records. A failed
// read finds nothing, so the page falls through to /lookup and its answer.
func (c *caller) inactiveRecord(cred, name string) (protocol.Record, bool) {
	var inactive []protocol.Record
	if err := c.get(cred, "/inactive", &inactive); err != nil {
		return protocol.Record{}, false
	}
	for _, r := range inactive {
		if r.Name == name {
			r.Status = protocol.StatusInactive
			return r, true
		}
	}
	return protocol.Record{}, false
}

func (c *caller) adminRoutes(mux *http.ServeMux, tls bool) {
	c.activityRoutes(mux)
	c.userRoutes(mux, tls)
	loadRecord := func(w http.ResponseWriter, r *http.Request, v *adminView, name string) bool {
		if err := c.get(cookie(r), "/lookup?name="+url.QueryEscape(name), &v.Record); err != nil {
			fail(w, r, v.You, err)
			return false
		}
		v.Current = recordSection(v.Record)
		return true
	}
	loadServiceDetails := func(w http.ResponseWriter, r *http.Request, v *adminView, name string) bool {
		if !loadRecord(w, r, v, name) {
			return false
		}
		if err := c.get(cookie(r), "/groups", &v.Groups); err != nil {
			fail(w, r, v.You, err)
			return false
		}
		// One directory answer supplies the caller-visible owner profile and its
		// local thumbnail. A hidden owner stays absent, and no page render ever
		// performs a provider or per-photo daemon lookup.
		var users []protocol.User
		if err := c.get(cookie(r), "/users", &users); err == nil {
			for i := range users {
				if users[i].Kind == protocol.DirectoryUser && users[i].Name == v.Record.Owner {
					owner := users[i]
					v.OwnerUser = &owner
					break
				}
			}
		}
		var points []protocol.ActivitySlot
		if err := c.get(cookie(r), "/activity?name="+url.QueryEscape(v.Record.Name), &points); err != nil {
			v.Activity = activityView(nil, v.Record.Name, v.Status.Up, false)
			v.Activity.Unavailable = sectionProblem("record activity", err)
		} else {
			v.Activity = activityView(points, v.Record.Name, v.Status.Up, false)
		}
		return true
	}
	listing := func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		records, err := c.allRecords(cookie(r))
		if err != nil {
			fail(w, r, v.You, err)
			return
		}
		v.Mine, v.State = r.URL.Query().Get("scope"), r.URL.Query().Get("state")
		v.Query = strings.TrimSpace(r.URL.Query().Get("q"))
		v.Kind = r.URL.Query().Get("kind")
		v.Readers, v.Work, v.Sort = r.URL.Query().Get("readers"), r.URL.Query().Get("work"), r.URL.Query().Get("sort")
		if v.Mine != "my" {
			v.Mine = ""
		}
		if v.State != "active" && v.State != "inactive" {
			v.State = ""
		}
		// Every section lists one kind, so no page has a kind filter: the
		// old combined channels page, which had one, redirects to the section
		// its kind now has.
		v.Kind = ""
		if v.Readers != "present" && v.Readers != "none" && v.Readers != "unavailable" {
			v.Readers = ""
		}
		if v.Work != "held" {
			v.Work = ""
		}
		if v.Sort != "updated" && v.Sort != "queued" {
			v.Sort = ""
		}
		v.Queues, v.PubSub, v.Agents = r.URL.Path == "/queues", r.URL.Path == "/pubsub", r.URL.Path == "/agents"
		v.Channels = v.Queues || v.PubSub
		v.Services, v.PersonalPage = r.URL.Path == "/services", r.URL.Path == "/personal"
		// The Personal page holds every kind, and each section's Personal tab
		// asks it for that section's own (docs/03-records.md#personal-and-shared).
		if k := r.URL.Query().Get("kind"); v.PersonalPage && personalKind(k) {
			v.Kind = k
		}
		if v.Services {
			// A service has no queue here, so none of the questions about one
			// has an answer to filter or sort by.
			// See docs/03-records.md#record-kinds.
			v.Readers, v.Work = "", ""
			if v.Sort == "queued" {
				v.Sort = ""
			}
		}
		if v.PubSub {
			// A pub/sub topic holds nothing, so there is no held work to
			// filter by; its work is what it accepted, which Sort still orders.
			v.Work = ""
		}
		switch {
		case v.Queues:
			v.Current = "queues"
		case v.PubSub:
			v.Current = "pubsub"
		case v.Agents:
			v.Current = "agents"
		case v.PersonalPage:
			v.Current = "personal"
		default:
			v.Current = "services"
		}
		if v.PersonalPage && !v.DaemonOwner && r.URL.Query().Has("owner") {
			// The owner selector belongs to the daemon owner's view. Do not
			// silently give the same shared URL a caller-dependent meaning.
			query := r.URL.Query()
			query.Del("owner")
			to := r.URL.Path
			if encoded := query.Encode(); encoded != "" {
				to += "?" + encoded
			}
			http.Redirect(w, r, to, http.StatusSeeOther)
			return
		}
		if v.PersonalPage {
			if v.DaemonOwner {
				v.OwnerFilter = r.URL.Query().Get("owner")
			} else {
				v.OwnerFilter = v.You
			}
		}
		owners := map[string]bool{}
		allServices, myServices, personalServices, allQueues, allPubSub, allAgents, myAgents := 0, 0, 0, 0, 0, 0, 0
		personalOf := map[string]int{}
		for _, record := range records {
			// A Group is a record from 0.7 and has its own page, Groups;
			// it is no agent, service or channel (docs/constitution.md#-group).
			if record.Kind == protocol.KindGroup {
				continue
			}
			// Every kind may be Personal from 0.7, and a User's own record
			// always is: the main collections show shared records only, and
			// the Personal tab the rest but for users, whose page is Users.
			// See docs/03-records.md#personal-and-shared.
			switch {
			case record.Personal:
				if record.Kind != protocol.KindUser && (v.DaemonOwner || record.Owner == v.You) {
					personalServices++
					personalOf[record.Kind]++
					if listPathFor(record.Kind) == r.URL.Path {
						v.PersonalHere++
					}
				}
			case pubsubRecord(record.Kind):
				allPubSub++
			case channelRecord(record.Kind):
				allQueues++
			case agentRecord(record.Kind):
				allAgents++
				if record.Owner == v.You {
					myAgents++
				}
			default:
				allServices++
				if record.Owner == v.You {
					myServices++
				}
			}
			if v.PersonalPage {
				if !record.Personal || record.Kind == protocol.KindUser {
					continue
				}
				owners[record.Owner] = true
				if v.OwnerFilter != "" && record.Owner != v.OwnerFilter {
					continue
				}
				if v.Kind != "" && record.Kind != v.Kind {
					continue
				}
			} else if listPathFor(record.Kind) != r.URL.Path || record.Personal {
				continue
			}
			if v.Mine == "my" && record.Owner != v.You {
				continue
			}
			// CategoryTotal is the selected view before toolbar filters. It lets
			// the face distinguish an empty registry category from a filter that
			// happens to match nothing.
			v.CategoryTotal++
			if v.State == protocol.StatusActive && recordInactive(record) || v.State == protocol.StatusInactive && !recordInactive(record) {
				continue
			}
			if !matchesReaderFilter(record.Readers, v.Readers) {
				continue
			}
			if v.Work == "held" && record.Queued == 0 {
				continue
			}
			if v.Query != "" {
				needle := strings.ToLower(v.Query)
				if !strings.Contains(strings.ToLower(record.Name), needle) &&
					!strings.Contains(strings.ToLower(record.Descr), needle) &&
					!strings.Contains(strings.ToLower(record.Owner), needle) {
					continue
				}
			}
			v.Records = append(v.Records, record)
		}
		stateQuery := url.Values{}
		if v.State != "" {
			stateQuery.Set("state", v.State)
		}
		if v.Query != "" {
			stateQuery.Set("q", v.Query)
		}
		if v.Readers != "" {
			stateQuery.Set("readers", v.Readers)
		}
		if v.Work != "" {
			stateQuery.Set("work", v.Work)
		}
		if v.Sort != "" {
			stateQuery.Set("sort", v.Sort)
		}
		switch {
		case v.Queues:
			v.SectionLinks = []viewLink{
				{Href: pageURL("/queues", stateQuery), Label: "All", Count: allQueues, Counted: true, Current: true},
				// Every kind may be Personal, and a Personal queue is on
				// the Personal page rather than here, so the way there is
				// offered here too (docs/03-records.md#personal-and-shared).
				{Href: "/personal?kind=queue", Label: "Personal", Class: "personal-view", Count: personalOf[protocol.KindQueue], Counted: true},
				{Href: "/queues/new", Label: "Register queue"},
			}
		case v.PubSub:
			v.SectionLinks = []viewLink{
				{Href: pageURL("/pubsub", stateQuery), Label: "All", Count: allPubSub, Counted: true, Current: true},
				{Href: "/personal?kind=pubsub", Label: "Personal", Class: "personal-view", Count: personalOf[protocol.KindPubSub], Counted: true},
				{Href: "/pubsub/new", Label: "Register pub/sub topic"},
			}
		case v.Agents, v.PersonalPage:
			agentQuery, myAgentQuery, personalQuery := cloneValues(stateQuery), cloneValues(stateQuery), cloneValues(stateQuery)
			myAgentQuery.Set("scope", "my")
			if v.DaemonOwner && v.OwnerFilter != "" {
				personalQuery.Set("owner", v.OwnerFilter)
			}
			v.SectionLinks = []viewLink{
				{Href: pageURL("/agents", agentQuery), Label: "All", Count: allAgents, Counted: true, Current: !v.PersonalPage && v.Mine == ""},
				{Href: pageURL("/agents", myAgentQuery), Label: "My", Class: "my-view", Count: myAgents, Counted: true, Current: !v.PersonalPage && v.Mine == "my"},
				personalTab(v, personalQuery, personalServices, personalOf),
				{Href: "/agents/new", Label: "Register agent"},
			}
		default:
			// Services, which is the only page left here. A Personal service
			// is on the Personal page, as a Personal channel is.
			allQuery, myQuery := cloneValues(stateQuery), cloneValues(stateQuery)
			myQuery.Set("scope", "my")
			v.SectionLinks = []viewLink{
				{Href: pageURL("/services", allQuery), Label: "All", Count: allServices, Counted: true, Current: v.Mine == ""},
				{Href: pageURL("/services", myQuery), Label: "My", Class: "my-view", Count: myServices, Counted: true, Current: v.Mine == "my"},
				{Href: "/personal?kind=service", Label: "Personal", Class: "personal-view", Count: personalOf[protocol.KindService], Counted: true},
				{Href: "/services/new", Label: "Register service"},
			}
		}
		filterBase := url.Values{}
		if v.Mine == "my" && !v.PersonalPage {
			filterBase.Set("scope", "my")
		}
		if v.PersonalPage && v.DaemonOwner && v.OwnerFilter != "" {
			filterBase.Set("owner", v.OwnerFilter)
		}
		if v.Query != "" {
			filterBase.Set("q", v.Query)
		}
		if v.Readers != "" {
			filterBase.Set("readers", v.Readers)
		}
		if v.Work != "" {
			filterBase.Set("work", v.Work)
		}
		if v.Sort != "" {
			filterBase.Set("sort", v.Sort)
		}
		if v.Kind != "" {
			filterBase.Set("kind", v.Kind)
		}
		filterPath := r.URL.Path
		if v.PersonalPage {
			kindBase := cloneValues(filterBase)
			kindBase.Del("kind")
			if v.State != "" {
				kindBase.Set("state", v.State)
			}
			v.KindLinks = []viewLink{{Href: pageURL(filterPath, cloneValues(kindBase)), Label: "All", Current: v.Kind == ""}}
			for _, k := range []string{protocol.KindAgent, protocol.KindService, protocol.KindQueue, protocol.KindPubSub} {
				v.KindLinks = append(v.KindLinks, viewLink{Href: queryWith(filterPath, kindBase, "kind", k), Label: entityLabel(k), Current: v.Kind == k})
			}
		}
		v.FilterLinks = []viewLink{
			{Href: pageURL(filterPath, cloneValues(filterBase)), Label: "All", Current: v.State == ""},
			{Href: queryWith(filterPath, filterBase, "state", "active"), Label: "Active", Current: v.State == "active"},
			{Href: queryWith(filterPath, filterBase, "state", "inactive"), Label: "Inactive", Current: v.State == "inactive"},
		}
		readerBase := cloneValues(filterBase)
		readerBase.Del("readers")
		if v.State != "" {
			readerBase.Set("state", v.State)
		}
		v.ReaderLinks = []viewLink{
			{Href: pageURL(filterPath, cloneValues(readerBase)), Label: "All", Current: v.Readers == ""},
			{Href: queryWith(filterPath, readerBase, "readers", "present"), Label: "Reading now", Current: v.Readers == "present"},
			{Href: queryWith(filterPath, readerBase, "readers", "none"), Label: "No reader now", Current: v.Readers == "none"},
			{Href: queryWith(filterPath, readerBase, "readers", "unavailable"), Label: "Unavailable", Current: v.Readers == "unavailable"},
		}
		workBase := cloneValues(filterBase)
		workBase.Del("work")
		if v.State != "" {
			workBase.Set("state", v.State)
		}
		v.WorkLinks = []viewLink{
			{Href: pageURL(filterPath, cloneValues(workBase)), Label: "All", Current: v.Work == ""},
			{Href: queryWith(filterPath, workBase, "work", "held"), Label: "Holding work", Current: v.Work == "held"},
		}
		for owner := range owners {
			v.Owners = append(v.Owners, owner)
		}
		sort.Strings(v.Owners)
		sort.SliceStable(v.Records, func(i, j int) bool {
			a, b := v.Records[i], v.Records[j]
			switch v.Sort {
			case "updated":
				if !a.At.Equal(b.At) {
					return a.At.After(b.At)
				}
			case "queued":
				aWork, bWork := a.Queued, b.Queued
				if v.PubSub {
					if a.Kind == protocol.KindPubSub {
						aWork = a.In
					}
					if b.Kind == protocol.KindPubSub {
						bWork = b.In
					}
				}
				if aWork != bWork {
					return aWork > bWork
				}
			}
			return a.Name < b.Name
		})
		v.Matched = len(v.Records)
		const pageSize = 25
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		last := max(1, (v.Matched+pageSize-1)/pageSize)
		page = min(max(1, page), last)
		listingQuery := cloneValues(stateQuery)
		if v.Mine == "my" && !v.PersonalPage {
			listingQuery.Set("scope", "my")
		}
		if v.PersonalPage && v.DaemonOwner && v.OwnerFilter != "" {
			listingQuery.Set("owner", v.OwnerFilter)
		}
		link := func(n int) string {
			q := cloneValues(listingQuery)
			q.Set("page", strconv.Itoa(n))
			return pageURL(filterPath, q)
		}
		v.Return = link(page)
		if page > 1 {
			v.Previous = link(page - 1)
		}
		if page < last {
			v.Next = link(page + 1)
		}
		start, end := (page-1)*pageSize, min(page*pageSize, v.Matched)
		if v.Matched > 0 {
			v.Start, v.End = start+1, end
		}
		v.Records = v.Records[start:end]
		clearQuery := url.Values{}
		if v.Mine == "my" && !v.PersonalPage {
			clearQuery.Set("scope", "my")
		}
		if v.PersonalPage && v.DaemonOwner && v.OwnerFilter != "" {
			clearQuery.Set("owner", v.OwnerFilter)
		}
		v.ClearFilters = pageURL(filterPath, clearQuery)
		v.HasFilters = v.Query != "" || v.State != "" || v.Readers != "" || v.Work != "" || v.Sort != ""
		render(w, serviceList, v)
	}
	mux.HandleFunc("GET /agents", listing)
	mux.HandleFunc("GET /services", listing)
	mux.HandleFunc("GET /queues", listing)
	mux.HandleFunc("GET /pubsub", listing)
	// The combined channels page was split into Queues and PubSub. A bookmark
	// of it lands on the section its kind filter named — PubSub for
	// kind=pubsub, Queues otherwise — with the rest of its query kept.
	mux.HandleFunc("GET /channels", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		to := "/queues"
		if q.Get("kind") == protocol.KindPubSub {
			to = "/pubsub"
		}
		q.Del("kind")
		http.Redirect(w, r, pageURL(to, q), http.StatusMovedPermanently)
	})
	mux.HandleFunc("GET /personal", listing)
	mux.HandleFunc("GET /services/new", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		v.Current, v.NewKind = "services", protocol.KindService
		render(w, serviceNew, v)
	})
	mux.HandleFunc("GET /agents/new", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		v.Current, v.Agents, v.NewKind = "agents", true, protocol.KindAgent
		render(w, serviceNew, v)
	})
	// Two channel kinds, two sections, two forms, each a place that can be
	// linked to.
	mux.HandleFunc("GET /queues/new", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		v.Current, v.Channels, v.Queues, v.NewKind = "queues", true, true, protocol.KindQueue
		render(w, serviceNew, v)
	})
	mux.HandleFunc("GET /pubsub/new", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		v.Current, v.Channels, v.PubSub, v.NewKind = "pubsub", true, true, protocol.KindPubSub
		render(w, serviceNew, v)
	})
	mux.HandleFunc("GET /channels/new", func(w http.ResponseWriter, r *http.Request) {
		to := "/queues/new"
		if r.URL.Query().Get("kind") == protocol.KindPubSub {
			to = "/pubsub/new"
		}
		http.Redirect(w, r, to, http.StatusMovedPermanently)
	})
	recordDetail := func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		// An inactive record is no such record to /lookup, and its one view
		// is the /inactive answer: a read-only page built from that entry
		// alone, with its reactivation for whoever may manage it
		// (docs/constitution.md#common-record-fields).
		if record, found := c.inactiveRecord(cookie(r), r.URL.Query().Get("name")); found {
			v.Record = record
			v.Current = recordSection(record)
			v.Return = recordReturn(r.URL.Query().Get("return"), record.Kind, record.Personal)
			render(w, inactiveDetail, v)
			return
		}
		if !loadServiceDetails(w, r, &v, r.URL.Query().Get("name")) {
			return
		}
		v.Return = recordReturn(r.URL.Query().Get("return"), v.Record.Kind, v.Record.Personal)
		render(w, serviceDetail, v)
	}
	mux.HandleFunc("GET /service", recordDetail) // one page, reached by the name of each listing
	mux.HandleFunc("GET /queue", recordDetail)
	mux.HandleFunc("GET /pubsub/topic", recordDetail)
	// The old combined addresses, kept for bookmarks: a queue or topic the
	// caller can see goes to its own section's address; anything else, an
	// inactive record included, is answered here as before.
	legacyChannel := func(page http.HandlerFunc, queue, topic string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var rec protocol.Record
			if name := r.URL.Query().Get("name"); name != "" && c.get(cookie(r), "/lookup?name="+url.QueryEscape(name), &rec) == nil {
				to := ""
				switch rec.Kind {
				case protocol.KindQueue:
					to = queue
				case protocol.KindPubSub:
					to = topic
				}
				if to != "" {
					http.Redirect(w, r, to+"?"+r.URL.RawQuery, http.StatusMovedPermanently)
					return
				}
			}
			page(w, r)
		}
	}
	mux.HandleFunc("GET /channel", legacyChannel(recordDetail, "/queue", "/pubsub/topic"))
	mux.HandleFunc("GET /agent", recordDetail)
	// The settings form is a page, not a panel on the detail page: it is the
	// registration form with the record already in it, so it has an address
	// of its own and one shape. Three paths for the three sections, one
	// handler, exactly as the detail page has.
	recordEditPage := func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		if !loadServiceDetails(w, r, &v, r.URL.Query().Get("name")) {
			return
		}
		if !v.Record.CanManage {
			fail(w, r, v.You, &busError{code: http.StatusForbidden, message: "only the owner or an assigned Maintainer can change this record's settings"})
			return
		}
		v.Editing = true
		v.Return = recordReturn(r.URL.Query().Get("return"), v.Record.Kind, v.Record.Personal)
		render(w, recordEdit, v)
	}
	mux.HandleFunc("GET /service/edit", recordEditPage)
	mux.HandleFunc("GET /queue/edit", recordEditPage)
	mux.HandleFunc("GET /pubsub/topic/edit", recordEditPage)
	mux.HandleFunc("GET /channel/edit", legacyChannel(recordEditPage, "/queue/edit", "/pubsub/topic/edit"))
	mux.HandleFunc("GET /agent/edit", recordEditPage)
	mux.HandleFunc("GET /service-deactivate", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		if !loadRecord(w, r, &v, r.URL.Query().Get("name")) {
			return
		}
		if !v.Record.CanManage || v.Record.Kind == protocol.KindUser {
			fail(w, r, v.You, &busError{code: http.StatusForbidden, message: "only the owner or an assigned Maintainer can deactivate this record"})
			return
		}
		v.Return = recordReturn(r.URL.Query().Get("return"), v.Record.Kind, v.Record.Personal)
		render(w, recordDeactivate, v)
	})
	mux.HandleFunc("GET /service-danger", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		if !loadRecord(w, r, &v, r.URL.Query().Get("name")) {
			return
		}
		if !v.Record.CanManage {
			fail(w, r, v.You, &busError{code: http.StatusForbidden, message: "only the owner or an assigned Maintainer can manage this record"})
			return
		}
		render(w, serviceDanger, v)
	})
	mux.HandleFunc("GET /groups", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		if err := c.loadGroups(r, &v); err != nil {
			fail(w, r, v.You, err)
			return
		}
		// Any User may create a group, which that User then owns
		// (docs/constitution.md#-group).
		v.SectionLinks = []viewLink{{Href: "/groups", Label: "All", Count: len(v.Groups), Counted: true, Current: true}, {Href: "/groups/new", Label: "Register group"}}
		render(w, groupList, v)
	})
	mux.HandleFunc("GET /group", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		if err := c.loadGroups(r, &v); err != nil {
			fail(w, r, v.You, err)
			return
		}
		name := r.URL.Query().Get("name")
		members, exists := v.Groups[name]
		if !exists {
			fail(w, r, v.You, &busError{code: http.StatusNotFound, message: "no such group"})
			return
		}
		v.Current = "groups"
		v.GroupName = name
		v.GroupMembers = append([]string(nil), members...)
		v.GroupRecord, v.GroupVisible = v.GroupRecords[name]
		v.CanEditGroup = v.mayEditGroup(name)
		v.GroupReferences = referencesToGroup(v.Records, v.Groups, name)
		render(w, groupDetail, v)
	})
	// Membership is edited where a group is registered: the same form.
	mux.HandleFunc("GET /group/edit", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		if err := c.loadGroups(r, &v); err != nil {
			fail(w, r, v.You, err)
			return
		}
		name := r.URL.Query().Get("name")
		members, exists := v.Groups[name]
		if !exists {
			fail(w, r, v.You, &busError{code: http.StatusNotFound, message: "no such group"})
			return
		}
		v.Current, v.Editing, v.GroupName = "groups", true, name
		v.GroupMembers = append([]string(nil), members...)
		v.GroupRecord, v.GroupVisible = v.GroupRecords[name]
		v.CanEditGroup = v.mayEditGroup(name)
		if !v.CanEditGroup {
			fail(w, r, v.You, &busError{code: http.StatusForbidden, message: "that group's membership cannot be changed by you"})
			return
		}
		render(w, groupEdit, v)
	})
	mux.HandleFunc("GET /groups/new", func(w http.ResponseWriter, r *http.Request) {
		v, ok := c.signedIn(w, r)
		if !ok {
			return
		}
		v.Current = "groups"
		render(w, groupNew, v)
	})
	// The view goes through to the handler so a refused submission can be
	// presented as the person who was refused, on the shell, rather than as a
	// bare status line (web-review.md W12).
	mutate := func(next func(http.ResponseWriter, *http.Request, adminView)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !sameOrigin(r, tls) {
				http.Error(w, "same-origin form required", http.StatusForbidden)
				return
			}
			v, ok := c.signedIn(w, r)
			if !ok {
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			if err := r.ParseForm(); err != nil {
				localProblem(w, r, v.You, http.StatusBadRequest, "The submitted form could not be read.")
				return
			}
			next(w, r, v)
		}
	}
	renderServiceFormError := func(w http.ResponseWriter, r *http.Request, v adminView, action string, code int, message string, field ...string) bool {
		setField := func(form formState) formState {
			if len(field) != 0 {
				form.Field = field[0]
			}
			return form
		}
		switch action {
		case "create":
			kind := r.PostForm.Get("kind")
			v.Channels, v.Agents, v.NewKind = channelRecord(kind), agentRecord(kind), kind
			v.Queues, v.PubSub = queueRecord(kind), pubsubRecord(kind)
			switch {
			case v.PubSub:
				v.Current = "pubsub"
			case v.Queues:
				v.Current = "queues"
			case v.Agents:
				v.Current = "agents"
			default:
				v.Current = "services"
			}
			// Everything the three forms between them ask for: a refusal on one
			// field must not empty the others, and the service form asks for an
			// address and a protocol the daemon will not do without.
			// Everything the forms between them ask for, and deliberately not
			// the secret: a rejected credential is still a credential and must
			// not come back in HTML.
			v.Form = setField(retainedForm(action, message, r.PostForm, "name", "descr", "kind", "addr", "protocol", "personal", "allow", "subs", "ttl", "bound", "overflow"))
			renderForm(w, code, serviceNew, v)
			return true
		case "save":
			if !loadServiceDetails(w, r, &v, r.PostForm.Get("name")) {
				return true
			}
			// Everything the one editor asks for, including the two flags
			// that say whether it carried the sharing fields at all: a retry
			// must send exactly what the first attempt did, or a refused
			// description would clear the Maintainers beside it.
			v.Form = setField(retainedForm(action, message, r.PostForm, "descr", "addr", "protocol", "ttl", "overflow", "bound", "allow", "edit_allow", "subs", "edit_subs", "personal", "maintainers", "edit_sharing", "edit_personal"))
			v.Editing = true
			v.Return = recordReturn(r.PostForm.Get("return"), v.Record.Kind, v.Record.Personal)
			renderForm(w, code, recordEdit, v)
			return true
		case "configure", "transfer":
			if !loadRecord(w, r, &v, r.PostForm.Get("name")) {
				return true
			}
			if !v.Record.CanManage {
				return false
			}
			// Configuration is deliberately absent: a rejected replacement is
			// still a secret and must not return in HTML.
			keys := []string(nil)
			if action == "transfer" {
				keys = []string{"owner"}
			}
			v.Form = setField(retainedForm(action, message, r.PostForm, keys...))
			renderForm(w, code, serviceDanger, v)
			return true
		}
		return false
	}
	mux.HandleFunc("POST /service-confirm", mutate(func(w http.ResponseWriter, r *http.Request, v adminView) {
		name, action := r.PostForm.Get("name"), r.PostForm.Get("action")
		if !loadRecord(w, r, &v, name) {
			return
		}
		confirm := serviceConfirmation{adminView: v, Action: action}
		switch action {
		case "transfer":
			if !v.Record.CanTransfer || v.Record.Name == v.Record.Owner {
				fail(w, r, v.You, &busError{code: http.StatusForbidden, message: "only this record's owner or the daemon owner can transfer it"})
				return
			}
			confirm.NewOwner = strings.TrimSpace(r.PostForm.Get("owner"))
			if confirm.NewOwner == "" {
				renderServiceFormError(w, r, v, action, http.StatusBadRequest, "New owner is required.", "owner")
				return
			}
		case "delete":
			if !v.Record.CanManage {
				fail(w, r, v.You, &busError{code: http.StatusForbidden, message: "only the owner or an assigned Maintainer can remove this record"})
				return
			}
		default:
			localProblem(w, r, v.You, http.StatusBadRequest, "That confirmation action is not available.")
			return
		}
		render(w, serviceConfirm, confirm)
	}))
	mux.HandleFunc("POST /service", mutate(func(w http.ResponseWriter, r *http.Request, v adminView) {
		name := r.PostForm.Get("name")
		action := r.PostForm.Get("action")
		change := core.Management{Name: name}
		var err error
		// The Personal flag travels with the kind because it decides the listing
		// as much as the kind does: a Personal agent is excluded from /agents.
		targetKind, targetPersonal := "", false
		confirmed := r.PostForm.Get("confirmed") == "1"
		if confirmed && (action == "transfer" || action == "delete") {
			if !loadRecord(w, r, &v, name) {
				return
			}
			targetKind, targetPersonal = v.Record.Kind, v.Record.Personal
			changed := v.Record.Owner != r.PostForm.Get("expected_owner")
			if action == "transfer" {
				changed = changed || !v.Record.CanTransfer || v.Record.Name == v.Record.Owner
			} else {
				expectedQueued, parseErr := strconv.Atoi(r.PostForm.Get("expected_queued"))
				changed = changed || parseErr != nil || v.Record.Queued != expectedQueued || readerSnapshot(v.Record.Readers) != r.PostForm.Get("expected_readers") || !v.Record.CanManage
			}
			if changed {
				conditionsChanged(w, r, v, "The record no longer has the owner, queue or reader state shown on the confirmation page.")
				return
			}
		}
		if action == "transfer" && !confirmed {
			var record protocol.Record
			if lookupErr := c.get(cookie(r), "/lookup?name="+url.QueryEscape(name), &record); lookupErr != nil {
				fail(w, r, v.You, lookupErr)
				return
			}
			targetKind, targetPersonal = record.Kind, record.Personal
		}
		switch action {
		case "save":
			descr := r.PostForm.Get("descr")
			change.Descr = &descr
			// Each form offers the fields its own kind has, and a save changes
			// only what its form showed: sending an empty value for a field the
			// page never rendered would clear it.
			// See docs/03-records.md#record-kinds.
			if r.PostForm.Has("addr") || r.PostForm.Has("protocol") {
				addr, proto := r.PostForm.Get("addr"), r.PostForm.Get("protocol")
				change.Addr, change.Proto = &addr, &proto
			}
			if r.PostForm.Has("bound") || r.PostForm.Has("ttl") || r.PostForm.Has("overflow") {
				bound, parseErr := strconv.Atoi(r.PostForm.Get("bound"))
				if parseErr != nil {
					renderServiceFormError(w, r, v, action, http.StatusBadRequest, "Queue capacity must be a whole number.", "bound")
					return
				}
				ttl, overflow := r.PostForm.Get("ttl"), r.PostForm.Get("overflow")
				change.TTL, change.Full, change.Bound = &ttl, &overflow, &bound
			}
			if r.PostForm.Has("edit_allow") {
				allow := strings.Fields(r.PostForm.Get("allow"))
				change.Allow = &allow
			}
			// Deliver-To is the other of a pub/sub topic's two lists, and the
			// same form carries both. Only that kind's editor shows it, so
			// the flag says the form had it rather than that the field was
			// left blank — blank is a real value here and empties the list.
			// See docs/04-messaging.md#subscribers.
			if r.PostForm.Has("edit_subs") {
				subs := strings.Fields(r.PostForm.Get("subs"))
				change.Subs = &subs
			}
			// Classification and sharing are in this one form now, shown to
			// everyone who may manage the record and editable only by its
			// Owner or a daemon administrator. A disabled control submits
			// nothing, and an unchecked box is indistinguishable from an
			// absent one, so the form states separately that it carried them:
			// without that, a Maintainer saving a description would clear the
			// Maintainer list it was not allowed to touch.
			if r.PostForm.Has("edit_sharing") {
				maintainers := protocol.MaintainerList(strings.Fields(r.PostForm.Get("maintainers")))
				change.Maintainers = &maintainers
			}
			if r.PostForm.Has("edit_personal") {
				personal := r.PostForm.Get("personal") == "on"
				change.Personal = &personal
			}
			if err = c.post(cookie(r), "/manage", change); err != nil {
				break
			}
			// The credential is its own verb, exactly as at registration, so
			// the one form is two calls. Empty means leave the stored one
			// alone — the daemon refuses an empty secret anyway, and a field
			// that is never filled in cannot distinguish "unchanged" from
			// "cleared" by being blank. See docs/06-services.md#secrets.
			if secret := formSecret(r.PostForm.Get("secret")); secret != "" {
				if secretErr := c.post(cookie(r), "/secret", struct {
					Name   string `json:"name"`
					Secret string `json:"secret"`
				}{name, secret}); secretErr != nil {
					// The settings landed. Saying the save failed would be a
					// lie, so this says which half did not and where the
					// credential can still be put.
					localProblem(w, r, v.You, http.StatusBadGateway,
						"The settings were saved and the secret was not stored: "+sectionProblem("secret", secretErr)+
							" Set it with: agent-bus secret "+name+" '...'")
					return
				}
			}
		case "deactivate", "reactivate":
			// A status-only edit: the one change an inactive record accepts
			// (docs/constitution.md#common-record-fields). A deactivation reads
			// the kind first, because afterwards /lookup answers unknown and
			// the redirect would lose its section; a reactivation is read
			// after the change, as every other edit is.
			status := protocol.StatusInactive
			if action == "reactivate" {
				status = protocol.StatusActive
			} else {
				var current protocol.Record
				if lookupErr := c.get(cookie(r), "/lookup?name="+url.QueryEscape(name), &current); lookupErr == nil {
					targetKind, targetPersonal = current.Kind, current.Personal
				}
			}
			change.Status = &status
			err = c.post(cookie(r), "/manage", change)
		case "transfer":
			owner := r.PostForm.Get("owner")
			change.Owner = &owner
			err = c.post(cookie(r), "/manage", change)
		case "configure":
			cfg := json.RawMessage(r.PostForm.Get("config"))
			if !json.Valid(cfg) {
				renderServiceFormError(w, r, v, action, http.StatusBadRequest, "Configuration must be valid JSON. The submitted configuration is not shown again.", "config")
				return
			}
			err = c.post(cookie(r), "/configure", struct {
				Name   string
				Config json.RawMessage
			}{name, cfg})
		case "create":
			kind := r.PostForm.Get("kind")
			if !protocol.ValidKind(kind) {
				renderServiceFormError(w, r, v, action, http.StatusBadRequest, "Choose a valid record kind.", "kind")
				return
			}
			targetKind, targetPersonal = kind, r.PostForm.Get("personal") == "on"
			record := protocol.Record{Name: name, Kind: kind, Addr: r.PostForm.Get("addr"), Proto: r.PostForm.Get("protocol"), Descr: r.PostForm.Get("descr"), Allow: strings.Fields(r.PostForm.Get("allow")), Personal: r.PostForm.Get("personal") == "on", Subs: strings.Fields(r.PostForm.Get("subs"))}
			// Only the queue form offers these, and they are read
			// unconditionally because a Record carries values rather than the
			// pointers the settings editor uses to tell "leave this alone"
			// from "clear it": a form that never showed the fields submits
			// nothing, and empty and zero are the defaults the daemon would
			// have applied anyway. An unparseable capacity is not.
			bound, parseErr := strconv.Atoi(r.PostForm.Get("bound"))
			if parseErr != nil && r.PostForm.Get("bound") != "" {
				renderServiceFormError(w, r, v, action, http.StatusBadRequest, "Queue capacity must be a whole number.", "bound")
				return
			}
			record.TTL, record.Full, record.Bound = r.PostForm.Get("ttl"), r.PostForm.Get("overflow"), bound
			if err = c.post(cookie(r), "/register", record); err != nil {
				break
			}
			// The secret is its own verb, so it is a second call: a
			// registration carries neither the bytes nor their digest
			// (docs/06-services.md#secrets). An empty field means no secret,
			// which is what the daemon refuses to store anyway.
			if secret := formSecret(r.PostForm.Get("secret")); secret != "" && holdsSecret(kind) {
				if secretErr := c.post(cookie(r), "/secret", struct {
					Name   string `json:"name"`
					Secret string `json:"secret"`
				}{name, secret}); secretErr != nil {
					// The record exists. Saying the registration failed would
					// be a lie, and returning the form would offer a name that
					// is now taken, so this says exactly what happened and
					// where the credential can still be put.
					localProblem(w, r, v.You, http.StatusBadGateway,
						"The "+strings.ToLower(recordNoun(kind))+" was registered and its secret was not stored: "+sectionProblem("secret", secretErr)+
							" Set it with: agent-bus secret "+name+" '...'")
					return
				}
			}
		case "unsubscribe":
			// Off is the only direction a name sends for itself: putting an
			// inbox on the list is the channel manager's, through the
			// editor above. See docs/04-messaging.md#subscribers.
			err = c.post(cookie(r), "/subscribe", struct {
				Channel string
				Off     bool
			}{name, true})
		case "remove-subscriber":
			err = c.post(cookie(r), "/subscriber/remove", map[string]string{"channel": name, "subscriber": r.PostForm.Get("subscriber")})
		case "delete":
			var record protocol.Record
			if lookupErr := c.get(cookie(r), "/lookup?name="+url.QueryEscape(name), &record); lookupErr != nil {
				fail(w, r, v.You, lookupErr)
				return
			}
			targetKind, targetPersonal = record.Kind, record.Personal
			err = c.post(cookie(r), "/unregister", map[string]string{"name": name})
		default:
			localProblem(w, r, v.You, http.StatusBadRequest, "That service action is not available.")
			return
		}
		if err != nil {
			code, message, preserve := formRefusal(err)
			// A refusal about one submitted line goes back to the form with
			// that line named, whichever code it came with: a route the
			// destination does not allow is a 403 about a line the person
			// typed, and a bare problem page would lose every other field.
			var field string
			if action == "save" || action == "create" {
				if code != 0 {
					field, message = lineRefusal(message, submittedLists(r.PostForm, action)...)
				}
				preserve = preserve || field != ""
			}
			if preserve {
				switch {
				case field != "":
				case action == "create" && code == http.StatusPreconditionFailed:
					field = "name"
				case action == "save" && strings.Contains(message, "maintainer"):
					field = "maintainers"
				}
				if renderServiceFormError(w, r, v, action, code, message, field) {
					return
				}
			}
			fail(w, r, v.You, err)
			return
		}
		// A mutation posted through the shared /service action returns to the
		// listing its record actually belongs to. The daemon's fresh answer,
		// rather than a hidden field, chooses the destination.
		if targetKind == "" && action != "create" && action != "delete" && action != "transfer" {
			var current protocol.Record
			if lookupErr := c.get(cookie(r), "/lookup?name="+url.QueryEscape(name), &current); lookupErr == nil {
				targetKind, targetPersonal = current.Kind, current.Personal
			}
		}
		next := detailPathFor(targetKind) + "?name=" + url.QueryEscape(name)
		switch {
		case action == "delete":
			next = listPathForRecord(targetKind, targetPersonal)
		case action == "transfer":
			var current protocol.Record
			// The redirect below must follow this exact daemon visibility answer;
			// do not replace it with cached or face-inferred authority.
			if lookupErr := c.get(cookie(r), "/lookup?name="+url.QueryEscape(name), &current); lookupErr != nil {
				next = listPathForRecord(targetKind, targetPersonal)
			}
		}
		http.Redirect(w, r, next, http.StatusSeeOther)
	}))
	mux.HandleFunc("POST /groups", mutate(func(w http.ResponseWriter, r *http.Request, v adminView) {
		// "delete" was an action here and is not one now: a group is retired
		// by emptying its membership. It is named rather than falling into the
		// default so that an old bookmark is refused instead of silently
		// saving whatever members the form carried.
		if action := r.PostForm.Get("action"); action != "save" {
			localProblem(w, r, v.You, http.StatusBadRequest, "That group action is not available.")
			return
		}
		name := r.PostForm.Get("name")
		created := r.PostForm.Get("new") == "1"
		members := strings.Fields(r.PostForm.Get("members"))
		if members == nil {
			members = []string{} // an empty list, not an absent one: JSON null would change nothing
		}
		sort.Strings(members)
		// A group is one form and, once it exists, one change: its members
		// are its allow list, so description, members, Maintainers and
		// Personal go to the daemon together and are refused together
		// (docs/constitution.md#-group). Registration is SetGroup, which is
		// the one call that creates a group; the protected group's members
		// change only through it too, and it takes the description alone.
		change := core.Management{Name: name}
		if r.PostForm.Has("descr") {
			descr := r.PostForm.Get("descr")
			change.Descr = &descr
		}
		if r.PostForm.Has("edit_sharing") {
			maintainers := protocol.MaintainerList(strings.Fields(r.PostForm.Get("maintainers")))
			if maintainers == nil {
				maintainers = protocol.MaintainerList{}
			}
			change.Maintainers = &maintainers
		}
		if r.PostForm.Has("edit_personal") {
			personal := r.PostForm.Get("personal") == "on"
			change.Personal = &personal
		}
		var err error
		viaSetGroup := created || strings.EqualFold(strings.TrimSpace(name), core.AdministratorsGroup)
		if !viaSetGroup {
			// A save naming no group registers one, as it always has: SetGroup
			// answers for an absent name exactly as a registration would.
			var groups map[string][]string
			if err := c.get(cookie(r), "/groups", &groups); err != nil {
				fail(w, r, v.You, err)
				return
			}
			if _, exists := groups[strings.ToLower(strings.TrimSpace(name))]; !exists {
				viaSetGroup, created = true, true
			}
		}
		if !viaSetGroup {
			withMembers := change
			withMembers.Allow = &members
			err = c.post(cookie(r), "/manage", withMembers)
		}
		// A Personal group is named for its owner, who registering is the
		// caller. Said before anything is created, so a refusal leaves no half
		// made group behind (docs/constitution.md#-group).
		if prefix := "@" + v.You + "/"; created && change.Personal != nil && *change.Personal &&
			!strings.HasPrefix(strings.ToLower(strings.TrimSpace(name)), strings.ToLower(prefix)) {
			localProblem(w, r, v.You, http.StatusBadRequest,
				"A Personal group is named for its owner: call it "+prefix+"<name>.")
			return
		}
		if viaSetGroup {
			err = c.post(cookie(r), "/group", struct {
				Name    string
				Members []string
			}{name, members})
			// What SetGroup does not carry follows it, and only when there is
			// something to say: a new group's Owner is its creator, so the
			// owner-only fields are theirs to set. The protected group takes
			// the description alone.
			if created && change.Descr != nil && *change.Descr == "" {
				change.Descr = nil
			}
			if !created || change.Personal != nil && !*change.Personal {
				change.Personal = nil
			}
			change.Maintainers = nil
			if err == nil && (change.Descr != nil || change.Personal != nil) {
				if err = c.post(cookie(r), "/manage", change); err != nil && created {
					localProblem(w, r, v.You, http.StatusBadGateway,
						"The group was registered and its description or classification was not stored: "+sectionProblem("group", err)+
							" Change them on its settings page.")
					return
				}
			}
		}
		// The secret is its own verb, exactly as for a service
		// (docs/06-services.md#secrets). Empty means leave the stored one.
		if secret := formSecret(r.PostForm.Get("secret")); err == nil && secret != "" {
			if secretErr := c.post(cookie(r), "/secret", struct {
				Name   string `json:"name"`
				Secret string `json:"secret"`
			}{name, secret}); secretErr != nil {
				localProblem(w, r, v.You, http.StatusBadGateway,
					"The group was saved and its secret was not stored: "+sectionProblem("secret", secretErr)+
						" Set it with: agent-bus secret "+name+" '...'")
				return
			}
		}
		if err != nil {
			code, message, preserve := formRefusal(err)
			// A refusal naming one of the typed lines is about that line,
			// whatever its code, and says so; the lines stay as typed.
			field := ""
			if code != 0 {
				lists := []listField{{"members", "Members", r.PostForm.Get("members")}}
				if r.PostForm.Has("edit_sharing") {
					lists = append(lists, listField{"maintainers", "Maintainers", r.PostForm.Get("maintainers")})
				}
				field, message = lineRefusal(message, lists...)
			}
			if preserve || field != "" {
				v.Form = retainedForm("save", message, r.PostForm, "name", "members", "new", "descr", "maintainers", "personal")
				v.Form.Field = field
				if r.PostForm.Get("new") == "1" {
					if field == "" {
						v.Form.Field = "name"
					}
					v.Current = "groups"
					renderForm(w, code, groupNew, v)
					return
				}
				if v.Form.Field == "" {
					v.Form.Field = "members"
				}
				v.GroupName = r.PostForm.Get("name")
				if groupsErr := c.loadGroups(r, &v); groupsErr != nil {
					fail(w, r, v.You, groupsErr)
					return
				}
				if _, exists := v.Groups[v.GroupName]; !exists {
					fail(w, r, v.You, err)
					return
				}
				v.GroupMembers = strings.Fields(r.PostForm.Get("members"))
				v.GroupRecord, v.GroupVisible = v.GroupRecords[v.GroupName]
				v.CanEditGroup = v.mayEditGroup(v.GroupName)
				if !v.CanEditGroup {
					fail(w, r, v.You, err)
					return
				}
				v.Editing = true
				renderForm(w, code, groupEdit, v)
				return
			}
			fail(w, r, v.You, err)
			return
		}
		http.Redirect(w, r, "/group?name="+url.QueryEscape(r.PostForm.Get("name")), http.StatusSeeOther)
	}))
}

var serviceList = template.Must(template.New("services").Funcs(template.FuncMap{"identityKind": identityKind, "readerCount": readerCount, "registrationUpdated": registrationUpdated, "entityLabel": entityLabel, "copies": copies, "deliveryLabel": deliveryLabel, "external": external, "titleMark": titleMark, "number": number, "href": detailPath}).Parse(shellTitle("records", `{{if .Queues}}Queues{{else if .PubSub}}PubSub{{else if .Agents}}Agents{{else if .PersonalPage}}Personal{{else}}Services{{end}}`) + `
<div class=page-title><h1>{{if .Queues}}{{titleMark "queues"}} Queues{{else if .PubSub}}{{titleMark "pubsub"}} PubSub{{else if .Agents}}{{titleMark "agent"}} Agents{{else if .PersonalPage}}{{titleMark "personal"}} Personal{{else}}{{titleMark "services"}} Services{{end}}</h1><button type=button class=help-button popovertarget=service-views-help aria-label="About service views">ⓘ</button></div>
<div popover id=service-views-help class=context-help><h2>About these records</h2><ul>
{{if .Queues}}<li>All counts the caller-visible queues. A user&rsquo;s own inbox is Personal and is on Users instead.</li><li>A queue holds work and hands each message to one reader, without a service process of its own.</li>{{else if .PubSub}}<li>All counts the caller-visible pub/sub topics.</li><li>A pub/sub topic copies each accepted message to the inboxes on its Deliver-To list and holds no backlog of its own.</li>{{else if .PersonalPage}}<li>Personal holds records of every kind but a user&rsquo;s own whose Owner marked them Personal: their audience is the Owner and the Owner&rsquo;s own agents. The shared pages omit them; a user&rsquo;s own record is always Personal and is on Users instead.</li><li>Hiding a record from the shared pages revokes nothing: whoever its allow list admits still reaches it.</li>{{else if .Agents}}<li>An agent is a name on this bus with a queue something reads. All and My omit Personal agents; My is the caller-owned subset. Personal is an owner-set classification and does not revoke access.</li>{{else}}<li>A service record is information about something external: where it is, how to speak to it, what it is for, and the credential to use. It is not on this bus, so nothing is sent to it and nothing reads it here.</li><li>Who may read that information is the record&rsquo;s allow list, exactly as for any other record.</li>{{end}}
<li>Category counts use the records visible to you before toolbar filters; they are not node-wide totals or page counts.</li>
<li>Status is Active or Inactive. An inactive record is hidden from every other listing and answers as unknown to every call but its reactivation; the Inactive filter is the one view of it.{{if not .Services}} Active does not establish that a send will be accepted; the owner&rsquo;s access, the ACL and queue capacity are checked separately.{{end}}</li>
{{if not .Services}}<li>Readers counts outstanding filtered and unfiltered reads. Zero may be between reads; a positive count proves neither a matching message nor completed work.</li>
<li>Accepted and Dequeued are cumulative across restarts. Dequeued means handed to a reader, not completed.</li>
<li>Reached external is a caller-supplied hint. None of it is health; the daemon does not observe whether a process is alive.</li>{{end}}
</ul></div>
<nav class=section-nav aria-label="{{if .Queues}}Queue{{else if .PubSub}}PubSub{{else if .PersonalPage}}Personal{{else if .Agents}}Agent{{else}}Service{{end}} views">{{range .SectionLinks}}{{if .Current}}<a href="{{.Href}}" aria-current=true class="{{.Class}}">{{else}}<a href="{{.Href}}" class="{{.Class}}">{{end}}{{.Label}}{{if .Counted}} ({{number .Count}}){{end}}</a>{{end}}</nav>
{{if .PersonalPage}}{{if .DaemonOwner}}<form method=get><label>Owner <select name=owner><option value="">All visible owners</option>{{range .Owners}}<option {{if eq . $.OwnerFilter}}selected{{end}}>{{.}}</option>{{end}}</select></label>{{with .State}}<input type=hidden name=state value="{{.}}">{{end}}{{with .Readers}}<input type=hidden name=readers value="{{.}}">{{end}}{{with .Work}}<input type=hidden name=work value="{{.}}">{{end}}{{with .Query}}<input type=hidden name=q value="{{.}}">{{end}}{{with .Kind}}<input type=hidden name=kind value="{{.}}">{{end}}{{with .Sort}}<input type=hidden name=sort value="{{.}}">{{end}} <button>Choose owner</button></form>{{else}}<p>Owned by <code>{{.You}}</code></p>{{end}}{{end}}
<div class=record-toolbar>
<form class=record-search method=get action="{{if .Queues}}/queues{{else if .PubSub}}/pubsub{{else if .Agents}}/agents{{else if .PersonalPage}}/personal{{else}}/services{{end}}">
<label for=record-query class=visually-hidden>Search records</label><input id=record-query type=search name=q value="{{.Query}}" placeholder="Search by name, owner, or description">
{{with .Mine}}<input type=hidden name=scope value="{{.}}">{{end}}{{with .State}}<input type=hidden name=state value="{{.}}">{{end}}{{with .Readers}}<input type=hidden name=readers value="{{.}}">{{end}}{{with .Work}}<input type=hidden name=work value="{{.}}">{{end}}{{with .Kind}}<input type=hidden name=kind value="{{.}}">{{end}}{{with .OwnerFilter}}<input type=hidden name=owner value="{{.}}">{{end}}
<div class=record-choices>{{if .PersonalPage}}<nav class=filter-nav aria-label="Kind filter"><span>Kind</span>{{range .KindLinks}}{{if .Current}}<a href="{{.Href}}" aria-current=true>{{else}}<a href="{{.Href}}">{{end}}{{.Label}}</a>{{end}}</nav>{{end}}<nav class=filter-nav aria-label="Status filter"><span>Status</span>{{range .FilterLinks}}{{if .Current}}<a href="{{.Href}}" aria-current=true>{{else}}<a href="{{.Href}}">{{end}}{{.Label}}</a>{{end}}</nav>{{if not (or .Services .PubSub)}}<nav class=filter-nav aria-label="Reader filter"><span>Readers</span>{{range .ReaderLinks}}{{if .Current}}<a href="{{.Href}}" aria-current=true>{{else}}<a href="{{.Href}}">{{end}}{{.Label}}</a>{{end}}</nav><nav class=filter-nav aria-label="Queue filter"><span>Queue</span>{{range .WorkLinks}}{{if .Current}}<a href="{{.Href}}" aria-current=true>{{else}}<a href="{{.Href}}">{{end}}{{.Label}}</a>{{end}}</nav>{{end}}</div>
<label for=record-sort>Sort</label><select id=record-sort name=sort data-submit-on-change><option value="" {{if eq .Sort ""}}selected{{end}}>Name (A&ndash;Z)</option><option value=updated {{if eq .Sort "updated"}}selected{{end}}>Recently updated</option>{{if not .Services}}<option value=queued {{if eq .Sort "queued"}}selected{{end}}>{{if .PubSub}}Accepted{{else}}Queued{{end}} (high&ndash;low)</option>{{end}}</select><noscript><button>Apply</button></noscript>
</form>
</div>
{{if eq .CategoryTotal 0}}
{{if and .PersonalHere (not .PersonalPage)}}<section class="empty-state editor-card personal-elsewhere"><h2>No shared {{if .Queues}}queues{{else if .PubSub}}pub/sub topics{{else if .Agents}}agents{{else}}services{{end}}</h2>
<p>{{number .PersonalHere}} Personal {{if .Queues}}queue{{else if .PubSub}}pub/sub topic{{else if .Agents}}agent{{else}}service{{end}}{{if eq .PersonalHere 1}} is{{else}}s are{{end}} under the <a class=personal-view href="/personal?kind={{if .Queues}}queue{{else if .PubSub}}pubsub{{else if .Agents}}agent{{else}}service{{end}}">Personal</a> tab, which this list omits.</p>
{{else}}<section class="empty-state editor-card"><h2>No {{if .Queues}}queues{{else if .PubSub}}pub/sub topics{{else if .Agents}}agents{{else if .PersonalPage}}Personal records{{else}}services{{end}} yet</h2>{{end}}
{{if .Queues}}<p>A queue holds work without a separate service process and hands each message to one reader.</p><p><a href=/queues/new>Register a queue</a></p>
{{else if .PubSub}}<p>A pub/sub topic copies each accepted message to the inboxes on its Deliver-To list and keeps nothing itself.</p><p><a href=/pubsub/new>Register a pub/sub topic</a></p>
{{else if .Agents}}<p>An agent is a name on this bus with a queue something reads. It carries no address of its own: callers send to the name and the daemon delivers.</p><p><a href=/agents/new>Register an agent</a></p>
{{else if .PersonalPage}}<p>Personal is an Owner-set classification for any record but a user&rsquo;s own. It moves the record off the shared pages and revokes no access.</p><p><a href=/agents/new>Register an agent</a></p>
{{else}}<p>A service record tells the people and agents its allow list admits where something external is, how to speak to it and what it is for. Nothing is sent to it here.</p><p><a href=/services/new>Register a service</a></p>{{end}}</section>
{{else}}
{{if .PersonalPage}}<p class=muted>{{if .DaemonOwner}}This per-owner view contains only Personal records visible through your normal access; it is not a node-wide inventory.{{else}}Your Personal records.{{end}}</p>{{end}}
{{if eq .Matched 0}}<section class="empty-state editor-card"><h2>No records match these filters</h2><p>Change the active filters above or <a href="{{.ClearFilters}}">clear filters</a>.</p></section>{{else}}
<table class=record-table><caption>Showing {{number .Start}}&ndash;{{number .End}} of {{number .Matched}} matching records, caller-visible on this page and not a count of this node.{{if .HasFilters}} <a href="{{.ClearFilters}}">Clear filters</a>{{end}}</caption>
{{if .Queues}}<thead><tr><th scope=col>Queue<th scope=col>Type<th scope=col>Owner<th scope=col>Status<th scope=col class=num>Readers<th scope=col class=num>Held<th scope=col class=num>Accepted<th scope=col class=num>Dequeued<th scope=col>Updated</tr></thead>
{{else if .PubSub}}<thead><tr><th scope=col>PubSub<th scope=col>Type<th scope=col>Owner<th scope=col>Status<th scope=col class=num>Accepted<th scope=col class=num>Copies out<th scope=col class=num>Deliver-To<th scope=col>Updated</tr></thead>
{{else if .Services}}<thead><tr><th scope=col>Service<th scope=col>Type<th scope=col>Owner<th scope=col>Status<th scope=col>Address<th scope=col>Protocol<th scope=col>Updated</tr></thead>
{{else}}<thead><tr><th scope=col>{{if .PersonalPage}}Record{{else if .Agents}}Agent{{else}}Service{{end}}<th scope=col>Type<th scope=col>Owner<th scope=col>Status<th scope=col class=num>Readers<th scope=col>Reached<th scope=col class=num>Queued<th scope=col class=num>Accepted<th scope=col class=num>Dequeued<th scope=col>Updated</tr></thead>{{end}}
<tbody>
{{range .Records}}<tr><td class="record-name-cell{{if eq .Owner $.You}} owned-record{{end}}{{if .Personal}} personal-record{{end}}"><a class=record-name href="{{href .}}?name={{.Name}}&return={{$.Return}}">{{if .Descr}}<span class=record-description>{{.Descr}}</span><code>{{.Name}}</code>{{else}}<code class=record-description>{{.Name}}</code>{{end}}</a>{{if .Personal}} <span class=personal-marker>Personal</span>{{end}}
{{if $.Queues}}<td data-label=Type>{{identityKind .Kind .Name $.NodeOwner}}<td data-label=Owner><code>{{.Owner}}</code><td data-label=Status>{{if eq .Status "inactive"}}<span class=status-glyph role=img aria-label=Inactive title=Inactive>🚫</span>{{else}}<span class=status-glyph role=img aria-label=Active title=Active>🔛</span>{{end}}<td class=num data-label=Readers>{{readerCount .Readers}}<td class=num data-label=Held>{{number .Queued}}{{if .AtBound}} <span class=warn>at capacity when observed</span>{{end}}<td class=num data-label=Accepted>{{number .In}}<td class=num data-label=Dequeued>{{number .Out}}<td data-label=Updated>{{if .At.IsZero}}<span class=muted>&iquest;</span>{{else}}{{registrationUpdated .At}}{{end}}
{{else if $.PubSub}}<td data-label=Type>{{identityKind .Kind .Name $.NodeOwner}}<td data-label=Owner><code>{{.Owner}}</code><td data-label=Status>{{if eq .Status "inactive"}}<span class=status-glyph role=img aria-label=Inactive title=Inactive>🚫</span>{{else}}<span class=status-glyph role=img aria-label=Active title=Active>🔛</span>{{end}}<td class=num data-label=Accepted>{{number .In}}<td class=num data-label="Copies out">{{number .Out}}<td class=num data-label=Deliver-To>{{number (len .Subs)}}<td data-label=Updated>{{if .At.IsZero}}<span class=muted>&iquest;</span>{{else}}{{registrationUpdated .At}}{{end}}
{{else if $.Services}}<td data-label=Type>{{identityKind .Kind .Name $.NodeOwner}}<td data-label=Owner><code>{{.Owner}}</code><td data-label=Status>{{if eq .Status "inactive"}}<span class=status-glyph role=img aria-label=Inactive title=Inactive>🚫</span>{{else}}<span class=status-glyph role=img aria-label=Active title=Active>🔛</span>{{end}}<td data-label=Address><code>{{.Addr}}</code><td data-label=Protocol><code>{{.Proto}}</code><td data-label=Updated>{{if .At.IsZero}}<span class=muted>&iquest;</span>{{else}}{{registrationUpdated .At}}{{end}}
{{else}}<td data-label=Type>{{identityKind .Kind .Name $.NodeOwner}}<td data-label=Owner><code>{{.Owner}}</code><td data-label=Status>{{if eq .Status "inactive"}}<span class=status-glyph role=img aria-label=Inactive title=Inactive>🚫</span>{{else}}<span class=status-glyph role=img aria-label=Active title=Active>🔛</span>{{end}}<td class=num data-label=Readers>{{readerCount .Readers}}<td data-label=Reached>{{if external .}}external{{else}}<span class=muted>&mdash;</span>{{end}}<td class=num data-label=Queued>{{number .Queued}}{{if .AtBound}} <span class=warn>at capacity when observed</span>{{end}}<td class=num data-label=Accepted>{{number .In}}<td class=num data-label=Dequeued>{{number .Out}}<td data-label=Updated>{{if .At.IsZero}}<span class=muted>&iquest;</span>{{else}}{{registrationUpdated .At}}{{end}}{{end}}</tr>{{end}}
</tbody></table>
{{end}}
<nav aria-label="Record pages">{{with .Previous}}<a href="{{.}}">Previous page</a>{{end}} {{with .Next}}<a href="{{.}}">Next page</a>{{end}}</nav>
{{end}}
`))

// serviceNew is every registration form. The fields follow the kind, and
// they are the same fields the settings page asks for: both render the one
// block in recordform.go. Every section registers one kind.
var serviceNew = template.Must(template.New("service-new").Funcs(recordFormFuncs).Parse(shellTitle("records", `{{template "new-heading" .}}`) + `
<p><a href="{{if .Agents}}/agents{{else if .Queues}}/queues{{else if .PubSub}}/pubsub{{else}}/services{{end}}">Back to {{if .Agents}}Agents{{else if .Queues}}Queues{{else if .PubSub}}PubSub{{else}}Services{{end}}</a></p>
<div class=page-title><h1>{{titleMark .NewKind}} {{template "new-heading" .}}</h1></div>
` + formErrorSummary + `<form id=form-create class="editor-card task-card" method=post action=/service><input type=hidden name=action value=create><input type=hidden name=kind value={{.NewKind}}>` +
	`{{template "record-fields" .}}<div class=form-actions><button>{{template "new-heading" .}}</button></div></form>
{{template "record-field-help" .}}
{{define "new-heading"}}Register {{if eq .NewKind "agent"}}agent{{else if eq .NewKind "service"}}service{{else if eq .NewKind "queue"}}queue{{else if eq .NewKind "pubsub"}}pub/sub topic{{else}}channel{{end}}{{end}}` + recordFields + recordFieldHelp))
var serviceDetail = template.Must(template.New("service").Funcs(template.FuncMap{"identityKind": identityKind, "onList": onList, "href": detailPath, "join": strings.Join, "readerCount": readerCount, "entityLabel": entityLabel, "copies": copies, "external": external, "deliveryMode": deliveryMode, "recordNoun": recordNoun, "titleMark": titleMark, "photoData": photoData, "profileInitial": profileInitial, "number": number, "routes": routes, "routeState": routeState}).Parse(shellTitle("records", `{{recordNoun .Record.Kind}} {{.Record.Name}}`) + `
<p><a href="{{.Return}}">Back to records</a></p>{{with .Record}}<div class=page-title><h1>{{titleMark .Kind}} {{.Name}}{{if .Personal}} <span class=muted>· Personal</span>{{end}}</h1></div>` + formErrorSummary + `<div class=detail-meta><span class=fact-pill>{{identityKind .Kind .Name $.NodeOwner}}</span><span>Owner: {{with $.OwnerUser}}<span class=identity-with-photo>{{with photoData .}}<img class=profile-photo src="{{.}}" alt="">{{else}}<span class=profile-initial aria-hidden=true>{{profileInitial .}}</span>{{end}}<a href="/user?name={{.Name}}"><code>{{.Name}}</code></a></span>{{else}}<code>{{.Owner}}</code>{{end}}</span>{{with .Maintainers}}<span>👮 Maintainers: {{join . ", "}}</span>{{end}}{{with deliveryMode .}}<span>Delivery: {{.}}</span>{{end}}</div>
<div class=service-dashboard><section class=fact-card><div class=page-title><h2>Status</h2><button type=button class=help-button popovertarget=status-help aria-label="About record status" data-tooltip="Active or inactive. Active does not establish that a send will be accepted: the daemon also checks owner access, the ACL and queue capacity.">ⓘ</button></div>
<p><strong><span class=status-glyph role=img aria-label=Active title=Active>🔛</span></strong> Active</p>{{if and .CanManage (ne .Kind "user")}}<form class=record-state-action method=get action=/service-deactivate><input type=hidden name=name value="{{.Name}}"><button class=danger-action>Deactivate…</button></form>{{end}}</section>
<div popover id=status-help class=context-help><h2>About record status</h2><ul><li>Active does not establish that a send will be accepted.</li><li>The daemon also checks owner access, the allow list and queue capacity.</li><li>An inactive record is hidden from listings and answers as unknown to every call but its reactivation. Its queued work is kept.</li><li>A record is also inactive while its owner is; it returns with its owner.</li></ul></div>
{{if external .}}<section class=fact-card><div class=page-title><h2>Where it is</h2><button type=button class=help-button popovertarget=external-help aria-label="About an external service" data-tooltip="What the record says about something outside this bus. The daemon stores these words and checks none of them.">ⓘ</button></div>
<dl><dt>Address<dd><strong><code>{{.Addr}}</code></strong><dt>Protocol<dd><strong><code>{{.Proto}}</code></strong><dt>Secret<dd>{{if .SecretSHA}}<code>{{.SecretSHA}}</code>{{else}}<span class=muted>none</span>{{end}}</dl>
<p class=muted>Updated {{if .At.IsZero}}&iquest;{{else}}{{.At.Format "2006-01-02 15:04:05"}}{{end}} · Config {{if .ConfigSHA}}<code>{{.ConfigSHA}}</code>{{else}}&mdash;{{end}}</p></section>
<div popover id=external-help class=context-help><h2>About an external service</h2><ul><li>This record is information for the people and agents its allow list admits: where the thing is, how to speak to it and what it is for.</li><li>The address and the protocol are the registration&rsquo;s own words. The daemon neither reaches the thing nor checks that it is there.</li><li>Nothing is sent to this name and nothing reads it here, so it has no delivery setting, no queue and no counters.</li><li>Secret is the digest of the stored credential, and this page shows nothing else of it. Whoever the allow list admits reads the bytes with <code>agent-bus secret {{.Name}}</code>.</li></ul></div>
{{else}}<section class=fact-card><div class=page-title><h2>Policy</h2><button type=button class=help-button popovertarget=policy-help aria-label="About record policy" data-tooltip="Queue values are this record's stored policy. External remains a caller hint, and unset values are described without guessing resolved defaults.">ⓘ</button></div>
<dl><dt>Reached<dd><strong>{{if .Proto}}external{{else}}this bus{{end}}</strong><dt>Queue bound<dd>{{if .Bound}}{{number .Bound}}{{else}}default{{end}}<dt>Retention<dd>{{if .TTL}}{{.TTL}}{{else}}none{{end}}<dt>When full<dd>{{if eq .Full "ring"}}drop the oldest{{else}}refuse{{end}}</dl></section>
<div popover id=policy-help class=context-help><h2>About record policy</h2><ul>{{if .Proto}}<li>External is a caller-supplied hint, not proof of anything.</li>{{end}}{{if not .Bound}}<li>An unset queue bound uses the daemon default; its resolved value is not readable here.</li>{{end}}{{if not .TTL}}<li>Unset retention means no queue-imposed expiry; a message may still specify its own.</li>{{end}}<li>The overflow policy always belongs to this record.</li></ul></div>
<section class=fact-card><div class=page-title><h2>Queue &amp; counters</h2><button type=button class=help-button popovertarget=observed-help aria-label="About live counters" data-tooltip="Readers are outstanding requests, not health. Queue reads may precede pruning. Counters survive restart; Dequeued is not completion.">ⓘ</button></div>
<dl><dt>Readers<dd><strong>{{readerCount .Readers}}</strong><dt>Held now<dd><strong>{{number .Queued}}</strong>{{if .AtBound}} <span class=warn>at capacity when observed</span>{{end}}<dt>Oldest held<dd>{{if .Oldest}}{{.Oldest}}{{else}}<span class=muted>&mdash;</span>{{end}}<dt>Accepted<dd>{{number .In}}<dt>Dequeued<dd>{{number .Out}}<dt>Dropped / expired<dd>{{number .Dropped}} / {{number .Expired}}</dl>
<p class=muted>Updated {{if .At.IsZero}}&iquest;{{else}}{{.At.Format "2006-01-02 15:04:05"}}{{end}} · Config {{if .ConfigSHA}}<code>{{.ConfigSHA}}</code>{{else}}&mdash;{{end}}</p></section>
<div popover id=observed-help class=context-help><h2>About observed counters</h2><ul><li>Readers counts outstanding filtered and unfiltered reads; zero is not an offline signal.</li><li>The read does not prune first, so Held and Oldest may include work already past its TTL.</li><li>Counters are cumulative across restarts. Dequeued is handed to a reader, which is not completed.</li></ul></div>{{end}}
{{template "activity-view" $}}
<p class=activity-fact><a href="/activity?name={{.Name}}">View all activity and slot values</a></p></div>
{{if routes .Kind}}<section class="dashboard-section route" id=route><h2>Deliver-To route</h2>
{{with .Subs}}{{$dest := index . 0}}<p>Forwards to <code>{{$dest}}</code>: a message sent to <code>{{$.Record.Name}}</code> moves there and is not kept here.</p>
{{$state := routeState $.Record}}{{if eq $state "allowed"}}<p class=route-state><strong>Route allowed now.</strong> <code>{{$dest}}</code>&rsquo;s allow list admits <code>{{$.Record.Name}}</code> at this moment.</p>{{else if eq $state "refused"}}<p class="route-state warn"><strong>Configured, but <code>{{$dest}}</code> does not allow <code>{{$.Record.Name}}</code> now</strong> &mdash; or <code>{{$dest}}</code> is inactive or no longer registered. Sends to <code>{{$.Record.Name}}</code> are refused until it does; the route stays configured.</p>{{else}}<p class=route-state>Whether this route is usable now was not reported.</p>{{end}}
<p>The identity checked at <code>{{$dest}}</code> is <code>{{$.Record.Name}}</code> itself &mdash; not the original sender, and not this record&rsquo;s Owner. A sender still has to pass <code>{{$.Record.Name}}</code>&rsquo;s own allow list: an allowed route admits nobody that list does not.</p>
{{else}}<p>No route. A message sent here stays in this {{recordNoun $.Record.Kind}}&rsquo;s own queue.</p>{{end}}
{{if .CanManage}}<p><a href="{{href .}}/edit?name={{.Name}}{{with $.Return}}&amp;return={{urlquery .}}{{end}}">{{if .Subs}}Replace or clear the route{{else}}Set a route{{end}} in the settings</a></p>{{end}}</section>{{end}}
{{if copies .Kind}}<details><summary>Deliver-To</summary>
<p class=muted>Who receives a copy of every publication. Whoever manages the channel writes this list; the allow list above it is who may publish.</p>
{{range .Subs}}<p><code>{{.}}</code>{{if $.Record.CanManage}} <form method=post action=/service><input type=hidden name=name value="{{$.Record.Name}}"><input type=hidden name=subscriber value="{{.}}"><button name=action value=remove-subscriber>Remove</button></form>{{end}}</p>{{else}}<p>Nobody. A publication here reaches no inbox.</p>{{end}}
{{if onList .Subs $.You}}<form method=post action=/service><input type=hidden name=name value="{{.Name}}"><button name=action value=unsubscribe>Take my inbox off this list</button></form>{{end}}</details>{{end}}
{{if .CanManage}}
<section class="editor-card compact-card"><h2>Settings</h2><p>Description, address, queue policy, access{{if copies .Kind}}, Deliver-To{{end}}{{if routes .Kind}}, the Deliver-To route{{end}}, Personal and Maintainers are edited on one page, the same one that registered this {{recordNoun .Kind}}.</p>
<p><a id=settings class=editor-link href="{{href .}}/edit?name={{.Name}}{{with $.Return}}&amp;return={{urlquery .}}{{end}}">Edit settings</a></p></section>
<p><a class=danger href="/service-danger?name={{.Name}}">Danger Zone</a></p>
{{else}}<p>{{.Descr}}</p><p>You can view this record; its owner and assigned maintainers can manage it.</p>{{end}}{{end}}` + activityViewTemplate))

// inactiveDetail is the one read-only view of an inactive record, built from
// its /inactive entry: nothing else answers for it
// (docs/constitution.md#common-record-fields).
var inactiveDetail = template.Must(template.New("inactive-record").Funcs(template.FuncMap{"identityKind": identityKind, "recordNoun": recordNoun, "titleMark": titleMark, "number": number, "external": external}).Parse(shellTitle("records", `{{recordNoun .Record.Kind}} {{.Record.Name}}`) + `
<p><a href="{{.Return}}">Back to records</a></p>{{with .Record}}<div class=page-title><h1>{{titleMark .Kind}} {{.Name}} <span class="state-badge state-inactive">INACTIVE</span></h1></div>
<div class=detail-meta><span class=fact-pill>{{identityKind .Kind .Name $.NodeOwner}}</span><span>Owner: <code>{{.Owner}}</code></span></div>
<section class=fact-card><div class=page-title><h2>Status</h2></div><p><strong><span class=status-glyph role=img aria-label=Inactive title=Inactive>🚫</span></strong> Inactive</p>
<ul><li>This record is hidden from listings and answers as unknown to every call but its reactivation.</li><li>Its queued work is kept{{if not (external .)}}: {{number .Queued}} held when observed{{end}}.</li><li>A record is also inactive while its owner is. It then returns with its owner, and reactivating it here is refused until then.</li></ul>
{{if and .CanManage (ne .Kind "user")}}<form class=record-state-action method=post action=/service><input type=hidden name=name value="{{.Name}}"><button name=action value=reactivate>Reactivate</button></form>{{else}}<p class=muted>Its owner and assigned maintainers can reactivate it.</p>{{end}}</section>{{end}}`))

// recordDeactivate confirms the one record edit that hides it.
var recordDeactivate = template.Must(template.New("record-deactivate").Funcs(template.FuncMap{"titleMark": titleMark, "href": detailPath, "number": number}).Parse(shellTitle("records", `Confirm deactivation · {{.Record.Name}}`) + `
{{with .Record}}<p><a href="{{href .}}?name={{.Name}}">Back to {{.Name}}</a></p>
<div class=page-title><h1>{{titleMark "problem"}} Confirm deactivation</h1></div>
<section class="editor-card compact-card"><p>Deactivate <code>{{.Name}}</code>?</p><ul><li>It is hidden from listings and answers as unknown to every call but its reactivation.</li><li>Queued work is kept ({{number .Queued}} held now), and running processes are not stopped.</li><li>Its owner or an assigned Maintainer can reactivate it from the Inactive view.</li></ul>
<form method=post action=/service><input type=hidden name=name value="{{.Name}}"><button class=danger-action name=action value=deactivate>Deactivate</button> <a href="{{href .}}?name={{.Name}}">Cancel</a></form></section>{{end}}`))
var serviceDanger = template.Must(template.New("service-danger").Funcs(template.FuncMap{"readerCount": readerCount, "href": detailPath, "titleMark": titleMark, "holdsConfig": holdsConfig}).Parse(shellTitle("records", `Danger Zone · {{.Record.Name}}`) + `
{{with .Record}}<p><a href="{{href .}}?name={{.Name}}">Back to {{.Name}}</a></p>
<div class=page-title><h1>{{titleMark "problem"}} Danger Zone · {{.Name}}</h1></div>` + formErrorSummary + `
{{if holdsConfig .Kind}}<h2>Replace configuration</h2><form id=form-configure method=post action=/service><input type=hidden name=name value="{{.Name}}"><input type=hidden name=action value=configure><label>New configuration <textarea name=config rows=6 cols=60 required autocomplete=off aria-invalid="{{if $.Form.Invalid "config"}}true{{else}}false{{end}}" aria-describedby="{{if $.Form.Invalid "config"}}configure-error{{end}}"></textarea></label><p>Existing private configuration and a refused replacement are never displayed.</p>{{if $.Form.Is "configure"}}<p class=warn id=configure-error>{{$.Form.Error}}</p>{{end}}<button>Replace configuration</button></form>{{end}}
{{if and .CanTransfer (ne .Name "@administrators")}}{{if ne .Name .Owner}}<h2>Transfer ownership</h2><form id=form-transfer method=post action=/service-confirm><input type=hidden name=name value="{{.Name}}"><input type=hidden name=action value=transfer><label>New owner <input name=owner required value="{{if $.Form.Is "transfer"}}{{$.Form.Value "owner"}}{{end}}" aria-invalid="{{if $.Form.Invalid "owner"}}true{{else}}false{{end}}" aria-describedby="{{if $.Form.Invalid "owner"}}transfer-error{{end}}"></label>{{if $.Form.Is "transfer"}}<p class=warn id=transfer-error>{{$.Form.Error}}</p>{{end}}<p>The new owner must have a registered identity. Existing service credentials remain valid; transfer does not revoke copies already held.</p><button>Continue to confirmation</button></form>{{end}}{{end}}
{{if ne .Kind "group"}}<h2>Remove registration</h2><form method=post action=/service-confirm><input type=hidden name=name value="{{.Name}}"><input type=hidden name=action value=delete><p><strong>No registration, no access:</strong> the credential goes with the address. Drain the queue and stop readers first; the confirmation page re-reads both before describing the consequence.</p><button>Continue to confirmation</button></form>{{else}}<p class=muted>A group is retired by emptying its members, never removed: its name is what every list naming it resolves.</p>{{end}}{{end}}`))
var serviceConfirm = template.Must(template.New("service-confirm").Funcs(template.FuncMap{"readerSnapshot": readerSnapshot, "readerCount": readerCount, "titleMark": titleMark}).Parse(shellTitle("records", `{{if eq .Action "transfer"}}Confirm ownership transfer{{else}}Confirm removal{{end}} · {{.Record.Name}}`) + `
{{with .Record}}<p><a href="/service-danger?name={{.Name}}">Back to the Danger Zone</a></p>
{{if eq $.Action "transfer"}}<div class=page-title><h1>{{titleMark "problem"}} Confirm ownership transfer</h1></div><p>Transfer <code>{{.Name}}</code> from <code>{{.Owner}}</code> to <code>{{$.NewOwner}}</code>?</p><p>The new owner must still be registered and active when the daemon applies this. Credentials already held are not revoked.</p><form method=post action=/service><input type=hidden name=action value=transfer><input type=hidden name=name value="{{.Name}}"><input type=hidden name=owner value="{{$.NewOwner}}"><input type=hidden name=expected_owner value="{{.Owner}}"><input type=hidden name=confirmed value=1><button>Transfer ownership</button></form>
{{else}}<div class=page-title><h1>{{titleMark "problem"}} Confirm removal</h1></div><p>Remove <code>{{.Name}}</code>? It currently holds <strong>{{.Queued}}</strong> messages and has <strong>{{readerCount .Readers}}</strong> outstanding reads.</p><p>The address and its credential go with it; nothing answers to this name afterwards. A person&rsquo;s own credential stays because it is not this record&rsquo;s to remove.</p><form method=post action=/service><input type=hidden name=action value=delete><input type=hidden name=name value="{{.Name}}"><input type=hidden name=expected_owner value="{{.Owner}}"><input type=hidden name=expected_queued value="{{.Queued}}"><input type=hidden name=expected_readers value="{{readerSnapshot .Readers}}"><input type=hidden name=confirmed value=1><button>Remove registration</button></form>{{end}}{{end}}`))
var groupList = template.Must(template.New("groups").Funcs(template.FuncMap{"groupGlyph": groupGlyph, "titleMark": titleMark, "number": number}).Parse(shell("groups", "Groups") + `
<div class=page-title><h1>{{titleMark "groups"}} Groups</h1><button type=button class=help-button popovertarget=groups-help aria-label="About group membership" data-tooltip="Groups are reusable authority lists. Any user may register one; its Owner, Maintainers and the daemon Administrators edit its members.">ⓘ</button></div><div popover id=groups-help class=context-help><h2>Group membership</h2><ul><li>Select a group to inspect or edit its membership.</li><li><code>@owner</code> is runtime ACL syntax and cannot be created or nested as a group.</li><li>Any signed-in user may register a group and becomes its Owner.</li><li>A group&rsquo;s Owner, its Maintainers and the daemon Administrators change its membership; only the daemon owner changes <code>@administrators</code>.</li><li>Owner, Maintainers and members are shown for the groups you may see: those you are in or manage, or every group if you are an Administrator.</li></ul></div><nav class=section-nav aria-label="Group views">{{range .SectionLinks}}{{if .Current}}<a href="{{.Href}}" aria-current=true>{{else}}<a href="{{.Href}}">{{end}}{{.Label}}{{if .Counted}} ({{number .Count}}){{end}}</a>{{end}}</nav>
<table class="record-table group-table"><caption>{{number (len .Groups)}} groups</caption><thead><tr><th scope=col>Group</th><th scope=col>Owner</th><th scope=col>Maintainers</th><th scope=col>Members</th></tr></thead><tbody>{{range $name,$members := .Groups}}<tr><td data-label=Group><span role=img aria-label="Group">{{groupGlyph}}</span> <a href="/group?name={{urlquery $name}}"><code>{{$name}}</code></a>{{if eq $name "@administrators"}} <span class=fact-pill>protected</span>{{end}}</td>{{with index $.GroupRecords $name}}<td data-label=Owner><code>{{.Owner}}</code></td><td data-label=Maintainers>{{range $i,$m := .Maintainers}}{{if $i}}, {{end}}<code>{{$m}}</code>{{else}}<span class=muted>None</span>{{end}}</td>{{else}}<td data-label=Owner><span class=muted>Not visible to you</span></td><td data-label=Maintainers><span class=muted>Not visible to you</span></td>{{end}}<td data-label=Members>{{if not ($.MembersVisible $name)}}<span class=muted>Not visible to you</span>{{else}}{{range $i,$member := $members}}{{if $i}}, {{end}}<code>{{$member}}</code>{{else}}<span class=muted>No members</span>{{end}}{{end}}</td></tr>{{else}}<tr><td colspan=4>No groups registered.</td></tr>{{end}}</tbody></table>`))
var groupDetail = template.Must(template.New("group-detail").Funcs(template.FuncMap{"join": strings.Join, "groupGlyph": groupGlyph, "titleMark": titleMark, "recordKindPath": recordKindPath, "entityLabel": entityLabel}).Parse(shellTitle("groups", `Group {{.GroupName}}`) + `
<p><a href=/groups>Back to Groups</a></p><div class=page-title><h1><span role=img aria-label="Group">{{groupGlyph}}</span> <code>{{.GroupName}}</code></h1>{{if eq .GroupName "@administrators"}}<span class=fact-pill>protected</span><button type=button class=help-button popovertarget=administrators-help aria-label="About the Administrators group" data-tooltip="Members administer users and ordinary groups. They do not automatically manage every Service or Channel; assign this group as a resource Maintainer when that is wanted. Only the daemon Owner changes membership.">ⓘ</button>{{end}}</div>{{if eq .GroupName "@administrators"}}<p class=muted>Daemon administration group. What membership grants is stated below; every other group on this node confers only what a resource assigns it.</p><div popover id=administrators-help class=context-help><h2>How this group behaves</h2><ul><li>It accepts direct user identities only. A snapshot that nests a group inside it is refused at startup.</li><li>It cannot be emptied, and the daemon Owner is always a member.</li><li>Adding an Administrator creates a user profile; removing the membership keeps that profile.</li><li>An ordinary group may name <code>@administrators</code>. Its direct members then receive that ordinary group&rsquo;s access or Maintainer grant, and no Administrator authority.</li></ul></div>
<section class="dashboard-section admin-rights"><h2>What membership grants</h2><ul><li><strong>Ordinary users.</strong> See the whole user directory, register new users, and edit, deactivate or reactivate users below your own level.</li><li><strong>Ordinary groups.</strong> Change the membership of any ordinary group, including one assigned as a resource&rsquo;s Maintainer &mdash; you may add yourself, or a user you created, without asking that resource&rsquo;s owner.</li><li><strong>Membership lists.</strong> Read the full membership of every group.</li></ul>
<h2>What it does not grant</h2><ul><li><strong>The Owner, or each other.</strong> An Administrator cannot edit the daemon Owner or another Administrator, and cannot grant either position.</li><li><strong>Services, Queues and PubSub.</strong> Administering the node is not managing its resources. That comes only from being named in a resource&rsquo;s Maintainers list; put <code>@administrators</code> there when every Administrator should manage it.</li><li><strong>This group.</strong> Only the daemon Owner changes who is in it.</li></ul></section>{{end}}
<div class=detail-meta>{{if .GroupVisible}}<span>Owner: <code>{{.GroupRecord.Owner}}</code></span>{{if eq .GroupName "@administrators"}}<span>👮 Maintainers: none &mdash; this group has none, and its Owner follows daemon ownership</span>{{else}}<span>👮 Maintainers: {{with .GroupRecord.Maintainers}}{{join . ", "}}{{else}}none{{end}}</span>{{end}}{{if .GroupRecord.Personal}}<span>Personal</span>{{end}}{{else}}<span class=muted>Owner and Maintainers are not visible to you.</span>{{end}}</div>{{if .GroupVisible}}{{with .GroupRecord.Descr}}<p class=group-description>{{.}}</p>{{end}}{{end}}` + formErrorSummary + `
{{if .CanEditGroup}}<section class="editor-card compact-card"><h2>Members</h2><div class=member-list>{{range .GroupMembers}}<code class=member-line>{{.}}</code>{{else}}<span class=muted>No members</span>{{end}}</div>
<p><a id=members-edit class=editor-link href="/group/edit?name={{urlquery .GroupName}}">Edit group</a></p></section>{{else}}<section class="editor-card compact-card"><h2>Members</h2><div class=member-list>{{if not (.MembersVisible .GroupName)}}<span class=muted>Membership is not visible to you.</span>{{else}}{{range .GroupMembers}}<code class=member-line>{{.}}</code>{{else}}<span class=muted>No members</span>{{end}}{{end}}</div>{{if eq .GroupName "@administrators"}}<p class=muted>Only the daemon owner changes this protected group.</p>{{else}}<p class=muted>Its Owner, its Maintainers and the daemon Administrators change this group&rsquo;s membership.</p>{{end}}</section>{{end}}
<section class=dashboard-section><h2>Used by visible records</h2>{{if .GroupReferences}}<table><thead><tr><th scope=col>Record</th><th scope=col>Kind</th><th scope=col>Uses this group</th></tr></thead><tbody>{{range .GroupReferences}}<tr><td><a href="{{recordKindPath .Name .Kind}}"><code>{{.Name}}</code></a></td><td>{{entityLabel .Kind}}</td><td>{{range $i,$use := .Uses}}{{if $i}} · {{end}}{{$use}}{{end}}</td></tr>{{end}}</tbody></table>{{else}}<p class=muted>No caller-visible record refers to this group.</p>{{end}}</section>
{{if and .GroupVisible .GroupRecord.CanManage}}<p><a class=danger href="/service-danger?name={{urlquery .GroupName}}">Danger Zone</a></p>{{end}}`))

// A group has one form, and registering one and changing its membership are
// the same form. The two used to be separate markup and had already drifted
// in what they said about `@owner` and in how much of the list they showed.
// See Plans/MVP/web/forms.md#rules.
const groupFields = `{{define "group-fields"}}{{$assign := .GroupMayAssign}}<div class=form-grid>
{{if .Editing}}<input type=hidden name=name value="{{.GroupName}}">
{{else}}<label class="form-field form-field-wide">Name <input name=name placeholder="@operators" required value="{{.Form.Value "name"}}" aria-invalid="{{if .Form.Invalid "name"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Invalid "name"}}group-error{{end}}"><small>One name for the set. It cannot be changed afterwards. You become its Owner.</small>{{if eq (.Form.Value "name") "@owner"}}<small><code>@owner</code> is runtime ACL syntax and cannot be registered as a group.</small>{{end}}</label>
{{end}}
<label class="form-field form-field-wide">Description <input name=descr {{if not .GroupDescrEditable}}disabled{{end}} value="{{.GroupValue "descr" .GroupRecord.Descr}}" placeholder="What this group is for"><small>{{if .GroupDescrEditable}}Shown beside the group&rsquo;s name.{{else}}This group&rsquo;s record is not visible to you, so its description is left as it is.{{end}}</small></label>
<label class="form-field form-field-wide">Members <textarea name=members rows=8 placeholder="user@realm&#10;#agent@realm&#10;@nested-group" aria-invalid="{{if .Form.Invalid "members"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Invalid "members"}}group-error{{end}}">{{if .Form.Is "save"}}{{.Form.Value "members"}}{{else}}{{join .GroupMembers "\n"}}{{end}}</textarea><small>One user, <code>#agent</code> or nested group per line; <code>@owner</code> is reserved for ACLs.</small></label>
{{if $assign}}<input type=hidden name=edit_personal value=1>{{end}}<fieldset class=form-field-wide><legend>Classification</legend><div class=choice-row><label><input type=checkbox name=personal {{if not $assign}}disabled{{end}} {{if .GroupTicked "personal" .GroupRecord.Personal}}checked{{end}}> Personal</label><span class=muted>Puts this group in its Owner&rsquo;s Personal view; its members still receive what it is granted.</span></div><small>{{if $assign}}A Personal group&rsquo;s members and Maintainers may name only its Owner and the Owner&rsquo;s own agents.{{else if eq .GroupName "@administrators"}}The protected group is never Personal.{{else}}Shown for reference: only this group&rsquo;s Owner or the daemon Owner may change it.{{end}}</small></fieldset>
{{if .Editing}}{{if $assign}}<input type=hidden name=edit_sharing value=1>{{end}}<label class="form-field form-field-wide">Maintainers<textarea name=maintainers rows=5 {{if not $assign}}disabled{{end}} aria-invalid="{{if .Form.Invalid "maintainers"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Invalid "maintainers"}}group-error{{end}}">{{.GroupValue "maintainers" (join .GroupRecord.Maintainers "\n")}}</textarea><small>{{if $assign}}One user, group or agent per line. They change this group&rsquo;s members as its Owner does.{{else if eq .GroupName "@administrators"}}The protected group has no Maintainers: only the daemon Owner changes it.{{else}}Shown for reference: only this group&rsquo;s Owner or the daemon Owner may change it.{{end}}</small></label>{{end}}
<label class="form-field form-field-wide">Secret<textarea name=secret rows=4 autocomplete=off spellcheck=false placeholder="TOKEN=..." {{if not .GroupSecretEditable}}disabled{{end}}></textarea><small>{{if not .GroupSecretEditable}}Only this group&rsquo;s Owner and Maintainers write its secret.{{else if .Editing}}Leave empty to keep the stored secret. Anything here replaces it; every member reads it back with <code>agent-bus secret {{.GroupName}}</code>.{{else}}Optional. Every member reads it back with <code>agent-bus secret</code>.{{end}}</small></label></div>
{{if .Form.Is "save"}}<p class=warn id=group-error>{{.Form.Error}}</p>{{end}}{{end}}`

var groupNew = template.Must(template.New("group-new").Funcs(template.FuncMap{"titleMark": titleMark, "join": strings.Join}).Parse(shell("groups", "Register group") + `
<p><a href=/groups>Back to Groups</a></p><div class=page-title><h1>{{titleMark "groups"}} Register group</h1></div>
` + formErrorSummary + `<form id=form-save class="editor-card task-card" method=post action=/groups><input type=hidden name=action value=save><input type=hidden name=new value=1>` +
	`{{template "group-fields" .}}<div class=form-actions><button>Register group</button></div></form>` + groupFields))

// groupEdit is that same form with the group already in it.
var groupEdit = template.Must(template.New("group-edit").Funcs(template.FuncMap{"titleMark": titleMark, "join": strings.Join}).Parse(shellTitle("groups", `Edit {{.GroupName}}`) + `
<p><a href="/group?name={{urlquery .GroupName}}">Back to {{.GroupName}}</a></p><div class=page-title><h1>{{titleMark "groups"}} Edit <code>{{.GroupName}}</code></h1></div>
` + formErrorSummary + `<form id=form-save class="editor-card task-card" method=post action=/groups><input type=hidden name=action value=save>` +
	`{{template "group-fields" .}}<div class=form-actions><button>Save group</button></div></form>` + groupFields))

// holdsConfig says whether a kind carries a configuration at all: an Agent, a
// Service and a Group do, and nothing else (docs/constitution.md#-private-values).
// routes says whether a kind has a one-slot Deliver-To route: an 👾 and a 📮
// forward, a 📣 fans out through a list instead (docs/constitution.md#-channels).
func routes(kind string) bool { return kind == protocol.KindAgent || kind == protocol.KindQueue }

// routeState is the daemon's route_allowed read as a word: "allowed" and
// "refused" are what it said, and "" is that it said nothing. The pointer is
// dereferenced here because a template's if is true for any non-nil pointer,
// false included.
func routeState(r protocol.Record) string {
	switch {
	case r.RouteAllowed == nil:
		return ""
	case *r.RouteAllowed:
		return "allowed"
	}
	return "refused"
}

func holdsConfig(kind string) bool {
	return kind == protocol.KindAgent || kind == protocol.KindService || kind == protocol.KindGroup
}

// personalKind is a kind the Personal page can be narrowed to: every kind a
// section lists, which a User's own record and a Group are not.
func personalKind(k string) bool {
	return k == protocol.KindAgent || k == protocol.KindService || k == protocol.KindQueue || k == protocol.KindPubSub
}

// personalTab is the Agents section's Personal tab, which is also the Personal
// page's own: there it counts what the page holds, one kind or all of them;
// from Agents it counts Personal agents and asks for those.
func personalTab(v adminView, query url.Values, all int, of map[string]int) viewLink {
	tab := viewLink{Label: "Personal", Class: "personal-view", Counted: true, Current: v.PersonalPage}
	switch {
	case v.PersonalPage && v.Kind != "":
		query.Set("kind", v.Kind)
		tab.Href, tab.Count = pageURL("/personal", query), of[v.Kind]
	case v.PersonalPage:
		tab.Href, tab.Count = pageURL("/personal", query), all
	default:
		query.Set("kind", protocol.KindAgent)
		tab.Href, tab.Count = pageURL("/personal", query), of[protocol.KindAgent]
	}
	return tab
}

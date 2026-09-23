package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestChannelJourneyNamesModesAndWorkWithoutServiceLanguage(t *testing.T) {
	m := meaningFixture(t)
	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "visitor@h"}, true); err != nil {
		t.Fatal(err)
	}
	m.register(protocol.Record{Name: "jobs@h", Owner: "admin@h", Kind: protocol.KindQueue, Descr: "Jobs", Allow: []string{"*"}})
	m.register(protocol.Record{Name: "news@h", Owner: "admin@h", Kind: protocol.KindPubSub, Descr: "News", Allow: []string{"*"}})
	m.register(protocol.Record{Name: "#worker@h", Owner: "admin@h", Kind: "agent", Descr: "Worker", Allow: []string{"*"}})
	m.register(protocol.Record{Name: "#visitor-box@h", Owner: "visitor@h", Kind: "agent"})
	if _, err := m.bus.Manage("admin@h", core.Management{Name: "news@h", Subs: &[]string{"#visitor-box@h"}}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "jobs@h", Body: "job"}); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 3; i++ {
		if _, err := m.bus.Send(protocol.Envelope{From: "admin@h", To: "news@h", Body: "news"}); err != nil {
			t.Fatal(err)
		}
	}

	page := m.get("/queues")
	topics := m.get("/pubsub")
	for _, want := range []string{
		`<title>Queues · agent-bus</title>`, `<th scope=col>Queue`,
		`<th scope=col>Type`, `<th scope=col class=num>Held`,
		`<th scope=col class=num>Accepted`, `<th scope=col class=num>Dequeued`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("Queues page lacks %q", want)
		}
	}
	for _, want := range []string{
		`<title>PubSub · agent-bus</title>`, `<th scope=col>PubSub`,
		`<th scope=col>Type`, `<th scope=col class=num>Accepted`,
		`<th scope=col class=num>Copies out`, `<th scope=col class=num>Deliver-To`,
	} {
		if !strings.Contains(topics, want) {
			t.Errorf("PubSub page lacks %q", want)
		}
	}
	// A topic holds nothing, so its page has no held column to leave empty.
	if strings.Contains(topics, `<th scope=col class=num>Held`) {
		t.Error("the PubSub page still has a Held column")
	}
	for name, body := range map[string]string{"Queues": page, "PubSub": topics} {
		if strings.Contains(body, `<th scope=col>Service`) {
			t.Errorf("%s page still uses the Service table vocabulary", name)
		}
	}
	agents := m.get("/agents")
	agent := m.row(agents, "#worker@h")
	for _, want := range []string{`data-label=Type>👾 Agent`, `/agent?name=%23worker%40h`} {
		if !strings.Contains(agent, want) {
			t.Errorf("agent row lacks %q: %s", want, agent)
		}
	}
	if strings.Contains(page, ">worker@h<") || strings.Contains(topics, ">worker@h<") {
		t.Error("an agent is still listed among the channels")
	}
	if channel := m.row(page, "jobs@h"); !strings.Contains(channel, `data-label=Type>📮 Queue`) {
		t.Errorf("a registered queue is not named one: %s", channel)
	}
	if services := m.get("/services"); strings.Contains(services, ">worker@h<") {
		t.Error("an agent is still listed among the services")
	}
	if strings.Contains(page, ">news@h<") || strings.Contains(topics, ">jobs@h<") {
		t.Error("a section lists the other channel kind")
	}
	queue := m.row(page, "jobs@h")
	for _, want := range []string{`data-label=Held>2`, `data-label=Accepted>2`, `/queue?name=jobs%40h`} {
		if !strings.Contains(queue, want) {
			t.Errorf("queue row lacks %q: %s", want, queue)
		}
	}
	pubsub := m.row(topics, "news@h")
	for _, want := range []string{`data-label=Type>📣 PubSub`, `data-label=Accepted>3`, `data-label=Deliver-To>1`, `/pubsub/topic?name=news%40h`} {
		if !strings.Contains(pubsub, want) {
			t.Errorf("pub/sub row lacks %q: %s", want, pubsub)
		}
	}
	// An old bookmark naming a kind is kept as the section that kind now has.
	if code, to := m.redirect("/channels?kind=pubsub&readers=none"); code != http.StatusMovedPermanently || to != "/pubsub?readers=none" {
		t.Errorf("the old pub/sub filter went to %d %q", code, to)
	}
	if workOrder := m.get("/pubsub?sort=queued"); !strings.Contains(workOrder, `selected>Accepted (high&ndash;low)</option>`) {
		t.Error("PubSub work sorting does not say it orders by what a topic accepted")
	}
	stateful := m.get("/queues?readers=none&state=active&sort=queued")
	for _, want := range []string{`readers=none&amp;sort=queued&amp;state=inactive`, `return=%2fqueues%3fpage%3d1%26readers%3dnone%26sort%3dqueued%26state%3dactive`, `selected>Queued (high&ndash;low)</option>`} {
		if !strings.Contains(stateful, want) {
			t.Errorf("Queues state lost %q", want)
		}
	}

	owner := m.get("/pubsub/topic?name=news@h&return=%2Fpubsub%3Freaders%3Dnone%26page%3D2")
	if !strings.Contains(owner, `<title>PubSub news@h · agent-bus</title>`) || !strings.Contains(owner, `href="/pubsub?readers=none&amp;page=2"`) || !strings.Contains(owner, `id=settings`) {
		t.Error("owner Channel detail lost its title, exact return or daemon-authorized editor")
	}
	visitor := m.as("visitor@h").get("/pubsub/topic?name=news@h")
	if strings.Contains(visitor, `id=settings`) || !strings.Contains(visitor, "Queue &amp; counters") || !strings.Contains(visitor, "Deliver-To") {
		t.Error("visitor Channel detail either offers the way to the editor or hides readable operational facts")
	}
	// And the form itself refuses, rather than only the link being absent:
	// a page nobody linked to is still a page somebody can type in.
	if _, status := getAs(t, m.as("visitor@h"), "/pubsub/topic/edit?name=news@h"); status != http.StatusForbidden {
		t.Errorf("a visitor reached the settings form directly: %d", status)
	}
	// Both directions: after a deactivation /lookup answers unknown, so the
	// section has to have been read before the change. An empty kind reads
	// as a channel path, so the agent is the case that can tell.
	for _, target := range []struct{ name, detail string }{
		{"jobs@h", "/queue?name=jobs%40h"}, {"#worker@h", "/agent?name=%23worker%40h"},
	} {
		for _, action := range []string{"deactivate", "reactivate"} {
			if location, status := postAs(t, m, "/service", url.Values{"action": {action}, "name": {target.name}}); status != http.StatusSeeOther || location != target.detail {
				t.Fatalf("%s %s returned to %q with %d, want %s", target.name, action, location, status, target.detail)
			}
		}
	}
	danger := m.get("/service-danger?name=jobs@h")
	if !strings.Contains(danger, `href="/queue?name=jobs%40h"`) {
		t.Error("Channel Danger Zone returns through the generic Service detail")
	}
	if location, status := postAs(t, m, "/service", url.Values{"action": {"create"}, "name": {"created@h"}, "kind": {protocol.KindQueue}, "allow": {"*"}}); status != http.StatusSeeOther || location != "/queue?name=created%40h" {
		t.Fatalf("Channel registration returned to %q with %d, want its Channel detail", location, status)
	}
}

func TestRecordListsDistinguishNoCategoryFromNoFilterMatches(t *testing.T) {
	empty := meaningFixture(t)
	for path, wants := range map[string][]string{
		"/queues": {"No queues yet", "A queue holds work", `href=/queues/new>Register a queue</a>`},
		"/pubsub": {"No pub/sub topics yet", "A pub/sub topic copies", `href=/pubsub/new>Register a pub/sub topic</a>`},
	} {
		channels := empty.get(path)
		for _, want := range wants {
			if !strings.Contains(channels, want) {
				t.Errorf("empty %s page lacks %q", path, want)
			}
		}
		if strings.Contains(channels, "No records match these filters") || strings.Contains(channels, `<table class=record-table>`) {
			t.Errorf("empty %s category was rendered as a filtered empty table", path)
		}
	}
	services := empty.get("/services")
	if !strings.Contains(services, "No services yet") || !strings.Contains(services, `href=/services/new>Register a service</a>`) {
		t.Error("empty Services category does not explain itself and offer registration")
	}
	agents := empty.get("/agents")
	if !strings.Contains(agents, "No agents yet") || !strings.Contains(agents, "a name on this bus") || !strings.Contains(agents, `href=/agents/new>Register an agent</a>`) {
		t.Error("empty Agents category does not explain itself and offer registration")
	}

	empty.register(protocol.Record{Name: "jobs@h", Owner: "admin@h", Kind: protocol.KindQueue})
	filtered := empty.get("/queues?q=absent")
	if !strings.Contains(filtered, "No records match these filters") || !strings.Contains(filtered, `href="/queues">clear filters</a>`) || strings.Contains(filtered, "No queues yet") {
		t.Error("zero filter matches were not distinguished from an empty Queues category")
	}
}

func TestRecordDetailDocumentTitleFollowsStatedKind(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Name: "#worker@h", Owner: "admin@h", Kind: protocol.KindAgent})
	m.register(protocol.Record{Name: "api@h", Owner: "admin@h", Kind: protocol.KindService, Addr: "host:1", Proto: "https"})
	m.register(protocol.Record{Name: "jobs@h", Owner: "admin@h", Kind: protocol.KindQueue})
	m.register(protocol.Record{Name: "news@h", Owner: "admin@h", Kind: protocol.KindPubSub})
	for name, want := range map[string]string{
		"#worker@h": "Agent", "api@h": "Service", "jobs@h": "Queue", "news@h": "PubSub",
	} {
		if body := m.get("/service?name=" + url.QueryEscape(name)); !strings.Contains(body, "<title>"+want+" "+name+" · agent-bus</title>") {
			t.Errorf("%s is not titled %q", name, want)
		}
	}
}

// Delivery is no longer a field a form can set: a queue and a pub/sub topic are
// kinds of their own, so what the form says the kind is, is what gets stored —
// and a stray `mode` value left over from the old shape changes nothing.
func TestTheChannelFormStoresTheKindAndNothingBesideIt(t *testing.T) {
	m := meaningFixture(t)
	if location, status := postAs(t, m, "/service", url.Values{
		"action": {"create"}, "name": {"#worker@h"}, "kind": {protocol.KindAgent},
		"mode": {protocol.KindPubSub}, "allow": {"*"},
	}); status != http.StatusSeeOther || location != "/agent?name=%23worker%40h" {
		t.Fatalf("agent registration returned to %q with %d, want its Agent detail", location, status)
	}
	if record, ok := m.bus.Lookup("admin@h", "#worker@h"); !ok || record.Kind != protocol.KindAgent {
		t.Errorf("the agent stored kind %q, want %s", record.Kind, protocol.KindAgent)
	}
	// Falsifiable the other way: the same form does register the kind that
	// copies, when that is the kind it names.
	if _, status := postAs(t, m, "/service", url.Values{
		"action": {"create"}, "name": {"news@h"}, "kind": {protocol.KindPubSub}, "allow": {"*"},
	}); status != http.StatusSeeOther {
		t.Fatalf("channel registration returned %d", status)
	}
	if channel, ok := m.bus.Lookup("admin@h", "news@h"); !ok || channel.Kind != protocol.KindPubSub {
		t.Errorf("a registered channel stored %q, want %s", channel.Kind, protocol.KindPubSub)
	}
}

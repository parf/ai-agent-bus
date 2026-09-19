package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestChannelJourneyNamesModesAndWorkWithoutServiceLanguage(t *testing.T) {
	m := meaningFixture(t)
	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "visitor@h"}, true); err != nil {
		t.Fatal(err)
	}
	m.register(protocol.Record{Name: "jobs@h", Owner: "admin@h", Kind: protocol.KindQueue, Descr: "Jobs", Allow: []string{"*"}})
	m.register(protocol.Record{Name: "news@h", Owner: "admin@h", Kind: protocol.KindPubSub, Descr: "News", Allow: []string{"*"}})
	m.register(protocol.Record{Name: "worker@h", Owner: "admin@h", Kind: "agent", Descr: "Worker", Allow: []string{"*"}})
	if _, err := m.bus.Subscribe("visitor@h", "news@h", true); err != nil {
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

	page := m.get("/channels")
	for _, want := range []string{
		`<title>Channels · agent-bus</title>`, `<th scope=col>Channel`,
		`<th scope=col>Type`, `<th scope=col>Delivery mode`, `<th scope=col class=num>Held`,
		`<th scope=col class=num>Accepted`, `<th scope=col class=num>Dequeued`,
		`<th scope=col class=num>Subscribers`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("Channels page lacks %q", want)
		}
	}
	if strings.Contains(page, `<th scope=col>Service`) {
		t.Error("Channels page still uses the Service table vocabulary")
	}
	agents := m.get("/agents")
	agent := m.row(agents, "worker@h")
	for _, want := range []string{`data-label=Type>👾 Agent`, `/agent?name=worker%40h`} {
		if !strings.Contains(agent, want) {
			t.Errorf("agent row lacks %q: %s", want, agent)
		}
	}
	if strings.Contains(page, ">worker@h<") {
		t.Error("an agent is still listed among the channels")
	}
	if channel := m.row(page, "jobs@h"); !strings.Contains(channel, `data-label=Type>📮 Queue`) {
		t.Errorf("a registered channel is not named one: %s", channel)
	}
	if services := m.get("/services"); strings.Contains(services, ">worker@h<") {
		t.Error("an agent is still listed among the services")
	}
	queue := m.row(page, "jobs@h")
	for _, want := range []string{"Queue · one at a time", `data-label=Held>2`, `data-label=Accepted>2`, `data-label=Subscribers><span class=muted>&mdash;</span>`} {
		if !strings.Contains(queue, want) {
			t.Errorf("queue channel row lacks %q: %s", want, queue)
		}
	}
	pubsub := m.row(page, "news@h")
	for _, want := range []string{"Pub/sub · copy to each", `data-label=Held><span class=muted>&mdash;</span>`, `data-label=Accepted>3`, `data-label=Subscribers>1`, `/channel?name=news%40h`} {
		if !strings.Contains(pubsub, want) {
			t.Errorf("pub/sub channel row lacks %q: %s", want, pubsub)
		}
	}
	// The page lists two kinds, so it offers the control that narrows to one.
	// Delivery is not a second filter beside it: the kind IS the delivery.
	queueOnly := m.get("/channels?kind=queue")
	if !strings.Contains(queueOnly, `aria-label="Kind filter"`) || !strings.Contains(queueOnly, `aria-current=true>📮 Queue</a>`) ||
		!strings.Contains(queueOnly, ">jobs@h<") || strings.Contains(queueOnly, ">news@h<") {
		t.Error("the Kind filter is missing or did not isolate the queues")
	}
	if pubsubOnly := m.get("/channels?kind=pubsub"); strings.Contains(pubsubOnly, ">jobs@h<") || !strings.Contains(pubsubOnly, ">news@h<") {
		t.Error("the pub/sub filter admitted a queue or lost its topic")
	}
	workOrder := m.get("/channels?sort=queued")
	if strings.Index(workOrder, ">news@h<") > strings.Index(workOrder, ">jobs@h<") || !strings.Contains(workOrder, ">Work (high&ndash;low)</option>") {
		t.Error("Channel work sorting did not compare pub/sub accepted with queue held work")
	}
	statefulKind := m.get("/channels?kind=pubsub&readers=none&state=active&sort=queued")
	for _, want := range []string{`name=kind value="pubsub"`, `kind=pubsub&amp;readers=none&amp;sort=queued&amp;state=inactive`, `return=%2fchannels%3fkind%3dpubsub%26page%3d1%26readers%3dnone%26sort%3dqueued%26state%3dactive`, `selected>Work (high&ndash;low)</option>`} {
		if !strings.Contains(statefulKind, want) {
			t.Errorf("Channel kind state lost %q", want)
		}
	}

	owner := m.get("/channel?name=news@h&return=%2Fchannels%3Freaders%3Dnone%26page%3D2")
	if !strings.Contains(owner, `<title>PubSub news@h · agent-bus</title>`) || !strings.Contains(owner, `href="/channels?readers=none&amp;page=2"`) || !strings.Contains(owner, `id=settings`) {
		t.Error("owner Channel detail lost its title, exact return or daemon-authorized editor")
	}
	visitor := m.as("visitor@h").get("/channel?name=news@h")
	if strings.Contains(visitor, `id=settings`) || !strings.Contains(visitor, "Queue &amp; counters") || !strings.Contains(visitor, "Subscriptions") {
		t.Error("visitor Channel detail either exposes an editor or hides readable operational facts")
	}
	if location, status := postAs(t, m, "/service", url.Values{"action": {"disable"}, "name": {"jobs@h"}}); status != http.StatusSeeOther || location != "/channel?name=jobs%40h" {
		t.Fatalf("Channel mutation returned to %q with %d, want its Channel detail", location, status)
	}
	danger := m.get("/service-danger?name=jobs@h")
	if !strings.Contains(danger, `href="/channel?name=jobs%40h"`) {
		t.Error("Channel Danger Zone returns through the generic Service detail")
	}
	if location, status := postAs(t, m, "/service", url.Values{"action": {"create"}, "name": {"created@h"}, "kind": {protocol.KindQueue}, "allow": {"*"}}); status != http.StatusSeeOther || location != "/channel?name=created%40h" {
		t.Fatalf("Channel registration returned to %q with %d, want its Channel detail", location, status)
	}
}

func TestRecordListsDistinguishNoCategoryFromNoFilterMatches(t *testing.T) {
	empty := meaningFixture(t)
	channels := empty.get("/channels")
	for _, want := range []string{"No channels yet", "A channel routes messages", `href=/channels/new>Register a channel</a>`} {
		if !strings.Contains(channels, want) {
			t.Errorf("empty Channels page lacks %q", want)
		}
	}
	if strings.Contains(channels, "No records match these filters") || strings.Contains(channels, `<table class=record-table>`) {
		t.Error("empty Channels category was rendered as a filtered empty table")
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
	filtered := empty.get("/channels?q=absent")
	if !strings.Contains(filtered, "No records match these filters") || !strings.Contains(filtered, `href="/channels">clear filters</a>`) || strings.Contains(filtered, "No channels yet") {
		t.Error("zero filter matches were not distinguished from an empty Channels category")
	}
}

func TestRecordDetailDocumentTitleFollowsStatedKind(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Name: "worker@h", Owner: "admin@h", Kind: protocol.KindAgent})
	m.register(protocol.Record{Name: "api@h", Owner: "admin@h", Kind: protocol.KindService, Addr: "host:1", Proto: "https"})
	m.register(protocol.Record{Name: "jobs@h", Owner: "admin@h", Kind: protocol.KindQueue})
	m.register(protocol.Record{Name: "news@h", Owner: "admin@h", Kind: protocol.KindPubSub})
	for name, want := range map[string]string{
		"worker@h": "Agent", "api@h": "Service", "jobs@h": "Queue", "news@h": "PubSub",
	} {
		if body := m.get("/service?name=" + name); !strings.Contains(body, "<title>"+want+" "+name+" · agent-bus</title>") {
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
		"action": {"create"}, "name": {"worker@h"}, "kind": {protocol.KindAgent},
		"mode": {protocol.KindPubSub}, "allow": {"*"},
	}); status != http.StatusSeeOther || location != "/agent?name=worker%40h" {
		t.Fatalf("agent registration returned to %q with %d, want its Agent detail", location, status)
	}
	if record, ok := m.bus.Lookup("admin@h", "worker@h"); !ok || record.Kind != protocol.KindAgent {
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

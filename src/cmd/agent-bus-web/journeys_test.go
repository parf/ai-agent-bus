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
	m.register(protocol.Record{Name: "jobs@h", Owner: "admin@h", Kind: protocol.KindTopic, Mode: protocol.ModeQueue, Descr: "Jobs", Allow: []string{"*"}})
	m.register(protocol.Record{Name: "news@h", Owner: "admin@h", Kind: protocol.KindTopic, Mode: protocol.ModePubSub, Descr: "News", Allow: []string{"*"}})
	// An agent's inbox is an implicit queue topic, so it is listed here and
	// not with the services. It stores no delivery mode of its own.
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
	inbox := m.row(page, "worker@h")
	for _, want := range []string{`data-label=Type>👾 Agent`, "Queue · one at a time", `/channel?name=worker%40h`} {
		if !strings.Contains(inbox, want) {
			t.Errorf("inbox row lacks %q: %s", want, inbox)
		}
	}
	if channel := m.row(page, "jobs@h"); !strings.Contains(channel, `data-label=Type>Channel`) {
		t.Errorf("a registered channel is not named one: %s", channel)
	}
	if services := m.get("/services"); strings.Contains(services, ">worker@h<") {
		t.Error("an inbox is still listed among the services")
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
	inboxOnly := m.get("/channels?kind=agent")
	if !strings.Contains(inboxOnly, `aria-label="Kind filter"`) || !strings.Contains(inboxOnly, `aria-current=true>👾 Agent</a>`) ||
		!strings.Contains(inboxOnly, ">worker@h<") || strings.Contains(inboxOnly, ">jobs@h<") || strings.Contains(inboxOnly, ">news@h<") {
		t.Error("the Inbox kind filter is missing or did not isolate the inboxes")
	}
	channelsOnly := m.get("/channels?kind=topic")
	if !strings.Contains(channelsOnly, ">jobs@h<") || !strings.Contains(channelsOnly, ">news@h<") || strings.Contains(channelsOnly, ">worker@h<") {
		t.Error("the Channel kind filter did not isolate the registered channels")
	}
	queueOnly := m.get("/channels?mode=queue")
	if !strings.Contains(queueOnly, `aria-label="Delivery mode filter"`) || !strings.Contains(queueOnly, `aria-current=true>Queue</a>`) || !strings.Contains(queueOnly, ">jobs@h<") || strings.Contains(queueOnly, ">news@h<") {
		t.Error("plain-valued Delivery mode filter does not isolate queue channels")
	}
	// An inbox stores no mode. It is a queue, so the queue filter keeps it and
	// the pub/sub filter does not.
	if !strings.Contains(queueOnly, ">worker@h<") {
		t.Error("an inbox was dropped by the queue filter because it stores no mode")
	}
	if pubsubOnly := m.get("/channels?mode=pubsub"); strings.Contains(pubsubOnly, ">worker@h<") || !strings.Contains(pubsubOnly, ">news@h<") {
		t.Error("the pub/sub filter admitted an inbox or lost its topic")
	}
	workOrder := m.get("/channels?sort=queued")
	if strings.Index(workOrder, ">news@h<") > strings.Index(workOrder, ">jobs@h<") || !strings.Contains(workOrder, ">Work (high&ndash;low)</option>") {
		t.Error("Channel work sorting did not compare pub/sub accepted with queue held work")
	}
	statefulMode := m.get("/channels?mode=pubsub&readers=none&state=active&sort=queued")
	for _, want := range []string{`name=mode value="pubsub"`, `mode=pubsub&amp;readers=none&amp;sort=queued&amp;state=inactive`, `return=%2fchannels%3fmode%3dpubsub%26page%3d1%26readers%3dnone%26sort%3dqueued%26state%3dactive`, `selected>Work (high&ndash;low)</option>`} {
		if !strings.Contains(statefulMode, want) {
			t.Errorf("Channel mode state lost %q", want)
		}
	}

	owner := m.get("/channel?name=news@h&return=%2Fchannels%3Freaders%3Dnone%26page%3D2")
	if !strings.Contains(owner, `<title>Channel news@h · agent-bus</title>`) || !strings.Contains(owner, `href="/channels?readers=none&amp;page=2"`) || !strings.Contains(owner, `id=settings`) {
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
	if location, status := postAs(t, m, "/service", url.Values{"action": {"create"}, "name": {"created@h"}, "kind": {protocol.KindTopic}, "mode": {protocol.ModeQueue}, "allow": {"*"}}); status != http.StatusSeeOther || location != "/channel?name=created%40h" {
		t.Fatalf("Channel registration returned to %q with %d, want its Channel detail", location, status)
	}
}

func TestRecordListsDistinguishNoCategoryFromNoFilterMatches(t *testing.T) {
	empty := meaningFixture(t)
	channels := empty.get("/channels")
	for _, want := range []string{"No channels or inboxes yet", "A channel routes messages", `href=/channels/new>Register a channel or inbox</a>`} {
		if !strings.Contains(channels, want) {
			t.Errorf("empty Channels page lacks %q", want)
		}
	}
	if strings.Contains(channels, "No records match these filters") || strings.Contains(channels, `<table class=record-table>`) {
		t.Error("empty Channels category was rendered as a filtered empty table")
	}
	services := empty.get("/services")
	if !strings.Contains(services, "No services yet") || !strings.Contains(services, "registered identity with an inbox") || !strings.Contains(services, `href=/services/new>Register a service</a>`) {
		t.Error("empty Services category does not explain itself and offer registration")
	}

	empty.register(protocol.Record{Name: "jobs@h", Owner: "admin@h", Kind: protocol.KindTopic, Mode: protocol.ModeQueue})
	filtered := empty.get("/channels?q=absent")
	if !strings.Contains(filtered, "No records match these filters") || !strings.Contains(filtered, `href="/channels">clear filters</a>`) || strings.Contains(filtered, "No channels yet") {
		t.Error("zero filter matches were not distinguished from an empty Channels category")
	}
}

func TestRecordDetailDocumentTitleFollowsStatedKind(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Name: "worker@h", Owner: "admin@h", Kind: "agent"})
	m.register(protocol.Record{Name: "api@h", Owner: "admin@h", Kind: "generic"})
	if body := m.get("/service?name=worker@h"); !strings.Contains(body, `<title>Inbox worker@h · agent-bus</title>`) {
		t.Error("Inbox detail retained the generic Service document title")
	}
	if body := m.get("/service?name=api@h"); !strings.Contains(body, `<title>Service api@h · agent-bus</title>`) {
		t.Error("Service detail lost its document title")
	}
}

// An inbox is the queue one agent reads, so it has no delivery of its own to
// choose. The channel form's delivery radio is submitted with every
// registration, and it must not travel with an inbox.
func TestRegisteringAnInboxKeepsTheChannelDeliveryChoiceOffIt(t *testing.T) {
	m := meaningFixture(t)
	if location, status := postAs(t, m, "/service", url.Values{
		"action": {"create"}, "name": {"worker@h"}, "kind": {"agent"},
		"mode": {protocol.ModePubSub}, "allow": {"*"},
	}); status != http.StatusSeeOther || location != "/channel?name=worker%40h" {
		t.Fatalf("inbox registration returned to %q with %d, want its Channel detail", location, status)
	}
	record, ok := m.bus.Lookup("admin@h", "worker@h")
	if !ok {
		t.Fatal("the inbox was not registered")
	}
	if record.Mode != "" {
		t.Errorf("the inbox stored a delivery mode %q from the channel form", record.Mode)
	}
	// The control itself is falsifiable: the same form does carry the choice
	// for the kind that has one.
	if _, status := postAs(t, m, "/service", url.Values{
		"action": {"create"}, "name": {"news@h"}, "kind": {protocol.KindTopic},
		"mode": {protocol.ModePubSub}, "allow": {"*"},
	}); status != http.StatusSeeOther {
		t.Fatalf("channel registration returned %d", status)
	}
	if channel, ok := m.bus.Lookup("admin@h", "news@h"); !ok || channel.Mode != protocol.ModePubSub {
		t.Errorf("a registered channel lost its delivery mode: %+v %v", channel, ok)
	}
}

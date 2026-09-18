package main

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestDangerZoneUsesFreshConfirmationAndResourceReturnPaths(t *testing.T) {
	p := personalWebFixture(t)
	for _, record := range []protocol.Record{
		{Name: "svc@h", Owner: "alice@h", Allow: []string{"alice@h"}},
		{Name: "jobs@h", Owner: "alice@h", Kind: protocol.KindTopic, Mode: "queue", Allow: []string{"alice@h"}},
		{Name: "busy@h", Owner: "alice@h", Allow: []string{"alice@h"}},
		{Name: "stale@h", Owner: "alice@h", Allow: []string{"alice@h", "bob@h"}},
		{Name: "view@h", Owner: "alice@h", Allow: []string{"bob@h"}},
		{Name: "queue-change@h", Owner: "alice@h", Allow: []string{"alice@h"}},
		{Name: "reader-change@h", Owner: "alice@h", Allow: []string{"alice@h"}},
		{Name: "handoff@h", Owner: "alice@h"},
		{Name: "handoff-channel@h", Owner: "alice@h", Kind: protocol.KindTopic, Mode: "queue"},
	} {
		if _, err := p.bus.Register(record); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := p.bus.Send(protocol.Envelope{From: "alice@h", To: "busy@h", Body: "held"}); err != nil {
		t.Fatal(err)
	}
	visible, _ := p.request("bob@h", "GET", "/service?name=view%40h", nil, http.StatusOK)
	if !strings.Contains(visible, "view@h") || strings.Contains(visible, "Danger Zone") || strings.Contains(visible, "Replace configuration") {
		t.Fatal("visible non-manager detail exposed a management entry")
	}
	refused, _ := p.request("bob@h", "GET", "/service-danger?name=view%40h", nil, http.StatusForbidden)
	if !strings.Contains(refused, "Not yours to see") || strings.Contains(refused, "Replace configuration") {
		t.Fatal("visible non-manager opened the Danger Zone")
	}

	for _, name := range []string{"svc@h", "jobs@h"} {
		detail, _ := p.request("alice@h", "GET", "/service?name="+url.QueryEscape(name), nil, http.StatusOK)
		if !strings.Contains(detail, ">Danger Zone</a>") {
			t.Fatalf("%s detail has no Danger Zone link", name)
		}
		for _, dangerous := range []string{"Replace configuration", "Transfer ownership", "Remove registration"} {
			if strings.Contains(detail, dangerous) {
				t.Fatalf("%s ordinary detail exposed %q", name, dangerous)
			}
		}
		danger, _ := p.request("alice@h", "GET", "/service-danger?name="+url.QueryEscape(name), nil, http.StatusOK)
		for _, heading := range []string{"<h2>Replace configuration</h2>", "<h2>Transfer ownership</h2>", "<h2>Remove registration</h2>"} {
			if !strings.Contains(danger, heading) {
				t.Fatalf("%s Danger Zone omitted %q", name, heading)
			}
		}
		detailPath := "/service"
		if name == "jobs@h" {
			detailPath = "/channel"
		}
		if !strings.Contains(danger, detailPath+`?name=`+url.QueryEscape(name)) {
			t.Fatalf("%s Danger Zone lost its return state", name)
		}
		if strings.Count(danger, `action=/service-confirm`) != 2 || strings.Count(danger, `action=/service>`) != 1 {
			t.Fatalf("%s Danger Zone does not route exactly transfer/removal through confirmation", name)
		}
	}

	confirm, _ := p.request("alice@h", "POST", "/service-confirm", url.Values{
		"action": {"transfer"}, "name": {"svc@h"}, "owner": {`new<&>@h`},
	}, http.StatusOK)
	if !strings.Contains(confirm, "Confirm ownership transfer") || !strings.Contains(confirm, "new&lt;&amp;&gt;@h") || strings.Contains(confirm, `new<&>@h`) {
		t.Fatal("transfer confirmation was absent or failed to escape the proposed owner")
	}
	if got, _ := p.bus.Lookup("alice@h", "svc@h"); got.Owner != "alice@h" {
		t.Fatal("confirmation mutated ownership")
	}

	removeConfirm, _ := p.request("alice@h", "POST", "/service-confirm", url.Values{
		"action": {"delete"}, "name": {"jobs@h"},
	}, http.StatusOK)
	if !strings.Contains(removeConfirm, "Confirm removal") || !strings.Contains(removeConfirm, `name=expected_queued value="0"`) || !strings.Contains(removeConfirm, `name=expected_readers value="0"`) {
		t.Fatal("removal confirmation did not carry freshly observed queue and reader facts")
	}
	if _, ok := p.bus.Lookup("alice@h", "jobs@h"); !ok {
		t.Fatal("confirmation removed the channel")
	}

	_, header := p.request("alice@h", "POST", "/service", url.Values{
		"action": {"configure"}, "name": {"jobs@h"}, "config": {`{"changed":true}`},
	}, http.StatusSeeOther)
	if header.Get("Location") != "/channel?name=jobs%40h" {
		t.Fatalf("channel configuration returned to %q", header.Get("Location"))
	}
	_, header = p.request("alice@h", "POST", "/service", url.Values{
		"action": {"save"}, "name": {"jobs@h"}, "bound": {"0"}, "overflow": {"strict"},
	}, http.StatusSeeOther)
	if header.Get("Location") != "/channel?name=jobs%40h" {
		t.Fatalf("routine channel edit returned to %q", header.Get("Location"))
	}
	_, header = p.request("alice@h", "POST", "/service", url.Values{
		"action": {"delete"}, "name": {"jobs@h"}, "confirmed": {"1"},
		"expected_owner": {"alice@h"}, "expected_queued": {"0"}, "expected_readers": {"0"},
	}, http.StatusSeeOther)
	if header.Get("Location") != "/channels" {
		t.Fatalf("removed channel returned to %q", header.Get("Location"))
	}

	busy, _ := p.request("alice@h", "POST", "/service", url.Values{
		"action": {"delete"}, "name": {"busy@h"}, "confirmed": {"1"},
		"expected_owner": {"alice@h"}, "expected_queued": {"1"}, "expected_readers": {"0"},
	}, http.StatusConflict)
	if !strings.Contains(busy, "That was refused") || strings.Contains(busy, "The conditions changed") {
		t.Fatal("an unchanged busy record was mislabeled as a stale confirmation")
	}
	if _, ok := p.bus.Lookup("alice@h", "busy@h"); !ok {
		t.Fatal("refused removal deleted the busy record")
	}

	if body, _ := p.request("alice@h", "POST", "/service-confirm", url.Values{"action": {"delete"}, "name": {"queue-change@h"}}, http.StatusOK); !strings.Contains(body, `name=expected_queued value="0"`) {
		t.Fatal("queue-change confirmation did not observe the empty queue")
	}
	if _, err := p.bus.Send(protocol.Envelope{From: "alice@h", To: "queue-change@h", Body: "arrived after confirmation"}); err != nil {
		t.Fatal(err)
	}
	queueChanged, _ := p.request("alice@h", "POST", "/service", url.Values{
		"action": {"delete"}, "name": {"queue-change@h"}, "confirmed": {"1"},
		"expected_owner": {"alice@h"}, "expected_queued": {"0"}, "expected_readers": {"0"},
	}, http.StatusConflict)
	if !strings.Contains(queueChanged, "The conditions changed") {
		t.Fatal("a queue change after confirmation was not detected")
	}

	if body, _ := p.request("alice@h", "POST", "/service-confirm", url.Values{"action": {"delete"}, "name": {"reader-change@h"}}, http.StatusOK); !strings.Contains(body, `name=expected_readers value="0"`) {
		t.Fatal("reader-change confirmation did not observe zero readers")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = p.bus.ConsumeAs(ctx, "reader-change@h", "reader-change@h", "", "", false, false)
	}()
	for deadline := time.Now().Add(time.Second); ; {
		record, _ := p.bus.Lookup("alice@h", "reader-change@h")
		if record.Readers != nil && *record.Readers == 1 {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			<-done
			t.Fatal("reader did not attach")
		}
		time.Sleep(time.Millisecond)
	}
	readerChanged, _ := p.request("alice@h", "POST", "/service", url.Values{
		"action": {"delete"}, "name": {"reader-change@h"}, "confirmed": {"1"},
		"expected_owner": {"alice@h"}, "expected_queued": {"0"}, "expected_readers": {"0"},
	}, http.StatusConflict)
	cancel()
	<-done
	if !strings.Contains(readerChanged, "The conditions changed") {
		t.Fatal("a reader change after confirmation was not detected")
	}

	staleConfirm, _ := p.request("admin@h", "POST", "/service-confirm", url.Values{
		"action": {"transfer"}, "name": {"stale@h"}, "owner": {"bob@h"},
	}, http.StatusOK)
	if !strings.Contains(staleConfirm, `name=expected_owner value="alice@h"`) {
		t.Fatal("transfer confirmation did not pin the freshly read owner")
	}
	newOwner := "bob@h"
	if _, err := p.bus.Manage("alice@h", core.Management{Name: "stale@h", Owner: &newOwner}); err != nil {
		t.Fatal(err)
	}
	changed, _ := p.request("admin@h", "POST", "/service", url.Values{
		"action": {"transfer"}, "name": {"stale@h"}, "owner": {"alice@h"},
		"confirmed": {"1"}, "expected_owner": {"alice@h"},
	}, http.StatusConflict)
	if !strings.Contains(changed, "The conditions changed") || !strings.Contains(changed, "Review the current Danger Zone") {
		t.Fatal("a stale confirmation did not render the conditions-changed recovery")
	}
	if got, _ := p.bus.Lookup("bob@h", "stale@h"); got.Owner != "bob@h" {
		t.Fatal("stale confirmation overwrote the newer owner")
	}

	for _, handoff := range []struct {
		name, want string
	}{
		{"handoff@h", "/services"},
		{"handoff-channel@h", "/channels"},
	} {
		_, header = p.request("alice@h", "POST", "/service", url.Values{
			"action": {"transfer"}, "name": {handoff.name}, "owner": {"bob@h"},
			"confirmed": {"1"}, "expected_owner": {"alice@h"},
		}, http.StatusSeeOther)
		if header.Get("Location") != handoff.want {
			t.Fatalf("transfer that removed visibility for %s returned to %q", handoff.name, header.Get("Location"))
		}
		if got, _ := p.bus.Lookup("bob@h", handoff.name); got.Owner != "bob@h" {
			t.Fatalf("transfer did not change %s owner", handoff.name)
		}
	}

	_, header = p.request("alice@h", "POST", "/service", url.Values{
		"action": {"delete"}, "name": {"svc@h"}, "confirmed": {"1"},
		"expected_owner": {"alice@h"}, "expected_queued": {"0"}, "expected_readers": {"0"},
	}, http.StatusSeeOther)
	if header.Get("Location") != "/services" {
		t.Fatalf("removed service returned to %q", header.Get("Location"))
	}
}

func TestTransferConfirmationRechecksApplicability(t *testing.T) {
	p := personalWebFixture(t)
	if _, err := p.bus.Register(protocol.Record{Name: "svc@h", Owner: "alice@h", Allow: []string{"alice@h"}}); err != nil {
		t.Fatal(err)
	}
	owner := "bob@h"
	if _, err := p.bus.Manage("alice@h", core.Management{Name: "svc@h", Owner: &owner}); err != nil {
		t.Fatal(err)
	}
	body, _ := p.request("alice@h", "POST", "/service-confirm", url.Values{
		"action": {"transfer"}, "name": {"svc@h"}, "owner": {"alice@h"},
	}, http.StatusForbidden)
	if strings.Contains(body, "Confirm ownership transfer") || !strings.Contains(body, "Not yours to see") {
		t.Fatal("confirmation rendered an inapplicable transfer after reloading authority")
	}
}

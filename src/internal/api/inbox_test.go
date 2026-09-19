package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestInboxSelectionIsIndependentOfMessageFilters(t *testing.T) {
	c := refusalFixture(t)
	for _, record := range []protocol.Record{
		{Kind: protocol.KindAgent, Name: "reader@h", Owner: "admin@h", Allow: []string{"admin@h"}},
		{Name: "jobs@h", Owner: "admin@h", Kind: protocol.KindQueue, Allow: []string{"reader@h", "admin@h"}},
		{Name: "filter@h", Owner: "admin@h", Kind: protocol.KindQueue, Allow: []string{"reader@h", "admin@h"}},
		{Kind: protocol.KindAgent, Name: "secret@h", Owner: "admin@h", Allow: []string{"admin@h"}},
	} {
		if _, err := c.bus.Register(record); err != nil {
			t.Fatal(err)
		}
	}
	reader := c.as("reader@h")
	get := func(path string, want int, text string) {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set(HeaderToken, reader)
		w := httptest.NewRecorder()
		c.srv.ServeHTTP(w, r)
		if w.Code != want || text != "" && !strings.Contains(w.Body.String(), text) {
			t.Fatalf("%s: %d %s, want %d containing %q", path, w.Code, w.Body.String(), want, text)
		}
	}
	send := func(to, body, topic, tag string) {
		t.Helper()
		if _, err := c.bus.Send(protocol.Envelope{From: "admin@h", To: to, Body: body, Topic: topic, Tag: tag}); err != nil {
			t.Fatal(err)
		}
	}

	send("reader@h", "default own inbox", "ordinary", "")
	get("/consume?wait=0s", http.StatusOK, "default own inbox")
	get("/consume?inbox=&wait=0s", http.StatusBadRequest, "")

	// The filter is deliberately a registered channel's full name. It still
	// filters the caller's inbox and never selects that channel's queue.
	send("reader@h", "distractor in own inbox", "other", "")
	send("reader@h", "filtered from own inbox", "filter@h", "")
	send("filter@h", "held in the channel inbox", "different", "")
	get("/consume?topic=filter@h&wait=0s", http.StatusOK, "filtered from own inbox")
	get("/consume?inbox=filter@h&wait=0s", http.StatusOK, "held in the channel inbox")

	send("jobs@h", "unfiltered explicit inbox", "work", "one")
	get("/consume?inbox=jobs@h&wait=0s", http.StatusOK, "unfiltered explicit inbox")
	send("jobs@h", "distractor in selected inbox", "other", "result")
	send("jobs@h", "filtered explicit inbox", "MyTopic", "result")
	get("/consume?inbox=jobs@h&topic=MyTopic&tag=result&wait=0s", http.StatusOK, "filtered explicit inbox")

	// An address-shaped filter that matches nothing is an empty wait, not the
	// retired overload's "no such topic" diagnostic.
	get("/consume?topic=missing@h&wait=1ms", http.StatusNoContent, "")

	c.bus.SampleActivity(time.Now().Add(-time.Second))
	get("/consume?inbox=secret@h&wait=0s", http.StatusForbidden, "")
	secret, err := c.bus.Activity("admin@h", "secret@h")
	if err != nil || len(secret) != 1 || secret[0].Refused != 1 {
		t.Fatalf("selected inbox did not receive its refusal: %+v, %v", secret, err)
	}
	readerActivity, err := c.bus.Activity("admin@h", "reader@h")
	if err != nil || len(readerActivity) != 1 || readerActivity[0].Refused != 0 {
		t.Fatalf("caller inbox was charged for target refusal: %+v, %v", readerActivity, err)
	}
	get("/consume?inbox=missing@h&wait=0s", http.StatusNotFound, "")
}

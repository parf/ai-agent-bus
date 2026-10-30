package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// `asks=only` on consume takes a queued ask and leaves ordinary work in the
// queue, in order; any other value is a 400 that names `asks`; an unfiltered
// read, with or without it, still gets the work.
func TestConsumeAsksOnly(t *testing.T) {
	c := refusalFixture(t)
	if _, err := c.bus.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "admin@h", Allow: []string{"admin@h"}}); err != nil {
		t.Fatal(err)
	}
	token := c.as("#svc@h")
	get := func(path string) (int, string) {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set(HeaderToken, token)
		w := httptest.NewRecorder()
		c.srv.ServeHTTP(w, r)
		return w.Code, w.Body.String()
	}
	for _, bad := range []string{"", "skip", "ONLY", "none"} {
		if code, body := get("/consume?wait=0s&asks=" + bad); code != http.StatusBadRequest || !strings.Contains(body, "asks") {
			t.Fatalf("asks=%q: %d %s, want a 400 naming asks", bad, code, body)
		}
	}
	for _, e := range []protocol.Envelope{{Body: "work one"}, {Ask: "help"}, {Body: "work two"}} {
		e.From, e.To = "admin@h", "#svc@h"
		if _, err := c.bus.Send(e); err != nil {
			t.Fatal(err)
		}
	}
	code, body := get("/consume?wait=0s&asks=only")
	var got protocol.Envelope
	if code != http.StatusOK || json.Unmarshal([]byte(body), &got) != nil || got.Ask != "help" {
		t.Fatalf("asks=only: %d %s, want the help ask", code, body)
	}
	if code, _ := get("/consume?wait=0s&asks=only"); code != http.StatusNoContent {
		t.Fatalf("with only work left an asks-only read answered %d, want 204", code)
	}
	for _, want := range []string{"work one", "work two"} {
		code, body := get("/consume?wait=0s")
		if code != http.StatusOK || json.Unmarshal([]byte(body), &got) != nil || got.Body != want {
			t.Fatalf("ordinary read: %d %s, want %q in order", code, body, want)
		}
	}
}
